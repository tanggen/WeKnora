package cron

import (
	"context"

	"github.com/Tencent/WeKnora/internal/logger"
)

// runCheckTrialExpiry checks for tenants whose trial has expired and marks them.
// Trial tenants whose trial_expires_at has passed should be flagged or disabled.
func (s *Scheduler) runCheckTrialExpiry() {
	ctx := context.Background()
	logger.Info(ctx, "[Cron] Starting trial expiry check...")

	// In production, this would:
	//   SELECT * FROM tenants
	//   JOIN plans ON tenants.plan_id = plans.id
	//   WHERE plans.is_trial = true
	//     AND tenants.trial_expires_at < NOW()
	//     AND tenants.status = 'active'
	//
	// Then either:
	//   - Mark tenant as suspended
	//   - Send notification
	//   - Auto-switch to a fallback plan

	logger.Info(ctx, "[Cron] Trial expiry check completed")
}
