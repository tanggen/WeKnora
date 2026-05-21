package cron

import (
	"context"

	"github.com/Tencent/WeKnora/internal/logger"
)

// runResetTokenQuota resets the monthly token usage counter for all tenants
// and updates the reset_at timestamp to the first day of next month.
func (s *Scheduler) runResetTokenQuota() {
	ctx := context.Background()
	logger.Info(ctx, "[Cron] Starting monthly token quota reset...")

	if s.quotaSvc != nil {
		if err := s.quotaSvc.ResetTokenQuotaMonthly(ctx); err != nil {
			logger.Errorf(ctx, "[Cron] Failed to reset token quota: %v", err)
			return
		}
	} else {
		// Fallback to DB-only reset
		affected, err := s.statsRepo.ResetTokenQuotaMonthly(ctx)
		if err != nil {
			logger.Errorf(ctx, "[Cron] Failed to reset token quota: %v", err)
			return
		}
		logger.Infof(ctx, "[Cron] Token quota reset completed, %d tenants affected", affected)
		return
	}

	logger.Info(ctx, "[Cron] Token quota reset completed")
}
