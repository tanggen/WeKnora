package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// AdminStatsHandler handles HTTP requests for admin statistics
type AdminStatsHandler struct {
	service interfaces.TenantStatsService
}

// NewAdminStatsHandler creates a new admin stats handler
func NewAdminStatsHandler(service interfaces.TenantStatsService) *AdminStatsHandler {
	return &AdminStatsHandler{service: service}
}

// GetAdminTenantStats returns paginated statistics for all tenants
func (h *AdminStatsHandler) GetAdminTenantStats(c *gin.Context) {
	ctx := c.Request.Context()

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	sortBy := c.DefaultQuery("sort_by", "token_used")
	order := c.DefaultQuery("order", "desc")
	keyword := c.Query("keyword")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	resp, err := h.service.GetAdminTenantStats(ctx, keyword, sortBy, order, page, pageSize)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to get admin stats").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp.Data,
		"total":   resp.Total,
		"page":    resp.Page,
		"page_size": resp.PageSize,
	})
}

// GetAdminOverview returns the global dashboard summary
func (h *AdminStatsHandler) GetAdminOverview(c *gin.Context) {
	ctx := c.Request.Context()

	overview, err := h.service.GetAdminOverview(ctx)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to get admin overview").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    overview,
	})
}
