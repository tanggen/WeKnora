package types

import (
	"time"
)

// TenantStats stores aggregated usage statistics for a tenant.
// All quota checks read from this table; NO real-time COUNT on business tables.
type TenantStats struct {
	// Tenant ID (primary key, references tenants.id)
	TenantID uint64 `json:"tenant_id"             gorm:"primaryKey"`

	// --- Counters ---
	UserCount           int   `json:"user_count"            gorm:"not null;default:0"`
	KnowledgeBaseCount  int   `json:"knowledge_base_count"  gorm:"not null;default:0"`
	KnowledgeCount      int   `json:"knowledge_count"       gorm:"not null;default:0"`
	ChunkCount          int64 `json:"chunk_count"           gorm:"not null;default:0"`
	StorageUsedBytes    int64 `json:"storage_used_bytes"    gorm:"not null;default:0"`
	TokenUsedTotal      int64 `json:"token_used_total"      gorm:"not null;default:0"` // all-time total
	TokenUsedMonthly    int64 `json:"token_used_monthly"    gorm:"not null;default:0"` // current month
	TokenQuotaResetAt   *time.Time `json:"token_quota_reset_at"`                       // next reset timestamp

	AgentCount   int   `json:"agent_count"           gorm:"not null;default:0"`
	SessionCount int64 `json:"session_count"         gorm:"not null;default:0"`

	// --- Plan snapshot ---
	PlanID           string `json:"plan_id"              gorm:"type:varchar(32)"`
	StorageQuotaBytes int64 `json:"storage_quota_bytes"  gorm:"not null;default:10737418240"`

	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the table name for GORM
func (TenantStats) TableName() string {
	return "tenant_stats"
}

// TenantStatsResponse is the API response for a single tenant's statistics
type TenantStatsResponse struct {
	TenantID           uint64     `json:"tenant_id"`
	UserCount          int        `json:"user_count"`
	KnowledgeBaseCount int        `json:"knowledge_base_count"`
	KnowledgeCount     int        `json:"knowledge_count"`
	ChunkCount         int64      `json:"chunk_count"`
	StorageUsedBytes   int64      `json:"storage_used_bytes"`
	StorageQuotaBytes  int64      `json:"storage_quota_bytes"`
	TokenUsed          int64      `json:"token_used"`
	TokenQuotaMonthly  int64      `json:"token_quota_monthly"`
	TokenQuotaResetAt  *time.Time `json:"token_quota_reset_at,omitempty"`
	AgentCount         int        `json:"agent_count"`
	SessionCount       int64      `json:"session_count"`
}

// AdminTenantStatsItem is a single row in the admin tenant stats list
type AdminTenantStatsItem struct {
	TenantID          uint64    `json:"tenant_id"`
	TenantName        string    `json:"tenant_name"`
	PlanName          string    `json:"plan_name"`
	UserCount         int       `json:"user_count"`
	StorageUsedBytes  int64     `json:"storage_used_bytes"`
	TokenUsed         int64     `json:"token_used"`
	TokenQuotaMonthly int64     `json:"token_quota_monthly"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
}

// AdminTenantStatsListResponse is the paginated response for admin tenant stats
type AdminTenantStatsListResponse struct {
	Data     []AdminTenantStatsItem `json:"data"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// AdminOverviewStats is the global dashboard summary
type AdminOverviewStats struct {
	TotalTenants           int   `json:"total_tenants"`
	ActiveTenants          int   `json:"active_tenants"`
	TotalUsers             int   `json:"total_users"`
	TotalKnowledgeBases    int   `json:"total_knowledge_bases"`
	TotalStorageUsedBytes  int64 `json:"total_storage_used_bytes"`
	TotalTokenUsedAllTime  int64 `json:"total_token_used_all_time"`
	TrialsActive           int   `json:"trials_active"`
	TrialsConverted        int   `json:"trials_converted"`
}

// PlanQuotaConfig is a lightweight struct used for quota checking.
// Extracted from PlanConfig to avoid loading the full plan on every check.
type PlanQuotaConfig struct {
	MaxUsers               int
	MaxKnowledgeBases      int
	MaxKnowledgePerKB      int
	MaxChunksPerKnowledge  int
	StorageQuotaBytes      int64
	TokenQuotaMonthly      int64
	MaxAgents              int
}
