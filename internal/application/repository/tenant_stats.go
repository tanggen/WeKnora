package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// tenantStatsRepository implements TenantStatsRepository
type tenantStatsRepository struct {
	db *gorm.DB
}

// NewTenantStatsRepository creates a new tenant stats repository
func NewTenantStatsRepository(db *gorm.DB) interfaces.TenantStatsRepository {
	return &tenantStatsRepository{db: db}
}

// ensureStatsExists ensures a tenant_stats row exists for the given tenant.
// Uses ON CONFLICT DO NOTHING to handle concurrent initial creates.
func (r *tenantStatsRepository) ensureStatsExists(ctx context.Context, tenantID uint64) error {
	stats := &types.TenantStats{
		TenantID: tenantID,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(stats).Error
}

// GetByTenantID retrieves stats for a tenant, auto-creating the row if missing
func (r *tenantStatsRepository) GetByTenantID(ctx context.Context, tenantID uint64) (*types.TenantStats, error) {
	var stats types.TenantStats
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&stats).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Auto-create the row
			if createErr := r.ensureStatsExists(ctx, tenantID); createErr != nil {
				return nil, createErr
			}
			// Re-read the newly created row
			if err2 := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&stats).Error; err2 != nil {
				return nil, err2
			}
			return &stats, nil
		}
		return nil, err
	}
	return &stats, nil
}

// GetForQuotaCheck retrieves stats for quota checking (same as GetByTenantID but semantically distinct)
func (r *tenantStatsRepository) GetForQuotaCheck(ctx context.Context, tenantID uint64) (*types.TenantStats, error) {
	return r.GetByTenantID(ctx, tenantID)
}

// --- Atomic Increment/Decrement Operations ---

// IncrementUserCount atomically adjusts user_count
func (r *tenantStatsRepository) IncrementUserCount(ctx context.Context, tenantID uint64, delta int) error {
	if err := r.ensureStatsExists(ctx, tenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&types.TenantStats{}).
		Where("tenant_id = ?", tenantID).
		Update("user_count", gorm.Expr("user_count + ?", delta)).Error
}

// IncrementKBCount atomically adjusts knowledge_base_count
func (r *tenantStatsRepository) IncrementKBCount(ctx context.Context, tenantID uint64, delta int) error {
	if err := r.ensureStatsExists(ctx, tenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&types.TenantStats{}).
		Where("tenant_id = ?", tenantID).
		Update("knowledge_base_count", gorm.Expr("knowledge_base_count + ?", delta)).Error
}

// IncrementKnowledgeCount atomically adjusts knowledge_count
func (r *tenantStatsRepository) IncrementKnowledgeCount(ctx context.Context, tenantID uint64, delta int) error {
	if err := r.ensureStatsExists(ctx, tenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&types.TenantStats{}).
		Where("tenant_id = ?", tenantID).
		Update("knowledge_count", gorm.Expr("knowledge_count + ?", delta)).Error
}

// IncrementChunkCount atomically adjusts chunk_count
func (r *tenantStatsRepository) IncrementChunkCount(ctx context.Context, tenantID uint64, delta int64) error {
	if err := r.ensureStatsExists(ctx, tenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&types.TenantStats{}).
		Where("tenant_id = ?", tenantID).
		Update("chunk_count", gorm.Expr("chunk_count + ?", delta)).Error
}

// IncrementStorageUsed atomically adjusts storage_used_bytes
func (r *tenantStatsRepository) IncrementStorageUsed(ctx context.Context, tenantID uint64, delta int64) error {
	if err := r.ensureStatsExists(ctx, tenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&types.TenantStats{}).
		Where("tenant_id = ?", tenantID).
		Update("storage_used_bytes", gorm.Expr("storage_used_bytes + ?", delta)).Error
}

