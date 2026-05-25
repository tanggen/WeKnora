package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/redis/go-redis/v9"
)

// TokenQuotaService provides token quota checking and recording for LLM calls.
// Uses Redis for fast atomic counters with DB fallback.
type TokenQuotaService struct {
	planRepo  interfaces.PlanRepository
	statsRepo interfaces.TenantStatsRepository
	redis     *redis.Client
}

// NewTokenQuotaService creates a new token quota service
func NewTokenQuotaService(
	planRepo interfaces.PlanRepository,
	statsRepo interfaces.TenantStatsRepository,
	redis *redis.Client,
) *TokenQuotaService {
	return &TokenQuotaService{
		planRepo:  planRepo,
		statsRepo: statsRepo,
		redis:     redis,
	}
}

const (
	tokenUsedMonthlyKeyFmt = "tenant:%d:token_used_monthly"
	tokenUsedDailyKeyFmt   = "tenant:%d:token_used:daily:%s"
)

// CheckQuota checks whether the estimated token usage would exceed the monthly limit.
// Returns nil if within quota, or an error if exceeded.
// When quota is 0 (unlimited), always returns nil.
func (s *TokenQuotaService) CheckQuota(ctx context.Context, tenantID uint64, estimatedTokens int) error {
	// Get plan config for this tenant
	stats, err := s.statsRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("get tenant stats: %w", err)
	}

	if stats.PlanID == "" {
		return nil // no plan assigned, skip check
	}

	plan, err := s.planRepo.GetByID(ctx, stats.PlanID)
	if err != nil {
		return fmt.Errorf("get plan: %w", err)
	}

	quota := plan.Config.TokenQuotaMonthly
	if quota == 0 {
		return nil // unlimited
	}

	// Read current usage from Redis
	used, err := s.getCurrentUsage(ctx, tenantID)
	if err != nil {
		logger.Warnf(ctx, "[TokenQuota] Redis read failed, falling back to DB: %v", err)
		// Fallback to DB
		used = stats.TokenUsedMonthly
	}

	proposed := used + int64(estimatedTokens)
	if proposed > quota {
		resetAt := ""
		if stats.TokenQuotaResetAt != nil {
			resetAt = stats.TokenQuotaResetAt.Format(time.RFC3339)
		}
		logger.Warnf(ctx, "[TokenQuota] Quota exceeded for tenant %d: used=%d estimated=%d quota=%d",
			tenantID, used, estimatedTokens, quota)
		return errors.NewTokenQuotaExceededError(used, quota, resetAt)
	}

	return nil
}

// RecordUsage records actual token consumption after an LLM call completes.
// Increments the monthly counter (and optionally daily counter) in Redis.
// Falls back to DB increment on Redis failure.
func (s *TokenQuotaService) RecordUsage(ctx context.Context, tenantID uint64, actualTokens int) {
	if actualTokens <= 0 {
		return
	}
	delta := int64(actualTokens)

	// Try Redis first
	if s.redis != nil {
		monthlyKey := fmt.Sprintf(tokenUsedMonthlyKeyFmt, tenantID)

		// Set TTL to next month's 1st if key doesn't exist
		if err := s.ensureMonthlyKeyTTL(ctx, monthlyKey); err != nil {
			logger.Warnf(ctx, "[TokenQuota] Failed to set monthly key TTL: %v", err)
		}

		if err := s.redis.IncrBy(ctx, monthlyKey, delta).Err(); err != nil {
			logger.Warnf(ctx, "[TokenQuota] Redis IncrBy failed for tenant %d, falling back to DB: %v", tenantID, err)
			s.recordToDB(ctx, tenantID, delta)
			return
		}

		// Daily counter (best-effort)
		dailyKey := fmt.Sprintf(tokenUsedDailyKeyFmt, tenantID, time.Now().Format("2006-01-02"))
		if err := s.redis.IncrBy(ctx, dailyKey, delta).Err(); err != nil {
			logger.Warnf(ctx, "[TokenQuota] Failed to record daily usage: %v", err)
		} else {
			// Set daily key TTL to 32 days
			s.redis.Expire(ctx, dailyKey, 32*24*time.Hour)
		}
		return
	}

	// Redis unavailable: write directly to DB
	s.recordToDB(ctx, tenantID, delta)
}

