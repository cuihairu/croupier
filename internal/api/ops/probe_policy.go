// probe 采样策略面板下发（评审简档 §13.3 C）：读写 game 库 config_versions
// 键 probe.policy（namespace=ops，与 wire 拉取侧 internal/server
// handleProbePolicyConfig 同键）。校验与 agent apply 同一真值——
// core/healthprobe.ParseProbePolicy（发布预检在发布前拦下「agent 必拒」的坏
// 版本）；发布成功 ≠ agent 已生效，生效真值是 agent 侧拉取行为（configsync
// 30s 轮询 + 下轮探测热跟随）。回滚不写历史：取目标版本内容再发一版。
package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// probePolicyConfigKey 策略在 config_versions 里的固定键（wire 拉取侧
// internal/server 同名常量，两侧键一致是 wire 契约）。
const probePolicyConfigKey = "probe.policy"

// ProbePolicyGetResponse 策略当前发布态。Version=0 表示该 scope 尚未发布过
// （content 为空串）——agent 语义为保持本地配置间隔。
type ProbePolicyGetResponse struct {
	Key       string `json:"key"`
	Version   int    `json:"version"`
	Content   string `json:"content"`
	Namespace string `json:"namespace,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
}

// ProbePolicyPutRequest 发布策略新版本（整份内容，非增量补丁）。BaseVersion
// 乐观锁基线：发布期间被他人抢先发布则 409 拒绝；0=不启用（回滚接口不适
// 用锁——内容取自服务端而非过期编辑态）。
type ProbePolicyPutRequest struct {
	Content     string `json:"content"`
	Message     string `json:"message,omitempty"`
	BaseVersion int    `json:"baseVersion,omitempty"`
}

// ProbePolicyPutResponse 发布结果（新版本号）。
type ProbePolicyPutResponse struct {
	Version int `json:"version"`
}

// ProbePolicyValidateRequest dry-run 校验（发布预检，不落库）：ParseProbePolicy
// 与 agent apply 同一校验真值。
type ProbePolicyValidateRequest struct {
	Content string `json:"content"`
}

// ProbePolicyValidateResponse 校验通过时回生效预览（编辑面口径；失败走 422，
// 错误文本即 agent apply 会报的同一句）。
type ProbePolicyValidateResponse struct {
	Valid        bool   `json:"valid"`
	BaseInterval string `json:"baseInterval"`
	Targets      int    `json:"targets"`
	Windows      int    `json:"windows"`
	Events       int    `json:"events"`
	AutoBoostOn  bool   `json:"autoBoostOn"`
}

// ProbePolicyRollbackRequest 以历史版本内容再发一版（版本日志不可变，回滚
// 本身也是一次新发布，不改写历史）。
type ProbePolicyRollbackRequest struct {
	Version int    `json:"version"`
	Message string `json:"message,omitempty"`
}

// ProbePolicyVersionItem 版本历史条目（不含内容：内容只在 latest GET 回显）。
type ProbePolicyVersionItem struct {
	Version   int    `json:"version"`
	Message   string `json:"message,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

// ProbePolicyGet handles GET /api/v1/ops/probe/policy.
func (h *Handler) ProbePolicyGet(c *gin.Context) {
	resp, err := h.service.ProbePolicyGet(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ProbePolicyPut handles PUT /api/v1/ops/probe/policy.
func (h *Handler) ProbePolicyPut(c *gin.Context) {
	var req ProbePolicyPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.ProbePolicyPut(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ProbePolicyValidate handles POST /api/v1/ops/probe/policy/validate.
// 无 scope 依赖（纯内容校验）；失败语义与发布一致（422 + 同一错误文本）。
func (h *Handler) ProbePolicyValidate(c *gin.Context) {
	var req ProbePolicyValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.ProbePolicyValidate(&req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ProbePolicyRollback handles POST /api/v1/ops/probe/policy/rollback.
func (h *Handler) ProbePolicyRollback(c *gin.Context) {
	var req ProbePolicyRollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.ProbePolicyRollback(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ProbePolicyVersionsGet handles GET /api/v1/ops/probe/policy/versions.
func (h *Handler) ProbePolicyVersionsGet(c *gin.Context) {
	resp, err := h.service.ProbePolicyVersionsGet(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ProbePolicyGet 返回当前 scope（顶栏 game/env）已发布的策略最新版本。
func (s *Service) ProbePolicyGet(ctx context.Context) (*ProbePolicyGetResponse, error) {
	scope := svc.GameScopeFromContext(ctx)
	empty := &ProbePolicyGetResponse{Key: probePolicyConfigKey}
	rec, err := s.svcCtx.ConfigVersionModel.FindLatestByScope(ctx, probePolicyConfigKey, scope.GameID, scope.Env)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return empty, nil
		}
		return nil, err
	}
	return &ProbePolicyGetResponse{
		Key:       rec.Key,
		Version:   rec.Version,
		Content:   rec.Value,
		Namespace: rec.Namespace,
		UpdatedAt: rec.UpdatedAt.Format("2006-01-02 15:04:05"),
		CreatedBy: rec.CreatedBy,
	}, nil
}

// ProbePolicyPut 校验后发布新版本（namespace=ops，version 递增；baseVersion>0
// 时乐观锁防并发丢更新）。
func (s *Service) ProbePolicyPut(ctx context.Context, req *ProbePolicyPutRequest) (*ProbePolicyPutResponse, error) {
	if req == nil || strings.TrimSpace(req.Content) == "" {
		return nil, errorx.NewValidationError("probe policy content is required")
	}
	return s.publishProbePolicy(ctx, strings.TrimSpace(req.Content), strings.TrimSpace(req.Message), req.BaseVersion)
}

// ProbePolicyValidate 发布预检 dry-run：ParseProbePolicy（agent apply 同一校验
// 真值）。不落库、不依赖 scope。没有这一步时，坏语义版本会「发布成功、
// 全员 agent apply 拒绝滞留 last-good」——预检把拦截点提前到发布前。
func (s *Service) ProbePolicyValidate(req *ProbePolicyValidateRequest) (*ProbePolicyValidateResponse, error) {
	content := ""
	if req != nil {
		content = strings.TrimSpace(req.Content)
	}
	p, err := healthprobe.ParseProbePolicy(content)
	if err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	return &ProbePolicyValidateResponse{
		Valid:        true,
		BaseInterval: p.BaseInterval().String(),
		Targets:      len(p.Targets),
		Windows:      len(p.Windows),
		Events:       len(p.Events),
		AutoBoostOn:  p.AutoBoostEnabled(),
	}, nil
}

// publishProbePolicy 校验后落库递增版本（发布与回滚共用路径）。game/env
// scope 必须非空：agent 注册即携带 scope，空 env 的发布永远拉不到，宁可发
// 布时拒绝。
func (s *Service) publishProbePolicy(ctx context.Context, content, message string, baseVersion int) (*ProbePolicyPutResponse, error) {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("probe policy requires game and env scope")
	}
	if _, err := healthprobe.ParseProbePolicy(content); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	rec, err := s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:         probePolicyConfigKey,
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
	return &ProbePolicyPutResponse{Version: rec.Version}, nil
}

// ProbePolicyRollback 以历史版本内容再发一版：历史内容重新过当前校验——
// 校验规则可能随版本演进，回退到会被当前规则拒绝的内容必须显式失败。
func (s *Service) ProbePolicyRollback(ctx context.Context, req *ProbePolicyRollbackRequest) (*ProbePolicyPutResponse, error) {
	if req == nil || req.Version <= 0 {
		return nil, errorx.NewValidationError("probe policy rollback version is required")
	}
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("probe policy requires game and env scope")
	}
	rec, err := s.svcCtx.ConfigVersionModel.FindByScope(ctx, probePolicyConfigKey, req.Version, scope.GameID, scope.Env)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound(fmt.Sprintf("probe policy version %d not found in scope %s/%s", req.Version, scope.GameID, scope.Env))
		}
		return nil, err
	}
	content := strings.TrimSpace(rec.Value)
	message := fmt.Sprintf("rollback from version %d", req.Version)
	if extra := strings.TrimSpace(req.Message); extra != "" {
		message = fmt.Sprintf("%s: %s", message, extra)
	}
	// baseVersion=0：回滚目标是显式版本号，内容取自服务端历史而非过期编辑
	// 态，乐观锁不适用。
	return s.publishProbePolicy(ctx, content, message, 0)
}

// ProbePolicyVersionsGet 返回当前 scope 的版本历史（新→旧）。未发布回空
// 数组而非 404——空历史是常态而非错误。
func (s *Service) ProbePolicyVersionsGet(ctx context.Context) ([]ProbePolicyVersionItem, error) {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("probe policy requires game and env scope")
	}
	recs, err := s.svcCtx.ConfigVersionModel.ListByScope(ctx, probePolicyConfigKey, scope.GameID, scope.Env)
	if err != nil {
		return nil, err
	}
	items := make([]ProbePolicyVersionItem, 0, len(recs))
	for _, rec := range recs {
		items = append(items, ProbePolicyVersionItem{
			Version:   rec.Version,
			Message:   rec.Message,
			CreatedBy: rec.CreatedBy,
			UpdatedAt: rec.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return items, nil
}
