package cron

import (
	"context"

	"github.com/Tencent/WeKnora/internal/logger"
)

// runSyncTokenCounter syncs Redis token usage counters to the database.
// Called every 10 minutes. Uses atomic GetDel to read-and-reset Redis counters,
// then persists the delta to tenant_stats.
func (s *Scheduler) runSyncTokenCounter() {
	ctx := context.Background()
	logger.Debug(ctx, "[Cron] Starting token counter sync from Redis to DB...")

	if s.quotaSvc == nil {
		logger.Warn(ctx, "[Cron] TokenQuotaService not available, skipping sync")
		return
	}

	if err := s.quotaSvc.SyncTokenCounterToDB(ctx); err != nil {
		logger.Errorf(ctx, "[Cron] Token counter sync failed: %v", err)
	}
}
