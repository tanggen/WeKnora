package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// planRepository implements PlanRepository
type planRepository struct {
	db *gorm.DB
}

// NewPlanRepository creates a new plan repository
func NewPlanRepository(db *gorm.DB) interfaces.PlanRepository {
	return &planRepository{db: db}
}

// Create creates a new plan with its config in a transaction
func (r *planRepository) Create(ctx context.Context, plan *types.Plan, config *types.PlanConfig) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(plan).Error; err != nil {
			return err
		}
		config.PlanID = plan.ID
		if err := tx.Create(config).Error; err != nil {
			return err
		}
		return nil
	})
}

// Update updates a plan and its config in a transaction
func (r *planRepository) Update(ctx context.Context, plan *types.Plan, config *types.PlanConfig) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(plan).Error; err != nil {
			return err
		}
		if config != nil {
			config.PlanID = plan.ID
			if err := tx.Save(config).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Delete deletes a plan (soft delete) and its config
func (r *planRepository) Delete(ctx context.Context, planID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", planID).Delete(&types.PlanConfig{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", planID).Delete(&types.Plan{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// GetByID retrieves a plan with its config
func (r *planRepository) GetByID(ctx context.Context, planID string) (*types.PlanWithConfig, error) {
	var plan types.Plan
	if err := r.db.WithContext(ctx).Where("id = ?", planID).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("plan not found: %s", planID)
		}
		return nil, err
	}

	var config types.PlanConfig
	if err := r.db.WithContext(ctx).Where("plan_id = ?", planID).First(&config).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	return &types.PlanWithConfig{Plan: plan, Config: config}, nil
}

// List returns all active plans ordered by sort_order
func (r *planRepository) List(ctx context.Context) ([]*types.PlanWithConfig, error) {
	var plans []types.Plan
	if err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("sort_order ASC").
		Find(&plans).Error; err != nil {
		return nil, err
	}

	return r.loadConfigs(ctx, plans)
}

// ListAll returns all plans including inactive
func (r *planRepository) ListAll(ctx context.Context) ([]*types.PlanWithConfig, error) {
	var plans []types.Plan
	if err := r.db.WithContext(ctx).
		Order("sort_order ASC").
		Find(&plans).Error; err != nil {
		return nil, err
	}

	return r.loadConfigs(ctx, plans)
}

// CountTenantsByPlan counts tenants using a specific plan
func (r *planRepository) CountTenantsByPlan(ctx context.Context, planID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.Tenant{}).
		Where("plan_id = ?", planID).
		Count(&count).Error
	return count, err
}

// loadConfigs loads plan configs for a list of plans
func (r *planRepository) loadConfigs(ctx context.Context, plans []types.Plan) ([]*types.PlanWithConfig, error) {
	if len(plans) == 0 {
		return []*types.PlanWithConfig{}, nil
	}

	planIDs := make([]string, len(plans))
	for i, p := range plans {
		planIDs[i] = p.ID
	}

	var configs []types.PlanConfig
	if err := r.db.WithContext(ctx).
		Where("plan_id IN ?", planIDs).
		Find(&configs).Error; err != nil {
		return nil, err
	}

	configMap := make(map[string]types.PlanConfig, len(configs))
	for _, c := range configs {
		configMap[c.PlanID] = c
	}

	result := make([]*types.PlanWithConfig, 0, len(plans))
	for _, p := range plans {
		p := p // capture
		cfg := configMap[p.ID]
		result = append(result, &types.PlanWithConfig{Plan: p, Config: cfg})
	}

	return result, nil
}

// Ensure planRepository implements PlanRepository
var _ interfaces.PlanRepository = (*planRepository)(nil)

// escapeLikeKeyword escapes special characters in LIKE patterns
func escapeLikeKeyword(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "%", "\\%"), "_", "\\_")
}
