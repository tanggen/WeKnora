package types

import (
	"time"

	"gorm.io/gorm"
)

// Plan represents a subscription plan (套餐)
type Plan struct {
	// Unique plan identifier: trial, basic, pro, enterprise
	ID string `json:"plan_id"     gorm:"type:varchar(32);primaryKey"`
	// Human-readable plan name
	Name string `json:"name"        gorm:"type:varchar(128);not null"`
	// Plan description
	Description string `json:"description" gorm:"type:text"`
	// Whether this is a trial plan
	IsTrial bool `json:"is_trial"    gorm:"not null;default:false"`
	// Trial duration in days (only when is_trial=true)
	TrialDays int `json:"trial_days"  gorm:"not null;default:0"`
	// Whether this plan is available for new subscriptions
	IsActive bool `json:"is_active"   gorm:"not null;default:true"`
	// Sort order for display
	SortOrder int            `json:"sort_order"  gorm:"not null;default:99"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"           gorm:"index"`
}

// TableName returns the table name for GORM
func (Plan) TableName() string {
	return "plans"
}

// PlanConfig stores resource limits and pricing for a plan
type PlanConfig struct {
	// Foreign key to plans.id
	PlanID string `json:"-"            gorm:"type:varchar(32);primaryKey"`

	// --- Resource limits (0 = unlimited) ---
	MaxUsers                int   `json:"max_users"                 gorm:"not null;default:5"`
	MaxKnowledgeBases       int   `json:"max_knowledge_bases"       gorm:"not null;default:20"`
	MaxKnowledgePerKB       int   `json:"max_knowledge_per_kb"      gorm:"not null;default:1000"`
	MaxChunksPerKnowledge   int   `json:"max_chunks_per_knowledge"  gorm:"not null;default:10000"`
	StorageQuotaBytes       int64 `json:"storage_quota_bytes"       gorm:"not null;default:10737418240"` // 10 GB
	TokenQuotaMonthly       int64 `json:"token_quota_monthly"       gorm:"not null;default:100000"`
	MaxAgents               int   `json:"max_agents"                gorm:"not null;default:5"`

	// --- Pricing (in CNY cents) ---
	PriceMonthlyCNY int `json:"price_monthly_cny" gorm:"not null;default:0"`
	PriceYearlyCNY  int `json:"price_yearly_cny"  gorm:"not null;default:0"`

	// --- JSON configs ---
	// Feature flags (e.g. ["chat","search","web_search","api_access"])
	Features StringArray `json:"features"            gorm:"type:jsonb;not null;default:'[]'"`
	// Marketing features for frontend display
	MarketingFeatures StringArray `json:"marketing_features"  gorm:"type:jsonb;not null;default:'[]'"`

	UpdatedAt time.Time `json:"updated_at"`
}

// TableName returns the table name for GORM
func (PlanConfig) TableName() string {
	return "plan_configs"
}

// PlanWithConfig combines Plan and PlanConfig for API responses
type PlanWithConfig struct {
	Plan
	Config PlanConfig `json:"config"`
}

// PlanListResponse is the API response for listing plans
type PlanListResponse struct {
	Data []PlanWithConfig `json:"data"`
}

// CreatePlanRequest is the request to create a new plan
type CreatePlanRequest struct {
	PlanID              string   `json:"plan_id"              binding:"required"`
	Name                string   `json:"name"                 binding:"required"`
	Description         string   `json:"description"`
	IsTrial             bool     `json:"is_trial"`
	TrialDays           int      `json:"trial_days"`
	MaxUsers            int      `json:"max_users"`
	MaxKnowledgeBases   int      `json:"max_knowledge_bases"`
	MaxKnowledgePerKB   int      `json:"max_knowledge_per_kb"`
	MaxChunksPerKnowledge int    `json:"max_chunks_per_knowledge"`
	StorageQuotaBytes   int64    `json:"storage_quota_bytes"`
	TokenQuotaMonthly   int64    `json:"token_quota_monthly"`
	MaxAgents           int      `json:"max_agents"`
	Features            []string `json:"features"`
	MarketingFeatures   []string `json:"marketing_features"`
	PriceMonthlyCNY     int      `json:"price_monthly_cny"`
	PriceYearlyCNY      int      `json:"price_yearly_cny"`
	SortOrder           int      `json:"sort_order"`
}

// UpdatePlanRequest is the request to update an existing plan
type UpdatePlanRequest struct {
	Name                *string   `json:"name"`
	Description         *string   `json:"description"`
	IsTrial             *bool     `json:"is_trial"`
	TrialDays           *int      `json:"trial_days"`
	MaxUsers            *int      `json:"max_users"`
	MaxKnowledgeBases   *int      `json:"max_knowledge_bases"`
	MaxKnowledgePerKB   *int      `json:"max_knowledge_per_kb"`
	MaxChunksPerKnowledge *int    `json:"max_chunks_per_knowledge"`
	StorageQuotaBytes   *int64    `json:"storage_quota_bytes"`
	TokenQuotaMonthly   *int64    `json:"token_quota_monthly"`
	MaxAgents           *int      `json:"max_agents"`
	Features            *[]string `json:"features"`
	MarketingFeatures   *[]string `json:"marketing_features"`
	PriceMonthlyCNY     *int      `json:"price_monthly_cny"`
	PriceYearlyCNY      *int      `json:"price_yearly_cny"`
	SortOrder           *int      `json:"sort_order"`
}

// SetPlanStatusRequest is the request to enable/disable a plan
type SetPlanStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// AssignPlanRequest is the request to assign a plan to a tenant
type AssignPlanRequest struct {
	PlanID string `json:"plan_id" binding:"required"`
}

// AssignPlanResponse is the response after assigning a plan
type AssignPlanResponse struct {
	TenantID       uint64    `json:"tenant_id"`
	PlanID         string    `json:"plan_id"`
	PlanName       string    `json:"plan_name"`
	EffectiveAt    time.Time `json:"effective_at"`
	PreviousPlanID string    `json:"previous_plan_id,omitempty"`
}
