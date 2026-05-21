package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// planService implements PlanService
type planService struct {
	planRepo    interfaces.PlanRepository
	statsRepo   interfaces.TenantStatsRepository
	menuService *MenuService
}

// NewPlanService creates a new plan service
func NewPlanService(planRepo interfaces.PlanRepository, statsRepo interfaces.TenantStatsRepository, menuService *MenuService) interfaces.PlanService {
	return &planService{
		planRepo:    planRepo,
		statsRepo:   statsRepo,
		menuService: menuService,
	}
}

// CreatePlan creates a new subscription plan
func (s *planService) CreatePlan(ctx context.Context, req *types.CreatePlanRequest) (*types.PlanWithConfig, error) {
	// Check uniqueness
	existing, err := s.planRepo.GetByID(ctx, req.PlanID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("plan already exists: %s", req.PlanID)
	}

	plan := &types.Plan{
		ID:          req.PlanID,
		Name:        req.Name,
		Description: req.Description,
		IsTrial:     req.IsTrial,
		TrialDays:   req.TrialDays,
		IsActive:    true,
		SortOrder:   req.SortOrder,
	}

	config := &types.PlanConfig{
		PlanID:                 req.PlanID,
		MaxUsers:               req.MaxUsers,
		MaxKnowledgeBases:      req.MaxKnowledgeBases,
		MaxKnowledgePerKB:      req.MaxKnowledgePerKB,
		MaxChunksPerKnowledge:  req.MaxChunksPerKnowledge,
		StorageQuotaBytes:      req.StorageQuotaBytes,
		TokenQuotaMonthly:      req.TokenQuotaMonthly,
		MaxAgents:              req.MaxAgents,
		PriceMonthlyCNY:        req.PriceMonthlyCNY,
		PriceYearlyCNY:         req.PriceYearlyCNY,
		Features:               types.StringArray(req.Features),
		MarketingFeatures:      types.StringArray(req.MarketingFeatures),
	}

	if err := s.planRepo.Create(ctx, plan, config); err != nil {
		return nil, err
	}

	return &types.PlanWithConfig{Plan: *plan, Config: *config}, nil
}

// UpdatePlan updates an existing plan
func (s *planService) UpdatePlan(ctx context.Context, planID string, req *types.UpdatePlanRequest) (*types.PlanWithConfig, error) {
	existing, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	// Apply partial updates
	if req.Name != nil {
		existing.Plan.Name = *req.Name
	}
	if req.Description != nil {
		existing.Plan.Description = *req.Description
	}
	if req.IsTrial != nil {
		existing.Plan.IsTrial = *req.IsTrial
	}
	if req.TrialDays != nil {
		existing.Plan.TrialDays = *req.TrialDays
	}
	if req.SortOrder != nil {
		existing.Plan.SortOrder = *req.SortOrder
	}

	if req.MaxUsers != nil {
		existing.Config.MaxUsers = *req.MaxUsers
	}
	if req.MaxKnowledgeBases != nil {
		existing.Config.MaxKnowledgeBases = *req.MaxKnowledgeBases
	}
	if req.MaxKnowledgePerKB != nil {
		existing.Config.MaxKnowledgePerKB = *req.MaxKnowledgePerKB
	}
	if req.MaxChunksPerKnowledge != nil {
		existing.Config.MaxChunksPerKnowledge = *req.MaxChunksPerKnowledge
	}
	if req.StorageQuotaBytes != nil {
		existing.Config.StorageQuotaBytes = *req.StorageQuotaBytes
	}
	if req.TokenQuotaMonthly != nil {
		existing.Config.TokenQuotaMonthly = *req.TokenQuotaMonthly
	}
	if req.MaxAgents != nil {
		existing.Config.MaxAgents = *req.MaxAgents
	}
	if req.Features != nil {
		existing.Config.Features = types.StringArray(*req.Features)
	}
	if req.MarketingFeatures != nil {
		existing.Config.MarketingFeatures = types.StringArray(*req.MarketingFeatures)
	}
	if req.PriceMonthlyCNY != nil {
		existing.Config.PriceMonthlyCNY = *req.PriceMonthlyCNY
	}
	if req.PriceYearlyCNY != nil {
		existing.Config.PriceYearlyCNY = *req.PriceYearlyCNY
	}

	if err := s.planRepo.Update(ctx, &existing.Plan, &existing.Config); err != nil {
		return nil, err
	}

	return existing, nil
}

// DeletePlan deletes a plan (only if no tenant is using it)
func (s *planService) DeletePlan(ctx context.Context, planID string) error {
	count, err := s.planRepo.CountTenantsByPlan(ctx, planID)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("plan in use by %d tenants", count)
	}
	return s.planRepo.Delete(ctx, planID)
}

// SetPlanStatus enables or disables a plan
func (s *planService) SetPlanStatus(ctx context.Context, planID string, isActive bool) error {
	existing, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return fmt.Errorf("plan not found: %s", planID)
	}

	if !isActive {
		// Check if any tenant is using this plan
		count, err := s.planRepo.CountTenantsByPlan(ctx, planID)
		if err != nil {
			return err
		}
		if count > 0 {
			return errors.New("plan in use")
		}
	}

	existing.Plan.IsActive = isActive
	return s.planRepo.Update(ctx, &existing.Plan, nil)
}

// GetPlan retrieves plan details
func (s *planService) GetPlan(ctx context.Context, planID string) (*types.PlanWithConfig, error) {
	return s.planRepo.GetByID(ctx, planID)
}

// ListPlans returns active plans for users
func (s *planService) ListPlans(ctx context.Context) ([]*types.PlanWithConfig, error) {
	return s.planRepo.List(ctx)
}

// ListAllPlans returns all plans for admin
func (s *planService) ListAllPlans(ctx context.Context) ([]*types.PlanWithConfig, error) {
	return s.planRepo.ListAll(ctx)
}

// AssignPlan assigns a plan to a tenant
func (s *planService) AssignPlan(ctx context.Context, tenantID uint64, planID string) (*types.AssignPlanResponse, error) {
	// System tenant (ID=1) is immutable
	if tenantID == 1 {
		return nil, errors.New("系统租户不允许切换套餐")
	}

	// Verify plan exists and is active
	planWithConfig, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}
	if !planWithConfig.Plan.IsActive {
		return nil, errors.New("plan not active")
	}

	// Get current tenant plan
	stats, err := s.statsRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant stats: %w", err)
	}

	previousPlanID := stats.PlanID
	if previousPlanID == planID {
		return nil, errors.New("same plan")
	}

	// Update tenant_stats snapshot
	if err := s.statsRepo.UpdatePlanSnapshot(ctx, tenantID, planID, planWithConfig.Config.StorageQuotaBytes); err != nil {
		return nil, fmt.Errorf("failed to update plan snapshot: %w", err)
	}

	// Invalidate menu caches for all users in this tenant
	s.InvalidateTenantMenuCache(ctx, tenantID)

	return &types.AssignPlanResponse{
		TenantID:       tenantID,
		PlanID:         planID,
		PlanName:       planWithConfig.Plan.Name,
		EffectiveAt:    time.Now(),
		PreviousPlanID: previousPlanID,
	}, nil
}

// InvalidateTenantMenuCache invalidates all menu caches for a tenant.
// This is called when plan assignment or plan features change.
func (s *planService) InvalidateTenantMenuCache(ctx context.Context, tenantID uint64) {
	if s.menuService != nil {
		s.menuService.InvalidateAllTenantCache(ctx, tenantID)
	}
}
