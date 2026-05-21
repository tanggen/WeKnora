// Package initialization provides startup data seeding for the WeKnora system.
// It ensures the system tenant, system admin, and plan cache are initialized
// before the application starts serving requests. All operations are idempotent.
package initialization

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// errRecordNotFound is the GORM error returned when a record is not found.
var errRecordNotFound = gorm.ErrRecordNotFound

// --- Constants ---

const (
	// System tenant
	systemTenantID   uint64 = 1
	systemTenantName        = "系统管理"
	systemTenantDesc        = "系统默认管理租户"

	// System admin
	systemAdminUserID   = "sys-admin-001"
	systemAdminUsername = "admin"
	systemAdminEmail    = "admin@system.local"
	systemAdminPassword = "admin_123456"
)

// SeedSystemData initializes the system tenant, admin user, and plan cache.
// It should be called once at application startup after database migrations.
// All operations are idempotent: existing data is never overwritten.
func SeedSystemData(
	tenantRepo interfaces.TenantRepository,
	userRepo interfaces.UserRepository,
	tenantUserRepo interfaces.TenantUserRepository,
	tenantStatsRepo interfaces.TenantStatsRepository,
	planRepo interfaces.PlanRepository,
	redisClient *redis.Client,
) error {
	ctx := context.Background()
	logger.Infof(ctx, "[Seed] Starting system data initialization...")

	// 1. System tenant (ID=1, is_system=true)
	if err := seedSystemTenant(ctx, tenantRepo); err != nil {
		return fmt.Errorf("seed system tenant: %w", err)
	}

	// 2. System admin user (username=admin, password_expired=true)
	if err := seedSystemAdmin(ctx, userRepo); err != nil {
		return fmt.Errorf("seed system admin: %w", err)
	}

	// 3. tenant_users association (role=system_admin)
	if err := seedTenantUser(ctx, tenantUserRepo); err != nil {
		return fmt.Errorf("seed tenant user: %w", err)
	}

	// 4. tenant_stats initialization with plan snapshot
	if err := seedTenantStats(ctx, tenantStatsRepo); err != nil {
		return fmt.Errorf("seed tenant stats: %w", err)
	}

	// 5. Preload plan cache to Redis (non-fatal)
	if err := preloadPlanCache(ctx, planRepo, redisClient); err != nil {
		logger.Warnf(ctx, "[Seed] Failed to preload plan cache: %v", err)
	}

	logger.Infof(ctx, "[Seed] System data initialization completed successfully")
	return nil
}

// seedSystemTenant creates the system tenant (ID=1) if it doesn't exist.
func seedSystemTenant(ctx context.Context, repo interfaces.TenantRepository) error {
	_, err := repo.GetTenantByID(ctx, systemTenantID)
	if err == nil {
		logger.Infof(ctx, "[Seed] System tenant (ID=%d) already exists, skipping", systemTenantID)
		return nil
	}

	// Only proceed if the error is a "not found" error
	if !errors.Is(err, errRecordNotFound) {
		return fmt.Errorf("check system tenant existence: %w", err)
	}

	// Generate a random API key for the system tenant
	apiKey, err := secutils.GenerateSecureRandomString(32)
	if err != nil {
		return fmt.Errorf("generate api key: %w", err)
	}

	tenant := &types.Tenant{
		ID:          systemTenantID,
		Name:        systemTenantName,
		Description: systemTenantDesc,
		APIKey:      apiKey,
		Status:      "active",
		IsSystem:    true,
		PlanID:      "trial",
		Business:    "系统管理",
		RetrieverEngines: types.RetrieverEngines{
			Engines: types.GetDefaultRetrieverEngines(),
		},
		StorageQuota: 10737418240, // 10 GB
	}

	if err := repo.CreateTenant(ctx, tenant); err != nil {
		return fmt.Errorf("create system tenant: %w", err)
	}
	logger.Infof(ctx, "[Seed] System tenant (ID=%d) created successfully", systemTenantID)
	return nil
}

// seedSystemAdmin creates the system admin user if it doesn't exist.
func seedSystemAdmin(ctx context.Context, repo interfaces.UserRepository) error {
	_, err := repo.GetUserByID(ctx, systemAdminUserID)
	if err == nil {
		logger.Infof(ctx, "[Seed] System admin user already exists, skipping")
		return nil
	}

	// Hash the default password at runtime
	passwordHash, err := secutils.HashPassword(systemAdminPassword)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	user := &types.User{
		ID:                  systemAdminUserID,
		Username:            systemAdminUsername,
		Email:               systemAdminEmail,
		PasswordHash:        passwordHash,
		PasswordExpired:     true, // First login must change password (HTTP 423)
		IsActive:            true,
		CanAccessAllTenants: true,
	}

	if err := repo.CreateUser(ctx, user); err != nil {
		return fmt.Errorf("create system admin: %w", err)
	}
	logger.Infof(ctx, "[Seed] System admin user created successfully")
	return nil
}

