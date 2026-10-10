package alert

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
)

// inboundSourceClosure 入站来源闭集（plugin-mechanism 设计 §5.3）。
var inboundSourceClosure = map[string]bool{
	"alertmanager": true,
	"generic":      true,
}

// defaultAlertmanagerLabelMapping alertmanager labels 键→告警字段默认映射
// （配置驱动，可在 alertInbound.sources.alertmanager.labelMapping 覆盖）。
var defaultAlertmanagerLabelMapping = map[string]string{
	"severity": "level",
	"summary":  "message",
	"instance": "type",
}

const (
	// inboundTimestampWindow 防重放时间窗（±5 分钟）。
	inboundTimestampWindow = 5 * time.Minute
	// inboundKindAlertInbound generic 来源固定 kind。
	inboundKindAlertInbound = "alert.inbound"
	// inboundKindAlertManager alertmanager 来源固定 kind。
	inboundKindAlertManager = "alertmanager"
)

// InboundResponse 入站 webhook 响应。
type InboundResponse struct {
	Alert   Alert `json:"alert"`
	Created bool  `json:"created"` // false = 幂等去重（已存在）
}

// Inbound handles POST /api/v1/alerts/inbound/{source}.
//
// 鉴权：X-Croupier-Signature（HMAC-SHA256 over raw body）+ X-Croupier-Timestamp
//
//	（±5 分钟防重放）。密钥走配置 secretEnv 环境变量引用（同 herald tokenEnv 口径）。
//
// 幂等：eventId → Alert.AlertID 唯一键，同 id 重推不重复落库。
func (h *Handler) Inbound(c *gin.Context) {
	source := c.Param("source")
	if !inboundSourceClosure[source] {
		response.Error(c, errorx.NewNotFound("unknown inbound source"))
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		response.Error(c, errorx.NewBadRequest("read body failed"))
		return
	}

	srcCfg := h.service.svcCtx.Config.AlertInbound.Sources[source]
	if srcCfg.SecretEnv == "" {
		response.Error(c, errorx.NewForbidden("source not configured"))
		return
	}
	secret := os.Getenv(srcCfg.SecretEnv)
	if secret == "" {
		response.Error(c, errorx.NewForbidden("source secret not provided"))
		return
	}

	// HMAC 验签
	if !verifyInboundSignature(secret, body, c.GetHeader("X-Croupier-Signature")) {
		h.service.auditInboundRejected(c, source, "signature_invalid")
		response.Error(c, errorx.NewBadRequestWithCode("signature_invalid", "签名校验失败", nil))
		return
	}

	// 时间戳防重放
	if !verifyInboundTimestamp(c.GetHeader("X-Croupier-Timestamp")) {
		h.service.auditInboundRejected(c, source, "timestamp_replay")
		response.Error(c, errorx.NewBadRequestWithCode("timestamp_invalid", "时间戳无效或已过期", nil))
		return
	}

	dto, created, err := h.service.ingestInbound(c.Request.Context(), source, body, srcCfg)
	if err != nil {
		response.Error(c, err)
		return
	}
	if created {
		response.Created(c, InboundResponse{Alert: dto, Created: true})
	} else {
		response.Success(c, InboundResponse{Alert: dto, Created: false})
	}
}

// ingestInbound 解析 payload → 映射 Alert → 幂等落库。
func (s *Service) ingestInbound(ctx context.Context, source string, body []byte, srcCfg config.AlertInboundSourceConfig) (Alert, bool, error) {
	if s.svcCtx.AlertModel == nil {
		return Alert{}, false, errors.New("告警模型未初始化")
	}

	var alert *model.Alert
	var err error
	if source == "alertmanager" {
		alert, err = mapAlertmanagerPayload(body, srcCfg.LabelMapping)
	} else {
		alert, err = mapGenericPayload(body, source)
	}
	if err != nil {
		return Alert{}, false, err
	}

	row, created, err := s.svcCtx.AlertModel.UpsertByAlertID(ctx, alert)
	if err != nil {
		return Alert{}, false, err
	}
	return toAlertDTO(row), created, nil
}

// inboundGenericPayload generic 来源直接使用 §5.2 信封。
type inboundGenericPayload struct {
	Kind             string `json:"kind"`
	Severity         string `json:"severity"`
	EventID          string `json:"eventId"`
	DedupKey         string `json:"dedupKey"`
	Title            string `json:"title"`
	Body             string `json:"body"`
	OccurredAtUnixMs int64  `json:"occurredAtUnixMs"`
	Scope            struct {
		GameID  string `json:"gameId"`
		Env     string `json:"env"`
		AgentID string `json:"agentId"`
	} `json:"scope"`
	Labels map[string]string `json:"labels"`
}

