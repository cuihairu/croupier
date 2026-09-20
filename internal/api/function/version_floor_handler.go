package function

import (
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// 函数级最低 SDK 版本门槛 REST 面（scoped by X-Game-ID/X-Env）。
// 判定语义见 control_handler.evaluateSDKVersionFloor：provider 自报
// sdk_version 低于配置值时该函数不随注册物化（只产生注册警告）。

type versionFloorResponse struct {
	FunctionID string `json:"functionId"`
	MinVersion string `json:"minVersion"`
	UpdatedBy  string `json:"updatedBy,omitempty"`
}

type versionFloorRequest struct {
	MinVersion string `json:"minVersion"`
}

// versionFloorScope 解析作用域与函数 id，并校验读权限（functions:read）。
// 写路径另行校验 functions:manage（checkVersionFloorWrite）。
func (h *Handler) versionFloorScope(c *gin.Context) (gameID, env, functionID string, err error) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	gameID = strings.TrimSpace(scope.GameID)
	env = strings.TrimSpace(scope.Env)
	if gameID == "" || env == "" {
		return "", "", "", errorx.NewBadRequest("X-Game-ID/X-Env is required")
	}
	functionID = strings.TrimSpace(c.Param("id"))
	if functionID == "" {
		return "", "", "", errorx.NewBadRequest("id is required")
	}
	return gameID, env, functionID, nil
}

// checkVersionFloorManage 校验 functions:manage（或 admin/*）——与
// descriptors_logic.checkReadPermission 同一准入面的写变体。
func (h *Handler) checkVersionFloorManage(c *gin.Context) error {
	ctx := c.Request.Context()
	username, _ := utils.CurrentUsername(ctx)
	if username == "" {
		return nil
	}
	_, rolesFromDB, err := utils.LoadCurrentAdmin(ctx, h.service.SvcCtx())
	if err != nil {
		return err
	}
	roleNames := utils.RoleNamesFromModels(rolesFromDB)
	permIDs, err := utils.PermissionIDsFromRoles(ctx, h.service.SvcCtx(), rolesFromDB)
	if err != nil {
		return err
	}
	if utils.HasAdminRole(roleNames) || utils.HasPermissionID(permIDs, "functions:manage") || utils.HasPermissionID(permIDs, "*") {
		return nil
	}
	return errorx.NewForbidden("无权设置函数版本门槛")
}

// VersionFloorGet handles GET /api/v1/functions/:id/version-floor。
// 未配置时 minVersion 为空串（不 404——设置面板直接回显空态）。
func (h *Handler) VersionFloorGet(c *gin.Context) {
	gameID, env, functionID, err := h.versionFloorScope(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	store := h.service.SvcCtx().RegistryStore
	if store == nil {
		response.Error(c, errorx.NewInternalError("registry store is not initialized"))
		return
	}
	response.Success(c, versionFloorResponse{
		FunctionID: functionID,
		MinVersion: store.GetFunctionVersionFloor(gameID, env, functionID),
	})
}

// VersionFloorPut handles PUT /api/v1/functions/:id/version-floor。
// minVersion 必须可解析（sdkversion.Parseable）；清空走 DELETE。
func (h *Handler) VersionFloorPut(c *gin.Context) {
	if err := h.checkVersionFloorManage(c); err != nil {
		response.Error(c, err)
		return
	}
	gameID, env, functionID, err := h.versionFloorScope(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var req versionFloorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	store := h.service.SvcCtx().RegistryStore
	if store == nil {
		response.Error(c, errorx.NewInternalError("registry store is not initialized"))
		return
	}
	updatedBy, _ := utils.CurrentUsername(c.Request.Context())
	if err := store.SetFunctionVersionFloor(gameID, env, functionID, req.MinVersion, updatedBy); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, versionFloorResponse{
		FunctionID: functionID,
		MinVersion: strings.TrimSpace(req.MinVersion),
		UpdatedBy:  updatedBy,
	})
}

// VersionFloorDelete handles DELETE /api/v1/functions/:id/version-floor。
func (h *Handler) VersionFloorDelete(c *gin.Context) {
	if err := h.checkVersionFloorManage(c); err != nil {
		response.Error(c, err)
		return
	}
	gameID, env, functionID, err := h.versionFloorScope(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	store := h.service.SvcCtx().RegistryStore
	if store == nil {
		response.Error(c, errorx.NewInternalError("registry store is not initialized"))
		return
	}
	if err := store.DeleteFunctionVersionFloor(gameID, env, functionID); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, versionFloorResponse{FunctionID: functionID})
}
