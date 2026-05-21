package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service/retriever"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/tracing/langfuse"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	maxImageSizeBytes  = 20 * 1024 * 1024 // 20 MB
	maxBatchImageCount = 20
)

// allowedImageTypes defines the whitelist of image formats for image-type knowledge bases.
var allowedImageTypes = map[string]bool{
	"jpg":  true,
	"jpeg": true,
	"png":  true,
	"gif":  true,
	"webp": true,
	"bmp":  true,
}

// IsAllowedImageType checks whether the given file extension is in the image format whitelist.
func IsAllowedImageType(fileType string) bool {
	if fileType == "" {
		return false
	}
	return allowedImageTypes[strings.ToLower(strings.TrimSpace(fileType))]
}

// imageHeaderMagic maps file extensions to their expected header magic bytes.
var imageHeaderMagic = map[string][]byte{
	"jpg":  {0xFF, 0xD8, 0xFF},
	"jpeg": {0xFF, 0xD8, 0xFF},
	"png":  {0x89, 0x50, 0x4E, 0x47},
	"gif":  {0x47, 0x49, 0x46},
	"webp": {0x52, 0x49, 0x46, 0x46},
	"bmp":  {0x42, 0x4D},
}

// isValidImageHeader checks the first bytes of data against the expected magic bytes.
func isValidImageHeader(data []byte, fileType string) bool {
	magic, ok := imageHeaderMagic[strings.ToLower(strings.TrimSpace(fileType))]
	if !ok {
		return true // unknown type, skip magic check
	}
	if len(data) < len(magic) {
		return false
	}
	for i, b := range magic {
		if data[i] != b {
			return false
		}
	}
	return true
}

// readImageBytesAndHash reads the image file content and returns bytes, MD5 hash, and file type.
func readImageBytesAndHash(file *multipart.FileHeader) ([]byte, string, string, error) {
	fileType := strings.ToLower(strings.TrimSpace(filepath.Ext(file.Filename)))
	if fileType != "" && fileType[0] == '.' {
		fileType = fileType[1:]
	}

	src, err := file.Open()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to open image file: %w", err)
	}
	defer src.Close()

	data, err := io.ReadAll(io.LimitReader(src, maxImageSizeBytes+1))
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to read image file: %w", err)
	}

	if len(data) > maxImageSizeBytes {
		return nil, "", "", fmt.Errorf("image file size exceeds limit of 20MB")
	}

	hash := md5.Sum(data)
	md5Hash := hex.EncodeToString(hash[:])

	return data, md5Hash, fileType, nil
}

// CreateKnowledgeFromImage creates a knowledge entry from an uploaded image for image-type KBs.
func (s *knowledgeService) CreateKnowledgeFromImage(ctx context.Context,
	kbID string, file *multipart.FileHeader, tagID string, channel string,
) (*types.Knowledge, error) {
	logger.Infof(ctx, "Start creating knowledge from image: KB=%s, file=%s, size=%d", kbID, file.Filename, file.Size)

	channel = defaultChannel(channel)

	// 1. Read image bytes, compute hash
	_, md5Hash, fileType, err := readImageBytesAndHash(file)
	if err != nil {
		logger.Errorf(ctx, "Failed to read image file: %v", err)
		return nil, err
	}
	logger.Infof(ctx, "Image read: type=%s, hash=%s", fileType, md5Hash)

	// 2. Validate image format
	if !IsAllowedImageType(fileType) {
		logger.Errorf(ctx, "Unsupported image type: %s", fileType)
		return nil, fmt.Errorf("不支持的文件类型，仅支持 jpg, png, gif, webp, bmp")
	}

	// 3. Get KB
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get knowledge base: %v", err)
		return nil, err
	}

	// 4. Validate KB type
	if kb.Type != types.KnowledgeBaseTypeImage {
		logger.Errorf(ctx, "KB is not image type: %s", kb.Type)
		return nil, fmt.Errorf("该知识库不是图片库类型")
	}

	// 5. Check storage engine
	if err := checkStorageEngineConfigured(ctx, kb); err != nil {
		return nil, err
	}

	// 6. Dedup check
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)
	exists, existingKnowledge, err := s.repo.CheckKnowledgeExists(ctx, tenantID, kbID, &types.KnowledgeCheckParams{
		Type:     types.KnowledgeTypeImage,
		FileHash: md5Hash,
		FileName: file.Filename,
		FileSize: file.Size,
	})
	if err != nil {
		logger.Errorf(ctx, "Failed to check knowledge existence: %v", err)
		return nil, err
	}
	if exists {
		logger.Infof(ctx, "Duplicate image: %s (hash=%s)", file.Filename, md5Hash)
		return existingKnowledge, types.NewDuplicateFileError(existingKnowledge)
	}

	// 7. Check storage quota
	tenantInfo := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
	if tenantInfo.StorageQuota > 0 && tenantInfo.StorageUsed >= tenantInfo.StorageQuota {
		logger.Error(ctx, "Storage quota exceeded")
		return nil, types.NewStorageQuotaExceededError()
	}

	// 8. Save image to storage
	filePath, err := s.resolveFileService(ctx, kb).SaveFile(ctx, file, tenantID, file.Filename)
	if err != nil {
		logger.Errorf(ctx, "Failed to save image file: %v", err)
		return nil, fmt.Errorf("failed to save image file: %w", err)
	}

	// 9. Create knowledge record
	knowledge := &types.Knowledge{
		TenantID:         tenantID,
		KnowledgeBaseID:  kbID,
		TagID:            tagID,
		Type:             types.KnowledgeTypeImage,
		Channel:          channel,
		Title:            file.Filename,
		FileName:         file.Filename,
		FileType:         fileType,
		FileSize:         file.Size,
		FileHash:         md5Hash,
		FilePath:         filePath,
		ParseStatus:      types.ParseStatusPending,
		EnableStatus:     "disabled",
		EmbeddingModelID: kb.EmbeddingModelID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := s.repo.CreateKnowledge(ctx, knowledge); err != nil {
		logger.Errorf(ctx, "Failed to create knowledge record: %v", err)
		return nil, err
	}

	// 10. Enqueue image processing task
	if err := s.enqueueImageProcessTask(ctx, knowledge, kb); err != nil {
		logger.Errorf(ctx, "Failed to enqueue image process task: %v", err)
		return knowledge, nil
	}

	logger.Infof(ctx, "Knowledge from image created, ID: %s", knowledge.ID)
	return knowledge, nil
}

