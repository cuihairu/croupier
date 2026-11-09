package audit

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/common/dbtype"

	"gorm.io/gorm"
)

// ErrChainSequenceConflict 标记 hash 链 sequence 分配与库内已有行冲突
// （chain_sequence 唯一约束拒绝）。多实例部署下 sequence 由各实例
// read-modify-write 分配（memCache 视角），并发窗口内会撞号；调用方
// （AuditService.Log）据此重查真实链尾重算后重试。
var ErrChainSequenceConflict = errors.New("audit chain sequence conflict")

// isChainSequenceConflict 跨方言识别 chain_sequence 唯一约束冲突：
// postgres 报约束名（idx_audit_records_chain_sequence），sqlite 报
// "UNIQUE constraint failed: audit_records.chain_sequence"。其他唯一
// 约束（audit_id）误判无害——重算链后重试仍失败则原样返回。
func isChainSequenceConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "idx_audit_records_chain_sequence") ||
		strings.Contains(msg, "audit_records.chain_sequence")
}

// AuditModel is the GORM model for audit records
type AuditModel struct {
	ID             uint        `gorm:"primaryKey;autoIncrement" json:"id"`
	AuditID        string      `gorm:"uniqueIndex;type:varchar(255)" json:"auditId"`
	Timestamp      time.Time   `gorm:"not null;index" json:"timestamp"`
	EventType      string      `gorm:"type:varchar(100);not null;index" json:"eventType"`
	Category       string      `gorm:"type:varchar(50);not null;index" json:"category"`
	Severity       string      `gorm:"type:varchar(20);not null;index" json:"severity"`
	ActorJSON      dbtype.JSON `json:"actorJson"`
	Action         string      `gorm:"type:varchar(255)" json:"action"`
	ResourceJSON   dbtype.JSON `json:"resourceJson"`
	DetailsJSON    dbtype.JSON `json:"detailsJson"`
	ChangesJSON    dbtype.JSON `json:"changesJson"`
	ContextJSON    dbtype.JSON `json:"contextJson"`
	Outcome        string      `gorm:"type:varchar(50);not null;index" json:"outcome"`
	ErrorMessage   string      `gorm:"type:text" json:"errorMessage"`
	ChainHash      string      `gorm:"type:varchar(64);not null;index" json:"chainHash"`
	ChainPrevHash  string      `gorm:"type:varchar(64)" json:"chainPrevHash"`
	ChainSequence  int64       `gorm:"not null;uniqueIndex" json:"chainSequence"`
	ChainSignerID  string      `gorm:"type:varchar(255)" json:"chainSignerId"`
	ChainSignature string      `gorm:"type:text" json:"chainSignature"`
	// ChainVersion pins the hash algorithm/field set: 1 = legacy DJB2,
	// 2 = canonical SHA-256 (ARCH-008). Added by migration 0049; existing
	// rows backfilled to 1.
	ChainVersion int `gorm:"not null;default:1" json:"chainVersion"`
	// Promoted first-class columns derived from Resource/Details at write
	// time (and backfilled for legacy rows). They make game/env scoping and
	// invocation analytics aggregable in plain SQL, keeping queries
	// dialect-neutral (postgres/sqlite have no portable JSON_EXTRACT).
	GameID     string    `gorm:"type:varchar(100);index" json:"gameId,omitempty"`
	Env        string    `gorm:"type:varchar(50);index" json:"env,omitempty"`
	FunctionID string    `gorm:"type:varchar(255);index" json:"functionId,omitempty"`
	DurationMs int64     `gorm:"not null;default:0" json:"durationMs,omitempty"`
	IP         string    `gorm:"type:varchar(64);index" json:"ip,omitempty"`
	ActorID    string    `gorm:"type:varchar(255);index" json:"actorId,omitempty"`
	CreatedAt  time.Time `gorm:"not null" json:"createdAt"`
}

// TableName returns the table name
func (AuditModel) TableName() string {
	return "audit_records"
}