// mapGenericPayload 把 §5.2 信封映射为 model.Alert。
func mapGenericPayload(body []byte, source string) (*model.Alert, error) {
	var p inboundGenericPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, errorx.NewBadRequest("invalid generic payload: " + err.Error())
	}
	if p.EventID == "" {
		return nil, errorx.NewBadRequest("eventId is required")
	}

	level := p.Severity
	if level == "" {
		level = "warning"
	}
	message := p.Title
	if p.Body != "" {
		if message != "" {
			message += ": " + p.Body
		} else {
			message = p.Body
		}
	}

	details := map[string]interface{}{
		"kind":             p.Kind,
		"eventId":          p.EventID,
		"dedupKey":         p.DedupKey,
		"occurredAtUnixMs": p.OccurredAtUnixMs,
	}
	if p.Scope.GameID != "" || p.Scope.Env != "" || p.Scope.AgentID != "" {
		details["scope"] = p.Scope
	}
	if len(p.Labels) > 0 {
		details["labels"] = p.Labels
	}

	return &model.Alert{
		AlertID: p.EventID,
		Type:    inboundKindAlertInbound,
		Level:   level,
		Message: message,
		Source:  source,
		Status:  "firing",
		Details: details,
	}, nil
}

// inboundAMAlert Alertmanager 兼容 payload 的单条告警。
type inboundAMAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
}

// inboundAlertmanagerPayload Alertmanager 兼容 payload。
type inboundAlertmanagerPayload struct {
	Alerts []inboundAMAlert `json:"alerts"`
}

// mapAlertmanagerPayload 把 Alertmanager payload 映射为 model.Alert（取首条）。
func mapAlertmanagerPayload(body []byte, labelMapping map[string]string) (*model.Alert, error) {
	var p inboundAlertmanagerPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, errorx.NewBadRequest("invalid alertmanager payload: " + err.Error())
	}
	if len(p.Alerts) == 0 {
		return nil, errorx.NewBadRequest("alerts is empty")
	}
	a := p.Alerts[0]

	if labelMapping == nil {
		labelMapping = defaultAlertmanagerLabelMapping
	}
	level, message, alertType, status, details := mapAMFields(a, labelMapping)

	return &model.Alert{
		AlertID: alertmanagerEventID(a),
		Type:    alertType,
		Level:   level,
		Message: message,
		Source:  "alertmanager",
		Status:  status,
		Details: details,
	}, nil
}

// mapAMFields 按映射配置把 alertmanager labels/annotations 映射为告警字段。
// 映射目标：level / message / type / status；其余进 details。
func mapAMFields(a inboundAMAlert, mapping map[string]string) (level, message, alertType, status string, details map[string]interface{}) {
	details = map[string]interface{}{}

	for k, v := range a.Labels {
		switch mapping[k] {
		case "level":
			level = v
		case "message":
			message = v
		case "type":
			alertType = v
		case "status":
			status = v
		default:
			details["label."+k] = v
		}
	}

	if message == "" {
		if s := a.Annotations["summary"]; s != "" {
			message = s
		} else if d := a.Annotations["description"]; d != "" {
			message = d
		}
	}
	if alertType == "" {
		alertType = inboundKindAlertManager
	}
	// 优先级：label 映射 status > payload 自带 status > firing 默认。
	if status == "" {
		status = a.Status
	}
	if status == "" {
		status = "firing"
	}
	if len(a.Annotations) > 0 {
		details["annotations"] = a.Annotations
	}
	if a.GeneratorURL != "" {
		details["generatorURL"] = a.GeneratorURL
	}
	return
}

// alertmanagerEventID 从 labels+status 派生确定性事件 id（幂等键）。
func alertmanagerEventID(a inboundAMAlert) string {
	h := sha256.New()
	keys := make([]string, 0, len(a.Labels))
	for k := range a.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(a.Labels[k]))
		h.Write([]byte(";"))
	}
	h.Write([]byte(a.Status))
	return "am:" + hex.EncodeToString(h.Sum(nil))[:16]
}

// verifyInboundSignature 校验 HMAC-SHA256 签名（常量时间比较）。
func verifyInboundSignature(secret string, body []byte, sig string) bool {
	if sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

// verifyInboundTimestamp 校验时间戳在 ±5 分钟窗口内。
func verifyInboundTimestamp(ts string) bool {
	if ts == "" {
		return false
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	age := time.Since(time.Unix(sec, 0))
	return age >= 0 && age <= inboundTimestampWindow
}

// auditInboundRejected 记录入站拒绝审计事件。
func (s *Service) auditInboundRejected(c *gin.Context, source, reason string) {
	if s.svcCtx.AuditService == nil {
		return
	}
	s.svcCtx.AuditService.Log(c.Request.Context(),
		audit.AuditEventType("alert.inbound_rejected"),
		audit.WithResourceID("alert_inbound", source),
		audit.WithDetails(map[string]interface{}{"source": source, "reason": reason}),
		audit.WithOutcome("rejected", reason),
	)
}

// toAlertDTO 把 model.Alert 转为 API DTO。
func toAlertDTO(a *model.Alert) Alert {
	return Alert{
		Id:        a.AlertID,
		Type:      a.Type,
		Level:     a.Level,
		Message:   a.Message,
		Source:    a.Source,
		Status:    a.Status,
		Details:   a.Details,
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
	}
}
