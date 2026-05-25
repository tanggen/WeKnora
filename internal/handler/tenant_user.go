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

// TenantUserHandler implements HTTP request handlers for tenant user management
type TenantUserHandler struct {
	service interfaces.TenantUserService
}

// NewTenantUserHandler creates a new tenant user handler instance
func NewTenantUserHandler(service interfaces.TenantUserService) *TenantUserHandler {
	return &TenantUserHandler{service: service}
}

// getTenantID extracts tenant ID from URL parameter
func (h *TenantUserHandler) getTenantID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("tenant_id"), 10, 64)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListUsers godoc
// @Summary      获取租户用户列表
// @Description  获取租户下的所有用户（分页）
// @Tags         租户用户管理
// @Accept       json
// @Produce      json
// @Param        tid       path      int     true  "租户ID"
// @Param        keyword   query     string  false "搜索关键词"
// @Param        role      query     string  false "角色过滤"
// @Param        status    query     string  false "状态过滤(active/disabled)"
// @Param        page      query     int     false "页码" default(1)
// @Param        page_size query     int     false "每页数量" default(20)
// @Success      200       {object}  types.TenantUserListResponse
// @Failure      400       {object}  errors.AppError
// @Security     Bearer
// @Router       /tenants/{tid}/users [get]
func (h *TenantUserHandler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := h.getTenantID(c)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	keyword := c.Query("keyword")
	role := c.Query("role")
	status := c.Query("status")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.service.ListUsers(ctx, tenantID, keyword, role, status, page, pageSize)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to list users").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// CreateUser godoc
// @Summary      在租户中创建新用户
// @Description  创建新用户并加入租户（管理员操作）
// @Tags         租户用户管理
// @Accept       json
// @Produce      json
// @Param        tid      path      int                            true "租户ID"
// @Param        request  body      types.CreateTenantUserRequest  true "创建用户请求"
// @Success      201      {object}  types.TenantUserCreatedResponse
// @Failure      400      {object}  errors.AppError
// @Security     Bearer
// @Router       /tenants/{tid}/users [post]
func (h *TenantUserHandler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := h.getTenantID(c)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	var req types.CreateTenantUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	result, err := h.service.CreateUser(ctx, tenantID, &req)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to create user").WithDetails(err.Error()))
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    result,
	})
}

// UpdateUser godoc
// @Summary      更新租户用户信息
// @Description  更新指定用户的个人资料
// @Tags         租户用户管理
// @Accept       json
// @Produce      json
// @Param        tid      path      int                            true "租户ID"
// @Param        uid      path      string                         true "用户ID"
// @Param        request  body      types.UpdateTenantUserRequest  true "更新用户请求"
// @Success      200      {object}  types.TenantUserInfo
// @Failure      400      {object}  errors.AppError
// @Security     Bearer
// @Router       /tenants/{tid}/users/{uid} [put]
func (h *TenantUserHandler) UpdateUser(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := h.getTenantID(c)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	userID := c.Param("uid")
	if userID == "" {
		c.Error(errors.NewBadRequestError("Invalid user ID"))
		return
	}

	var req types.UpdateTenantUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	result, err := h.service.UpdateUser(ctx, tenantID, userID, &req)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update user").WithDetails(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// SetUserStatus godoc
// @Summary      启用/禁用用户
// @Description  设置用户在租户内的状态
// @Tags         租户用户管理
// @Accept       json
// @Produce      json
// @Param        tid      path      int                         true "租户ID"
// @Param        uid      path      string                      true "用户ID"
// @Param        request  body      types.SetUserStatusRequest  true "状态请求"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  errors.AppError
// @Security     Bearer
// @Router       /tenants/{tid}/users/{uid}/status [put]
func (h *TenantUserHandler) SetUserStatus(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := h.getTenantID(c)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	userID := c.Param("uid")
	if userID == "" {
		c.Error(errors.NewBadRequestError("Invalid user ID"))
		return
	}

	var req types.SetUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	if err := h.service.SetUserStatus(ctx, tenantID, userID, req.IsActive); err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update user status").WithDetails(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User status updated successfully",
	})
}

// DeleteUser godoc
// @Summary      从租户中移除用户
// @Description  软删除租户中的用户
// @Tags         租户用户管理
// @Accept       json
// @Produce      json
// @Param        tid  path  int     true "租户ID"
// @Param        uid  path  string  true "用户ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  errors.AppError
// @Security     Bearer
// @Router       /tenants/{tid}/users/{uid} [delete]
func (h *TenantUserHandler) DeleteUser(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := h.getTenantID(c)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	userID := c.Param("uid")
	if userID == "" {
		c.Error(errors.NewBadRequestError("Invalid user ID"))
		return
	}

	if err := h.service.DeleteUser(ctx, tenantID, userID); err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to delete user").WithDetails(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User removed from tenant successfully",
	})
}

// SetUserRole godoc
// @Summary      修改用户角色
// @Description  更改用户在租户内的角色
// @Tags         租户用户管理
// @Accept       json
// @Produce      json
// @Param        tid      path      int                       true "租户ID"
// @Param        uid      path      string                    true "用户ID"
// @Param        request  body      types.SetUserRoleRequest  true "角色请求"
// @Success      200      {object}  types.TenantUserInfo
// @Failure      400      {object}  errors.AppError
// @Security     Bearer
// @Router       /tenants/{tid}/users/{uid}/role [put]
func (h *TenantUserHandler) SetUserRole(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, err := h.getTenantID(c)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid tenant ID"))
		return
	}

	userID := c.Param("uid")
	if userID == "" {
		c.Error(errors.NewBadRequestError("Invalid user ID"))
		return
	}

	var req types.SetUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	result, err := h.service.SetUserRole(ctx, tenantID, userID, &req)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update user role").WithDetails(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