// ToRecord converts model to domain type
func (m *AuditModel) ToRecord() (*AuditRecord, error) {
	record := &AuditRecord{
		ID:           m.AuditID,
		Timestamp:    m.Timestamp,
		EventType:    AuditEventType(m.EventType),
		Category:     AuditCategory(m.Category),
		Severity:     AuditSeverity(m.Severity),
		Action:       m.Action,
		Outcome:      m.Outcome,
		ErrorMessage: m.ErrorMessage,
		ChainInfo: ChainInfo{
			Hash:         m.ChainHash,
			PrevHash:     m.ChainPrevHash,
			Sequence:     m.ChainSequence,
			SignerID:     m.ChainSignerID,
			Signature:    m.ChainSignature,
			ChainVersion: m.ChainVersion,
		},
	}

	if m.ActorJSON != nil {
		if err := json.Unmarshal(m.ActorJSON, &record.Actor); err != nil {
			return nil, err
		}
	}

	if m.ResourceJSON != nil {
		if err := json.Unmarshal(m.ResourceJSON, &record.Resource); err != nil {
			return nil, err
		}
	}

	if m.DetailsJSON != nil {
		if err := json.Unmarshal(m.DetailsJSON, &record.Details); err != nil {
			return nil, err
		}
	}

	if m.ChangesJSON != nil {
		if err := json.Unmarshal(m.ChangesJSON, &record.Changes); err != nil {
			return nil, err
		}
	}

	if m.ContextJSON != nil {
		if err := json.Unmarshal(m.ContextJSON, &record.Context); err != nil {
			return nil, err
		}
	}

	return record, nil
}

// FromRecord creates model from domain type
func FromRecord(r *AuditRecord) (*AuditModel, error) {
	model := &AuditModel{
		AuditID:        r.ID,
		Timestamp:      r.Timestamp,
		EventType:      string(r.EventType),
		Category:       string(r.Category),
		Severity:       string(r.Severity),
		Action:         r.Action,
		Outcome:        r.Outcome,
		ErrorMessage:   r.ErrorMessage,
		ChainHash:      r.ChainInfo.Hash,
		ChainPrevHash:  r.ChainInfo.PrevHash,
		ChainSequence:  r.ChainInfo.Sequence,
		ChainSignerID:  r.ChainInfo.SignerID,
		ChainSignature: r.ChainInfo.Signature,
		CreatedAt:      time.Now(),
	}

	// ActorInfo/ResourceInfo/AuditContext 均为纯 string/bool/[]string/
	// map[string]string 字段，Marshal 恒成功，err 分支为死代码，已删除
	//（Details/Changes 含 interface{}，仍保留 err 检查并已有 NaN 注入用例）。
	// ChainVersion 0（未设置）按版本 1（旧算法）落库，与 calculateHash 的
	// 0→legacy 分派一致。
	chainVersion := r.ChainInfo.ChainVersion
	if chainVersion == 0 {
		chainVersion = ChainVersionLegacy
	}
	model.ChainVersion = chainVersion
	if r.Actor.ID != "" {
		data, _ := json.Marshal(r.Actor)
		model.ActorJSON = data
	}

	if r.Resource.ID != "" {
		data, _ := json.Marshal(r.Resource)
		model.ResourceJSON = data
	}

	if r.Details != nil {
		data, err := json.Marshal(r.Details)
		if err != nil {
			return nil, err
		}
		model.DetailsJSON = data
	}

	if r.Changes != nil {
		data, err := json.Marshal(r.Changes)
		if err != nil {
			return nil, err
		}
		model.ChangesJSON = data
	}

	if r.Context.RequestID != "" {
		data, _ := json.Marshal(r.Context)
		model.ContextJSON = data
	}

	model.GameID, model.Env, model.FunctionID, model.DurationMs = derivePromotedFields(r)
	model.IP = strings.TrimSpace(r.Actor.IPAddress)
	model.ActorID = strings.TrimSpace(r.Actor.ID)

	return model, nil
}

