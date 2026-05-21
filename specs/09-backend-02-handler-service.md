# Agent 开发卡片：图片库 — 后端 API Handler 与 Service

> 模块编号：09-BACKEND-02 | 优先级：P0 | 预估工作量：4-5h | 依赖：09-BACKEND-01

---

## 1. API 契约

### 端点 1：上传图片到图片库（单张）

```
POST /api/v1/knowledge-bases/:id/knowledge/image
  Auth: Bearer Token OR X-API-Key
  Content-Type: multipart/form-data

  Request:
    file:  <binary>              (必填, 图片文件, ≤20MB, 格式 jpg/png/gif/webp/bmp)
    tag_id: string               (可选, 分类ID, VARCHAR(36))
    channel: string              (可选, 来源渠道, 默认 "web")

  Response 201:
    {
      "success": true,
      "data": {
        "id": "uuid",
        "tenant_id": 1,
        "knowledge_base_id": "kb-uuid",
        "type": "image",
        "title": "image_001.png",
        "file_name": "image_001.png",
        "file_type": "png",
        "file_size": 2048576,
        "file_hash": "abc123def456",
        "file_path": "local://tenant/images/abc123.png",
        "parse_status": "pending",
        "enable_status": "disabled",
        "tag_id": "tag-uuid",
        "created_at": "2026-05-21T10:00:00Z"
      }
    }

  Errors:
    400 INVALID_FILE_TYPE      — 不支持的文件类型（仅 jpg/png/gif/webp/bmp）
    400 FILE_TOO_LARGE          — 文件超过 20MB
    400 NOT_IMAGE_KB            — 该知识库不是图片库类型（type != "image"）
    409 DUPLICATE_IMAGE         — 同一图片库内已存在相同 MD5 的图片
    403 FORBIDDEN               — 无写入权限
    422 VLM_NOT_CONFIGURED      — 图片库的 VLM 未配置（应创建KB时拦截，此处为防御性检查）
    429 TOO_MANY_REQUESTS       — 上传频率过高
    500 INTERNAL_ERROR          — 文件存储失败
```

### 端点 2：批量上传图片

```
POST /api/v1/knowledge-bases/:id/knowledge/image/batch
  Auth: Bearer Token OR X-API-Key
  Content-Type: multipart/form-data

  Request:
    files: <binary[]>           (必填, 图片文件数组, 最多20张)
    tag_id: string              (可选, 所有图片统一分类)
    channel: string             (可选, 默认 "web")

  Response 201:
    {
      "success": true,
      "data": {
        "success_count": 15,
        "failed": [
          { "file_name": "corrupt.png", "reason": "无法解析图片文件" },
          { "file_name": "duplicate.jpg", "reason": "图片已存在於图片库中" }
        ],
        "knowledges": [...]     // 成功创建的 Knowledge 列表
      }
    }

  Errors:
    400 INVALID_FILE_TYPE       — 所有文件都不是支持的类型
    400 FILE_TOO_LARGE          — 任一文件超过 20MB
    400 BATCH_TOO_LARGE         — 超过 20 张
    400 NOT_IMAGE_KB            — 该知识库不是图片库类型
```

### 端点 3：图片库列表（无新增，复用现有）

```
GET /api/v1/knowledge-bases/:id/knowledge?page=1&page_size=20&tag_id=xxx&file_type=jpg

  对 image 类型 KB，返回的 Knowledge 条目额外包含：
    - file_type: "jpg" | "png" | ...
    - file_size: 字节数
    - thumbnail_url: "provider://.../_thumb.jpg" （P1，如未生成则为空）
    - metadata.ocr_text: OCR 文本摘要（P1，用于列表展示）
    - metadata.caption: AI 描述摘要（P1）
```

### 端点 4：创建图片库时的校验增强

```
POST /api/v1/knowledge-bases
  (对现有端点增加 image 类型校验)

  type = "image" 时额外校验：
    vlm_config.enabled MUST be true
    vlm_config.model_id MUST be non-empty
    embedding_model_id MUST be non-empty

  type = "image" 时自动设置：
    indexing_strategy.vector_enabled = true
    indexing_strategy.keyword_enabled = false
    indexing_strategy.wiki_enabled = false
    indexing_strategy.graph_enabled = false

  Error:
    422 VLM_REQUIRED_FOR_IMAGE_KB  — 图片库必须启用 VLM 并绑定模型
    422 EMBEDDING_REQUIRED_FOR_IMAGE_KB — 图片库必须绑定 Embedding 模型
```

---

## 2. 数据库 DDL

**零变更。** 完全复用现有表。参考卡片 09-BACKEND-01。

---

## 3. Redis Key Schema

**无新增。** 图片处理进度通过 `knowledges.parse_status` 字段反映。

---

## 4. 业务规则汇总

### 4.1 CreateKnowledgeFromImage Handler 逻辑

