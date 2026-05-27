package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
)

// AdminTenantHandler handles HTTP requests for system admin tenant management
type AdminTenantHandler struct {
	adminService *service.AdminService
}

// NewAdminTenantHandler creates a new admin tenant handler
func NewAdminTenantHandler(adminService *service.AdminService) *AdminTenantHandler {
	return &AdminTenantHandler{adminService: adminService}
}

// CreateTenantRequest is the request body for creating a new tenant via admin
type CreateTenantRequest struct {
	Name        string `json:"name"        binding:"required,max=128"`
	Description string `json:"description"`
	Business    string `json:"business"`
	PlanID      string `json:"plan_id"     binding:"required"`
}

// CreateTenant creates a new tenant with a selected plan
func (h *AdminTenantHandler) CreateTenant(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request body").WithDetails(err.Error()))
		return
	}

	tenant, err := h.adminService.CreateTenant(ctx, &service.CreateTenantRequest{
		Name:        req.Name,
		Description: req.Description,
		Business:    req.Business,
		PlanID:      req.PlanID,
	})
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to create tenant").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    tenant,
	})
}

// ListTenants returns a paginated list of all tenants with optional filters
func (h *AdminTenantHandler) ListTenants(c *gin.Context) {
	ctx := c.Request.Context()

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")
	status := c.Query("status")
	planID := c.Query("plan_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	items, total, err := h.adminService.ListTenants(ctx, keyword, status, planID, page, pageSize)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to list tenants").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetTenant returns detailed info for a single tenant
func (h *AdminTenantHandler) GetTenant(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := strconv.ParseUint(c.Param("tenant_id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	detail, err := h.adminService.GetTenantDetail(ctx, tenantID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to get tenant detail").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    detail,
	})
}

// UpdateTenant updates a tenant's name and description
func (h *AdminTenantHandler) UpdateTenant(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := strconv.ParseUint(c.Param("tenant_id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	var req service.UpdateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request body").WithDetails(err.Error()))
		return
	}

	if err := h.adminService.UpdateTenant(ctx, tenantID, &req); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		// Pass the error through — it may be a known AppError
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Tenant updated successfully",
	})
}

// SetTenantStatus enables or disables a tenant
func (h *AdminTenantHandler) SetTenantStatus(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := strconv.ParseUint(c.Param("tenant_id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	var req service.SetTenantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request body").WithDetails(err.Error()))
		return
	}

	if err := h.adminService.SetTenantStatus(ctx, tenantID, req.Status); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Tenant status updated successfully",
	})
}

// GetTenantUsers returns paginated users within a tenant (admin view)
func (h *AdminTenantHandler) GetTenantUsers(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := strconv.ParseUint(c.Param("tenant_id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")
	role := c.Query("role")
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	items, total, err := h.adminService.GetTenantUsers(ctx, tenantID, keyword, role, status, page, pageSize)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to list tenant users").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