// derivePromotedFields extracts the queryable dimensions (game/env/
// function/duration) from a record's Resource and Details payloads so they
// can be stored as first-class columns. Explicit Resource fields win over
// Details keys; duration prefers duration_ms and falls back to elapsed_ms
// (console page.execute semantics).
func derivePromotedFields(r *AuditRecord) (gameID, env, functionID string, durationMs int64) {
	if r == nil {
		return "", "", "", 0
	}
	gameID = strings.TrimSpace(r.Resource.GameID)
	env = strings.TrimSpace(r.Resource.Environment)
	if r.Details != nil {
		if gameID == "" {
			gameID, _ = r.Details["game_id"].(string)
		}
		if env == "" {
			env, _ = r.Details["env"].(string)
		}
		if v, ok := r.Details["function_id"].(string); ok {
			functionID = v
		}
		if d, ok := detailsNumeric(r.Details["duration_ms"]); ok {
			durationMs = int64(d)
		} else if d, ok := detailsNumeric(r.Details["elapsed_ms"]); ok {
			durationMs = int64(d)
		}
	}
	if functionID == "" && r.Resource.Type == "function" {
		functionID = strings.TrimSpace(r.Resource.ID)
	}
	return gameID, env, functionID, durationMs
}

// detailsNumeric coerces JSON numbers (float64 after unmarshal) and ints.
func detailsNumeric(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

// SQLAuditStore implements AuditStore using SQL
type SQLAuditStore struct {
	db       *gorm.DB
	memCache *auditMemCache
}

type auditMemCache struct {
	records map[string]*AuditRecord
	latest  *AuditRecord
	mu      sync.RWMutex
}

// NewSQLAuditStore creates a new SQL audit store
func NewSQLAuditStore(db *gorm.DB) (*SQLAuditStore, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	// Schema note: the audit table is created by the versioned migration
	// baseline (internal/svc autoMigrate/autoMigrateMeta). Constructors must
	// not run DDL (docs/architecture/database-migration-strategy.md).

	store := &SQLAuditStore{
		db: db,
		memCache: &auditMemCache{
			records: make(map[string]*AuditRecord),
		},
	}

	// Load latest record into cache
	var model AuditModel
	if err := db.Order("chain_sequence DESC").First(&model).Error; err == nil {
		record, _ := model.ToRecord()
		store.memCache.latest = record
	}

	// Best-effort backfill of promoted columns for legacy invocation rows
	// (written before game_id/env/function_id/duration_ms were first-class
	// columns). Dialect-neutral: parse JSON payloads in Go, batched and
	// capped so startup stays fast.
	store.backfillPromotedFields()

	return store, nil
}

// backfillPromotedFields fills game_id/env/function_id/duration_ms for
// function.invoke / page.execute rows that predate the promoted columns.
func (s *SQLAuditStore) backfillPromotedFields() {
	const (
		batchSize = 500
		maxRows   = 50000
	)
	updated := 0
	for updated < maxRows {
		var rows []AuditModel
		if err := s.db.
			Where("event_type IN ? AND (function_id = '' OR function_id IS NULL)", []string{string(EventFunctionInvoke), string(EventPageExecute)}).
			Order("id").Limit(batchSize).
			Find(&rows).Error; err != nil {
			return
		}
		if len(rows) == 0 {
			return
		}
		fixed := 0
		for _, m := range rows {
			record, err := m.ToRecord()
			if err != nil {
				continue
			}
			gameID, env, functionID, durationMs := derivePromotedFields(record)
			if functionID == "" {
				// Nothing derivable; this row would match the SELECT
				// forever, so don't count it as progress.
				continue
			}
			if err := s.db.Model(&AuditModel{}).Where("id = ?", m.ID).Updates(map[string]interface{}{
				"gameId":      gameID,
				"env":         env,
				"function_id": functionID,
				"duration_ms": durationMs,
			}).Error; err != nil {
				return
			}
			fixed++
			updated++
		}
		if len(rows) < batchSize || fixed == 0 {
			return
		}
	}
}
