package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/redis/go-redis/v9"
)

// MenuService provides menu tree building, permission resolution, and Redis caching
type MenuService struct {
	tuRepo     interfaces.TenantUserRepository
	tenantRepo interfaces.TenantRepository
	planRepo   interfaces.PlanRepository
	redis      *redis.Client
}

// NewMenuService creates a new menu service
func NewMenuService(
	tuRepo interfaces.TenantUserRepository,
	tenantRepo interfaces.TenantRepository,
	planRepo interfaces.PlanRepository,
	redis *redis.Client,
) *MenuService {
	return &MenuService{
		tuRepo:     tuRepo,
		tenantRepo: tenantRepo,
		planRepo:   planRepo,
		redis:      redis,
	}
}

const (
	menuCachePrefix     = "menu:"
	permCachePrefix     = "perm:"
	menuCacheTTL        = 5 * time.Minute
	permCacheTTL        = 5 * time.Minute
)

// menuCacheKey returns the Redis key for menu cache
func menuCacheKey(tenantID uint64, userID string) string {
	return fmt.Sprintf("%s%d:%s", menuCachePrefix, tenantID, userID)
}

// permCacheKey returns the Redis key for permissions cache
func permCacheKey(tenantID uint64, userID string) string {
	return fmt.Sprintf("%s%d:%s", permCachePrefix, tenantID, userID)
}

// ──────────────────────────────────────────────
//  Public API
// ──────────────────────────────────────────────

// MenuResponse is the API response for GET /menus
type MenuResponse struct {
	Menus    []*config.MenuItem `json:"menus"`
	CacheTTL int                `json:"cache_ttl"`
}

// PermissionResponse is the API response for GET /auth/permissions
type PermissionResponse struct {
	UserID          string     `json:"user_id"`
	TenantID        uint64     `json:"tenant_id"`
	Role            types.Role `json:"role"`
	Permissions     []string   `json:"permissions"`
	PlanID          string     `json:"plan_id"`
	PlanFeatures    []string   `json:"plan_features"`
	IsTrial         bool       `json:"is_trial"`
	TrialExpiresAt  *time.Time `json:"trial_expires_at,omitempty"`
}

