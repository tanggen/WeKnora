// Package initialization provides startup data seeding for the WeKnora system.
// It ensures the system tenant, system admin, and plan cache are initialized
// before the application starts serving requests. All operations are idempotent.
package initialization

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
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
	planRepo interfaces.PlanRepository,
	db *gorm.DB,
	redisClient *redis.Client,
) error {
	ctx := context.Background()
	logger.Infof(ctx, "[Seed] Starting system data initialization...")

	// 0. Safety: ensure required columns exist (defensive fix for missed migrations)
	ensureRequiredColumns(ctx, db)

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
	if err := seedTenantStats(ctx, db); err != nil {
		return fmt.Errorf("seed tenant stats: %w", err)
	}

	// 5. Seed plan data (defensive: ensures trial/basic/pro plans exist)
	seedPlanData(ctx, db)

	// 6. Preload plan cache to Redis (non-fatal)
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
	// The repository wraps gorm.ErrRecordNotFound as a custom "tenant not found" error,
	// so we must check both.
	if !errors.Is(err, errRecordNotFound) && !errors.Is(err, repository.ErrTenantNotFound) &&
		!strings.Contains(err.Error(), "not found") {
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

	// Only proceed if the error is a "not found" error
	if !errors.Is(err, errRecordNotFound) && !errors.Is(err, repository.ErrUserNotFound) &&
		!strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("check system admin existence: %w", err)
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

	// Only proceed if the error is a "not found" error
	if !errors.Is(err, errRecordNotFound) &&
		!strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("check tenant-user association existence: %w", err)
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
// Uses INSERT ON CONFLICT DO NOTHING to be truly idempotent: user_count is only
// set to 1 on the first run, and never incremented again on subsequent restarts.
func seedTenantStats(ctx context.Context, db *gorm.DB) error {
	// INSERT ON CONFLICT DO NOTHING — returns 0 rows affected if already exists
	result := db.WithContext(ctx).Exec(
		`INSERT INTO tenant_stats (tenant_id, user_count, plan_id, storage_quota_bytes, updated_at)
		 VALUES (?, 1, 'trial', 10737418240, NOW())
		 ON CONFLICT (tenant_id) DO NOTHING`,
		systemTenantID,
	)
	if result.Error != nil {
		return fmt.Errorf("insert tenant_stats: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		logger.Infof(ctx, "[Seed] System tenant stats created (user_count=1)")
	} else {
		logger.Infof(ctx, "[Seed] System tenant stats already exists, skipping")
	}

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

// seedPlanData defensively inserts the default plans if they don't exist.
// This ensures the system has trial/basic/pro plans even if migration 000063 was skipped.
func seedPlanData(ctx context.Context, db *gorm.DB) {
	// Check if any plans exist
	var count int64
	db.WithContext(ctx).Model(&types.Plan{}).Count(&count)
	if count > 0 {
		logger.Infof(ctx, "[Seed] Plans already exist (%d), skipping", count)
		return
	}

	logger.Infof(ctx, "[Seed] Seeding default plan data...")

	// Insert plans (ON CONFLICT DO NOTHING for idempotency)
	db.WithContext(ctx).Exec(`
		INSERT INTO plans (id, name, description, is_trial, trial_days, is_active, sort_order, created_at, updated_at)
		VALUES
			('trial', '体验版', '免费体验3天，感受AI知识库的完整功能', true, 3, true, 1, NOW(), NOW()),
			('basic', '基础版', '适合小团队，管理少量知识库', false, 0, true, 2, NOW(), NOW()),
			('pro', '专业版', '适合中型团队，大容量存储与更多Token', false, 0, true, 3, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING
	`)

	// Insert plan configs
	db.WithContext(ctx).Exec(`
		INSERT INTO plan_configs (
			plan_id, max_users, max_knowledge_bases, max_knowledge_per_kb,
			max_chunks_per_knowledge, storage_quota_bytes, token_quota_monthly,
			max_agents, price_monthly_cny, price_yearly_cny, features, marketing_features, updated_at
		) VALUES
		('trial', 1, 5, 100, 1000, 524288000, 50000, 1, 0, 0,
		 '["chat","search","web_search"]', '["体验3天完整功能","支持1个用户","5个知识库","50MB存储空间","每月5万Token"]', NOW()),
		('basic', 5, 20, 1000, 10000, 5368709120, 500000, 5, 9900, 95040,
		 '["chat","search","web_search"]', '["支持5个用户","20个知识库","5GB存储空间","每月50万Token","5个Agent"]', NOW()),
		('pro', 20, 100, 10000, 50000, 10737418240, 5000000, 20, 29900, 287040,
		 '["chat","search","web_search","api_access","advanced_analytics"]',
		 '["支持20个用户","100个知识库","10GB存储空间","每月500万Token","20个Agent","API访问","高级分析"]', NOW())
		ON CONFLICT (plan_id) DO NOTHING
	`)

	logger.Infof(ctx, "[Seed] Default plan data seeded (trial, basic, pro)")
}

// ensureRequiredColumns defensively adds columns that the seed code depends on.
// This protects against migration 000080 (and future migrations) being skipped
// or failing silently at startup. All statements use IF NOT EXISTS, so they are idempotent.
func ensureRequiredColumns(ctx context.Context, db *gorm.DB) {
	stmts := []struct {
		desc string
		sql  string
	}{
		// Migration 000040: tenant_users table
		{
			desc: "tenant_users table",
			sql: `CREATE TABLE IF NOT EXISTS tenant_users (
				tenant_id BIGINT NOT NULL,
				user_id VARCHAR(36) NOT NULL,
				role VARCHAR(32) NOT NULL DEFAULT 'viewer',
				permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ,
				PRIMARY KEY (tenant_id, user_id)
			)`,
		},
		{
			desc: "idx_tenant_users_tenant_id",
			sql:  "CREATE INDEX IF NOT EXISTS idx_tenant_users_tenant_id ON tenant_users(tenant_id)",
		},
		{
			desc: "idx_tenant_users_user_id",
			sql:  "CREATE INDEX IF NOT EXISTS idx_tenant_users_user_id ON tenant_users(user_id)",
		},
		{
			desc: "idx_tenant_users_role",
			sql:  "CREATE INDEX IF NOT EXISTS idx_tenant_users_role ON tenant_users(tenant_id, role)",
		},
		// Migration 000041: users phone + password_expired
		{
			desc: "users.phone",
			sql:  "ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(20)",
		},
		{
			desc: "users.password_expired",
			sql:  "ALTER TABLE users ADD COLUMN IF NOT EXISTS password_expired BOOLEAN NOT NULL DEFAULT FALSE",
		},
		{
			desc: "idx_users_phone",
			sql:  "CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone ON users(phone) WHERE phone IS NOT NULL AND phone != ''",
		},
		// Migration 000060: plans + plan_configs tables
		{
			desc: "plans table",
			sql: `CREATE TABLE IF NOT EXISTS plans (
				id VARCHAR(32) PRIMARY KEY,
				name VARCHAR(128) NOT NULL,
				description TEXT,
				is_trial BOOLEAN NOT NULL DEFAULT FALSE,
				trial_days INT NOT NULL DEFAULT 0,
				is_active BOOLEAN NOT NULL DEFAULT TRUE,
				sort_order INT NOT NULL DEFAULT 99,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
		},
		{
			desc: "idx_plans_is_active",
			sql:  "CREATE INDEX IF NOT EXISTS idx_plans_is_active ON plans(is_active)",
		},
		{
			desc: "idx_plans_sort_order",
			sql:  "CREATE INDEX IF NOT EXISTS idx_plans_sort_order ON plans(sort_order)",
		},
		{
			desc: "plan_configs table",
			sql: `CREATE TABLE IF NOT EXISTS plan_configs (
				plan_id VARCHAR(32) PRIMARY KEY REFERENCES plans(id) ON DELETE CASCADE,
				max_users INT NOT NULL DEFAULT 5,
				max_knowledge_bases INT NOT NULL DEFAULT 20,
				max_knowledge_per_kb INT NOT NULL DEFAULT 1000,
				max_chunks_per_knowledge INT NOT NULL DEFAULT 10000,
				storage_quota_bytes BIGINT NOT NULL DEFAULT 10737418240,
				token_quota_monthly BIGINT NOT NULL DEFAULT 100000,
				max_agents INT NOT NULL DEFAULT 5,
				price_monthly_cny INT NOT NULL DEFAULT 0,
				price_yearly_cny INT NOT NULL DEFAULT 0,
				features JSONB NOT NULL DEFAULT '["chat","search"]'::jsonb,
				marketing_features JSONB NOT NULL DEFAULT '[]'::jsonb,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		},
		// Migration 000061: tenant_stats table
		{
			desc: "tenant_stats table",
			sql: `CREATE TABLE IF NOT EXISTS tenant_stats (
				tenant_id BIGINT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
				user_count INT NOT NULL DEFAULT 0,
				knowledge_base_count INT NOT NULL DEFAULT 0,
				knowledge_count INT NOT NULL DEFAULT 0,
				chunk_count BIGINT NOT NULL DEFAULT 0,
				storage_used_bytes BIGINT NOT NULL DEFAULT 0,
				token_used_total BIGINT NOT NULL DEFAULT 0,
				token_used_monthly BIGINT NOT NULL DEFAULT 0,
				token_quota_reset_at TIMESTAMPTZ,
				agent_count INT NOT NULL DEFAULT 0,
				session_count BIGINT NOT NULL DEFAULT 0,
				plan_id VARCHAR(32),
				storage_quota_bytes BIGINT NOT NULL DEFAULT 10737418240,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		},
		{
			desc: "idx_tenant_stats_plan_id",
			sql:  "CREATE INDEX IF NOT EXISTS idx_tenant_stats_plan_id ON tenant_stats(plan_id)",
		},
		// Migration 000062: tenants plan_id + trial_expires_at
		{
			desc: "tenants.plan_id",
			sql:  "ALTER TABLE tenants ADD COLUMN IF NOT EXISTS plan_id VARCHAR(32) DEFAULT 'trial'",
		},
		{
			desc: "tenants.trial_expires_at",
			sql:  "ALTER TABLE tenants ADD COLUMN IF NOT EXISTS trial_expires_at TIMESTAMPTZ",
		},
		{
			desc: "idx_tenants_plan_id",
			sql:  "CREATE INDEX IF NOT EXISTS idx_tenants_plan_id ON tenants(plan_id)",
		},
		// Migration 000080: tenants.is_system
		{
			desc: "tenants.is_system",
			sql:  "ALTER TABLE tenants ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE",
		},
		// users.can_access_all_tenants (no migration exists)
		{
			desc: "users.can_access_all_tenants",
			sql:  "ALTER TABLE users ADD COLUMN IF NOT EXISTS can_access_all_tenants BOOLEAN NOT NULL DEFAULT FALSE",
		},
	}

	for _, s := range stmts {
		if err := db.Exec(s.sql).Error; err != nil {
			logger.Warnf(ctx, "[Seed] Failed to ensure column %s: %v", s.desc, err)
		} else {
			logger.Debugf(ctx, "[Seed] Column %s ensured", s.desc)
		}
	}
}
