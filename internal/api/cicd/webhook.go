package cicd

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

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
// 鉴权：integration 配置了 Token 时要求请求头 X-CICD-Token 精确匹配
// （constant-time 比较），未配置 Token 的接入为开放端点（边界见交付说明）。
// 幂等：按 (integration_id, external_id) upsert，重复投递只更新状态。
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
	if want := strings.TrimSpace(row.Token); want != "" {
		if !constantTimeEqual(gotToken, want) {
			return nil, errorx.NewUnauthorized("webhook token 不匹配")
		}
	}
	status, verr := model.ValidateCicdWebhook(p.Pipeline, p.ExternalID, p.Status)
	if verr != nil {
		return nil, errorx.NewBadRequest(verr.Error())
	}
	now := time.Now()
	build := &model.CicdBuild{
		GameID:           firstNonEmpty(row.GameID, p.GameID),
		Env:              firstNonEmpty(row.Env, p.Env),
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
	return &WebhookResponse{Build: buildBuildDTO(saved)}, nil
}

// constantTimeEqual 用 crypto/subtle 防时序侧信道（空串一律不等）。
func constantTimeEqual(a, b string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
