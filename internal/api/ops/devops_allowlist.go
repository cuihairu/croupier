// devops 会话白名单面板下发（agent-core 简档 §7.6；capture gates 同款通道
// 语义）：读写 game 库 config_versions 键 devops.allowlist（namespace=ops）。
// allowlist_json 对 server 半透明——本层只做解析 lint，语义校验复用
// agents/devops/config 的 ParseAllowlistJSON + ValidateAllowlist（与 agent
// apply 同一校验真值，发布预检在发布前拦下「agent 必拒」的坏版本）。发布
// 成功 ≠ agent 已生效，生效真值是 agent 侧拉取行为（configsync 30s 轮询
// + "devops allowlist version applied" 日志）。回滚不写历史：取目标版本
// 内容再发一版。
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

// devopsAllowlistConfigKey 白名单在 config_versions 里的固定键（wire 拉取
// 侧 internal/server 同名常量，两侧键一致是 wire 契约）。
const devopsAllowlistConfigKey = "devops.allowlist"

// DevopsAllowlistGetResponse 白名单当前发布态。Version=0 表示该 scope 尚未
// 发布过（content 为空串）——agent 语义为保持文件配置兜底。
type DevopsAllowlistGetResponse struct {
	Key       string `json:"key"`
	Version   int    `json:"version"`
	Content   string `json:"content"`
	Namespace string `json:"namespace,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
}

// DevopsAllowlistPutRequest 发布白名单新版本（整份内容，非增量补丁）。
// BaseVersion 乐观锁基线：发布期间被他人抢先发布则 409 拒绝；0=不启用。
type DevopsAllowlistPutRequest struct {
	Content     string `json:"content"`
	Message     string `json:"message,omitempty"`
	BaseVersion int    `json:"baseVersion,omitempty"`
}

// DevopsAllowlistPutResponse 发布结果（新版本号）。
type DevopsAllowlistPutResponse struct {
	Version int `json:"version"`
}

// DevopsAllowlistRollbackRequest 以历史版本内容再发一版（版本日志不可变）。
type DevopsAllowlistRollbackRequest struct {
	Version int    `json:"version"`
	Message string `json:"message,omitempty"`
}

// DevopsAllowlistValidateRequest dry-run 校验（发布预检，不落库）。
type DevopsAllowlistValidateRequest struct {
	Content string `json:"content"`
}

// DevopsAllowlistValidateResponse 校验通过时回生效预览（编辑面口径；失败走
// 422，错误文本即 agent apply 会报的同一句）。
type DevopsAllowlistValidateResponse struct {
	Valid   bool `json:"valid"`
	Entries int  `json:"entries"`
	Ports   int  `json:"ports"`
}

// DevopsAllowlistVersionItem 版本历史条目（不含内容：内容只在 latest GET 回
// 显；回滚走 POST rollback 由服务端取历史，前端不搬运整份白名单 JSON）。
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
// 无 scope 依赖（纯内容校验）；失败语义与发布一致（422 + 同一错误文本）。
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

// DevopsAllowlistGet 返回当前 scope（顶栏 game/env）已发布白名单最新版本。
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

// DevopsAllowlistPut 校验后发布新版本（namespace=ops，version 递增；
// baseVersion>0 时乐观锁防并发丢更新）。
func (s *Service) DevopsAllowlistPut(ctx context.Context, req *DevopsAllowlistPutRequest) (*DevopsAllowlistPutResponse, error) {
	if req == nil || strings.TrimSpace(req.Content) == "" {
		return nil, errorx.NewValidationError("devops allowlist content is required")
	}
	return s.publishDevopsAllowlist(ctx, strings.TrimSpace(req.Content), strings.TrimSpace(req.Message), req.BaseVersion)
}

// DevopsAllowlistValidate 发布预检 dry-run：解析 + ValidateAllowlist（agent
// apply 同一校验真值）。不落库、不依赖 scope。
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

// DevopsAllowlistRollback 以历史版本内容再发一版：版本日志不可变，回滚本身
// 也是一次新发布（新版本号，agent 侧照常版本轮询热生效）。历史内容重新过
// 当前校验：校验规则可能随服务端版本演进，回退到会被当前规则拒绝的内容必须
// 显式失败，而不是静默发布坏版本。
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
	// baseVersion=0：回滚目标是显式版本号，内容取自服务端历史而非过期编辑态。
	return s.publishDevopsAllowlist(ctx, content, message, 0)
}

// DevopsAllowlistVersionsGet 返回当前 scope 的版本历史（新→旧）。未发布回
// 空数组而非 404——空历史是常态而非错误。
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

// parseDevopsAllowlist 发布/预检共用解析+校验（agent apply 同一真值）。
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

// publishDevopsAllowlist 校验后落库递增版本（发布与回滚共用路径）。game/env
// scope 必须非空：devops-agent 注册即携带 scope，空 env 的发布永远拉不到，
// 宁可发布时拒绝。
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