// enqueueImageProcessTask enqueues an image processing task to Asynq.
func (s *knowledgeService) enqueueImageProcessTask(ctx context.Context, knowledge *types.Knowledge, kb *types.KnowledgeBase) error {
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)
	lang, _ := types.LanguageFromContext(ctx)

	taskPayload := types.ImageProcessPayload{
		TenantID:        tenantID,
		KnowledgeID:     knowledge.ID,
		KnowledgeBaseID: knowledge.KnowledgeBaseID,
		ImageURL:        knowledge.FilePath,
		EnableOCR:       true,
		EnableCaption:   true,
		Language:        lang,
	}

	langfuse.InjectTracing(ctx, &taskPayload)
	payloadBytes, err := json.Marshal(taskPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal image process payload: %w", err)
	}

	task := asynq.NewTask(types.TypeImageProcess, payloadBytes, asynq.Queue("default"), asynq.MaxRetry(3))
	info, err := s.task.Enqueue(task)
	if err != nil {
		return fmt.Errorf("failed to enqueue image process task: %w", err)
	}
	logger.Infof(ctx, "Enqueued image process task: id=%s queue=%s knowledge_id=%s", info.ID, info.Queue, knowledge.ID)
	return nil
}

// ProcessImageKnowledge handles the Asynq image:process task.
func (s *knowledgeService) ProcessImageKnowledge(ctx context.Context, t *asynq.Task) error {
	var payload types.ImageProcessPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		logger.Errorf(ctx, "failed to unmarshal image process payload: %v", err)
		return nil
	}

	ctx = logger.WithField(ctx, "image_process", payload.KnowledgeID)
	ctx = context.WithValue(ctx, types.TenantIDContextKey, payload.TenantID)
	if payload.Language != "" {
		ctx = context.WithValue(ctx, types.LanguageContextKey, payload.Language)
	}

	retryCount, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	isLastRetry := retryCount >= maxRetry

	tenantInfo, err := s.tenantRepo.GetTenantByID(ctx, payload.TenantID)
	if err != nil {
		logger.Errorf(ctx, "failed to get tenant: %v", err)
		return nil
	}
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenantInfo)

	logger.Infof(ctx, "Processing image task: knowledge_id=%s, url=%s, retry=%d/%d",
		payload.KnowledgeID, payload.ImageURL, retryCount, maxRetry)

	knowledge, err := s.repo.GetKnowledgeByID(ctx, payload.TenantID, payload.KnowledgeID)
	if err != nil {
		logger.Errorf(ctx, "failed to get knowledge: %v", err)
		return nil
	}
	if knowledge == nil {
		logger.Warnf(ctx, "knowledge not found: %s", payload.KnowledgeID)
		return nil
	}
	if knowledge.ParseStatus == types.ParseStatusCompleted {
		logger.Infof(ctx, "already completed, skipping: %s", payload.KnowledgeID)
		return nil
	}
	if knowledge.ParseStatus == types.ParseStatusDeleting {
		logger.Infof(ctx, "being deleted, skipping: %s", payload.KnowledgeID)
		return nil
	}

	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, payload.KnowledgeBaseID)
	if err != nil {
		logger.Errorf(ctx, "failed to get KB: %v", err)
		_ = markImageFailed(ctx, s.repo, knowledge, fmt.Sprintf("failed to get KB: %v", err))
		return nil
	}

	knowledge.ParseStatus = types.ParseStatusProcessing
	knowledge.UpdatedAt = time.Now()
	if err := s.repo.UpdateKnowledge(ctx, knowledge); err != nil {
		logger.Errorf(ctx, "failed to update knowledge status: %v", err)
		return nil
	}

	if err := s.processImageKnowledge(ctx, kb, knowledge, payload, isLastRetry); err != nil {
		logger.Errorf(ctx, "image processing failed: %v", err)
		return err
	}

	logger.Infof(ctx, "Image processing completed: %s", knowledge.ID)
	return nil
}

