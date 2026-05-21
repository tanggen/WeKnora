package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// AdminService provides system admin business logic including tenant management,
// system tenant protection, and disable cascading operations.
type AdminService struct {
	tenantRepo     interfaces.TenantRepository
	tenantUserRepo interfaces.TenantUserRepository
	userRepo       interfaces.UserRepository
	statsRepo      interfaces.TenantStatsRepository
	planRepo       interfaces.PlanRepository
	menuService    *MenuService
}

// NewAdminService creates a new admin service
func NewAdminService(
	tenantRepo interfaces.TenantRepository,
	tenantUserRepo interfaces.TenantUserRepository,
	userRepo interfaces.UserRepository,
	statsRepo interfaces.TenantStatsRepository,
	planRepo interfaces.PlanRepository,
	menuService *MenuService,
) *AdminService {
	return &AdminService{
		tenantRepo:     tenantRepo,
		tenantUserRepo: tenantUserRepo,
		userRepo:       userRepo,
		statsRepo:      statsRepo,
		planRepo:       planRepo,
		menuService:    menuService,
	}
}

const (
	systemTenantID = 1
	tenantActive   = "active"
	tenantDisabled = "disabled"
)

// ──────────────────────────────────────────────
//  Types
// ──────────────────────────────────────────────

// AdminTenantListItem is a single row in the admin tenant list
type AdminTenantListItem struct {
	TenantID            uint64     `json:"tenant_id"`
	Name                string     `json:"name"`
	Description         string     `json:"description"`
	PlanID              string     `json:"plan_id"`
	PlanName            string     `json:"plan_name"`
	Status              string     `json:"status"`
	IsSystem            bool       `json:"is_system"`
	TrialExpiresAt      *time.Time `json:"trial_expires_at,omitempty"`
	UserCount           int        `json:"user_count"`
	StorageUsedBytes    int64      `json:"storage_used_bytes"`
	TokenUsedMonthly    int64      `json:"token_used_monthly"`
	KnowledgeBaseCount  int        `json:"knowledge_base_count"`
	CreatedAt           time.Time  `json:"created_at"`
}

// AdminTenantDetail is the detailed view of a tenant for admin
type AdminTenantDetail struct {
	TenantID            uint64                `json:"tenant_id"`
	Name                string                `json:"name"`
	Description         string                `json:"description"`
	PlanID              string                `json:"plan_id"`
	PlanName            string                `json:"plan_name"`
	Status              string                `json:"status"`
	IsSystem            bool                  `json:"is_system"`
	TrialExpiresAt      *time.Time            `json:"trial_expires_at,omitempty"`
	StorageQuotaBytes   int64                 `json:"storage_quota_bytes"`
	CreatedAt           time.Time             `json:"created_at"`
	Users               []*TenantUserInfoItem  `json:"users"`
	Stats               *TenantDetailStats     `json:"stats"`
}

// TenantUserInfoItem is a user row within admin tenant detail
type TenantUserInfoItem struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Role     types.Role `json:"role"`
	IsActive bool      `json:"is_active"`
	JoinedAt time.Time `json:"joined_at"`
}

// TenantDetailStats is the stats snapshot within tenant detail
type TenantDetailStats struct {
	KnowledgeBaseCount int   `json:"knowledge_base_count"`
	KnowledgeCount     int   `json:"knowledge_count"`
	ChunkCount         int64 `json:"chunk_count"`
	StorageUsedBytes   int64 `json:"storage_used_bytes"`
	TokenUsedMonthly   int64 `json:"token_used_monthly"`
	TokenUsedTotal     int64 `json:"token_used_total"`
	AgentCount         int   `json:"agent_count"`
	SessionCount       int64 `json:"session_count"`
}

// UpdateTenantRequest is the request body for updating tenant info
type UpdateTenantRequest struct {
	Name        string `json:"name"        binding:"omitempty,max=128"`
	Description string `json:"description"`
}