```go
// internal/handler/knowledge.go
func (h *KnowledgeHandler) CreateKnowledgeFromImage(c *gin.Context) {
    // 1. 权限校验（复用 validateKnowledgeBaseAccess）
    _, kbID, effectiveTenantID, permission, err := h.validateKnowledgeBaseAccess(c)

    // 2. 写权限检查 (editor+)
    if permission != types.OrgRoleAdmin && permission != types.OrgRoleEditor {
        return FORBIDDEN
    }

    // 3. 知识库类型校验
    kb, _ := h.kbService.GetKnowledgeBaseByID(ctx, kbID)
    if kb.Type != types.KnowledgeBaseTypeImage {
        return NOT_IMAGE_KB
    }

    // 4. 获取上传文件
    file, err := c.FormFile("file")

    // 5. 文件格式校验
    fileType := getFileType(file.Filename)
    if !IsAllowedImageType(fileType) {
        return INVALID_FILE_TYPE
    }

    // 6. 文件大小校验（≤20MB）
    if file.Size > maxImageSize {
        return FILE_TOO_LARGE
    }

    // 7. 委托给 Service 层
    knowledge, err := h.kgService.CreateKnowledgeFromImage(
        ctx, kbID, file, tagID, channel,
    )

    // 8. 处理重复图片
    if dupErr, ok := err.(*types.DuplicateKnowledgeError); ok {
        return 409 DUPLICATE_IMAGE { data: dupErr.ExistingKnowledge }
    }

    // 9. 返回成功
    c.JSON(201, gin.H{"success": true, "data": knowledge})
}
```

### 4.2 CreateKnowledgeFromImage Service 逻辑

```go
// internal/application/service/knowledge.go
func (s *knowledgeService) CreateKnowledgeFromImage(
    ctx context.Context,
    kbID string,
    file *multipart.FileHeader,
    tagID string,
    channel string,
) (*types.Knowledge, error) {

    // 1. 读取文件字节，计算 MD5
    imageBytes, md5Hash := readAndHashFile(file)

    // 2. 验证是否为真实图片（检查文件头魔数）
    if !isValidImageHeader(imageBytes) {
        return nil, ErrInvalidImage
    }

    // 3. 获取 KB 配置
    kb, _ := s.kbService.GetKnowledgeBaseByID(ctx, kbID)

    // 4. 去重检查
    exists, existing, _ := s.repo.CheckKnowledgeExists(ctx, tenantID, kbID, &types.KnowledgeCheckParams{
        Type:     types.KnowledgeTypeImage,
        FileHash: md5Hash,
    })
    if exists {
        return existing, types.NewDuplicateKnowledgeError(existing)
    }

    // 5. 存储图片到 provider://
    filePath, err := s.fileSvc.SaveFile(ctx, imageBytes, fileName, tenantID)

    // 6. 创建 Knowledge 记录
    knowledge := &types.Knowledge{
        Type:             types.KnowledgeTypeImage,
        KnowledgeBaseID:  kbID,
        TenantID:         tenantID,
        Title:            fileName,
        FileName:         fileName,
        FileType:         fileType,
        FileSize:         file.Size,
        FileHash:         md5Hash,
        FilePath:         filePath,
        ParseStatus:      "pending",
        EnableStatus:     "disabled",
        EmbeddingModelID: kb.EmbeddingModelID,
        TagID:            tagID,
    }
    s.repo.CreateKnowledge(ctx, knowledge)

    // 7. 入队图片处理任务
    s.enqueueImageProcessTask(ctx, knowledge)

    return knowledge, nil
}
```

### 4.3 processImageKnowledge 核心逻辑

```go
// internal/application/service/image_knowledge.go (新文件)
func (s *knowledgeService) processImageKnowledge(
    ctx context.Context,
    payload types.ImageProcessPayload,
) error {

    // 1. 加载 Knowledge 和 KB
    knowledge, kb := s.loadContext(ctx, payload)

    // 2. 更新状态为 processing
    knowledge.ParseStatus = "processing"
    s.repo.UpdateKnowledge(ctx, knowledge)

    // 3. 读取图片字节（从 provider:// 读取）
    imgBytes := s.readImageBytes(ctx, payload.ImageURL)

    // 4. 生成缩略图（P1）
    thumbBytes := generateThumbnail(imgBytes, 200, 200)
    s.fileSvc.SaveFile(ctx, thumbBytes, thumbFileName, tenantID)

    // 5. VLM OCR
    ocrText, _ := s.callVLM(ctx, kb, imgBytes, ocrPrompt)

    // 6. VLM Caption
    caption, _ := s.callVLM(ctx, kb, imgBytes, captionPrompt)

    // 7. 构建 ImageInfo
    imageInfo := types.ImageInfo{
        URL:      payload.ImageURL,
        Caption:  caption,
        OCRText:  ocrText,
    }
    imageInfoJSON, _ := json.Marshal([]types.ImageInfo{imageInfo})

    // 8. 创建 OCR Chunk
    ocrChunk := &types.Chunk{
        KnowledgeID:     knowledge.ID,
        KnowledgeBaseID: kb.ID,
        TenantID:        payload.TenantID,
        Content:         ocrText,
        ChunkType:       types.ChunkTypeImageOCR,
        ImageInfo:       string(imageInfoJSON),
        IsEnabled:       true,
    }

    // 9. 创建 Caption Chunk
    captionChunk := &types.Chunk{
        KnowledgeID:     knowledge.ID,
        KnowledgeBaseID: kb.ID,
        TenantID:        payload.TenantID,
        Content:         caption,
        ChunkType:       types.ChunkTypeImageCaption,
        ImageInfo:       string(imageInfoJSON),
        IsEnabled:       true,
    }

    // 10. 设置关联
    // (captionChunk 和 ocrChunk 互相引用，用于检索时同时返回)
    ocrChunk.ParentChunkID = ""  // P2 阶段可设置互相关联
    captionChunk.ParentChunkID = ""

    // 11. 写入数据库
    s.chunkRepo.CreateChunks(ctx, []*types.Chunk{ocrChunk, captionChunk})

    // 12. 向量化 + 索引
    s.vectorizeAndIndex(ctx, kb, knowledge, []*types.Chunk{ocrChunk, captionChunk})

    // 13. 更新 Knowledge 状态
    knowledge.ParseStatus = "completed"
    knowledge.EnableStatus = "enabled"
    knowledge.ProcessedAt = now()
    s.repo.UpdateKnowledge(ctx, knowledge)

    return nil
}
```

