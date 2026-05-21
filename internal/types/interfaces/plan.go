package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// PlanRepository defines the data access interface for plans
type PlanRepository interface {
	// Create creates a new plan with its config
	Create(ctx context.Context, plan *types.Plan, config *types.PlanConfig) error
	// Update updates a plan and its config
	Update(ctx context.Context, plan *types.Plan, config *types.PlanConfig) error
	// Delete deletes a plan and its config
	Delete(ctx context.Context, planID string) error
	// GetByID retrieves a plan with its config
	GetByID(ctx context.Context, planID string) (*types.PlanWithConfig, error)
	// List returns all active plans (for user-facing display)
	List(ctx context.Context) ([]*types.PlanWithConfig, error)
	// ListAll returns all plans including inactive (for admin)
	ListAll(ctx context.Context) ([]*types.PlanWithConfig, error)
	// CountTenantsByPlan counts tenants using a specific plan
	CountTenantsByPlan(ctx context.Context, planID string) (int64, error)
}

// TenantStatsRepository defines the data access interface for tenant statistics
type TenantStatsRepository interface {
	// GetByTenantID retrieves stats for a tenant (creates row if missing)
	GetByTenantID(ctx context.Context, tenantID uint64) (*types.TenantStats, error)
	// GetForQuotaCheck retrieves minimal stats for quota checking
	GetForQuotaCheck(ctx context.Context, tenantID uint64) (*types.TenantStats, error)

	// Atomic increment/decrement operations
	IncrementUserCount(ctx context.Context, tenantID uint64, delta int) error
	IncrementKBCount(ctx context.Context, tenantID uint64, delta int) error
	IncrementKnowledgeCount(ctx context.Context, tenantID uint64, delta int) error
	IncrementChunkCount(ctx context.Context, tenantID uint64, delta int64) error
	IncrementStorageUsed(ctx context.Context, tenantID uint64, delta int64) error
	IncrementTokenUsed(ctx context.Context, tenantID uint64, delta int64) error
	IncrementAgentCount(ctx context.Context, tenantID uint64, delta int) error
	IncrementSessionCount(ctx context.Context, tenantID uint64, delta int64) error

	// UpdatePlanSnapshot updates the plan-related fields
	UpdatePlanSnapshot(ctx context.Context, tenantID uint64, planID string, storageQuota int64) error

	// ResetTokenQuotaMonthly resets monthly token usage for all tenants
	ResetTokenQuotaMonthly(ctx context.Context) (int64, error)

	// ListAll returns all tenant stats (for admin dashboard)
	ListAll(ctx context.Context, keyword string, sortBy string, order string, page, pageSize int) ([]*types.AdminTenantStatsItem, int64, error)

	// GetOverview returns the global dashboard summary
	GetOverview(ctx context.Context) (*types.AdminOverviewStats, error)
}

// PlanService defines the business logic interface for plans
type PlanService interface {
	// CreatePlan creates a new subscription plan
	CreatePlan(ctx context.Context, req *types.CreatePlanRequest) (*types.PlanWithConfig, error)
	// UpdatePlan updates an existing plan
	UpdatePlan(ctx context.Context, planID string, req *types.UpdatePlanRequest) (*types.PlanWithConfig, error)
	// DeletePlan deletes a plan (only if unused)
	DeletePlan(ctx context.Context, planID string) error
	// SetPlanStatus enables or disables a plan
	SetPlanStatus(ctx context.Context, planID string, isActive bool) error
	// GetPlan retrieves plan details
	GetPlan(ctx context.Context, planID string) (*types.PlanWithConfig, error)
	// ListPlans returns active plans for users
	ListPlans(ctx context.Context) ([]*types.PlanWithConfig, error)
	// ListAllPlans returns all plans for admin
	ListAllPlans(ctx context.Context) ([]*types.PlanWithConfig, error)
	// AssignPlan assigns a plan to a tenant
	AssignPlan(ctx context.Context, tenantID uint64, planID string) (*types.AssignPlanResponse, error)
}

// TenantStatsService defines the business logic interface for tenant statistics
type TenantStatsService interface {
	// GetTenantStats returns the current stats for a tenant
	GetTenantStats(ctx context.Context, tenantID uint64) (*types.TenantStatsResponse, error)
	// GetAdminTenantStats returns paginated stats for all tenants (admin only)
	GetAdminTenantStats(ctx context.Context, keyword string, sortBy string, order string, page, pageSize int) (*types.AdminTenantStatsListResponse, error)
	// GetAdminOverview returns the global dashboard summary
	GetAdminOverview(ctx context.Context) (*types.AdminOverviewStats, error)
	// GetQuotaConfig returns the current quota limits for a tenant
	GetQuotaConfig(ctx context.Context, tenantID uint64) (*types.PlanQuotaConfig, error)
	// CheckQuota checks if a specific operation would exceed the quota
	CheckQuota(ctx context.Context, tenantID uint64, resource string, delta int64) error
}