// SetTenantStatusRequest is the request body for changing tenant status
type SetTenantStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// ──────────────────────────────────────────────
//  Tenant listing
// ──────────────────────────────────────────────

// ListTenants returns paginated tenant list with optional filters (admin only)
func (s *AdminService) ListTenants(ctx context.Context, keyword, status, planID string, page, pageSize int) ([]*AdminTenantListItem, int64, error) {
	tenants, total, err := s.tenantRepo.FindAll(ctx, keyword, status, planID, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("find tenants: %w", err)
	}

	items := make([]*AdminTenantListItem, 0, len(tenants))
	for _, t := range tenants {
		item := &AdminTenantListItem{
			TenantID:       t.ID,
			Name:           t.Name,
			Description:    t.Description,
			PlanID:         t.PlanID,
			Status:         t.Status,
			IsSystem:       t.IsSystem,
			TrialExpiresAt: t.TrialExpiresAt,
			CreatedAt:      t.CreatedAt,
		}

		// Fetch stats for counts
		stats, statsErr := s.statsRepo.GetByTenantID(ctx, t.ID)
		if statsErr == nil {
			item.UserCount = stats.UserCount
			item.StorageUsedBytes = stats.StorageUsedBytes
			item.TokenUsedMonthly = stats.TokenUsedMonthly
			item.KnowledgeBaseCount = stats.KnowledgeBaseCount
		}

		// Fetch plan name
		if t.PlanID != "" {
			plan, planErr := s.planRepo.GetByID(ctx, t.PlanID)
			if planErr == nil {
				item.PlanName = plan.Name
			} else {
				item.PlanName = t.PlanID
			}
		}

		items = append(items, item)
	}

	return items, total, nil
}

// ──────────────────────────────────────────────
//  Tenant detail
// ──────────────────────────────────────────────

// GetTenantDetail returns detailed info for a single tenant (admin only)
func (s *AdminService) GetTenantDetail(ctx context.Context, tenantID uint64) (*AdminTenantDetail, error) {
	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	detail := &AdminTenantDetail{
		TenantID:       tenant.ID,
		Name:           tenant.Name,
		Description:    tenant.Description,
		PlanID:         tenant.PlanID,
		Status:         tenant.Status,
		IsSystem:       tenant.IsSystem,
		TrialExpiresAt: tenant.TrialExpiresAt,
		CreatedAt:      tenant.CreatedAt,
	}

	// Plan info
	if tenant.PlanID != "" {
		plan, planErr := s.planRepo.GetByID(ctx, tenant.PlanID)
		if planErr == nil {
			detail.PlanName = plan.Name
			detail.StorageQuotaBytes = plan.Config.StorageQuotaBytes
		} else {
			detail.PlanName = tenant.PlanID
			detail.StorageQuotaBytes = tenant.StorageQuota
		}
	}

	// Stats snapshot
	stats, statsErr := s.statsRepo.GetByTenantID(ctx, tenantID)
	if statsErr == nil {
		detail.Stats = &TenantDetailStats{
			KnowledgeBaseCount: stats.KnowledgeBaseCount,
			KnowledgeCount:     stats.KnowledgeCount,
			ChunkCount:         stats.ChunkCount,
			StorageUsedBytes:   stats.StorageUsedBytes,
			TokenUsedMonthly:   stats.TokenUsedMonthly,
			TokenUsedTotal:     stats.TokenUsedTotal,
			AgentCount:         stats.AgentCount,
			SessionCount:       stats.SessionCount,
		}
	}

	// List users (first 50)
	tus, _, err := s.tenantUserRepo.ListByTenant(ctx, tenantID, "", "", nil, 1, 50)
	if err == nil {
		userItems := make([]*TenantUserInfoItem, 0, len(tus))
		for _, tu := range tus {
			item := &TenantUserInfoItem{
				UserID:   tu.UserID,
				Role:     tu.Role,
				JoinedAt: tu.CreatedAt,
			}
			if user, err := s.userRepo.GetUserByID(ctx, tu.UserID); err == nil && user != nil {
				item.Username = user.Username
				item.Email = user.Email
				item.IsActive = user.IsActive
			}
			userItems = append(userItems, item)
		}
		detail.Users = userItems
	}

	return detail, nil
}

