// capture 闸规则面板下发（capture-agent 简档 §6/§8.5）：读写 game 库
// config_versions 键 capture.gates（namespace=ops）。gates_json 对 server
// 半透明——本层只做结构 lint（顶层对象 + 三段形状），语义校验（表路由
// 唯一性/阈值取值等）由 capture-agent 在 apply 时全量执行（validator of
// last resort，非法即 keep last-good）；发布成功 ≠ agent 已生效，生效
// 真值是 agent 侧 "capture gates version applied" 日志与拉取行为。
package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// captureGatesConfigKey 闸规则在 config_versions 里的固定键（agent 拉取
// 侧同名常量在 agents/capture/runner，两侧键一致是 wire 契约）。
const captureGatesConfigKey = "capture.gates"

// CaptureGatesGetResponse 闸规则当前发布态。Version=0 表示该 scope 尚未
// 发布过（content 为空串）——agent 语义为保持文件规则。
type CaptureGatesGetResponse struct {
	Key       string `json:"key"`
	Version   int    `json:"version"`
	Content   string `json:"content"`
	Namespace string `json:"namespace,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
}

// CaptureGatesPutRequest 发布闸规则新版本（整份内容，非增量补丁）。
type CaptureGatesPutRequest struct {
	Content string `json:"content"`
	Message string `json:"message,omitempty"`
}

// CaptureGatesPutResponse 发布结果（新版本号）。
type CaptureGatesPutResponse struct {
	Version int `json:"version"`
}

// CaptureGatesGet handles GET /api/v1/ops/capture/gates.
func (h *Handler) CaptureGatesGet(c *gin.Context) {
	resp, err := h.service.CaptureGatesGet(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CaptureGatesPut handles PUT /api/v1/ops/capture/gates.
func (h *Handler) CaptureGatesPut(c *gin.Context) {
	var req CaptureGatesPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.CaptureGatesPut(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CaptureGatesGet 返回当前 scope（顶栏 game/env）已发布的闸规则最新版本。
func (s *Service) CaptureGatesGet(ctx context.Context) (*CaptureGatesGetResponse, error) {
	scope := svc.GameScopeFromContext(ctx)
	empty := &CaptureGatesGetResponse{Key: captureGatesConfigKey}
	rec, err := s.svcCtx.ConfigVersionModel.FindLatestByScope(ctx, captureGatesConfigKey, scope.GameID, scope.Env)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return empty, nil
		}
		return nil, err
	}
	return &CaptureGatesGetResponse{
		Key:       rec.Key,
		Version:   rec.Version,
		Content:   rec.Value,
		Namespace: rec.Namespace,
		UpdatedAt: rec.UpdatedAt.Format("2006-01-02 15:04:05"),
		CreatedBy: rec.CreatedBy,
	}, nil
}

// CaptureGatesPut 结构 lint 后发布新版本（namespace=ops，version 递增）。
// game/env scope 必须非空：capture-agent 注册即携带 scope（Agent 清单 §7），
// 空 env 的发布永远拉不到，宁可发布时拒绝。
func (s *Service) CaptureGatesPut(ctx context.Context, req *CaptureGatesPutRequest) (*CaptureGatesPutResponse, error) {
	if req == nil || strings.TrimSpace(req.Content) == "" {
		return nil, errorx.NewValidationError("capture gates content is required")
	}
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("capture gates require game and env scope")
	}
	content := strings.TrimSpace(req.Content)
	if err := lintCaptureGatesJSON(content); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	rec, err := s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:       captureGatesConfigKey,
		Content:   content,
		Format:    "json",
		GameID:    scope.GameID,
		Env:       scope.Env,
		Namespace: model.ConfigNamespaceOps,
		Message:   strings.TrimSpace(req.Message),
	}, captureGatesAuthor(ctx))
	if err != nil {
		return nil, err
	}
	return &CaptureGatesPutResponse{Version: rec.Version}, nil
}

// lintCaptureGatesJSON 结构 lint：合法 JSON + 顶层对象 + 已知段形状
// （reconcile/lineage 的 rules、cep 的 offHours/burst/probing、event 的 ops
// 必须是数组）。未知键放行（agent 侧规则集前向扩展不需要改面板）；数组
// 元素内容不查——那是 agent 的语义校验职责。
func lintCaptureGatesJSON(content string) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &top); err != nil {
		return fmt.Errorf("gates json is not a valid object: %w", err)
	}
	for _, section := range []string{"reconcile", "cep", "lineage", "event"} {
		raw, ok := top[section]
		if !ok {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return fmt.Errorf("gates.%s must be an object", section)
		}
		for _, list := range []string{"rules", "offHours", "burst", "probing", "ops"} {
			arr, ok := fields[list]
			if !ok {
				continue
			}
			var items []json.RawMessage
			if err := json.Unmarshal(arr, &items); err != nil || items == nil {
				return fmt.Errorf("gates.%s.%s must be an array", section, list)
			}
		}
	}
	return nil
}

// captureGatesAuthor 发布人（与 api/config 同口径：ctx username，缺省 system）。
func captureGatesAuthor(ctx context.Context) string {
	if ctx == nil {
		return "system"
	}
	if username, ok := ctx.Value("username").(string); ok {
		if trimmed := strings.TrimSpace(username); trimmed != "" {
			return trimmed
		}
	}
	return "system"
}
