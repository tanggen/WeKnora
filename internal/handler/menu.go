package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// MenuHandler handles HTTP requests for dynamic menu and permissions
type MenuHandler struct {
	menuService *service.MenuService
}

// NewMenuHandler creates a new menu handler
func NewMenuHandler(menuService *service.MenuService) *MenuHandler {
	return &MenuHandler{menuService: menuService}
}

// GetMenus returns the menu tree visible to the current user.
//
//	@Summary	获取当前用户可见菜单
//	@Tags		菜单
//	@Produce	json
//	@Success	200	{object}	map[string]interface{}	"Menu tree"
//	@Router		/menus [get]
func (h *MenuHandler) GetMenus(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, ok := ctx.Value(types.TenantIDContextKey).(uint64)
	if !ok || tenantID == 0 {
		logger.Error(ctx, "Tenant ID not found in context")
		c.Error(errors.NewUnauthorizedError("Unauthorized: tenant context missing"))
		return
	}

	userID, ok := ctx.Value(types.UserIDContextKey).(string)
	if !ok || userID == "" {
		logger.Error(ctx, "User ID not found in context")
		c.Error(errors.NewUnauthorizedError("Unauthorized: user context missing"))
		return
	}

	resp, err := h.menuService.GetUserMenus(ctx, tenantID, userID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to get menus").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// GetPermissions returns the current user's role, permissions, and plan info.
//
//	@Summary	获取当前用户权限信息
//	@Tags		认证
//	@Produce	json
//	@Success	200	{object}	map[string]interface{}	"User permissions"
//	@Router		/auth/permissions [get]
func (h *MenuHandler) GetPermissions(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, ok := ctx.Value(types.TenantIDContextKey).(uint64)
	if !ok || tenantID == 0 {
		logger.Error(ctx, "Tenant ID not found in context")
		c.Error(errors.NewUnauthorizedError("Unauthorized: tenant context missing"))
		return
	}

	userID, ok := ctx.Value(types.UserIDContextKey).(string)
	if !ok || userID == "" {
		logger.Error(ctx, "User ID not found in context")
		c.Error(errors.NewUnauthorizedError("Unauthorized: user context missing"))
		return
	}

	resp, err := h.menuService.GetUserPermissions(ctx, tenantID, userID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to get permissions").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}
