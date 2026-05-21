package cron

import (
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Scheduler manages scheduled background tasks
type Scheduler struct {
	cron       *cron.Cron
	statsRepo  interfaces.TenantStatsRepository
	planRepo   interfaces.PlanRepository
	redis      *redis.Client
	quotaSvc   *service.TokenQuotaService
}

// NewScheduler creates a new cron scheduler and registers all periodic tasks
func NewScheduler(
	statsRepo interfaces.TenantStatsRepository,
	planRepo interfaces.PlanRepository,
	redis *redis.Client,
	quotaSvc *service.TokenQuotaService,
) *Scheduler {
	s := &Scheduler{
		cron:      cron.New(cron.WithSeconds()),
		statsRepo: statsRepo,
		planRepo:  planRepo,
		redis:     redis,
		quotaSvc:  quotaSvc,
	}

	// Register tasks
	s.registerTasks()

	return s
}

// registerTasks adds all periodic tasks to the scheduler
func (s *Scheduler) registerTasks() {
	// Every 10 minutes: Sync token counter from Redis to DB
	s.cron.AddFunc("0 */10 * * * *", func() {
		s.runSyncTokenCounter()
	})

	// Daily at 3:00 AM: Reconcile tenant stats
	s.cron.AddFunc("0 0 3 * * *", func() {
		s.runReconcileTenantStats()
	})

	// 1st of every month at midnight: Reset token quota
	s.cron.AddFunc("0 0 0 1 * *", func() {
		s.runResetTokenQuota()
	})

	// Daily at 2:00 AM: Check trial expiry
	s.cron.AddFunc("0 0 2 * * *", func() {
		s.runCheckTrialExpiry()
	})
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}
