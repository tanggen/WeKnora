package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// RequireRole creates a middleware that checks if the current user has at least the specified role.
// It reads the TenantUser association from the service and injects Role/Permissions into the context.
//
// Role hierarchy: system_admin > tenant_admin > editor > viewer
//
// Usage:
//
//	router.Use(middleware.RequireRole(tuService, types.RoleTenantAdmin))
func RequireRole(tuService interfaces.TenantUserService, minRole types.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Get tenant ID from context
		tenantID, ok := ctx.Value(types.TenantIDContextKey).(uint64)
		if !ok || tenantID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: tenant context missing"})
			c.Abort()
			return
		}

		// Get user ID from context
		userID, ok := ctx.Value(types.UserIDContextKey).(string)
		if !ok || userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: user context missing"})
			c.Abort()
			return
		}

		// Look up the tenant-user association
		tu, err := tuService.GetTenantUser(ctx, tenantID, userID)
		if err != nil || tu == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: not a member of this tenant"})
			c.Abort()
			return
		}

		// Check role hierarchy
		if !hasMinRole(tu.Role, minRole) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: insufficient role"})
			c.Abort()
			return
		}

		// Inject role and permissions into context for downstream use
		perms := []string(tu.Permissions)
		c.Set("role", tu.Role)
		c.Set("permissions", perms)
		c.Request = c.Request.WithContext(
			context.WithValue(
				context.WithValue(ctx, types.RoleContextKey, tu.Role),
				types.PermissionsContextKey, perms,
			),
		)
		c.Next()
	}
}

// RequirePermission creates a middleware that checks for a specific permission.
// The user must have already gone through RequireRole (or similar) which sets permissions in context.
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		perms, ok := c.Get("permissions")
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: permissions not loaded"})
			c.Abort()
			return
		}
		permList, ok := perms.([]string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: invalid permissions format"})
			c.Abort()
			return
		}

		if !hasPermissionStr(permList, perm) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: missing permission " + perm})
			c.Abort()
			return
		}
		c.Next()
	}
}

// roleHierarchy defines the numeric level for each role
var roleHierarchy = map[types.Role]int{
	types.RoleSystemAdmin: 4,
	types.RoleTenantAdmin: 3,
	types.RoleEditor:      2,
	types.RoleViewer:      1,
}

// hasMinRole checks if the user's role meets the minimum required role
func hasMinRole(userRole, minRole types.Role) bool {
	userLevel, ok := roleHierarchy[userRole]
	if !ok {
		return false
	}
	minLevel, ok := roleHierarchy[minRole]
	if !ok {
		return false
	}
	return userLevel >= minLevel
}

// hasPermissionStr checks if the permission list contains the required permission
// Supports wildcards like "kb:*" which matches "kb:view", "kb:create", etc.
func hasPermissionStr(perms []string, required string) bool {
	for _, p := range perms {
		if p == required {
			return true
		}
		// Check wildcard: if permission ends with ":*", match any with the same prefix
		if strings.HasSuffix(p, ":*") {
			prefix := p[:len(p)-1] // e.g., "kb:" from "kb:*"
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}