// recordToDB falls back to direct DB increment when Redis is unavailable
func (s *TokenQuotaService) recordToDB(ctx context.Context, tenantID uint64, delta int64) {
	if err := s.statsRepo.IncrementTokenUsed(ctx, tenantID, delta); err != nil {
		logger.Errorf(ctx, "[TokenQuota] CRITICAL: Failed to record token usage to DB: tenant=%d delta=%d err=%v",
			tenantID, delta, err)
	}
}

// getCurrentUsage reads the current monthly token usage from Redis
func (s *TokenQuotaService) getCurrentUsage(ctx context.Context, tenantID uint64) (int64, error) {
	if s.redis == nil {
		return 0, fmt.Errorf("redis not available")
	}
	key := fmt.Sprintf(tokenUsedMonthlyKeyFmt, tenantID)
	val, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

// ensureMonthlyKeyTTL sets TTL on the monthly counter key to expire at the 1st of next month
func (s *TokenQuotaService) ensureMonthlyKeyTTL(ctx context.Context, key string) error {
	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return nil // key already has TTL set (set when first created)
	}
	// Set TTL to next month's 1st
	return s.redis.ExpireAt(ctx, key, nextMonthFirst()).Err()
}

// nextMonthFirst returns the timestamp of the 1st day of next month at 00:00:00 UTC
func nextMonthFirst() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}

// GetRedisMonthlyKey returns the Redis key for monthly token usage of a tenant
func GetRedisMonthlyKey(tenantID uint64) string {
	return fmt.Sprintf(tokenUsedMonthlyKeyFmt, tenantID)
}

// TokenUsageDetail contains parsed token usage information from Redis + DB
type TokenUsageDetail struct {
	CurrentMonth CurrentMonthUsage `json:"current_month"`
	AllTime      AllTimeUsage      `json:"all_time"`
}

// CurrentMonthUsage represents the current month's token usage
type CurrentMonthUsage struct {
	TokenUsed    int64   `json:"token_used"`
	TokenQuota   int64   `json:"token_quota"`
	QuotaResetAt string  `json:"quota_reset_at"`
	UsagePercent float64 `json:"usage_percent"`
	Remaining    int64   `json:"remaining"`
}

// AllTimeUsage represents all-time token usage
type AllTimeUsage struct {
	TokenUsedTotal int64 `json:"token_used_total"`
}

// GetTokenUsageDetail returns detailed token usage for a tenant.
// Reads current month from Redis (with DB fallback) and all-time from DB.
func (s *TokenQuotaService) GetTokenUsageDetail(ctx context.Context, tenantID uint64) (*TokenUsageDetail, error) {
	stats, err := s.statsRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get tenant stats: %w", err)
	}

	// Get monthly usage from Redis, fallback to DB
	var monthlyUsed int64
	if s.redis != nil {
		used, redisErr := s.getCurrentUsage(ctx, tenantID)
		if redisErr == nil {
			monthlyUsed = used
		} else {
			monthlyUsed = stats.TokenUsedMonthly
		}
	} else {
		monthlyUsed = stats.TokenUsedMonthly
	}
	// Redis counter and DB counter are additive: Redis accumulates between syncs
	monthlyUsed += stats.TokenUsedMonthly

	// Get quota from plan
	var quota int64
	if stats.PlanID != "" {
		plan, err := s.planRepo.GetByID(ctx, stats.PlanID)
		if err == nil {
			quota = plan.Config.TokenQuotaMonthly
		}
	}

	resetAt := ""
	if stats.TokenQuotaResetAt != nil {
		resetAt = stats.TokenQuotaResetAt.Format(time.RFC3339)
	}

	var usagePercent float64
	var remaining int64
	if quota > 0 {
		usagePercent = float64(monthlyUsed) / float64(quota) * 100
		remaining = quota - monthlyUsed
		if remaining < 0 {
			remaining = 0
		}
	}

	return &TokenUsageDetail{
		CurrentMonth: CurrentMonthUsage{
			TokenUsed:    monthlyUsed,
			TokenQuota:   quota,
			QuotaResetAt: resetAt,
			UsagePercent: usagePercent,
			Remaining:    remaining,
		},
		AllTime: AllTimeUsage{
			TokenUsedTotal: stats.TokenUsedTotal,
		},
	}, nil
}

