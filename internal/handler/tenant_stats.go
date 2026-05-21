package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TenantStatsHandler handles HTTP requests for tenant statistics
type TenantStatsHandler struct {
	service interfaces.TenantStatsService
}

// NewTenantStatsHandler creates a new tenant stats handler
func NewTenantStatsHandler(service interfaces.TenantStatsService) *TenantStatsHandler {
	return &TenantStatsHandler{service: service}
}

// GetTenantStats returns statistics for the current tenant
func (h *TenantStatsHandler) GetTenantStats(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := strconv.ParseUint(c.Param("tenant_id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	stats, err := h.service.GetTenantStats(ctx, tenantID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to get tenant stats").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}
