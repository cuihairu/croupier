// Package recovery defines business-neutral contracts for validating and
// applying journaled CDC changes against a destination database.
package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/journal"
)

type Mode string

const (
	ModeDryRun Mode = "dryRun"
	ModeApply  Mode = "apply"
)

// TargetConfig identifies a destination adapter without embedding credentials.
// DSNEnv names an environment variable resolved by the adapter at Open time;
// Options may contain only non-secret adapter settings.
type TargetConfig struct {
	ID      string            `json:"id"`
	Adapter string            `json:"adapter"`
	DSNEnv  string            `json:"dsnEnv"`
	Options map[string]string `json:"options,omitempty"`
}

// Validate rejects incomplete target configuration and common credential keys
// in Options. Credentials must be delivered through the secret environment.
func (c TargetConfig) Validate() error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Adapter) == "" {
		return errors.New("recovery: target id and adapter are required")
	}
	if !envName(c.DSNEnv) {
		return errors.New("recovery: target dsnEnv must be an environment variable name")
	}
	for key := range c.Options {
		lower := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
		for _, secretPart := range []string{"password", "passwd", "secret", "token", "credential", "dsn", "connectionstring"} {
			if strings.Contains(lower, secretPart) {
				return fmt.Errorf("recovery: target option %q looks sensitive; use a secret environment variable", key)
			}
		}
	}
	return nil
}

func envName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// Snapshot records the immutable base state from which a replay range starts.
// A provider must verify that its position precedes the requested event range
// and that every requested table is covered by the same consistent snapshot.
type Snapshot struct {
	ID             string           `json:"id"`
	SourceID       string           `json:"sourceId"`
	Position       cdc.Position     `json:"position"`
	Tables         []string         `json:"tables"`
	Consistent     bool             `json:"consistent"`
	SchemaVersions map[string]int64 `json:"schemaVersions,omitempty"`
}

// Plan describes an explicitly scoped recovery operation. Tables must be
// qualified exact names; wildcard or database-wide plans are rejected.
type Plan struct {
	RequestID            string       `json:"requestId"`
	Principal            string       `json:"principal"`
	SourceID             string       `json:"sourceId"`
	TargetID             string       `json:"targetId"`
	FromSequence         uint64       `json:"fromSequence"`
	ToSequence           uint64       `json:"toSequence"`
	FromPosition         cdc.Position `json:"fromPosition"`
	ToPosition           cdc.Position `json:"toPosition"`
	Tables               []string     `json:"tables"`
	Mode                 Mode         `json:"mode"`
	RequireBeforeImage   bool         `json:"requireBeforeImage,omitempty"`
	RequireSchemaHistory bool         `json:"requireSchemaHistory,omitempty"`
	Snapshot             Snapshot     `json:"snapshot"`
}

// Validate enforces a narrow, auditable recovery scope.
func (p Plan) Validate() error {
	for name, value := range map[string]string{
		"requestId": p.RequestID, "principal": p.Principal, "sourceId": p.SourceID, "targetId": p.TargetID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("recovery: %s is required", name)
		}
	}
	if p.FromSequence == 0 || p.ToSequence < p.FromSequence {
		return fmt.Errorf("recovery: invalid journal sequence range %d..%d", p.FromSequence, p.ToSequence)
	}
	if !p.FromPosition.Valid() || p.FromPosition.Value == "" || !p.ToPosition.Valid() || p.ToPosition.Value == "" ||
		p.FromPosition.Kind != p.ToPosition.Kind || p.Snapshot.Position.Kind != p.FromPosition.Kind || p.Snapshot.Position.Value == "" {
		return errors.New("recovery: snapshot and replay positions must use one supported source position kind")
	}
	if p.Mode != ModeDryRun && p.Mode != ModeApply {
		return fmt.Errorf("recovery: unsupported mode %q", p.Mode)
	}
	if len(p.Tables) == 0 {
		return errors.New("recovery: at least one exact table is required")
	}
	seen := make(map[string]struct{}, len(p.Tables))
	for _, table := range p.Tables {
		parts := strings.Split(table, ".")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(table, "*") {
			return fmt.Errorf("recovery: table %q must be an exact database.table name", table)
		}
		if _, ok := seen[table]; ok {
			return fmt.Errorf("recovery: duplicate table %q", table)
		}
		seen[table] = struct{}{}
	}
	if p.Snapshot.ID == "" || p.Snapshot.SourceID != p.SourceID || !p.Snapshot.Position.Valid() || !p.Snapshot.Consistent {
		return errors.New("recovery: a consistent source snapshot is required")
	}
	covered := make(map[string]struct{}, len(p.Snapshot.Tables))
	for _, table := range p.Snapshot.Tables {
		covered[table] = struct{}{}
	}
	for _, table := range p.Tables {
		if _, ok := covered[table]; !ok {
			return fmt.Errorf("recovery: snapshot %q does not cover table %q", p.Snapshot.ID, table)
		}
	}
	if p.RequireSchemaHistory {
		for _, table := range p.Tables {
			if p.Snapshot.SchemaVersions[table] <= 0 {
				return fmt.Errorf("recovery: snapshot has no schema version for table %q", table)
			}
		}
	}
	return nil
}

