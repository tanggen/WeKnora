package types

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Role represents a user's role within a tenant
type Role string

const (
	RoleSystemAdmin Role = "system_admin"
	RoleTenantAdmin Role = "tenant_admin"
	RoleEditor      Role = "editor"
	RoleViewer      Role = "viewer"
)

// ValidRoles returns all valid roles
func ValidRoles() []Role {
	return []Role{RoleSystemAdmin, RoleTenantAdmin, RoleEditor, RoleViewer}
}

// IsValidRole checks if a role string is valid
func IsValidRole(role string) bool {
	for _, r := range ValidRoles() {
		if string(r) == role {
			return true
		}
	}
	return false
}

// IsTenantRole checks if a role can be assigned through tenant user management API
// system_admin cannot be assigned through the tenant API
func IsTenantRole(role Role) bool {
	return role == RoleTenantAdmin || role == RoleEditor || role == RoleViewer
}

// DefaultPermissions returns the default permissions for a given role
func DefaultPermissions(role Role) StringArray {
	switch role {
	case RoleSystemAdmin:
		return StringArray{
			"system:admin", "kb:*", "agent:*", "chat:use",
			"tenant:manage", "user:manage", "model:manage",
		}
	case RoleTenantAdmin:
		return StringArray{
			"kb:create", "kb:edit", "kb:delete",
			"agent:create", "agent:edit", "agent:delete",
			"chat:use", "tenant:manage", "user:manage", "model:manage",
		}
	case RoleEditor:
		return StringArray{
			"kb:create", "kb:edit:own", "kb:delete:own",
			"agent:create", "agent:edit:own", "agent:delete:own",
			"chat:use",
		}
	case RoleViewer:
		return StringArray{"kb:view", "agent:view", "chat:use"}
	default:
		return StringArray{}
	}
}

// Permission constants
const (
	PermKBView         = "kb:view"
	PermKBCreate       = "kb:create"
	PermKBEdit         = "kb:edit"
	PermKBEditOwn      = "kb:edit:own"
	PermKBDelete       = "kb:delete"
	PermKBDeleteOwn    = "kb:delete:own"
	PermAgentView      = "agent:view"
	PermAgentCreate    = "agent:create"
	PermAgentEdit      = "agent:edit"
	PermAgentEditOwn   = "agent:edit:own"
	PermAgentDelete    = "agent:delete"
	PermAgentDeleteOwn = "agent:delete:own"
	PermChatUse        = "chat:use"
	PermTenantManage   = "tenant:manage"
	PermUserView       = "user:view"
	PermUserManage     = "user:manage"
	PermModelManage    = "model:manage"
	PermSystemAdmin    = "system:admin"
)

// TenantUser represents the association between a tenant and a user with a role
type TenantUser struct {
	// Tenant ID
	TenantID uint64 `json:"tenant_id"  gorm:"primaryKey"`
	// User ID
	UserID string `json:"user_id"     gorm:"primaryKey;type:varchar(36)"`
	// Role in the tenant
	Role Role `json:"role"        gorm:"type:varchar(32);not null;default:'viewer'"`
	// Permissions granted to this user (JSON array of permission strings)
	Permissions StringArray `json:"permissions" gorm:"type:jsonb;not null;default:'[]'"`
	// Creation time
	CreatedAt time.Time `json:"created_at"`
	// Soft delete
	DeletedAt gorm.DeletedAt `json:"deleted_at"  gorm:"index"`
}

// TableName returns the table name for GORM
func (TenantUser) TableName() string {
	return "tenant_users"
}

// BeforeCreate is a GORM hook that sets default permissions if empty
func (tu *TenantUser) BeforeCreate(tx *gorm.DB) error {
	if len(tu.Permissions) == 0 {
		tu.Permissions = DefaultPermissions(tu.Role)
	}
	return nil
}

// HasPermission checks if the user has a specific permission
// Supports wildcard permissions like "kb:*" which matches "kb:view", "kb:create", etc.
func (tu *TenantUser) HasPermission(perm string) bool {
	for _, p := range tu.Permissions {
		ps := string(p)
		if ps == perm {
			return true
		}
		// Check wildcard: if stored permission ends with ":*", match any permission with the same prefix
		if strings.HasSuffix(ps, ":*") {
			prefix := strings.TrimSuffix(ps, "*")
			if strings.HasPrefix(perm, prefix) {
				return true
			}
		}
	}
	return false
}

// CreateTenantUserRequest represents a request to add a user to a tenant
type CreateTenantUserRequest struct {
	Username string `json:"username" binding:"required,min=2,max=64"`
	Email    string `json:"email"    binding:"required,email,max=255"`
	Phone    string `json:"phone"`
	Role     Role   `json:"role"     binding:"required"`
}

// UpdateTenantUserRequest represents a request to update a user's profile
type UpdateTenantUserRequest struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
}

// SetUserRoleRequest represents a request to change a user's role
type SetUserRoleRequest struct {
	Role Role `json:"role" binding:"required"`
}

// SetUserStatusRequest represents a request to enable/disable a user
type SetUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// TenantUserInfo represents a tenant user in API responses
type TenantUserInfo struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone,omitempty"`
	Role        Role      `json:"role"`
	IsActive    bool      `json:"is_active"`
	Permissions []string  `json:"permissions"`
	JoinedAt    time.Time `json:"joined_at"`
}

// TenantUserCreatedResponse is returned after successfully creating a user
type TenantUserCreatedResponse struct {
	UserID          string    `json:"user_id"`
	Username        string    `json:"username"`
	Email           string    `json:"email"`
	Phone           string    `json:"phone,omitempty"`
	Role            Role      `json:"role"`
	InitialPassword string    `json:"initial_password"`
	IsActive        bool      `json:"is_active"`
	JoinedAt        time.Time `json:"joined_at"`
}

// TenantUserListResponse represents the paginated list response
type TenantUserListResponse struct {
	Data     []TenantUserInfo `json:"data"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// MarshalJSON ensures StringArray is serialized as []string
func (tu *TenantUser) MarshalJSON() ([]byte, error) {
	type Alias TenantUser
	return json.Marshal(&struct {
		Permissions []string `json:"permissions"`
		*Alias
	}{
		Permissions: []string(tu.Permissions),
		Alias:       (*Alias)(tu),
	})
}
