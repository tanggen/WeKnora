package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// PlanHandler handles HTTP requests for plan management
type PlanHandler struct {
	service interfaces.PlanService
}

// NewPlanHandler creates a new plan handler
func NewPlanHandler(service interfaces.PlanService) *PlanHandler {
	return &PlanHandler{service: service}
}

// ListPlans returns all active plans for regular users
func (h *PlanHandler) ListPlans(c *gin.Context) {
	ctx := c.Request.Context()
	plans, err := h.service.ListPlans(ctx)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to list plans").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    plans,
	})
}

// GetPlan retrieves a single plan
func (h *PlanHandler) GetPlan(c *gin.Context) {
	ctx := c.Request.Context()
	planID := c.Param("plan_id")

	plan, err := h.service.GetPlan(ctx, planID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewNotFoundError("Plan not found").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    plan,
	})
}

// ListAllPlans returns all plans for admin
func (h *PlanHandler) ListAllPlans(c *gin.Context) {
	ctx := c.Request.Context()
	plans, err := h.service.ListAllPlans(ctx)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to list plans").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    plans,
	})
}

// CreatePlan creates a new plan
func (h *PlanHandler) CreatePlan(c *gin.Context) {
	ctx := c.Request.Context()
	var req types.CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse plan request", err)
		c.Error(errors.NewValidationError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	plan, err := h.service.CreatePlan(ctx, &req)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to create plan").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    plan,
	})
}

// UpdatePlan updates an existing plan
func (h *PlanHandler) UpdatePlan(c *gin.Context) {
	ctx := c.Request.Context()
	planID := c.Param("plan_id")

	var req types.UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse update request", err)
		c.Error(errors.NewValidationError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	plan, err := h.service.UpdatePlan(ctx, planID, &req)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to update plan").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    plan,
	})
}

// DeletePlan deletes a plan
func (h *PlanHandler) DeletePlan(c *gin.Context) {
	ctx := c.Request.Context()
	planID := c.Param("plan_id")

	if err := h.service.DeletePlan(ctx, planID); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to delete plan").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"plan_id": planID},
	})
}

// SetPlanStatus enables or disables a plan
func (h *PlanHandler) SetPlanStatus(c *gin.Context) {
	ctx := c.Request.Context()
	planID := c.Param("plan_id")

	var req types.SetPlanStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse status request", err)
		c.Error(errors.NewValidationError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	if err := h.service.SetPlanStatus(ctx, planID, req.IsActive); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		if err.Error() == "plan in use" {
			c.Error(errors.NewConflictError("Plan in use: cannot deactivate").WithDetails(err.Error()))
			return
		}
		c.Error(errors.NewInternalServerError("Failed to update plan status").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"plan_id": planID, "is_active": req.IsActive},
	})
}

// AssignPlan assigns a plan to a tenant
func (h *PlanHandler) AssignPlan(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := strconv.ParseUint(c.Param("tenant_id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	var req types.AssignPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse assign request", err)
		c.Error(errors.NewValidationError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	resp, err := h.service.AssignPlan(ctx, tenantID, req.PlanID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to assign plan").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}