// processImageKnowledge executes VLM OCR + Caption → chunk creation → vector indexing.
func (s *knowledgeService) processImageKnowledge(
	ctx context.Context, kb *types.KnowledgeBase, knowledge *types.Knowledge,
	payload types.ImageProcessPayload, isLastRetry bool,
) error {
	logger.Infof(ctx, "Starting image processing pipeline: knowledge=%s", knowledge.ID)

	// 1. Resolve VLM model (model_id MUST exist, validated at KB creation)
	vlmModel, err := s.modelService.GetVLMModel(ctx, kb.VLMConfig.ModelID)
	if err != nil {
		logger.Errorf(ctx, "failed to get VLM model: %v", err)
		_ = markImageFailed(ctx, s.repo, knowledge, fmt.Sprintf("VLM model error: %v", err))
		return nil
	}

	// 2. Read image from storage
	imageBytes, err := s.readImageFromStorage(ctx, kb, payload.ImageURL, payload.TenantID)
	if err != nil {
		logger.Errorf(ctx, "failed to read image: %v", err)
		if isLastRetry {
			_ = markImageFailed(ctx, s.repo, knowledge, fmt.Sprintf("Failed to read image: %v", err))
		}
		return err
	}

	// 3. VLM OCR
	var ocrText string
	if payload.EnableOCR {
		result, ocrErr := vlmModel.Predict(ctx, [][]byte{imageBytes}, vlmOCRPrompt)
		if ocrErr != nil {
			logger.Warnf(ctx, "OCR failed for %s: %v", payload.ImageURL, ocrErr)
		} else {
			result = strings.TrimSpace(result)
			if result != "" && result != "No text content" && result != "No text content." {
				ocrText = result
			}
		}
	}

	// 4. VLM Caption
	var caption string
	if payload.EnableCaption {
		result, capErr := vlmModel.Predict(ctx, [][]byte{imageBytes}, vlmCaptionPrompt)
		if capErr != nil {
			logger.Warnf(ctx, "Caption failed for %s: %v", payload.ImageURL, capErr)
		} else {
			caption = strings.TrimSpace(result)
		}
	}

	// If both results are empty, mark as failed
	if ocrText == "" && caption == "" {
		logger.Warnf(ctx, "Both OCR and caption returned empty for %s", payload.ImageURL)
		_ = markImageFailed(ctx, s.repo, knowledge, "OCR and caption both returned empty results")
		return nil
	}

	// 5. Build ImageInfo
	imageInfo := types.ImageInfo{
		URL:     payload.ImageURL,
		Caption: caption,
		OCRText: ocrText,
	}
	imageInfoJSON, err := json.Marshal([]types.ImageInfo{imageInfo})
	if err != nil {
		logger.Errorf(ctx, "Failed to marshal image info: %v", err)
		_ = markImageFailed(ctx, s.repo, knowledge, fmt.Sprintf("Failed to marshal: %v", err))
		return nil
	}

	// 6. Create chunks
	now := time.Now()
	var chunks []*types.Chunk

	if ocrText != "" {
		chunks = append(chunks, &types.Chunk{
			ID:              uuid.New().String(),
			TenantID:        payload.TenantID,
			KnowledgeID:     knowledge.ID,
			KnowledgeBaseID: kb.ID,
			Content:         ocrText,
			ChunkType:       types.ChunkTypeImageOCR,
			ImageInfo:       string(imageInfoJSON),
			IsEnabled:       true,
			Flags:           types.ChunkFlagRecommended,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}

	if caption != "" {
		chunks = append(chunks, &types.Chunk{
			ID:              uuid.New().String(),
			TenantID:        payload.TenantID,
			KnowledgeID:     knowledge.ID,
			KnowledgeBaseID: kb.ID,
			Content:         caption,
			ChunkType:       types.ChunkTypeImageCaption,
			ImageInfo:       string(imageInfoJSON),
			IsEnabled:       true,
			Flags:           types.ChunkFlagRecommended,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}

	// 7. Save chunks
	if err := s.chunkRepo.CreateChunks(ctx, chunks); err != nil {
		logger.Errorf(ctx, "Failed to create image chunks: %v", err)
		_ = markImageFailed(ctx, s.repo, knowledge, fmt.Sprintf("Failed to create chunks: %v", err))
		return nil
	}
	logger.Infof(ctx, "Created %d chunks for image %s", len(chunks), knowledge.ID)

	// 8. Vector indexing
	s.indexImageChunks(ctx, kb, payload, chunks)

	// 9. Mark completed
	now = time.Now()
	knowledge.ParseStatus = types.ParseStatusCompleted
	knowledge.EnableStatus = "enabled"
	knowledge.ProcessedAt = &now
	knowledge.UpdatedAt = now
	if err := s.repo.UpdateKnowledge(ctx, knowledge); err != nil {
		logger.Errorf(ctx, "Failed to update knowledge status: %v", err)
		return nil
	}

	logger.Infof(ctx, "Image knowledge processing completed: %s", knowledge.ID)
	return nil
}

// indexImageChunks indexes image chunks into the vector store.
func (s *knowledgeService) indexImageChunks(
	ctx context.Context, kb *types.KnowledgeBase,
	payload types.ImageProcessPayload, chunks []*types.Chunk,
) {
	if !kb.NeedsEmbeddingModel() {
		logger.Infof(ctx, "Vector indexing disabled for KB %s, skipping %d image chunks", kb.ID, len(chunks))
		for _, chunk := range chunks {
			chunk.Status = int(types.ChunkStatusIndexed)
			if err := s.chunkRepo.UpdateChunk(ctx, chunk); err != nil {
				logger.Warnf(ctx, "Failed to update chunk %s status: %v", chunk.ID, err)
			}
		}
		return
	}

	embeddingModel, err := s.modelService.GetEmbeddingModel(ctx, kb.EmbeddingModelID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get embedding model: %v", err)
		return
	}

	tenantInfo, err := s.tenantRepo.GetTenantByID(ctx, payload.TenantID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get tenant: %v", err)
		return
	}

	engine, err := retriever.NewCompositeRetrieveEngine(s.retrieveEngine, tenantInfo.GetEffectiveEngines())
	if err != nil {
		logger.Errorf(ctx, "Failed to create composite retrieve engine: %v", err)
		return
	}

	indexInfoList := make([]*types.IndexInfo, 0, len(chunks))
	for _, chunk := range chunks {
		indexInfoList = append(indexInfoList, &types.IndexInfo{
			Content:         chunk.Content,
			SourceID:        chunk.ID,
			SourceType:      types.ChunkSourceType,
			ChunkID:         chunk.ID,
			KnowledgeID:     chunk.KnowledgeID,
			KnowledgeBaseID: chunk.KnowledgeBaseID,
		})
	}

	if err := engine.BatchIndex(ctx, embeddingModel, indexInfoList); err != nil {
		logger.Errorf(ctx, "Failed to index image chunks: %v", err)
		return
	}

	for _, chunk := range chunks {
		chunk.Status = int(types.ChunkStatusIndexed)
		if err := s.chunkRepo.UpdateChunk(ctx, chunk); err != nil {
			logger.Warnf(ctx, "Failed to update chunk %s status: %v", chunk.ID, err)
		}
	}

	logger.Infof(ctx, "Indexed %d image chunks for knowledge %s", len(chunks), payload.KnowledgeID)
}

// readImageFromStorage reads image bytes from provider:// URL via the KB's file service.
func (s *knowledgeService) readImageFromStorage(
	ctx context.Context, kb *types.KnowledgeBase, imageURL string, tenantID uint64,
) ([]byte, error) {
	fileSvc := s.resolveFileServiceForPath(ctx, kb, imageURL)
	reader, err := fileSvc.GetFile(ctx, imageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get image file from storage: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}
	return data, nil
}

// markImageFailed marks the image knowledge as failed.
func markImageFailed(ctx context.Context, repo interfaces.KnowledgeRepository, knowledge *types.Knowledge, errMsg string) error {
	now := time.Now()
	knowledge.ParseStatus = types.ParseStatusFailed
	knowledge.ErrorMessage = errMsg
	knowledge.UpdatedAt = now
	return repo.UpdateKnowledge(ctx, knowledge)
}
