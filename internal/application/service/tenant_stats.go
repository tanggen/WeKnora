package service

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// tenantStatsService implements TenantStatsService
type tenantStatsService struct {
	statsRepo interfaces.TenantStatsRepository
	planRepo  interfaces.PlanRepository
}

// NewTenantStatsService creates a new tenant stats service
func NewTenantStatsService(
	statsRepo interfaces.TenantStatsRepository,
	planRepo interfaces.PlanRepository,
) interfaces.TenantStatsService {
	return &tenantStatsService{
		statsRepo: statsRepo,
		planRepo:  planRepo,
	}
}

// GetTenantStats returns the current stats for a tenant
func (s *tenantStatsService) GetTenantStats(ctx context.Context, tenantID uint64) (*types.TenantStatsResponse, error) {
	stats, err := s.statsRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Get plan config for quota info
	planQuota := int64(0)
	if stats.PlanID != "" {
		plan, err := s.planRepo.GetByID(ctx, stats.PlanID)
		if err == nil {
			planQuota = plan.Config.TokenQuotaMonthly
		}
	}

	return &types.TenantStatsResponse{
		TenantID:           stats.TenantID,
		UserCount:          stats.UserCount,
		KnowledgeBaseCount: stats.KnowledgeBaseCount,
		KnowledgeCount:     stats.KnowledgeCount,
		ChunkCount:         stats.ChunkCount,
		StorageUsedBytes:   stats.StorageUsedBytes,
		StorageQuotaBytes:  stats.StorageQuotaBytes,
		TokenUsed:          stats.TokenUsedMonthly,
		TokenQuotaMonthly:  planQuota,
		TokenQuotaResetAt:  stats.TokenQuotaResetAt,
		AgentCount:         stats.AgentCount,
		SessionCount:       stats.SessionCount,
	}, nil
}

// GetAdminTenantStats returns paginated stats for all tenants
func (s *tenantStatsService) GetAdminTenantStats(
	ctx context.Context,
	keyword string,
	sortBy string,
	order string,
	page int,
	pageSize int,
) (*types.AdminTenantStatsListResponse, error) {
	items, total, err := s.statsRepo.ListAll(ctx, keyword, sortBy, order, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &types.AdminTenantStatsListResponse{
		Data:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetAdminOverview returns the global dashboard summary
func (s *tenantStatsService) GetAdminOverview(ctx context.Context) (*types.AdminOverviewStats, error) {
	return s.statsRepo.GetOverview(ctx)
}

// GetQuotaConfig returns the current quota limits for a tenant
func (s *tenantStatsService) GetQuotaConfig(ctx context.Context, tenantID uint64) (*types.PlanQuotaConfig, error) {
	stats, err := s.statsRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if stats.PlanID == "" {
		return &types.PlanQuotaConfig{}, nil
	}

	plan, err := s.planRepo.GetByID(ctx, stats.PlanID)
	if err != nil {
		return nil, err
	}

	return &types.PlanQuotaConfig{
		MaxUsers:               plan.Config.MaxUsers,
		MaxKnowledgeBases:      plan.Config.MaxKnowledgeBases,
		MaxKnowledgePerKB:      plan.Config.MaxKnowledgePerKB,
		MaxChunksPerKnowledge:  plan.Config.MaxChunksPerKnowledge,
		StorageQuotaBytes:      plan.Config.StorageQuotaBytes,
		TokenQuotaMonthly:      plan.Config.TokenQuotaMonthly,
		MaxAgents:              plan.Config.MaxAgents,
	}, nil
}

// CheckQuota checks if a specific operation would exceed the quota.
// resource: one of "users", "knowledge_bases", "knowledge", "chunks", "storage", "tokens", "agents"
// delta: the proposed change (e.g. +1 for creating, file_size for uploading)
func (s *tenantStatsService) CheckQuota(ctx context.Context, tenantID uint64, resource string, delta int64) error {
	stats, err := s.statsRepo.GetForQuotaCheck(ctx, tenantID)
	if err != nil {
		return err
	}

	// Get plan config (could be cached in production)
	quota, err := s.GetQuotaConfig(ctx, tenantID)
	if err != nil {
		return err
	}

	switch resource {
	case "users":
		if quota.MaxUsers > 0 && int64(stats.UserCount)+delta > int64(quota.MaxUsers) {
			return fmt.Errorf("user limit exceeded: %d/%d", stats.UserCount, quota.MaxUsers)
		}
	case "knowledge_bases":
		if quota.MaxKnowledgeBases > 0 && int64(stats.KnowledgeBaseCount)+delta > int64(quota.MaxKnowledgeBases) {
			return fmt.Errorf("knowledge base limit exceeded: %d/%d", stats.KnowledgeBaseCount, quota.MaxKnowledgeBases)
		}
	case "knowledge":
		if quota.MaxKnowledgePerKB > 0 && int64(stats.KnowledgeCount)+delta > int64(quota.MaxKnowledgePerKB) {
			return fmt.Errorf("knowledge limit exceeded: %d/%d", stats.KnowledgeCount, quota.MaxKnowledgePerKB)
		}
	case "chunks":
		if quota.MaxChunksPerKnowledge > 0 && stats.ChunkCount+delta > int64(quota.MaxChunksPerKnowledge) {
			return fmt.Errorf("chunk limit exceeded: %d/%d", stats.ChunkCount, quota.MaxChunksPerKnowledge)
		}
	case "storage":
		if quota.StorageQuotaBytes > 0 && stats.StorageUsedBytes+delta > quota.StorageQuotaBytes {
			return fmt.Errorf("storage quota exceeded: %d/%d", stats.StorageUsedBytes, quota.StorageQuotaBytes)
		}
	case "tokens":
		if quota.TokenQuotaMonthly > 0 && stats.TokenUsedMonthly+delta > quota.TokenQuotaMonthly {
			return fmt.Errorf("token quota exceeded: %d/%d", stats.TokenUsedMonthly, quota.TokenQuotaMonthly)
		}
	case "agents":
		if quota.MaxAgents > 0 && int64(stats.AgentCount)+delta > int64(quota.MaxAgents) {
			return fmt.Errorf("agent limit exceeded: %d/%d", stats.AgentCount, quota.MaxAgents)
		}
	}

	return nil
}
