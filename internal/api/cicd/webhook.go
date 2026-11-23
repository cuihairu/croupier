package cicd

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

var errBadPathID = errorx.NewBadRequest("路径 id 无效")

// WebhookPayload 是外部 CI 系统状态回写的通用轻量契约（generic 字段命名；
// 各 provider 的原生 webhook 由调用侧（Jenkins HTTP Request step / GitLab
// / GitHub 侧转发脚本）归一到此形态——平台不做全量原生 payload 解析）。
type WebhookPayload struct {
	Pipeline    string            `json:"pipeline"`
	ExternalID  string            `json:"externalId"`
	Status      string            `json:"status"`
	Version     string            `json:"version"`
	WebURL      string            `json:"webUrl"`
	ArtifactURL string            `json:"artifactUrl"`
	Checksum    string            `json:"checksum"`
	GameID      string            `json:"gameId"`
	Env         string            `json:"env"`
	Extra       map[string]string `json:"extra"`
}

// Webhook handles POST /cicd/webhooks/:id —— 外部系统构建状态回写。
//
// 鉴权：要求请求头 X-CICD-Token 与 integration 配置的 Token 精确匹配
// （constant-time 比较）。未配置 Token 的接入一律拒绝投递（fail-closed，
// AUD-COV-001 P1：构建状态可伪造版本/artifact/校验和，无凭据的开放回写
// 曾是审计确认的越权写入口）。幂等：按 (integration_id, external_id)
// upsert，重复投递只更新状态。
func (h *Handler) Webhook(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		response.Error(c, errBadPathID)
		return
	}
	var payload WebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, err)
		return
	}
	resp, err := h.service.IngestWebhook(c.Request.Context(), id, c.GetHeader("X-CICD-Token"), &payload)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

type WebhookResponse struct {
	Build Build `json:"build"`
}

// IngestWebhook 校验令牌与必填并幂等落库。
func (s *Service) IngestWebhook(ctx context.Context, integrationID uint, gotToken string, p *WebhookPayload) (*WebhookResponse, error) {
	row, err := s.integrationByID(ctx, integrationID)
	if err != nil {
		return nil, err
	}
	// fail-closed：token 未配置 = 拒绝投递，不再退化为开放端点
	want := strings.TrimSpace(row.Token)
	if want == "" {
		// AUD-049：denied 路径同样留痕（外部 token 面，actor 记固定机器身份）。
		audit.LogWrite(s.svcCtx.AuditService, ctx, audit.EventCicdWebhookDenied,
			audit.StateDenied, webhookActor, "cicd_integration", fmt.Sprint(row.ID),
			map[string]interface{}{"reason": "token_unconfigured", "pipeline": p.Pipeline, "externalId": p.ExternalID})
		return nil, errorx.NewUnauthorized("integration 未配置 webhook token，拒绝回写")
	}
	if !constantTimeEqual(gotToken, want) {
		audit.LogWrite(s.svcCtx.AuditService, ctx, audit.EventCicdWebhookDenied,
			audit.StateDenied, webhookActor, "cicd_integration", fmt.Sprint(row.ID),
			map[string]interface{}{"reason": "token_mismatch", "pipeline": p.Pipeline, "externalId": p.ExternalID})
		return nil, errorx.NewUnauthorized("webhook token 不匹配")
	}
	// AUD-043：回写 payload 的 game/env 只可确认 integration 配置，不可注入。
	// 外部 CI 自报的作用域不可信，此前 integration 未配作用域时 payload 可
	// 写入任意 scope（跨域写入）。统一 fail-closed：payload 提供作用域时必须
	// 与 integration 精确一致，否则拒绝；未提供时作用域仅由 integration 派生。
	gameID, env, serr := validateWebhookScope(row.GameID, row.Env, p.GameID, p.Env)
	if serr != nil {
		// AUD-049：跨 scope / scope 注入回写是外部写入尝试，留痕 denied。
		audit.LogWrite(s.svcCtx.AuditService, ctx, audit.EventCicdWebhookDenied,
			audit.StateDenied, webhookActor, "cicd_integration", fmt.Sprint(row.ID),
			map[string]interface{}{"reason": "scope_conflict", "pipeline": p.Pipeline, "externalId": p.ExternalID})
		return nil, serr
	}
	status, verr := model.ValidateCicdWebhook(p.Pipeline, p.ExternalID, p.Status)
	if verr != nil {
		audit.LogWrite(s.svcCtx.AuditService, ctx, audit.EventCicdWebhookDenied,
			audit.StateDenied, webhookActor, "cicd_integration", fmt.Sprint(row.ID),
			map[string]interface{}{"reason": "invalid_payload", "pipeline": p.Pipeline, "externalId": p.ExternalID})
		return nil, errorx.NewBadRequest(verr.Error())
	}
	now := time.Now()
	build := &model.CicdBuild{
		GameID:           gameID,
		Env:              env,
		IntegrationID:    row.ID,
		Kind:             row.Kind,
		Pipeline:         firstNonEmpty(p.Pipeline, "default"),
		ExternalID:       strings.TrimSpace(p.ExternalID),
		Status:           status,
		TriggeredBy:      "webhook",
		Version:          strings.TrimSpace(p.Version),
		WebURL:           p.WebURL,
		ArtifactURL:      p.ArtifactURL,
		ArtifactChecksum: p.Checksum,
	}
	if status == model.CicdBuildSuccess || status == model.CicdBuildFailed || status == model.CicdBuildCancelled {
		build.FinishedAt = &now
	}
	raw := datatypes.JSONMap{}
	for k, v := range p.Extra {
		raw[k] = v
	}
	build.Raw = raw
	id, err := s.svcCtx.CicdBuildModel.UpsertByExternalKey(ctx, build)
	if err != nil {
		return nil, err
	}
	saved, err := s.svcCtx.CicdBuildModel.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// AUD-049：成功回写留痕（状态推进是外部 CI 驱动的数据变更）。
	audit.LogWrite(s.svcCtx.AuditService, ctx, audit.EventCicdWebhookIngest,
		audit.StateSuccess, webhookActor, "cicd_build", fmt.Sprint(saved.ID),
		map[string]interface{}{
			"integrationId": row.ID, "pipeline": build.Pipeline, "externalId": build.ExternalID,
			"status": build.Status, "gameId": build.GameID, "env": build.Env,
		})
	return &WebhookResponse{Build: buildBuildDTO(saved)}, nil
}