// IncrementTokenUsed atomically adjusts both token_used_total and token_used_monthly
func (r *tenantStatsRepository) IncrementTokenUsed(ctx context.Context, tenantID uint64, delta int64) error {
	if err := r.ensureStatsExists(ctx, tenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&types.TenantStats{}).
		Where("tenant_id = ?", tenantID).
		Updates(map[string]interface{}{
			"token_used_total":   gorm.Expr("token_used_total + ?", delta),
			"token_used_monthly": gorm.Expr("token_used_monthly + ?", delta),
		}).Error
}

// IncrementAgentCount atomically adjusts agent_count
func (r *tenantStatsRepository) IncrementAgentCount(ctx context.Context, tenantID uint64, delta int) error {
	if err := r.ensureStatsExists(ctx, tenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&types.TenantStats{}).
		Where("tenant_id = ?", tenantID).
		Update("agent_count", gorm.Expr("agent_count + ?", delta)).Error
}

// IncrementSessionCount atomically adjusts session_count
func (r *tenantStatsRepository) IncrementSessionCount(ctx context.Context, tenantID uint64, delta int64) error {
	if err := r.ensureStatsExists(ctx, tenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&types.TenantStats{}).
		Where("tenant_id = ?", tenantID).
		Update("session_count", gorm.Expr("session_count + ?", delta)).Error
}

// UpdatePlanSnapshot updates plan-related fields in tenant_stats
func (r *tenantStatsRepository) UpdatePlanSnapshot(ctx context.Context, tenantID uint64, planID string, storageQuota int64) error {
	if err := r.ensureStatsExists(ctx, tenantID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Model(&types.TenantStats{}).
		Where("tenant_id = ?", tenantID).
		Updates(map[string]interface{}{
			"plan_id":             planID,
			"storage_quota_bytes": storageQuota,
		}).Error
}

// ResetTokenQuotaMonthly resets token_used_monthly to 0 for all tenants
func (r *tenantStatsRepository) ResetTokenQuotaMonthly(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&types.TenantStats{}).
		Where("token_used_monthly > 0").
		Update("token_used_monthly", 0)
	return result.RowsAffected, result.Error
}