// ──────────────────────────────────────────────
//  Cron helpers (called by cron tasks)
// ──────────────────────────────────────────────

// SyncTokenCounterToDB atomically reads and resets Redis token counters,
// then persists delta to DB. Safe for concurrent access.
func (s *TokenQuotaService) SyncTokenCounterToDB(ctx context.Context) error {
	if s.redis == nil {
		logger.Warn(ctx, "[TokenQuota] SyncTokenCounterToDB: Redis not available, skip")
		return nil
	}

	// Scan all monthly token usage keys
	var cursor uint64
	var syncedCount int
	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, "tenant:*:token_used_monthly", 100).Result()
		if err != nil {
			return fmt.Errorf("scan redis keys: %w", err)
		}

		for _, key := range keys {
			// Parse tenant ID from key
			var tenantID uint64
			if _, err := fmt.Sscanf(key, "tenant:%d:token_used_monthly", &tenantID); err != nil {
				logger.Warnf(ctx, "[TokenQuota] Failed to parse tenant ID from key %s: %v", key, err)
				continue
			}

			// Atomic get-and-delete
			val, err := s.redis.GetDel(ctx, key).Result()
			if err == redis.Nil {
				continue
			}
			if err != nil {
				logger.Warnf(ctx, "[TokenQuota] GetDel failed for key %s: %v", key, err)
				continue
			}

			delta, err := strconv.ParseInt(val, 10, 64)
			if err != nil || delta <= 0 {
				continue
			}

			// Persist to DB
			if err := s.statsRepo.IncrementTokenUsed(ctx, tenantID, delta); err != nil {
				// Rollback: add delta back to Redis to prevent data loss
				if rbErr := s.redis.IncrBy(ctx, key, delta).Err(); rbErr != nil {
					logger.Errorf(ctx, "[TokenQuota] CRITICAL: Rollback failed for tenant %d: redis=%v db=%v",
						tenantID, rbErr, err)
				}
				logger.Errorf(ctx, "[TokenQuota] DB update failed for tenant %d, rolled back Redis: %v", tenantID, err)
				continue
			}

			syncedCount++
			logger.Debugf(ctx, "[TokenQuota] Synced tenant %d: +%d tokens", tenantID, delta)
		}

		if nextCursor == 0 {
			break
		}
		cursor = nextCursor
	}

	logger.Infof(ctx, "[TokenQuota] SyncTokenCounterToDB completed: %d tenants synced", syncedCount)
	return nil
}

// ResetTokenQuotaMonthly resets monthly counters in both DB and Redis.
func (s *TokenQuotaService) ResetTokenQuotaMonthly(ctx context.Context) error {
	// 1. Reset DB counters
	affected, err := s.statsRepo.ResetTokenQuotaMonthly(ctx)
	if err != nil {
		return fmt.Errorf("reset db: %w", err)
	}
	logger.Infof(ctx, "[TokenQuota] DB token quota reset: %d tenants affected", affected)

	// 2. Clean Redis monthly keys
	if s.redis != nil {
		var cursor uint64
		var deletedCount int
		for {
			keys, nextCursor, err := s.redis.Scan(ctx, cursor, "tenant:*:token_used_monthly", 100).Result()
			if err != nil {
				logger.Errorf(ctx, "[TokenQuota] Failed to scan Redis keys for reset: %v", err)
				break
			}
			if len(keys) > 0 {
				if err := s.redis.Del(ctx, keys...).Err(); err != nil {
					logger.Errorf(ctx, "[TokenQuota] Failed to delete Redis keys for reset: %v", err)
				}
				deletedCount += len(keys)
			}
			if nextCursor == 0 {
				break
			}
			cursor = nextCursor
		}
		logger.Infof(ctx, "[TokenQuota] Redis token keys reset: %d keys deleted", deletedCount)
	}

	return nil
}