// webhookActor 是外部 CI 回写的固定审计身份（外部系统无平台用户，
// actor 无法取自 JWT；记录经 eventType + integrationId 关联到具体接入）。
const webhookActor = "cicd-webhook"

// constantTimeEqual 用 crypto/subtle 防时序侧信道（空串一律不等）。
func constantTimeEqual(a, b string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// validateWebhookScope 把回写 payload 的 game/env 绑定到 integration 配置
// （AUD-043）。integration 是作用域的唯一来源（创建/编辑时由授权操作员声明），
// 外部 CI 的 payload 只能复述确认、不能注入或替换：
//
//	payload 未提供（空） → 放行，作用域取 integration 配置；
//	payload 与配置精确一致 → 放行，作用域取 integration 配置；
//	payload 冲突但 integration 已配该作用域 → 拒绝（跨 scope 回写）；
//	payload 提供了 integration 未配置的作用域 → 拒绝（scope 注入）。
//
// 返回 integration 派生的 (gameID, env)，供构建记录落库，payload 值永不被
// 采信写入。
func validateWebhookScope(rowGameID, rowEnv, payloadGameID, payloadEnv string) (string, string, error) {
	rowGameID = strings.TrimSpace(rowGameID)
	rowEnv = strings.TrimSpace(rowEnv)
	if err := bindScopeField("game_id", rowGameID, strings.TrimSpace(payloadGameID)); err != nil {
		return "", "", err
	}
	if err := bindScopeField("env", rowEnv, strings.TrimSpace(payloadEnv)); err != nil {
		return "", "", err
	}
	return rowGameID, rowEnv, nil
}

// bindScopeField 校验单个作用域字段：payload 与 integration 配置不一致即拒绝。
func bindScopeField(field, configured, fromPayload string) error {
	if fromPayload == "" || fromPayload == configured {
		return nil
	}
	if configured == "" {
		return errorx.NewBadRequestWithCode("scope_mismatch",
			fmt.Sprintf("webhook 回写 game/env 与 integration 配置不一致：该接入未配置 %s，不能由 payload 注入（请先在 integration 上声明作用域）", field),
			nil)
	}
	return errorx.NewBadRequestWithCode("scope_mismatch",
		fmt.Sprintf("webhook 回写 game/env 与 integration 配置不一致，已拒绝跨作用域回写（%s）", field),
		nil)
}
