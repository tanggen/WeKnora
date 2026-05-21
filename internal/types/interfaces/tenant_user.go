package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// TenantUserRepository defines the data access interface for tenant users
type TenantUserRepository interface {
	// Create adds a user to a tenant
	Create(ctx context.Context, tu *types.TenantUser) error
	// GetByTenantAndUser retrieves a specific tenant-user association
	GetByTenantAndUser(ctx context.Context, tenantID uint64, userID string) (*types.TenantUser, error)
	// GetByUserID retrieves the tenant-user association for a user (unique per user)
	GetByUserID(ctx context.Context, userID string) (*types.TenantUser, error)
	// ListByTenant returns paginated users within a tenant
	ListByTenant(ctx context.Context, tenantID uint64, keyword string, role string, isActive *bool, page, pageSize int) ([]*types.TenantUser, int64, error)
	// Update updates a tenant-user association
	Update(ctx context.Context, tu *types.TenantUser) error
	// Delete soft-deletes a tenant-user association
	Delete(ctx context.Context, tenantID uint64, userID string) error
	// CountActiveAdmins counts active tenant_admin users in a tenant
	CountActiveAdmins(ctx context.Context, tenantID uint64) (int64, error)
}

// TenantUserService defines the business logic interface for tenant users
type TenantUserService interface {
	// ListUsers returns paginated users within a tenant
	ListUsers(ctx context.Context, tenantID uint64, keyword, role, status string, page, pageSize int) (*types.TenantUserListResponse, error)
	// CreateUser creates a new user and adds them to the tenant
	CreateUser(ctx context.Context, tenantID uint64, req *types.CreateTenantUserRequest) (*types.TenantUserCreatedResponse, error)
	// UpdateUser updates a user's profile
	UpdateUser(ctx context.Context, tenantID uint64, userID string, req *types.UpdateTenantUserRequest) (*types.TenantUserInfo, error)
	// SetUserStatus enables or disables a user
	SetUserStatus(ctx context.Context, tenantID uint64, userID string, isActive bool) error
	// DeleteUser soft-deletes a user
	DeleteUser(ctx context.Context, tenantID uint64, userID string) error
	// SetUserRole changes a user's role
	SetUserRole(ctx context.Context, tenantID uint64, userID string, req *types.SetUserRoleRequest) (*types.TenantUserInfo, error)
	// GetTenantUser retrieves the tenant-user association
	GetTenantUser(ctx context.Context, tenantID uint64, userID string) (*types.TenantUser, error)
}
