package function

import (
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/registry/sdkversion"
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

type versionFloorsListResponse struct {
	Floors map[string]string `json:"floors"`
}

type versionFloorBatchRequest struct {
	FunctionIds []string `json:"functionIds" binding:"required"`
	MinVersion  string   `json:"minVersion"`
}

type versionFloorBatchResponse struct {
	Updated    int      `json:"updated"`
	Failed     []string `json:"failed"`
	MinVersion string   `json:"minVersion,omitempty"`
}

// versionFloorScopeOnly 解析作用域（批量端点共用，无 :id param）。
func (h *Handler) versionFloorScopeOnly(c *gin.Context) (gameID, env string, err error) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	gameID = strings.TrimSpace(scope.GameID)
	env = strings.TrimSpace(scope.Env)
	if gameID == "" || env == "" {
		return "", "", errorx.NewBadRequest("X-Game-ID/X-Env is required")
	}
	return gameID, env, nil
}

// versionFloorScope 解析作用域与函数 id。读路径只受认证 + scope 保护
// （与注释历史上声称的 functions:read 校验不符，为既有边界）；写路径
// 另行校验 functions:manage（checkVersionFloorManage）。
func (h *Handler) versionFloorScope(c *gin.Context) (gameID, env, functionID string, err error) {
	gameID, env, err = h.versionFloorScopeOnly(c)
	if err != nil {
		return "", "", "", err
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

// VersionFloorsList handles GET /api/v1/functions/version-floors。
// 返回当前 scope 下全部函数级最低版本（functionId → minVersion），函数
// 目录「最低SDK版本」列数据源；未配置的函数不在 map 中。
func (h *Handler) VersionFloorsList(c *gin.Context) {
	gameID, env, err := h.versionFloorScopeOnly(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	store := h.service.SvcCtx().RegistryStore
	if store == nil {
		response.Error(c, errorx.NewInternalError("registry store is not initialized"))
		return
	}
	response.Success(c, versionFloorsListResponse{Floors: store.GetFunctionVersionFloors(gameID, env)})
}

// VersionFloorBatch handles POST /api/v1/functions/version-floor/batch。
// 统一值语义：minVersion 空 = 批量清除（对齐单函数 DELETE）；非空先对
// 统一值整体校验可解析（一个坏值零写入），再逐函数 Set/Delete，个别失
// 败收集进 failed（对齐 batch 家族部分成功语义；Set/Delete 幂等，重试安全）。
func (h *Handler) VersionFloorBatch(c *gin.Context) {
	if err := h.checkVersionFloorManage(c); err != nil {
		response.Error(c, err)
		return
	}
	gameID, env, err := h.versionFloorScopeOnly(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var req versionFloorBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ids := make([]string, 0, len(req.FunctionIds))
	seen := make(map[string]struct{}, len(req.FunctionIds))
	for _, id := range req.FunctionIds {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		response.BadRequest(c, "functionIds is required")
		return
	}
	minVersion := strings.TrimSpace(req.MinVersion)
	if minVersion != "" && !sdkversion.Parseable(minVersion) {
		response.BadRequest(c, fmt.Sprintf("min_version %q is not a parseable semantic version", minVersion))
		return
	}
	store := h.service.SvcCtx().RegistryStore
	if store == nil {
		response.Error(c, errorx.NewInternalError("registry store is not initialized"))
		return
	}
	updatedBy, _ := utils.CurrentUsername(c.Request.Context())
	resp := versionFloorBatchResponse{Failed: []string{}, MinVersion: minVersion}
	for _, id := range ids {
		var setErr error
		if minVersion == "" {
			setErr = store.DeleteFunctionVersionFloor(gameID, env, id)
		} else {
			setErr = store.SetFunctionVersionFloor(gameID, env, id, minVersion, updatedBy)
		}
		if setErr != nil {
			resp.Failed = append(resp.Failed, id)
			continue
		}
		resp.Updated++
	}
	response.Success(c, resp)
}
