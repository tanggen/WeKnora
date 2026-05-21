package repository

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// tenantUserRepository implements TenantUserRepository
type tenantUserRepository struct {
	db *gorm.DB
}

// NewTenantUserRepository creates a new tenant user repository
func NewTenantUserRepository(db *gorm.DB) interfaces.TenantUserRepository {
	return &tenantUserRepository{db: db}
}

// Create adds a user to a tenant
func (r *tenantUserRepository) Create(ctx context.Context, tu *types.TenantUser) error {
	return r.db.WithContext(ctx).Create(tu).Error
}

// GetByTenantAndUser retrieves a specific tenant-user association
func (r *tenantUserRepository) GetByTenantAndUser(ctx context.Context, tenantID uint64, userID string) (*types.TenantUser, error) {
	var tu types.TenantUser
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&tu).Error
	if err != nil {
		return nil, err
	}
	return &tu, nil
}

// GetByUserID retrieves the tenant-user association for a user (unique per user)
func (r *tenantUserRepository) GetByUserID(ctx context.Context, userID string) (*types.TenantUser, error) {
	var tu types.TenantUser
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&tu).Error
	if err != nil {
		return nil, err
	}
	return &tu, nil
}

// ListByTenant returns paginated users within a tenant with optional filters
func (r *tenantUserRepository) ListByTenant(ctx context.Context, tenantID uint64, keyword string, role string, isActive *bool, page, pageSize int) ([]*types.TenantUser, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&types.TenantUser{}).
		Joins("JOIN users ON users.id = tenant_users.user_id").
		Where("tenant_users.tenant_id = ?", tenantID)

	// Apply filters
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(users.username ILIKE ? OR users.email ILIKE ?)", like, like)
	}
	if role != "" {
		query = query.Where("tenant_users.role = ?", role)
	}
	if isActive != nil {
		query = query.Where("users.is_active = ?", *isActive)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Paginate
	var results []*types.TenantUser
	offset := (page - 1) * pageSize
	err := query.
		Select("tenant_users.*").
		Offset(offset).
		Limit(pageSize).
		Order("tenant_users.created_at DESC").
		Find(&results).Error

	return results, total, err
}

// Update updates a tenant-user association
func (r *tenantUserRepository) Update(ctx context.Context, tu *types.TenantUser) error {
	return r.db.WithContext(ctx).Save(tu).Error
}

// Delete soft-deletes a tenant-user association
func (r *tenantUserRepository) Delete(ctx context.Context, tenantID uint64, userID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&types.TenantUser{}).Error
}

// CountActiveAdmins counts active tenant_admin users in a tenant
func (r *tenantUserRepository) CountActiveAdmins(ctx context.Context, tenantID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.TenantUser{}).
		Joins("JOIN users ON users.id = tenant_users.user_id").
		Where("tenant_users.tenant_id = ? AND tenant_users.role = ? AND users.is_active = ?",
			tenantID, types.RoleTenantAdmin, true).
		Count(&count).Error
	return count, err
}
