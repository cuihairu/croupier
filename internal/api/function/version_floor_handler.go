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

// REST surface for function-level minimum function version floors (scoped by
// X-Game-ID/X-Env). See control_handler.evaluateSDKVersionFloor for the
// evaluation semantics: when the function descriptor version is below the
// configured value, the older declaration is not materialized on registration
// (it only raises a registration warning) -- guarding against contract rollback.

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

// versionFloorScopeOnly resolves the scope (shared by the batch endpoints, no
// :id param).
func (h *Handler) versionFloorScopeOnly(c *gin.Context) (gameID, env string, err error) {
	scope := svc.GameScopeFromContext(c.Request.Context())
	gameID = strings.TrimSpace(scope.GameID)
	env = strings.TrimSpace(scope.Env)
	if gameID == "" || env == "" {
		return "", "", errorx.NewBadRequest("X-Game-ID/X-Env is required")
	}
	return gameID, env, nil
}

// versionFloorScope resolves the scope and the function id. The read path is
// only guarded by authentication plus scope (inconsistent with the
// functions:read check the comment historically claimed; a pre-existing
// boundary); the write path separately validates functions:manage
// (checkVersionFloorManage).
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

// checkVersionFloorManage validates functions:manage (or admin/*) -- the
// write variant of the same admission surface as
// descriptors_logic.checkReadPermission.
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

// VersionFloorGet handles GET /api/v1/functions/:id/version-floor.
// When not configured, minVersion is an empty string (no 404 -- the settings
// panel echoes the empty state directly).
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

// VersionFloorPut handles PUT /api/v1/functions/:id/version-floor.
// minVersion must be parseable (sdkversion.Parseable); clearing goes through
// DELETE.
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

// VersionFloorsList handles GET /api/v1/functions/version-floors.
// Returns every function-level minimum version in the current scope
// (functionId -> minVersion), the data source of the "minimum SDK version"
// column in the function directory; functions without a floor are absent from
// the map.
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

// VersionFloorBatch handles POST /api/v1/functions/version-floor/batch.
// Unified value semantics: an empty minVersion means batch clear (mirroring the
// single-function DELETE); a non-empty value is first validated for
// parseability as a whole (one bad value means zero writes), then Set/Delete
// runs per function with individual failures collected into failed (mirroring
// the partial-success semantics of the batch family; Set/Delete are idempotent,
// so retries are safe).
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