// seedTenantUser creates the tenant_users association for the system admin.
func seedTenantUser(ctx context.Context, repo interfaces.TenantUserRepository) error {
	_, err := repo.GetByTenantAndUser(ctx, systemTenantID, systemAdminUserID)
	if err == nil {
		logger.Infof(ctx, "[Seed] System tenant-user association already exists, skipping")
		return nil
	}

	tu := &types.TenantUser{
		TenantID:    systemTenantID,
		UserID:      systemAdminUserID,
		Role:        types.RoleSystemAdmin,
		Permissions: types.DefaultPermissions(types.RoleSystemAdmin),
	}

	if err := repo.Create(ctx, tu); err != nil {
		return fmt.Errorf("create tenant-user association: %w", err)
	}
	logger.Infof(ctx, "[Seed] System tenant-user association created successfully")
	return nil
}

// seedTenantStats ensures the tenant_stats row exists and sets the plan snapshot.
func seedTenantStats(ctx context.Context, repo interfaces.TenantStatsRepository) error {
	// GetByTenantID auto-creates the row if missing
	if _, err := repo.GetByTenantID(ctx, systemTenantID); err != nil {
		return fmt.Errorf("get/create tenant stats: %w", err)
	}

	// Set plan snapshot for the system tenant
	if err := repo.UpdatePlanSnapshot(ctx, systemTenantID, "trial", 10737418240); err != nil {
		logger.Warnf(ctx, "[Seed] Failed to update plan snapshot for system tenant: %v", err)
	}

	// Bump user_count to 1 (system admin)
	if err := repo.IncrementUserCount(ctx, systemTenantID, 1); err != nil {
		logger.Warnf(ctx, "[Seed] Failed to increment user_count: %v", err)
	}

	logger.Infof(ctx, "[Seed] System tenant stats initialized")
	return nil
}

// preloadPlanCache loads all active plan configurations into Redis for fast retrieval.
func preloadPlanCache(ctx context.Context, planRepo interfaces.PlanRepository, redisClient *redis.Client) error {
	if redisClient == nil {
		logger.Infof(ctx, "[Seed] Redis not available, skipping plan cache preload")
		return nil
	}

	plans, err := planRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list plans: %w", err)
	}

	if len(plans) == 0 {
		logger.Warnf(ctx, "[Seed] No plans found in database, skipping plan cache preload")
		return nil
	}

	// Preload individual plan configs
	for _, pw := range plans {
		configMap := map[string]interface{}{
			"plan_id":                 pw.ID,
			"name":                    pw.Name,
			"description":             pw.Description,
			"is_trial":                pw.IsTrial,
			"trial_days":              pw.TrialDays,
			"max_users":               pw.Config.MaxUsers,
			"max_knowledge_bases":     pw.Config.MaxKnowledgeBases,
			"max_knowledge_per_kb":    pw.Config.MaxKnowledgePerKB,
			"max_chunks_per_knowledge": pw.Config.MaxChunksPerKnowledge,
			"storage_quota_bytes":     pw.Config.StorageQuotaBytes,
			"token_quota_monthly":     pw.Config.TokenQuotaMonthly,
			"max_agents":              pw.Config.MaxAgents,
			"price_monthly_cny":       pw.Config.PriceMonthlyCNY,
			"price_yearly_cny":        pw.Config.PriceYearlyCNY,
			"features":                pw.Config.Features,
			"marketing_features":      pw.Config.MarketingFeatures,
		}

		configBytes, err := json.Marshal(configMap)
		if err != nil {
			logger.Warnf(ctx, "[Seed] Failed to marshal plan config for %s: %v", pw.ID, err)
			continue
		}

		key := fmt.Sprintf("plan:%s:config", pw.ID)
		redisClient.Set(ctx, key, configBytes, 1*time.Hour)
		logger.Debugf(ctx, "[Seed] Plan cache preloaded: %s", pw.ID)
	}

	// Preload active plan list
	var activePlans []map[string]interface{}
	for _, pw := range plans {
		if pw.IsActive {
			activePlans = append(activePlans, map[string]interface{}{
				"plan_id":     pw.ID,
				"name":        pw.Name,
				"description": pw.Description,
				"is_trial":    pw.IsTrial,
				"trial_days":  pw.TrialDays,
				"sort_order":  pw.SortOrder,
			})
		}
	}

	activeBytes, err := json.Marshal(activePlans)
	if err != nil {
		return fmt.Errorf("marshal active plan list: %w", err)
	}
	redisClient.Set(ctx, "plan:list:active", activeBytes, 30*time.Minute)

	logger.Infof(ctx, "[Seed] Plan cache preloaded (%d plans) to Redis", len(plans))
	return nil
}
