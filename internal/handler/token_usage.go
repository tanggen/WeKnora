package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
)

// TokenUsageHandler handles HTTP requests for detailed token usage information
type TokenUsageHandler struct {
	quotaService *service.TokenQuotaService
}

// NewTokenUsageHandler creates a new token usage handler
func NewTokenUsageHandler(quotaService *service.TokenQuotaService) *TokenUsageHandler {
	return &TokenUsageHandler{quotaService: quotaService}
}

// GetTokenUsage returns detailed token usage for a tenant including monthly and all-time stats
func (h *TokenUsageHandler) GetTokenUsage(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := strconv.ParseUint(c.Param("tenant_id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	detail, err := h.quotaService.GetTokenUsageDetail(ctx, tenantID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to get token usage").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    detail,
	})
}