// Authorizer maps an authenticated caller to the exact source/target/table
// scope. Product-specific game/env authorization belongs in an adapter.
type Authorizer interface {
	Authorize(context.Context, string, Plan) error
}

// SnapshotVerifier validates snapshot provenance, consistency and confirms that
// Snapshot.Position precedes Plan.FromPosition using native source semantics.
type SnapshotVerifier interface {
	VerifySnapshot(context.Context, Snapshot, Plan) error
}

// TargetCapabilities states the minimum guarantees required by recovery.
type TargetCapabilities struct {
	AtomicTransactions     bool
	IdempotentTransactions bool
	SupportedOperations    []cdc.Operation
}

// Applier validates and applies a single source transaction. ApplyTransaction
// must commit all changes atomically or roll all of them back on error. It must
// be idempotent by requestID + transactionID to support retry after an unknown
// result (for example, a lost acknowledgement after commit).
type Applier interface {
	Capabilities(context.Context) (TargetCapabilities, error)
	// ValidateTransaction must be read-only and report schema, key and operation
	// incompatibilities before an apply transaction begins.
	ValidateTransaction(context.Context, Plan, []cdc.ChangeEvent) error
	// ApplyTransaction must either commit the whole batch or roll it all back.
	ApplyTransaction(context.Context, string, string, []cdc.ChangeEvent) error
}

// AuditRecord is intentionally metadata-only; row images and credentials must
// not be copied into the audit system.
type AuditRecord struct {
	RequestID  string    `json:"requestId"`
	Principal  string    `json:"principal"`
	SourceID   string    `json:"sourceId"`
	TargetID   string    `json:"targetId"`
	SnapshotID string    `json:"snapshotId"`
	Stage      string    `json:"stage"`
	Mode       Mode      `json:"mode"`
	Sequence   uint64    `json:"sequence,omitempty"`
	EventIDs   []string  `json:"eventIds,omitempty"`
	TableNames []string  `json:"tables,omitempty"`
	ErrorClass string    `json:"errorClass,omitempty"`
	At         time.Time `json:"at"`
}

// Auditor must durably append each record before returning nil.
type Auditor interface {
	Append(context.Context, AuditRecord) error
}

// Executor coordinates preflight, authorization, audit and journal replay.
type Executor struct {
	Journal      *journal.Journal
	TargetConfig TargetConfig
	Authorizer   Authorizer
	Snapshots    SnapshotVerifier
	Target       Applier
	Audit        Auditor
	Source       cdc.Capabilities
}

// Result reports the last transaction sequence accepted by the target (or
// validated by dry-run). A zero sequence means no transaction completed.
type Result struct {
	LastSequence uint64
	Transactions int
}

// Execute runs a validated, authorized replay plan. Dry-run performs snapshot,
// event and destination validation and emits audit records without writes.
func (x *Executor) Execute(ctx context.Context, plan Plan) (Result, error) {
	var result Result
	if err := plan.Validate(); err != nil {
		return result, err
	}
	if x.Journal == nil || x.Authorizer == nil || x.Snapshots == nil || x.Target == nil || x.Audit == nil {
		return result, errors.New("recovery: journal, authorizer, snapshot verifier, target and auditor are required")
	}
	if err := x.TargetConfig.Validate(); err != nil || x.TargetConfig.ID != plan.TargetID {
		if err != nil {
			return result, err
		}
		return result, errors.New("recovery: target config ID does not match plan target ID")
	}
	if plan.ToSequence > x.Journal.LastSequence() {
		err := fmt.Errorf("recovery: requested end sequence %d exceeds journal end %d", plan.ToSequence, x.Journal.LastSequence())
		_ = x.audit(ctx, plan, "preflight_failed", 0, nil, err)
		return result, err
	}
	if err := x.audit(ctx, plan, "requested", 0, nil, nil); err != nil {
		return result, fmt.Errorf("recovery: write request audit: %w", err)
	}
	if err := x.Authorizer.Authorize(ctx, plan.Principal, plan); err != nil {
		_ = x.audit(ctx, plan, "denied", 0, nil, err)
		return result, fmt.Errorf("recovery: authorization denied: %w", err)
	}
	if !x.Source.Snapshot || !x.Source.TransactionMetadata {
		err := errors.New("recovery: source lacks snapshot or transaction-boundary capability")
		_ = x.audit(ctx, plan, "preflight_failed", 0, nil, err)
		return result, err
	}
	if plan.RequireBeforeImage && !x.Source.BeforeImage {
		err := errors.New("recovery: source does not provide required before images")
		_ = x.audit(ctx, plan, "preflight_failed", 0, nil, err)
		return result, err
	}
	if plan.RequireSchemaHistory && !x.Source.SchemaHistory {
		err := errors.New("recovery: source does not provide schema history")
		_ = x.audit(ctx, plan, "preflight_failed", 0, nil, err)
		return result, err
	}
	if err := x.Snapshots.VerifySnapshot(ctx, plan.Snapshot, plan); err != nil {
		_ = x.audit(ctx, plan, "preflight_failed", 0, nil, err)
		return result, fmt.Errorf("recovery: verify snapshot: %w", err)
	}
	caps, err := x.Target.Capabilities(ctx)
	if err != nil {
		_ = x.audit(ctx, plan, "preflight_failed", 0, nil, err)
		return result, fmt.Errorf("recovery: read target capabilities: %w", err)
	}
	if !caps.AtomicTransactions || !caps.IdempotentTransactions {
		err := errors.New("recovery: target must provide atomic and idempotent transactions")
		_ = x.audit(ctx, plan, "preflight_failed", 0, nil, err)
		return result, err
	}
	if err := x.audit(ctx, plan, "started", 0, nil, nil); err != nil {
		return result, fmt.Errorf("recovery: write start audit: %w", err)
	}
	var supported = make(map[cdc.Operation]struct{}, len(caps.SupportedOperations))
	for _, op := range caps.SupportedOperations {
		supported[op] = struct{}{}
	}
	sink := &recoverySink{executor: x, plan: plan, supported: supported}
	last, err := x.Journal.Replay(ctx, plan.FromSequence, plan.ToSequence, sink)
	result.LastSequence = last
	result.Transactions = sink.transactions
	if err != nil {
		_ = x.audit(ctx, plan, "failed", last, nil, err)
		return result, err
	}
	if err := x.audit(ctx, plan, "completed", last, nil, nil); err != nil {
		return result, fmt.Errorf("recovery: replay completed but completion audit failed: %w", err)
	}
	return result, nil
}