// ListAll returns paginated admin tenant stats with optional keyword search
func (r *tenantStatsRepository) ListAll(ctx context.Context, keyword string, sortBy string, order string, page, pageSize int) ([]*types.AdminTenantStatsItem, int64, error) {
	// Validate sort field (allowlist to prevent SQL injection)
	allowedSortFields := map[string]string{
		"user_count":         "ts.user_count",
		"storage_used_bytes": "ts.storage_used_bytes",
		"token_used":         "ts.token_used_monthly",
		"created_at":         "t.created_at",
	}
	sortCol, ok := allowedSortFields[sortBy]
	if !ok {
		sortCol = "ts.token_used_monthly" // default
	}
	if order != "asc" {
		order = "desc"
	}

	query := r.db.WithContext(ctx).
		Table("tenant_stats AS ts").
		Select("ts.tenant_id, t.name AS tenant_name, COALESCE(p.name, ts.plan_id) AS plan_name, " +
			"ts.user_count, ts.storage_used_bytes, ts.token_used_monthly AS token_used, " +
			"ts.token_used_monthly, t.status, t.created_at").
		Joins("JOIN tenants AS t ON ts.tenant_id = t.id").
		Joins("LEFT JOIN plans AS p ON ts.plan_id = p.id")

	if keyword != "" {
		query = query.Where("t.name ILIKE ?", "%"+escapeLikeKeyword(keyword)+"%")
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch paginated results
	offset := (page - 1) * pageSize

	// We need to handle the query properly - use raw SQL for complex joins
	// Use a simpler approach with direct table reads
	// Query tenant_stats with tenant info
	var tenantStats []types.TenantStats
	if err := r.db.WithContext(ctx).
		Offset(offset).Limit(pageSize).
		Order(fmt.Sprintf("%s %s", sortCol, order)).
		Find(&tenantStats).Error; err != nil {
		return nil, 0, err
	}

	// For each tenant stat, fetch tenant and plan info
	items := make([]*types.AdminTenantStatsItem, 0, len(tenantStats))
	for _, ts := range tenantStats {
		item := &types.AdminTenantStatsItem{
			TenantID:          ts.TenantID,
			UserCount:         ts.UserCount,
			StorageUsedBytes:  ts.StorageUsedBytes,
			TokenUsed:         ts.TokenUsedMonthly,
			TokenQuotaMonthly: ts.TokenUsedMonthly,
		}

		// Fetch tenant info
		var tenant types.Tenant
		if err := r.db.WithContext(ctx).Where("id = ?", ts.TenantID).First(&tenant).Error; err == nil {
			item.TenantName = tenant.Name
			item.IsActive = tenant.Status == "active"
			item.CreatedAt = tenant.CreatedAt
			if keyword != "" && !strings.Contains(strings.ToLower(tenant.Name), strings.ToLower(keyword)) {
				continue
			}
		}

		// Fetch plan info
		var plan types.Plan
		if err := r.db.WithContext(ctx).Where("id = ?", ts.PlanID).First(&plan).Error; err == nil {
			item.PlanName = plan.Name
		} else {
			item.PlanName = ts.PlanID
		}

		items = append(items, item)
	}

	return items, total, nil
}

// GetOverview returns the global dashboard summary
func (r *tenantStatsRepository) GetOverview(ctx context.Context) (*types.AdminOverviewStats, error) {
	overview := &types.AdminOverviewStats{}

	// Total tenant count
	var totalTenants int64
	r.db.WithContext(ctx).Model(&types.Tenant{}).Count(&totalTenants)
	overview.TotalTenants = int(totalTenants)

	// Active tenants
	var activeTenants int64
	r.db.WithContext(ctx).Model(&types.Tenant{}).Where("status = ?", "active").Count(&activeTenants)
	overview.ActiveTenants = int(activeTenants)

	// Sum user counts from tenant_stats
	r.db.WithContext(ctx).Model(&types.TenantStats{}).
		Select("COALESCE(SUM(user_count), 0)").
		Scan(&overview.TotalUsers)

	// Sum knowledge base counts
	r.db.WithContext(ctx).Model(&types.TenantStats{}).
		Select("COALESCE(SUM(knowledge_base_count), 0)").
		Scan(&overview.TotalKnowledgeBases)

	// Sum storage used
	r.db.WithContext(ctx).Model(&types.TenantStats{}).
		Select("COALESCE(SUM(storage_used_bytes), 0)").
		Scan(&overview.TotalStorageUsedBytes)

	// Sum token used total
	r.db.WithContext(ctx).Model(&types.TenantStats{}).
		Select("COALESCE(SUM(token_used_total), 0)").
		Scan(&overview.TotalTokenUsedAllTime)

	// Trials: tenants on trial plan created within last 3 days and still active
	var trialsActive int64
	r.db.WithContext(ctx).Model(&types.Tenant{}).
		Where("plan_id IN (SELECT id FROM plans WHERE is_trial = true)").
		Where("status = ?", "active").
		Where("created_at >= NOW() - INTERVAL '3 days'").
		Count(&trialsActive)
	overview.TrialsActive = int(trialsActive)

	// Trials converted: tenants that were on trial but now on non-trial plans
	var trialsConverted int64
	r.db.WithContext(ctx).Model(&types.Tenant{}).
		Where("plan_id IN (SELECT id FROM plans WHERE is_trial = false)").
		Where("trial_expires_at IS NOT NULL").
		Count(&trialsConverted)
	overview.TrialsConverted = int(trialsConverted)

	return overview, nil
}

// Ensure tenantStatsRepository implements TenantStatsRepository
var _ interfaces.TenantStatsRepository = (*tenantStatsRepository)(nil)
