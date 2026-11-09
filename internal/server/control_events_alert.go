package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// handleAlertEvent 落库 capture 规则命中告警（幂等：alertID 由
// agent|rule|gtid|table|key|ts 哈希派生，重推折叠为 duplicate），随后
// 可选分发出口（Manager nil = 只落库）。severity 经闸层校验只有
// warn/critical，仍防御性归一：未知值按 warning 处理。
func (s *ControlService) handleAlertEvent(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.CaptureRuleHitReport{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal CaptureRuleHitReport: %w", err)
	}

	s.mu.RLock()
	alertModel := s.alertModel
	outletManager := s.outletManager
	s.mu.RUnlock()
	if alertModel == nil {
		return nil, fmt.Errorf("alert model not configured")
	}

	rule := strings.TrimSpace(req.GetRule())
	table := strings.TrimSpace(req.GetTable())
	if rule == "" || table == "" {
		return nil, fmt.Errorf("rule and table are required")
	}
	severity := strings.ToLower(strings.TrimSpace(req.GetSeverity()))
	switch severity {
	case "critical":
		severity = outlet.SeverityCritical
	case "warn", "warning":
		severity = outlet.SeverityWarning
	default:
		severity = outlet.SeverityWarning
	}

	detail := req.GetDetail()
	occurredAt := req.GetEventTsUnixMs()
	if occurredAt <= 0 {
		occurredAt = time.Now().UnixMilli()
	}
	alertID := captureAlertID(req.GetAgentId(), rule, req.GetGtid(), table, req.GetKey(), occurredAt)
	title := fmt.Sprintf("[%s] %s", rule, table)

	alert := &model.Alert{
		AlertID: alertID,
		Type:    outlet.KindCaptureRuleHit,
		Level:   severity,
		Message: strings.TrimSpace(title + ": " + detail),
		Source:  "capture",
		Status:  "firing",
		Details: map[string]interface{}{
			"kind":             outlet.KindCaptureRuleHit,
			"rule":             rule,
			"table":            table,
			"key":              req.GetKey(),
			"detail":           detail,
			"gtid":             req.GetGtid(),
			"eventOp":          req.GetEventOp(),
			"occurredAtUnixMs": occurredAt,
			"scope": map[string]interface{}{
				"gameId":  req.GetGameId(),
				"env":     req.GetEnv(),
				"agentId": req.GetAgentId(),
			},
		},
		// 4862：capture 告警归属 = 请求 scope（game/env 已在 Details.scope 内）。
		GameID: req.GetGameId(),
		Env:    req.GetEnv(),
	}
	_, created, err := alertModel.UpsertByAlertID(ctx, alert)
	if err != nil {
		return nil, fmt.Errorf("persist capture alert: %w", err)
	}

	if outletManager != nil {
		outletManager.Dispatch(outlet.AlertEvent{
			Kind:             outlet.KindCaptureRuleHit,
			Severity:         severity,
			EventID:          alertID,
			DedupKey:         alertID,
			Title:            title,
			Body:             detail,
			OccurredAtUnixMs: occurredAt,
			Scope: outlet.Scope{
				GameID:  req.GetGameId(),
				Env:     req.GetEnv(),
				AgentID: req.GetAgentId(),
			},
			Metadata: map[string]string{
				"rule":  rule,
				"table": table,
			},
		})
	}

	resp := &opsv1.CaptureRuleHitAck{Accepted: true, Duplicate: !created}
	return proto.Marshal(resp)
}

func (s *ControlService) normalizeAlertEventRequest(data []byte, identity *AgentIdentity) (*opsv1.CaptureRuleHitReport, error) {
	req := &opsv1.CaptureRuleHitReport{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal CaptureRuleHitReport: %w", err)
	}
	req.AgentId = identity.AgentID
	req.GameId = identity.GameID
	req.Env = identity.Env
	return req, nil
}

func (s *ControlService) handleAlertEventAsAgent(ctx context.Context, data []byte, identity *AgentIdentity) ([]byte, error) {
	req, err := s.normalizeAlertEventRequest(data, identity)
	if err != nil {
		return nil, err
	}
	normalized, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal CaptureRuleHitReport: %w", err)
	}
	return s.handleAlertEvent(ctx, normalized)
}

// captureAlertID 派生 ≤64 字符的幂等键（model.Alert.AlertID 是
// size:64 唯一索引）："capture." + 摘要前 16 hex。
func captureAlertID(agentID, rule, gtid, table, key string, occurredAt int64) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		agentID, rule, gtid, table, key, strconv.FormatInt(occurredAt, 10),
	}, "|")))
	return "capture." + hex.EncodeToString(sum[:])[:16]
}

// captureConfigKey config_versions 里的闸规则键（namespace=ops；面板与
// agent 两侧同键）。gates_json 对 server 不透明——闸层结构由 agent 在
// apply 时全量校验（validator of last resort）。
const captureConfigKey = "capture.gates"

// handleCaptureConfig 响应 capture-agent 的闸规则下行拉取：从注册会话取
// scope（game/env），读 config_versions 最新版本回给 agent。无记录/读失败/
// 分库解析失败统一回空版本（agent 语义 = 保持现状）——拉取是周期轮询，
// 单轮读异常不应放大为通道错误打断节奏。
func (s *ControlService) handleCaptureConfig(ctx context.Context, data []byte) ([]byte, error) {
	req := &opsv1.CaptureGatesConfigRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal CaptureGatesConfigRequest: %w", err)
	}

	s.mu.RLock()
	configModel := s.configModel
	gameDBs := s.gameDBs
	s.mu.RUnlock()
	if configModel == nil {
		return nil, fmt.Errorf("config model not configured")
	}

	agentID := strings.TrimSpace(req.GetAgentId())
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	s.registry.Mu().RLock()
	sess := s.registry.AgentsUnsafe()[agentID]
	s.registry.Mu().RUnlock()
	if sess == nil {
		return nil, fmt.Errorf("agent session not found: %s", agentID)
	}
	gameID, env := strings.TrimSpace(sess.GameID), strings.TrimSpace(sess.Env)
	if gameID == "" || env == "" {
		return nil, fmt.Errorf("agent %s missing game scope", agentID)
	}

	dbCtx := ctx
	if gameDBs != nil {
		resolved, _, err := gameDBs.Resolve(ctx, gameID, env)
		if err != nil {
			s.logger.Warn("capture config: resolve game db failed", "agent", agentID,
				"gameId", gameID, "env", env, "error", err)
			return proto.Marshal(&opsv1.CaptureGatesConfigResponse{})
		}
		dbCtx = resolved
	}

	rec, err := configModel.FindLatestByScope(dbCtx, captureConfigKey, gameID, env)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("capture config: read config_versions failed", "agent", agentID,
				"gameId", gameID, "env", env, "error", err)
		}
		return proto.Marshal(&opsv1.CaptureGatesConfigResponse{})
	}
	return proto.Marshal(&opsv1.CaptureGatesConfigResponse{
		Version:   strconv.Itoa(rec.Version),
		GatesJson: rec.Value,
	})
}