func (x *Executor) audit(ctx context.Context, plan Plan, stage string, sequence uint64, ids []string, cause error) error {
	row := AuditRecord{
		RequestID: plan.RequestID, Principal: plan.Principal, SourceID: plan.SourceID,
		TargetID: plan.TargetID, SnapshotID: plan.Snapshot.ID, Stage: stage, Mode: plan.Mode, Sequence: sequence,
		EventIDs: append([]string(nil), ids...), TableNames: append([]string(nil), plan.Tables...), At: time.Now().UTC(),
	}
	if cause != nil {
		row.ErrorClass = fmt.Sprintf("%T", cause)
	}
	return x.Audit.Append(ctx, row)
}

type recoverySink struct {
	executor     *Executor
	plan         Plan
	supported    map[cdc.Operation]struct{}
	transactions int
}

func (s *recoverySink) Apply(ctx context.Context, events []cdc.ChangeEvent) error {
	if len(events) == 0 {
		return errors.New("recovery: empty source transaction")
	}
	allowedTables := make(map[string]struct{}, len(s.plan.Tables))
	for _, table := range s.plan.Tables {
		allowedTables[table] = struct{}{}
	}
	ids := make([]string, 0, len(events))
	for _, event := range events {
		if event.SourceID != s.plan.SourceID {
			return fmt.Errorf("recovery: event source %q is outside plan scope", event.SourceID)
		}
		if _, ok := allowedTables[event.QualifiedTable()]; !ok {
			return fmt.Errorf("recovery: event table %q is outside plan scope", event.QualifiedTable())
		}
		if _, ok := s.supported[event.Operation]; !ok {
			return fmt.Errorf("recovery: target does not support operation %q", event.Operation)
		}
		if event.ID == "" {
			return errors.New("recovery: journal event has no stable ID")
		}
		sum := sha256.Sum256([]byte(event.ID))
		ids = append(ids, hex.EncodeToString(sum[:]))
	}
	if err := s.executor.Target.ValidateTransaction(ctx, s.plan, events); err != nil {
		_ = s.executor.audit(ctx, s.plan, "transaction_validation_failed", 0, ids, err)
		return err
	}
	if err := s.executor.audit(ctx, s.plan, "transaction_validated", 0, ids, nil); err != nil {
		return err
	}
	if s.plan.Mode == ModeApply {
		txID := events[0].Tx.ID
		if txID == "" {
			txID = events[0].ID
		}
		if err := s.executor.Target.ApplyTransaction(ctx, s.plan.RequestID, txID, events); err != nil {
			_ = s.executor.audit(ctx, s.plan, "transaction_failed", 0, ids, err)
			return err
		}
		if err := s.executor.audit(ctx, s.plan, "transaction_applied", 0, ids, nil); err != nil {
			return fmt.Errorf("target transaction applied but audit failed: %w", err)
		}
	}
	s.transactions++
	return nil
}

// NormalizeTables canonicalizes a table list for stable plan hashes and audits.
func NormalizeTables(tables []string) []string {
	result := append([]string(nil), tables...)
	sort.Strings(result)
	return result
}
