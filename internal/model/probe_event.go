package model

import (
	"context"
	"strings"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// ProbeEvent stores accepted synthetic probe uplinks for durable ops queries.
type ProbeEvent struct {
	ID         uint      `gorm:"primaryKey"`
	CreatedAt  time.Time `gorm:"index"`
	AgentID    string    `gorm:"size:128;index"`
	GameID     string    `gorm:"size:128;index"`
	Env        string    `gorm:"size:64;index"`
	Kind       string    `gorm:"size:32;index"`
	Target     string    `gorm:"size:512;index"`
	Status     string    `gorm:"size:32;index"`
	LatencyMS  int64
	ErrorClass string `gorm:"size:64;index"`
	Payload    []byte `gorm:"type:blob"`
}

func (ProbeEvent) TableName() string { return "probe_events" }

type ProbeEventModel struct{ db *gorm.DB }

func NewProbeEventModel(db *gorm.DB) *ProbeEventModel { return &ProbeEventModel{db: db} }

func (m *ProbeEventModel) Create(ctx context.Context, agentID, gameID, env string, ev *opsv1.AlertEvent) error {
	if m == nil || m.db == nil {
		return gorm.ErrInvalidDB
	}
	data, err := proto.Marshal(ev)
	if err != nil {
		return err
	}
	row := ProbeEvent{CreatedAt: time.Now(), AgentID: agentID, GameID: gameID, Env: env, Kind: ev.GetKind(), Payload: data}
	if w := ev.GetWindow(); w != nil {
		row.Target = w.GetTarget()
		row.Status = w.GetWindowKind()
		row.ErrorClass = probeErrorClass(w.GetLastReason())
	}
	if d := ev.GetDetail(); d != nil {
		row.Target = d.GetTarget()
		row.Status = "failed"
		if d.GetOk() {
			row.Status = "success"
		}
		row.LatencyMS = d.GetLatencyMs()
		row.ErrorClass = probeErrorClass(d.GetErr())
	}
	return m.db.WithContext(ctx).Create(&row).Error
}

func (m *ProbeEventModel) Recent(ctx context.Context, gameID, env, agentID string, limit int) ([]ProbeEvent, error) {
	if m == nil || m.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	q := m.db.WithContext(ctx).Model(&ProbeEvent{}).Where("game_id = ? AND env = ?", gameID, env)
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	var rows []ProbeEvent
	err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func probeErrorClass(err string) string {
	switch {
	case err == "":
		return ""
	case containsFold(err, "timeout", "deadline"):
		return "timeout"
	case containsFold(err, "refused", "reset", "unreachable", "connect"):
		return "network"
	case containsFold(err, "status", "http"):
		return "http"
	default:
		return "probe"
	}
}

func containsFold(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(strings.ToLower(s), n) {
			return true
		}
	}
	return false
}