// ──────────────────────────────────────────────
//  Tenant update
// ──────────────────────────────────────────────

// UpdateTenant updates tenant name/description (admin only).
// System tenant (ID=1) is protected from editing.
func (s *AdminService) UpdateTenant(ctx context.Context, tenantID uint64, req *UpdateTenantRequest) error {
	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return err
	}

	// System tenant protection
	if tenant.IsSystem {
		return newSystemTenantProtectedError()
	}

	if req.Name != "" && req.Name != tenant.Name {
		// Check name uniqueness (best effort — not strictly enforced but prevents accidental duplicates)
		tenant.Name = req.Name
	}

	if req.Description != "" {
		tenant.Description = req.Description
	}

	return s.tenantRepo.UpdateTenant(ctx, tenant)
}

// SetTenantStatus enables or disables a tenant (admin only).
// System tenant (ID=1) is protected from status changes.
// On disable: clears all tenant user menu/permission caches.
func (s *AdminService) SetTenantStatus(ctx context.Context, tenantID uint64, status string) error {
	if status != tenantActive && status != tenantDisabled {
		return &errors.AppError{
			Code:     errors.ErrTenantInvalidStatus,
			Message:  fmt.Sprintf("非法状态值: %s，仅支持 active 和 disabled", status),
			HTTPCode: http.StatusUnprocessableEntity,
		}
	}

	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return err
	}

	// System tenant protection
	if tenant.IsSystem {
		return newSystemTenantProtectedError()
	}

	tenant.Status = status
	if err := s.tenantRepo.UpdateTenant(ctx, tenant); err != nil {
		return fmt.Errorf("update tenant status: %w", err)
	}

	// Cascade: clear menu cache for all users in this tenant when disabling
	if status == tenantDisabled && s.menuService != nil {
		s.menuService.InvalidateAllTenantCache(ctx, tenantID)
		logger.Infof(ctx, "[Admin] Tenant %d disabled, cleared all user menu caches", tenantID)
	}

	return nil
}

// ──────────────────────────────────────────────
//  Tenant users
// ──────────────────────────────────────────────

// GetTenantUsers returns paginated users within a tenant (admin view)
func (s *AdminService) GetTenantUsers(ctx context.Context, tenantID uint64, keyword, role, status string, page, pageSize int) ([]*TenantUserInfoItem, int64, error) {
	var isActive *bool
	if status == "active" {
		t := true
		isActive = &t
	} else if status == "disabled" {
		f := false
		isActive = &f
	}

	tus, total, err := s.tenantUserRepo.ListByTenant(ctx, tenantID, keyword, role, isActive, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list tenant users: %w", err)
	}

	items := make([]*TenantUserInfoItem, 0, len(tus))
	for _, tu := range tus {
		item := &TenantUserInfoItem{
			UserID:   tu.UserID,
			Role:     tu.Role,
			JoinedAt: tu.CreatedAt,
		}
		if user, err := s.userRepo.GetUserByID(ctx, tu.UserID); err == nil && user != nil {
			item.Username = user.Username
			item.Email = user.Email
			item.IsActive = user.IsActive
		}
		items = append(items, item)
	}

	return items, total, nil
}

// newSystemTenantProtectedError creates a standard error for system tenant protection
func newSystemTenantProtectedError() *errors.AppError {
	return &errors.AppError{
		Code:     errors.ErrTenantInvalidStatus,
		Message:  "不允许操作系统租户",
		HTTPCode: http.StatusUnprocessableEntity,
	}
}