### 4.4 图片校验规则

| 规则 | 验证方式 |
|------|----------|
| 格式白名单 | 扩展名检查 + MIME 类型检查 |
| 文件大小 | `file.Size ≤ 20MB` |
| 文件魔数 | 读前 512 字节检测是否为真实图片 (jpg: FF D8 FF, png: 89 50 4E 47, gif: 47 49 46, webp: 52 49 46 46, bmp: 42 4D) |
| 去重 | 同一 KB 内 `file_hash` 唯一 |

### 4.5 VLM 调用超时与重试

- 单次 VLM 调用超时：60 秒（复用现有 `image_multimodal.go` 逻辑）
- 重试：Asynq 层面 3 次，`asynq.MaxRetry(3)`
- 重试延迟：Asynq 默认指数退避（30s → 60s → 120s）

---

## 5. 验收测试场景

### 正例 1：上传 PNG 图片到图片库

```
Given: 图片库 KB-IMG-001 (type=image, VLM已配置)
When: POST /api/v1/knowledge-bases/KB-IMG-001/knowledge/image
     file = test_image.png (800x600, 1.5MB)
Then: 返回 201
      knowledge.type = "image"
      knowledge.file_path = "local://..."
      knowledge.parse_status = "pending"
      1分钟后 parse_status = "completed"
      生成 2 个 Chunk (image_ocr, image_caption)
```

### 正例 2：创建图片库时自动设置索引策略

```
Given: VLM 模型 vlm-001 可用
When: POST /api/v1/knowledge-bases
     { type: "image", vlm_config: { enabled:true, model_id:"vlm-001" }, ... }
Then: 返回 201
      kb.indexing_strategy.vector_enabled = true
      kb.indexing_strategy.wiki_enabled = false
      kb.indexing_strategy.graph_enabled = false
```

### 反例 1：对 document 类型 KB 调用图片上传

```
Given: KB-DOC-001 (type=document)
When: POST /api/v1/knowledge-bases/KB-DOC-001/knowledge/image
     file = test.jpg
Then: 返回 400 NOT_IMAGE_KB
      "该知识库不是图片库类型"
```

### 反例 2：上传损坏的图片

```
Given: 图片库 KB-IMG-001
When: 上传一个内容为 "NOT_AN_IMAGE" 但扩展名为 .jpg 的文件
Then: 返回 400 INVALID_FILE_TYPE
      "无法解析图片文件，请确认文件格式正确"
```

### 反例 3：上传超过大小限制的图片

```
Given: 图片库 KB-IMG-001
When: 上传 25MB 的 PNG 图片
Then: 返回 400 FILE_TOO_LARGE
      "文件大小不能超过20MB"
```

---

## 6. 文件清单

### 需要修改的文件

| 文件路径 | 改动说明 |
|----------|----------|
| `internal/handler/knowledge.go` | 新增 `CreateKnowledgeFromImage` + `CreateKnowledgeFromImageBatch` handler |
| `internal/handler/knowledgebase.go` | `CreateKnowledgeBase` 增加 image 类型 VLM 强制校验 |
| `internal/application/service/knowledge.go` | 新增 `CreateKnowledgeFromImage` service 方法 |
| `internal/router/router.go` | `RegisterKnowledgeRoutes` 新增 `/image` 和 `/image/batch` 路由 |
| `internal/types/interfaces/knowledge.go` | 接口新增方法签名 |

### 需要新建的文件

| 文件路径 | 改动说明 |
|----------|----------|
| `internal/application/service/image_knowledge.go` | `processImageKnowledge()` (VLM调用 + Chunk生成 + 向量化) |