// GetUserMenus returns the menu tree visible to the given user
func (s *MenuService) GetUserMenus(ctx context.Context, tenantID uint64, userID string) (*MenuResponse, error) {
	// 1. Try Redis cache
	if cached, ok := s.getMenuFromCache(ctx, tenantID, userID); ok {
		return cached, nil
	}

	// 2. Resolve user permissions and plan features
	resolved, err := s.resolveUserContext(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	// 3. Build menu tree
	menus := BuildMenuTree(config.DefaultMenuItems, resolved.Permissions, resolved.PlanFeatures)

	resp := &MenuResponse{
		Menus:    menus,
		CacheTTL: int(menuCacheTTL.Seconds()),
	}

	// 4. Cache to Redis
	s.setMenuToCache(ctx, tenantID, userID, resp)

	return resp, nil
}

// GetUserPermissions returns the current user's permissions and plan info
func (s *MenuService) GetUserPermissions(ctx context.Context, tenantID uint64, userID string) (*PermissionResponse, error) {
	// 1. Try Redis cache
	if cached, ok := s.getPermFromCache(ctx, tenantID, userID); ok {
		return cached, nil
	}

	// 2. Resolve
	resolved, err := s.resolveUserContext(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	// 3. Determine trial status
	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	isTrial := false
	var trialExpiresAt *time.Time
	if tenant.TrialExpiresAt != nil && tenant.TrialExpiresAt.After(time.Now()) {
		isTrial = true
		trialExpiresAt = tenant.TrialExpiresAt
	}

	resp := &PermissionResponse{
		UserID:         userID,
		TenantID:       tenantID,
		Role:           resolved.Role,
		Permissions:    resolved.Permissions,
		PlanID:         tenant.PlanID,
		PlanFeatures:   resolved.PlanFeatures,
		IsTrial:        isTrial,
		TrialExpiresAt: trialExpiresAt,
	}

	// 4. Cache
	s.setPermToCache(ctx, tenantID, userID, resp)

	return resp, nil
}

// InvalidateCache removes the menu and permission caches for a specific user
func (s *MenuService) InvalidateCache(ctx context.Context, tenantID uint64, userID string) {
	if s.redis == nil {
		return
	}
	keys := []string{menuCacheKey(tenantID, userID), permCacheKey(tenantID, userID)}
	if err := s.redis.Del(ctx, keys...).Err(); err != nil {
		logger.Warnf(ctx, "[Menu] Failed to invalidate cache for tenant %d user %s: %v", tenantID, userID, err)
	} else {
		logger.Debugf(ctx, "[Menu] Invalidated cache for tenant %d user %s", tenantID, userID)
	}
}

// InvalidateAllTenantCache removes all menu/permission caches for all users in a tenant
func (s *MenuService) InvalidateAllTenantCache(ctx context.Context, tenantID uint64) {
	if s.redis == nil {
		return
	}
	patterns := []string{
		fmt.Sprintf("%s%d:*", menuCachePrefix, tenantID),
		fmt.Sprintf("%s%d:*", permCachePrefix, tenantID),
	}
	for _, pattern := range patterns {
		keys, err := s.redis.Keys(ctx, pattern).Result()
		if err != nil {
			logger.Warnf(ctx, "[Menu] Failed to scan cache keys with pattern %s: %v", pattern, err)
			continue
		}
		if len(keys) == 0 {
			continue
		}
		if err := s.redis.Del(ctx, keys...).Err(); err != nil {
			logger.Warnf(ctx, "[Menu] Failed to delete cache keys: %v", err)
		} else {
			logger.Infof(ctx, "[Menu] Invalidated %d cache keys for tenant %d", len(keys), tenantID)
		}
	}
}

// ──────────────────────────────────────────────
//  Resolved context
// ──────────────────────────────────────────────

type resolvedContext struct {
	Role         types.Role
	Permissions  []string
	PlanFeatures []string
}

// resolveUserContext loads tenant_user + tenant + plan data
func (s *MenuService) resolveUserContext(ctx context.Context, tenantID uint64, userID string) (*resolvedContext, error) {
	// 1. Get tenant-user association
	tu, err := s.tuRepo.GetByTenantAndUser(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("get tenant user: %w", err)
	}

	// 2. Get tenant
	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	// 3. Check trial expiry
	if isTrialExpired(tenant) {
		// Trial expired: only return plan-info permissions
		return &resolvedContext{
			Role:         tu.Role,
			Permissions:  []string{},
			PlanFeatures: []string{},
		}, nil
	}

	// 4. Get plan features
	planFeatures := []string{}
	if tenant.PlanID != "" {
		plan, err := s.planRepo.GetByID(ctx, tenant.PlanID)
		if err == nil && plan != nil {
			planFeatures = []string(plan.Config.Features)
		}
	}

	return &resolvedContext{
		Role:         tu.Role,
		Permissions:  []string(tu.Permissions),
		PlanFeatures: planFeatures,
	}, nil
}

// ──────────────────────────────────────────────
//  Redis cache helpers
// ──────────────────────────────────────────────

func (s *MenuService) getMenuFromCache(ctx context.Context, tenantID uint64, userID string) (*MenuResponse, bool) {
	if s.redis == nil {
		return nil, false
	}
	data, err := s.redis.Get(ctx, menuCacheKey(tenantID, userID)).Bytes()
	if err != nil {
		return nil, false
	}
	var resp MenuResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		logger.Warnf(ctx, "[Menu] Failed to unmarshal menu cache: %v", err)
		return nil, false
	}
	return &resp, true
}

func (s *MenuService) setMenuToCache(ctx context.Context, tenantID uint64, userID string, resp *MenuResponse) {
	if s.redis == nil {
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		logger.Warnf(ctx, "[Menu] Failed to marshal menu cache: %v", err)
		return
	}
	if err := s.redis.Set(ctx, menuCacheKey(tenantID, userID), data, menuCacheTTL).Err(); err != nil {
		logger.Warnf(ctx, "[Menu] Failed to set menu cache: %v", err)
	}
}

func (s *MenuService) getPermFromCache(ctx context.Context, tenantID uint64, userID string) (*PermissionResponse, bool) {
	if s.redis == nil {
		return nil, false
	}
	data, err := s.redis.Get(ctx, permCacheKey(tenantID, userID)).Bytes()
	if err != nil {
		return nil, false
	}
	var resp PermissionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		logger.Warnf(ctx, "[Menu] Failed to unmarshal perm cache: %v", err)
		return nil, false
	}
	return &resp, true
}

