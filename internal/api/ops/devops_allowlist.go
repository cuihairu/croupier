// The devops session allowlist panel is served here (agent-core spec §7.6; the
// same channel semantics as capture gates): it reads/writes the config_versions
// key devops.allowlist (namespace=ops) in the game database. allowlist_json is
// opaque to the server — this layer only parses/lints; the semantic validation
// reuses ParseAllowlistJSON + ValidateAllowlist from agents/devops/config (the
// same validation truth as agent apply, so a release precheck rejects bad
// versions that the agent would refuse before they ship). A successful publish
// does not mean the agent has applied it — the source of truth for being in
// effect is the agent's pull behavior (configsync 30s polling plus the
// "devops allowlist version applied" log line). Rollback writes no history: it
// fetches the target version's content and publishes it as a new version.
package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	devopsconfig "github.com/cuihairu/croupier/agents/devops/config"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// devopsAllowlistConfigKey is the fixed key the allowlist uses in config_versions
// (the wire-pull side has an identically named constant in internal/server;
// matching keys on both sides are the wire contract).
const devopsAllowlistConfigKey = "devops.allowlist"

// DevopsAllowlistGetResponse is the currently published state of the allowlist.
// Version=0 means the scope has never published (content is the empty string) —
// the agent semantics are to keep the file config as a fallback.
type DevopsAllowlistGetResponse struct {
	Key       string `json:"key"`
	Version   int    `json:"version"`
	Content   string `json:"content"`
	Namespace string `json:"namespace,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
}

// DevopsAllowlistPutRequest publishes a new allowlist version (the whole content,
// not an incremental patch). BaseVersion is the optimistic-lock baseline: if
// someone else publishes first, the publish is rejected with 409; 0 = disabled.
type DevopsAllowlistPutRequest struct {
	Content     string `json:"content"`
	Message     string `json:"message,omitempty"`
	BaseVersion int    `json:"baseVersion,omitempty"`
}

// DevopsAllowlistPutResponse is the publish result (new version number).
type DevopsAllowlistPutResponse struct {
	Version int `json:"version"`
}

// DevopsAllowlistRollbackRequest re-publishes an older version's content as a new
// version (the version log is immutable).
type DevopsAllowlistRollbackRequest struct {
	Version int    `json:"version"`
	Message string `json:"message,omitempty"`
}

// DevopsAllowlistValidateRequest is a dry-run validation (release precheck, no
// database write).
type DevopsAllowlistValidateRequest struct {
	Content string `json:"content"`
}

// DevopsAllowlistValidateResponse returns an effective preview when validation
// passes (editor-view semantics; on failure it goes 422 with the same error text
// agent apply would report).
type DevopsAllowlistValidateResponse struct {
	Valid   bool `json:"valid"`
	Entries int  `json:"entries"`
	Ports   int  `json:"ports"`
}

// DevopsAllowlistVersionItem is a version-history entry (no content: the content
// is returned only by the latest GET; rollback uses POST rollback where the
// server fetches the history, so the front end never carries the whole
// allowlist JSON).
type DevopsAllowlistVersionItem struct {
	Version   int    `json:"version"`
	Message   string `json:"message,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

// DevopsAllowlistGet handles GET /api/v1/ops/devops/allowlist.
func (h *Handler) DevopsAllowlistGet(c *gin.Context) {
	resp, err := h.service.DevopsAllowlistGet(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DevopsAllowlistPut handles PUT /api/v1/ops/devops/allowlist.
func (h *Handler) DevopsAllowlistPut(c *gin.Context) {
	var req DevopsAllowlistPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.DevopsAllowlistPut(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DevopsAllowlistValidate handles POST /api/v1/ops/devops/allowlist/validate.
// No scope dependency (pure content validation); failure semantics match a
// publish (422 + the same error text).
func (h *Handler) DevopsAllowlistValidate(c *gin.Context) {
	var req DevopsAllowlistValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.DevopsAllowlistValidate(&req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DevopsAllowlistRollback handles POST /api/v1/ops/devops/allowlist/rollback.
func (h *Handler) DevopsAllowlistRollback(c *gin.Context) {
	var req DevopsAllowlistRollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.DevopsAllowlistRollback(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DevopsAllowlistVersionsGet handles GET /api/v1/ops/devops/allowlist/versions.
func (h *Handler) DevopsAllowlistVersionsGet(c *gin.Context) {
	resp, err := h.service.DevopsAllowlistVersionsGet(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DevopsAllowlistGet returns the latest published allowlist version for the
// current scope (top-bar game/env).
func (s *Service) DevopsAllowlistGet(ctx context.Context) (*DevopsAllowlistGetResponse, error) {
	scope := svc.GameScopeFromContext(ctx)
	empty := &DevopsAllowlistGetResponse{Key: devopsAllowlistConfigKey}
	rec, err := s.svcCtx.ConfigVersionModel.FindLatestByScope(ctx, devopsAllowlistConfigKey, scope.GameID, scope.Env)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return empty, nil
		}
		return nil, err
	}
	return &DevopsAllowlistGetResponse{
		Key:       rec.Key,
		Version:   rec.Version,
		Content:   rec.Value,
		Namespace: rec.Namespace,
		UpdatedAt: rec.UpdatedAt.Format("2006-01-02 15:04:05"),
		CreatedBy: rec.CreatedBy,
	}, nil
}

// DevopsAllowlistPut publishes a new version after validation (namespace=ops,
// version increments; optimistic lock guards against lost concurrent updates
// when baseVersion>0).
func (s *Service) DevopsAllowlistPut(ctx context.Context, req *DevopsAllowlistPutRequest) (*DevopsAllowlistPutResponse, error) {
	if req == nil || strings.TrimSpace(req.Content) == "" {
		return nil, errorx.NewValidationError("devops allowlist content is required")
	}
	return s.publishDevopsAllowlist(ctx, strings.TrimSpace(req.Content), strings.TrimSpace(req.Message), req.BaseVersion)
}

// DevopsAllowlistValidate is the release-precheck dry-run: parse + ValidateAllowlist
// (the same validation truth as agent apply). No database write, no scope dependency.
func (s *Service) DevopsAllowlistValidate(req *DevopsAllowlistValidateRequest) (*DevopsAllowlistValidateResponse, error) {
	content := ""
	if req != nil {
		content = strings.TrimSpace(req.Content)
	}
	if content == "" {
		return nil, errorx.NewValidationError("devops allowlist content is required")
	}
	entries, err := parseDevopsAllowlist(content)
	if err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	ports := 0
	for _, e := range entries {
		ports += len(e.Ports)
	}
	return &DevopsAllowlistValidateResponse{Valid: true, Entries: len(entries), Ports: ports}, nil
}

// DevopsAllowlistRollback re-publishes an older version's content as a new version:
// the version log is immutable, so a rollback is itself a publish (new version
// number; the agent's usual version polling applies it live). Historical content
// re-runs the current validation: validation rules may evolve with the server
// version, and rolling back to content the current rules would reject must fail
// explicitly rather than silently publish a bad version.
func (s *Service) DevopsAllowlistRollback(ctx context.Context, req *DevopsAllowlistRollbackRequest) (*DevopsAllowlistPutResponse, error) {
	if req == nil || req.Version <= 0 {
		return nil, errorx.NewValidationError("devops allowlist rollback version is required")
	}
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("devops allowlist require game and env scope")
	}
	rec, err := s.svcCtx.ConfigVersionModel.FindByScope(ctx, devopsAllowlistConfigKey, req.Version, scope.GameID, scope.Env)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound(fmt.Sprintf("devops allowlist version %d not found in scope %s/%s", req.Version, scope.GameID, scope.Env))
		}
		return nil, err
	}
	content := strings.TrimSpace(rec.Value)
	if _, err := parseDevopsAllowlist(content); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	message := fmt.Sprintf("rollback from version %d", req.Version)
	if extra := strings.TrimSpace(req.Message); extra != "" {
		message = fmt.Sprintf("%s: %s", message, extra)
	}
	// baseVersion=0: the rollback target is an explicit version number and the
	// content comes from the server history, not from a stale edit state.
	return s.publishDevopsAllowlist(ctx, content, message, 0)
}

// DevopsAllowlistVersionsGet returns the current scope's version history
// (newest → oldest). Never-published returns an empty array rather than 404 —
// an empty history is the norm, not an error.
func (s *Service) DevopsAllowlistVersionsGet(ctx context.Context) ([]DevopsAllowlistVersionItem, error) {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("devops allowlist require game and env scope")
	}
	recs, err := s.svcCtx.ConfigVersionModel.ListByScope(ctx, devopsAllowlistConfigKey, scope.GameID, scope.Env)
	if err != nil {
		return nil, err
	}
	items := make([]DevopsAllowlistVersionItem, 0, len(recs))
	for _, rec := range recs {
		items = append(items, DevopsAllowlistVersionItem{
			Version:   rec.Version,
			Message:   rec.Message,
			CreatedBy: rec.CreatedBy,
			UpdatedAt: rec.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return items, nil
}

// parseDevopsAllowlist is the shared parse+validate used by publish and
// precheck (the same truth as agent apply).
func parseDevopsAllowlist(content string) ([]devopsconfig.AllowHost, error) {
	entries, err := devopsconfig.ParseAllowlistJSON([]byte(content))
	if err != nil {
		return nil, err
	}
	if err := devopsconfig.ValidateAllowlist(entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// publishDevopsAllowlist validates and writes the database, incrementing the
// version (shared by publish and rollback). The game/env scope must be
// non-empty: the devops agent carries its scope from registration, so a publish
// with an empty env would never be pulled — refusing at publish time is
// preferable.
func (s *Service) publishDevopsAllowlist(ctx context.Context, content, message string, baseVersion int) (*DevopsAllowlistPutResponse, error) {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("devops allowlist require game and env scope")
	}
	if _, err := parseDevopsAllowlist(content); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	rec, err := s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:         devopsAllowlistConfigKey,
		Content:     content,
		Format:      "json",
		GameID:      scope.GameID,
		Env:         scope.Env,
		Namespace:   model.ConfigNamespaceOps,
		Message:     message,
		BaseVersion: baseVersion,
	}, captureGatesAuthor(ctx))
	if err != nil {
		if errors.Is(err, model.ErrConfigVersionConflict) {
			return nil, errorx.NewConflict(err.Error())
		}
		return nil, err
	}
	return &DevopsAllowlistPutResponse{Version: rec.Version}, nil
}