func (s *MenuService) setPermToCache(ctx context.Context, tenantID uint64, userID string, resp *PermissionResponse) {
	if s.redis == nil {
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		logger.Warnf(ctx, "[Menu] Failed to marshal perm cache: %v", err)
		return
	}
	if err := s.redis.Set(ctx, permCacheKey(tenantID, userID), data, permCacheTTL).Err(); err != nil {
		logger.Warnf(ctx, "[Menu] Failed to set perm cache: %v", err)
	}
}

// ──────────────────────────────────────────────
//  Menu tree building (pure functions)
// ──────────────────────────────────────────────

// BuildMenuTree filters items by permissions and features, then builds a tree.
// If trial is expired, callers should pass empty perms/features and only plan-info
// will be returned (as it has no RequiredPermission or RequiredFeature).
func BuildMenuTree(allItems []config.MenuItem, userPerms []string, planFeatures []string) []*config.MenuItem {
	permSet := toSet(userPerms)
	featureSet := toSet(planFeatures)

	// Step 1: Filter leaf nodes
	itemMap := make(map[string]*config.MenuItem)
	for i := range allItems {
		item := &allItems[i]

		// Permission check
		if item.RequiredPermission != "" && !hasPermissionMatch(permSet, item.RequiredPermission) {
			continue
		}
		// Plan feature check
		if item.RequiredFeature != "" && !featureSet[item.RequiredFeature] {
			continue
		}

		copyItem := *item
		copyItem.Children = nil
		itemMap[item.Key] = &copyItem
	}

	// Step 2: Build tree
	var roots []*config.MenuItem
	for _, item := range itemMap {
		if item.ParentKey == "" {
			roots = append(roots, item)
		} else {
			if parent, ok := itemMap[item.ParentKey]; ok {
				parent.Children = append(parent.Children, item)
			}
		}
	}

	// Step 3: Remove empty parent nodes (those with no visible children)
	roots = pruneEmptyParents(roots)

	// Step 4: Sort by sort_order
	sortMenuTree(roots)

	return roots
}

// pruneEmptyParents recursively removes empty parent nodes
func pruneEmptyParents(items []*config.MenuItem) []*config.MenuItem {
	result := make([]*config.MenuItem, 0, len(items))
	for _, item := range items {
		// Process children first
		item.Children = pruneEmptyParents(item.Children)
		// Keep the item if it's a leaf (no children expected) or has visible children
		// A parent with children defined in DefaultMenuItems but all filtered out → hide
		if hasChildrenInDefinition(item.Key) && len(item.Children) == 0 {
			continue
		}
		result = append(result, item)
	}
	return result
}

// hasChildrenInDefinition checks if this key has any child items in the default definition
func hasChildrenInDefinition(key string) bool {
	for _, item := range config.DefaultMenuItems {
		if item.ParentKey == key {
			return true
		}
	}
	return false
}

// sortMenuTree recursively sorts menu items by sort_order
func sortMenuTree(items []*config.MenuItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].SortOrder < items[j].SortOrder
	})
	for _, item := range items {
		sortMenuTree(item.Children)
	}
}

// isTrialExpired checks if the tenant's trial has expired
func isTrialExpired(tenant *types.Tenant) bool {
	if tenant.TrialExpiresAt == nil {
		return false
	}
	return time.Now().After(*tenant.TrialExpiresAt)
}

// toSet converts a slice of strings to a set (map[string]bool)
func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// hasPermissionMatch checks if the permission set grants a specific permission.
// Supports wildcards: e.g. "kb:*" in the set matches "kb:create", "kb:edit", etc.
func hasPermissionMatch(permSet map[string]bool, required string) bool {
	if permSet[required] {
		return true
	}
	// Check wildcard prefixes
	for p := range permSet {
		if strings.HasSuffix(p, ":*") {
			prefix := p[:len(p)-1] // "kb:" from "kb:*"
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}
