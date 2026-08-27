package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FunctionMeta describes a function capability on an agent.
type FunctionMeta struct {
	Enabled bool
	Version string

	Tags        []string
	Summary     string
	Description string
	OperationID string
	Deprecated  bool

	InputSchema  string
	OutputSchema string

	Resource          string
	Operation         string
	Capability        string
	Execution         string
	ApprovalRequired  bool
	ApprovalPolicyKey string
	Risk              string
	Permission        string
}

// ProviderSession represents a single provider registered to an agent (via SDK->Agent local registry).
type ProviderSession struct {
	ProviderID   string
	GameID       string
	Env          string
	Addr         string
	Version      string
	SDKLanguage  string // SDK 语言（go, java, python, cpp, csharp, custom）
	SDKVersion   string // SDK 版本
	SDKName      string // SDK 显示名（如 croupier-js-sdk），用户可自定义
	LastSeenUnix int64
	FunctionIDs  []string
	OpenAPIDoc   json.RawMessage
}

// AgentSession represents a registered agent instance in the registry.
// RPCAddr is retained only as a compatibility mirror while the runtime moves
// to session-first routing.
type AgentSession struct {
	AgentID   string
	GameID    string
	Env       string
	Addr      string
	Version   string
	Region    string
	Zone      string
	Labels    map[string]string
	Functions map[string]FunctionMeta
	Providers []ProviderSession
	ExpireAt  time.Time
	LastSeen  time.Time // 最后活跃时间
}

// Store keeps lightweight agent registry state in-memory.
type Store struct {
	mu sync.RWMutex
	// registrationMu serializes local registration projection changes. It keeps
	// snapshot classification and cross-database compensation based on one
	// coherent in-memory agent view.
	registrationMu sync.Mutex
	agents         map[string]*AgentSession // agent_id -> session
	// OpenAPI operations storage (replaces legacy manifest)
	openapiOperations    map[string]*openapi3.Operation  // function_id -> OpenAPI operation
	openapiProviders     map[string]*OpenAPIProviderCaps // provider_id -> OpenAPI caps
	registrationWarnings map[string]*FunctionRegistrationWarning
	// Optional database for dual-write persistence
	db *gorm.DB
	// scopeContext resolves the DB/scope context used by game-scoped
	// contract rebuilds triggered outside an HTTP request.
	scopeContext func(gameID, env string) context.Context
	// Contract service for FunctionContract persistence (optional)
	contractService contractMaterializer
}

// contractMaterializer is the registration-side projection boundary. It keeps
// registry snapshots independent from persistence and page-generation details.
type contractMaterializer interface {
	RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta spec.FunctionContractInput) error
	RemoveFunctionContract(ctx context.Context, gameID, env, functionID string) (resourceKey string, err error)
	RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error
	RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error
	RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error
}

type FunctionRegistrationWarning struct {
	Key        string
	GameID     string
	Env        string
	AgentID    string
	FunctionID string
	Version    string
	Code       string
	Message    string
	Count      int
	FirstSeen  time.Time
	LastSeen   time.Time
}

type RegistrationWarningFilter struct {
	GameID     string
	Env        string
	AgentID    string
	FunctionID string
	Code       string
	Status     string
	Limit      int
}

// functionSnapshotDiff is the deterministic difference between two complete
// registration snapshots. It is intentionally internal to the registry
// projection path; callers consume the resulting materialized state.
type functionSnapshotDiff struct {
	Added     []string
	Changed   []string
	Removed   []string
	Resources []string
}

// OpenAPIProviderCaps represents provider capabilities in OpenAPI format.
type OpenAPIProviderCaps struct {
	ID        string
	Version   string
	Lang      string
	SDK       string
	UpdatedAt time.Time
	// OpenAPI 3.0.3 document as JSON
	OpenAPIDoc []byte
}

func NewStore() *Store {
	return &Store{
		agents:               map[string]*AgentSession{},
		openapiOperations:    make(map[string]*openapi3.Operation),
		openapiProviders:     make(map[string]*OpenAPIProviderCaps),
		registrationWarnings: make(map[string]*FunctionRegistrationWarning),
		db:                   nil,
		scopeContext:         defaultScopeContext,
	}
}

// NewStoreWithDB creates a new Store with database dual-write enabled.
func NewStoreWithDB(db *gorm.DB) *Store {
	return &Store{
		agents:               map[string]*AgentSession{},
		openapiOperations:    make(map[string]*openapi3.Operation),
		openapiProviders:     make(map[string]*OpenAPIProviderCaps),
		registrationWarnings: make(map[string]*FunctionRegistrationWarning),
		db:                   db,
		scopeContext:         defaultScopeContext,
	}
}

// SetContractService sets the contract service for FunctionContract persistence.
func (s *Store) SetContractService(svc contractMaterializer) {
	s.contractService = svc
}

// SessionPersistenceEnabled reports whether the registry itself persists
// AgentSession records. ControlService uses this to avoid writing the same
// session twice when it also receives an AgentSessionLoader.
func (s *Store) SessionPersistenceEnabled() bool {
	return s != nil && s.db != nil
}

// SetScopeContextResolver configures how background registration rebuilds
// acquire the correct game-scoped database context.
func (s *Store) SetScopeContextResolver(resolve func(gameID, env string) context.Context) {
	if resolve == nil {
		s.scopeContext = defaultScopeContext
		return
	}
	s.scopeContext = resolve
}

// Mu exposes the lock for read/update operations when callers need batch views.
func (s *Store) Mu() *sync.RWMutex { return &s.mu }

// AgentsUnsafe returns the internal agents map without copying. Callers MUST hold Mu().RLock/Lock.
func (s *Store) AgentsUnsafe() map[string]*AgentSession { return s.agents }

// UpsertAgent inserts or updates an agent session by AgentID.
// Contract and proposal materialization is part of registration and must
// succeed before the session becomes visible in the runtime registry.
func (s *Store) UpsertAgent(a *AgentSession) error {
	if a == nil || a.AgentID == "" {
		return nil
	}
	if err := validateAgentFunctionContracts(a.Functions); err != nil {
		return err
	}
	s.registrationMu.Lock()
	defer s.registrationMu.Unlock()

	previousSession := s.previousAgentSession(a.AgentID)
	previous := functionSnapshot(previousSession)
	diff := classifyFunctionSnapshot(previous, a.Functions)
	scopeCtx := s.rebuildContext(a.GameID, a.Env)

	materialize := s.materializeAgent

	// A single-database deployment can use one database transaction. A
	// database-per-game deployment uses a durable operation record plus
	// compensation: the game projection commits first, then the meta session
	// and operation status commit together. Any meta failure restores the
	// previous game projection immediately or leaves an explicit recovery row.
	if s.db != nil && dbctx.Get(scopeCtx) == nil {
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			txCtx := dbctx.WithDB(scopeCtx, tx)
			if err := materialize(txCtx, a, diff); err != nil {
				return err
			}
			if err := s.writeToDB(txCtx, a); err != nil {
				return fmt.Errorf("write agent session to database: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}
	} else if s.db != nil && dbctx.Get(scopeCtx) != nil && s.contractService != nil && a.Functions != nil {
		operation, err := s.prepareRegistrationOperation(a, previousSession)
		if err != nil {
			return err
		}
		if err := s.materializeScopedTransaction(scopeCtx, a, materialize, diff); err != nil {
			s.markRegistrationOperation(operation.OperationID, "aborted", err)
			return err
		}
		metaErr := s.db.Transaction(func(tx *gorm.DB) error {
			txCtx := dbctx.WithDB(context.Background(), tx)
			if err := s.writeToDB(txCtx, a); err != nil {
				return fmt.Errorf("write agent session to database: %w", err)
			}
			return s.markRegistrationOperationWithDB(tx, operation.OperationID, "committed", "")
		})
		if metaErr != nil {
			previousForRestore := previousSession
			if previousForRestore == nil {
				previousForRestore = &AgentSession{
					AgentID:   a.AgentID,
					GameID:    a.GameID,
					Env:       a.Env,
					Functions: map[string]FunctionMeta{},
				}
			}
			reverseDiff := classifyFunctionSnapshot(a.Functions, previousForRestore.Functions)
			compensationErr := s.materializeScopedTransaction(scopeCtx, previousForRestore, materialize, reverseDiff)
			if compensationErr != nil {
				s.markRegistrationOperation(operation.OperationID, "compensation_required", compensationErr)
				return fmt.Errorf("write agent session to database: %w; registration compensation failed: %w", metaErr, compensationErr)
			}
			s.markRegistrationOperation(operation.OperationID, "compensated", metaErr)
			return metaErr
		}
	} else {
		if err := materialize(scopeCtx, a, diff); err != nil {
			return err
		}
		if s.db != nil {
			if err := s.writeToDB(context.Background(), a); err != nil {
				return fmt.Errorf("write agent session to database: %w", err)
			}
		}
	}

	// Always write to memory (primary store)
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.agents[a.AgentID]
	if cur == nil {
		s.agents[a.AgentID] = a
		return nil
	}
	// Merge minimal fields. RPCAddr remains a compatibility mirror and should
	// not be treated as the primary runtime route.
	cur.GameID, cur.Env, cur.Addr, cur.Version = a.GameID, a.Env, a.Addr, a.Version
	cur.Region, cur.Zone = a.Region, a.Zone
	// merge labels: new labels replace old ones
	if a.Labels != nil {
		if cur.Labels == nil {
			cur.Labels = make(map[string]string)
		}
		for k, v := range a.Labels {
			cur.Labels[k] = v
		}
	}
	if a.Functions != nil {
		cur.Functions = a.Functions
	}
	if a.Providers != nil {
		cur.Providers = a.Providers
	}
	cur.ExpireAt = a.ExpireAt
	cur.LastSeen = a.LastSeen
	return nil
}

func (s *Store) materializeScopedTransaction(
	scopeCtx context.Context,
	session *AgentSession,
	materialize func(context.Context, *AgentSession, functionSnapshotDiff) error,
	diff functionSnapshotDiff,
) error {
	scopeDB := dbctx.Resolve(scopeCtx, s.db)
	if scopeDB == nil {
		return fmt.Errorf("registration projection database is not initialized")
	}
	return scopeDB.Transaction(func(tx *gorm.DB) error {
		return materialize(dbctx.WithDB(scopeCtx, tx), session, diff)
	})
}

func (s *Store) materializeAgent(ctx context.Context, session *AgentSession, sessionDiff functionSnapshotDiff) error {
	if s.contractService == nil || session == nil || session.Functions == nil {
		return nil
	}
	var rebuildErrors []error
	resources := stringSet(sessionDiff.Resources)
	standaloneFunctions := make(map[string]bool)
	for _, functionID := range sortedFunctionIDs(session.Functions) {
		meta := session.Functions[functionID]
		if err := s.rebuildFunctionContract(ctx, session.GameID, session.Env, functionID, meta, resources, standaloneFunctions); err != nil {
			rebuildErrors = append(rebuildErrors, err)
		}
	}
	for _, functionID := range sessionDiff.Removed {
		if meta, ok := s.survivingFunctionMeta(session.AgentID, session.GameID, session.Env, functionID); ok {
			if err := s.rebuildFunctionContract(ctx, session.GameID, session.Env, functionID, meta, resources, standaloneFunctions); err != nil {
				rebuildErrors = append(rebuildErrors, err)
			}
			continue
		}
		resourceKey, err := s.contractService.RemoveFunctionContract(ctx, session.GameID, session.Env, functionID)
		if err != nil {
			rebuildErrors = append(rebuildErrors, fmt.Errorf("remove function contract %s: %w", functionID, err))
			continue
		}
		if resourceKey != "" {
			resources[resourceKey] = true
		}
	}
	for _, resource := range sortedStringSet(resources) {
		if err := s.contractService.RebuildResourceCapability(ctx, session.GameID, session.Env, resource); err != nil {
			rebuildErrors = append(rebuildErrors, fmt.Errorf("rebuild resource capability %s: %w", resource, err))
			continue
		}
		if err := s.contractService.RebuildProposalsForResource(ctx, session.GameID, session.Env, resource); err != nil {
			rebuildErrors = append(rebuildErrors, fmt.Errorf("rebuild page proposals for %s: %w", resource, err))
		}
	}
	for _, functionID := range sortedStringSet(standaloneFunctions) {
		if err := s.contractService.RebuildProposalForFunction(ctx, session.GameID, session.Env, functionID); err != nil {
			rebuildErrors = append(rebuildErrors, fmt.Errorf("rebuild standalone page proposal %s: %w", functionID, err))
		}
	}
	if len(rebuildErrors) > 0 {
		return fmt.Errorf("agent registration contract rebuild failed: %w", errors.Join(rebuildErrors...))
	}
	return nil
}

func (s *Store) previousAgentSession(agentID string) *AgentSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneAgentSession(s.agents[agentID])
}

func functionSnapshot(session *AgentSession) map[string]FunctionMeta {
	if session == nil || session.Functions == nil {
		return nil
	}
	functions := make(map[string]FunctionMeta, len(session.Functions))
	for functionID, meta := range session.Functions {
		functions[functionID] = meta
	}
	return functions
}

func cloneAgentSession(session *AgentSession) *AgentSession {
	if session == nil {
		return nil
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		return nil
	}
	var clone AgentSession
	if err := json.Unmarshal(encoded, &clone); err != nil {
		return nil
	}
	return &clone
}

func (s *Store) prepareRegistrationOperation(target, previous *AgentSession) (*AgentRegistrationOperationDB, error) {
	targetJSON, err := json.Marshal(target)
	if err != nil {
		return nil, fmt.Errorf("marshal target registration session: %w", err)
	}
	previousJSON, err := json.Marshal(previous)
	if err != nil {
		return nil, fmt.Errorf("marshal previous registration session: %w", err)
	}
	operation := &AgentRegistrationOperationDB{
		OperationID:     uuid.NewString(),
		AgentID:         target.AgentID,
		GameID:          target.GameID,
		Env:             target.Env,
		PreviousSession: string(previousJSON),
		TargetSession:   string(targetJSON),
		Status:          "pending",
	}
	if err := s.db.Create(operation).Error; err != nil {
		return nil, fmt.Errorf("create registration recovery operation: %w", err)
	}
	return operation, nil
}

func (s *Store) markRegistrationOperation(operationID, status string, cause error) {
	if s == nil || s.db == nil || strings.TrimSpace(operationID) == "" {
		return
	}
	lastError := ""
	if cause != nil {
		lastError = cause.Error()
	}
	if err := s.markRegistrationOperationWithDB(s.db, operationID, status, lastError); err != nil {
		slog.Default().Error("failed to update registration recovery operation", "operation_id", operationID, "status", status, "error", err)
	}
}

func (s *Store) markRegistrationOperationWithDB(db *gorm.DB, operationID, status, lastError string) error {
	if db == nil {
		return fmt.Errorf("registration recovery database is not initialized")
	}
	return db.Model(&AgentRegistrationOperationDB{}).
		Where("operation_id = ?", operationID).
		Updates(map[string]interface{}{"status": status, "last_error": lastError}).Error
}

func (s *Store) rebuildFunctionContract(
	ctx context.Context,
	gameID string,
	env string,
	functionID string,
	meta FunctionMeta,
	resources map[string]bool,
	standaloneFunctions map[string]bool,
) error {
	input := spec.FunctionContractInput{
		ID:                functionID,
		Version:           meta.Version,
		Enabled:           meta.Enabled,
		Deprecated:        meta.Deprecated,
		Summary:           meta.Summary,
		Description:       meta.Description,
		InputSchema:       meta.InputSchema,
		OutputSchema:      meta.OutputSchema,
		Resource:          meta.Resource,
		Operation:         meta.Operation,
		Capability:        meta.Capability,
		Execution:         meta.Execution,
		ApprovalRequired:  meta.ApprovalRequired,
		ApprovalPolicyKey: meta.ApprovalPolicyKey,
		Risk:              meta.Risk,
		Permission:        meta.Permission,
		Tags:              meta.Tags,
	}
	if err := s.contractService.RebuildContractFromFunctionMeta(ctx, gameID, env, "sdk", input); err != nil {
		return fmt.Errorf("rebuild function contract %s: %w", functionID, err)
	}
	if meta.Resource != "" {
		resources[meta.Resource] = true
	} else {
		standaloneFunctions[functionID] = true
	}
	return nil
}

func (s *Store) previousFunctions(agentID string) map[string]FunctionMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.agents[agentID]
	if current == nil || current.Functions == nil {
		return nil
	}
	functions := make(map[string]FunctionMeta, len(current.Functions))
	for functionID, meta := range current.Functions {
		functions[functionID] = meta
	}
	return functions
}

func (s *Store) survivingFunctionMeta(agentID, gameID, env, functionID string) (FunctionMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agentIDs := make([]string, 0, len(s.agents))
	for id := range s.agents {
		agentIDs = append(agentIDs, id)
	}
	sort.Strings(agentIDs)
	for _, id := range agentIDs {
		if id == agentID {
			continue
		}
		session := s.agents[id]
		if session == nil || session.GameID != gameID || session.Env != env {
			continue
		}
		meta, ok := session.Functions[functionID]
		if ok {
			return meta, true
		}
	}
	return FunctionMeta{}, false
}

func sortedFunctionIDs(functions map[string]FunctionMeta) []string {
	ids := make([]string, 0, len(functions))
	for functionID := range functions {
		ids = append(ids, functionID)
	}
	sort.Strings(ids)
	return ids
}

func classifyFunctionSnapshot(previous, current map[string]FunctionMeta) functionSnapshotDiff {
	// nil means the registration omitted the snapshot; it must not delete all
	// existing functions during heartbeat-compatible registrations.
	if current == nil {
		return functionSnapshotDiff{}
	}
	diff := functionSnapshotDiff{}
	resources := map[string]struct{}{}
	for functionID, previousMeta := range previous {
		currentMeta, ok := current[functionID]
		if !ok {
			diff.Removed = append(diff.Removed, functionID)
			if resource := strings.TrimSpace(previousMeta.Resource); resource != "" {
				resources[resource] = struct{}{}
			}
			continue
		}
		if !reflect.DeepEqual(previousMeta, currentMeta) {
			diff.Changed = append(diff.Changed, functionID)
		}
		for _, resource := range []string{previousMeta.Resource, currentMeta.Resource} {
			if resource = strings.TrimSpace(resource); resource != "" {
				resources[resource] = struct{}{}
			}
		}
	}
	for functionID, currentMeta := range current {
		if _, ok := previous[functionID]; !ok {
			diff.Added = append(diff.Added, functionID)
		}
		if resource := strings.TrimSpace(currentMeta.Resource); resource != "" {
			resources[resource] = struct{}{}
		}
	}
	sort.Strings(diff.Added)
	sort.Strings(diff.Changed)
	sort.Strings(diff.Removed)
	for resource := range resources {
		diff.Resources = append(diff.Resources, resource)
	}
	sort.Strings(diff.Resources)
	return diff
}

func sortedStringSet(values map[string]bool) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			items = append(items, value)
		}
	}
	sort.Strings(items)
	return items
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = true
		}
	}
	return set
}

// validateAgentFunctionContracts keeps the registry write boundary aligned
// with the SDK and OpenAPI adapters. A caller that bypasses the control
// handler must not persist a dashboard presentation extension in a schema.
func validateAgentFunctionContracts(functions map[string]FunctionMeta) error {
	functionIDs := make([]string, 0, len(functions))
	for functionID := range functions {
		functionIDs = append(functionIDs, functionID)
	}
	sort.Strings(functionIDs)

	for _, functionID := range functionIDs {
		meta := functions[functionID]
		if violation, ok := registrationguard.FindPresentationViolation(nil, meta.InputSchema, meta.OutputSchema); ok {
			return fmt.Errorf("function %q %s contains forbidden presentation field %q", functionID, violation.Location, violation.Field)
		}
	}
	return nil
}

func (s *Store) rebuildContext(gameID, env string) context.Context {
	if s != nil && s.scopeContext != nil {
		if ctx := s.scopeContext(gameID, env); ctx != nil {
			return ctx
		}
	}
	return defaultScopeContext(gameID, env)
}

func defaultScopeContext(string, string) context.Context {
	return context.Background()
}

// writeToDB persists agent session to database for recovery purposes.
// Only called when db is enabled.
func (s *Store) writeToDB(ctx context.Context, a *AgentSession) error {
	if s.db == nil {
		return nil
	}

	// Convert to database model — initialise JSON columns to valid empty
	// JSON so nil maps/slices don't produce empty strings that PostgreSQL's
	// json type rejects.
	dbSess := struct {
		AgentID   string
		GameID    string
		Env       string
		Version   string
		Region    string
		Zone      string
		Labels    string `gorm:"type:text"`
		Functions string `gorm:"type:text"`
		Providers string `gorm:"type:text"`
		ExpireAt  time.Time
		LastSeen  time.Time
	}{
		AgentID:   a.AgentID,
		GameID:    a.GameID,
		Env:       a.Env,
		Version:   a.Version,
		Region:    a.Region,
		Zone:      a.Zone,
		ExpireAt:  a.ExpireAt,
		LastSeen:  a.LastSeen,
		Labels:    "{}",
		Functions: "{}",
		Providers: "[]",
	}

	// Marshal Labels to JSON
	if a.Labels != nil {
		if labelsJSON, err := json.Marshal(a.Labels); err == nil {
			dbSess.Labels = string(labelsJSON)
		}
	}

	// Marshal Functions to JSON
	if a.Functions != nil {
		if functionsJSON, err := json.Marshal(a.Functions); err == nil {
			dbSess.Functions = string(functionsJSON)
		}
	}

	// Marshal Providers to JSON
	if a.Providers != nil {
		if providersJSON, err := json.Marshal(a.Providers); err == nil {
			dbSess.Providers = string(providersJSON)
		}
	}

	// Perform upsert using GORM
	return dbctx.Resolve(ctx, s.db).WithContext(ctx).
		Table("agent_sessions").
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "agent_id"}},
			UpdateAll: true,
		}).
		Create(&dbSess).Error
}

// ========== OpenAPI 3.0.3 Methods (replaces legacy manifest) ==========

// UpsertOpenAPI inserts or updates an OpenAPI operation by function ID.
func (s *Store) UpsertOpenAPI(functionID string, operation *openapi3.Operation) error {
	if functionID == "" || operation == nil {
		return fmt.Errorf("function ID and operation are required")
	}
	clone, err := cloneOpenAPIOperation(operation)
	if err != nil {
		return fmt.Errorf("clone operation failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.openapiOperations[functionID] = clone
	return nil
}

// GetOpenAPI retrieves an OpenAPI operation by function ID.
func (s *Store) GetOpenAPI(functionID string) (*openapi3.Operation, error) {
	if functionID == "" {
		return nil, fmt.Errorf("function ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	op, exists := s.openapiOperations[functionID]
	if !exists {
		return nil, fmt.Errorf("operation not found: %s", functionID)
	}

	return cloneOpenAPIOperation(op)
}

// ListOpenAPIOperations returns all OpenAPI operations.
func (s *Store) ListOpenAPIOperations() map[string]*openapi3.Operation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*openapi3.Operation, len(s.openapiOperations))
	for id, op := range s.openapiOperations {
		clone, err := cloneOpenAPIOperation(op)
		if err != nil {
			continue
		}
		result[id] = clone
	}

	return result
}

func cloneOpenAPIOperation(op *openapi3.Operation) (*openapi3.Operation, error) {
	if op == nil {
		return nil, fmt.Errorf("operation is nil")
	}
	b, err := op.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var clone openapi3.Operation
	if err := clone.UnmarshalJSON(b); err != nil {
		return nil, err
	}
	return &clone, nil
}

// UpsertOpenAPIProvider inserts or updates provider capabilities in OpenAPI format.
func (s *Store) UpsertOpenAPIProvider(caps OpenAPIProviderCaps) error {
	if caps.ID == "" {
		return fmt.Errorf("provider ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	caps.UpdatedAt = time.Now()
	s.openapiProviders[caps.ID] = &caps
	return nil
}

// GetOpenAPIProvider retrieves OpenAPI provider capabilities by provider ID.
func (s *Store) GetOpenAPIProvider(providerID string) (*OpenAPIProviderCaps, error) {
	if providerID == "" {
		return nil, fmt.Errorf("provider ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	caps, exists := s.openapiProviders[providerID]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	return caps, nil
}

// ListOpenAPIProviders returns all OpenAPI providers.
func (s *Store) ListOpenAPIProviders() []*OpenAPIProviderCaps {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*OpenAPIProviderCaps, 0, len(s.openapiProviders))
	for _, caps := range s.openapiProviders {
		result = append(result, caps)
	}

	return result
}

// BuildOpenAPISpec constructs a complete OpenAPI 3.0.3 specification from all registered operations.
func (s *Store) BuildOpenAPISpec() (*openapi3.T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       "Croupier Functions",
			Description: "Auto-generated OpenAPI specification from registered functions",
			Version:     "1.0.0",
		},
		Paths: openapi3.NewPaths(),
	}

	// Add paths from operations
	for functionID, op := range s.openapiOperations {
		if op == nil {
			continue
		}

		pathItem := &openapi3.PathItem{}
		pathItem.Post = op

		path := fmt.Sprintf("/functions/%s", functionID)
		doc.Paths.Set(path, pathItem)
	}

	return doc, nil
}

// DeleteOpenAPI removes an OpenAPI operation by function ID.
func (s *Store) DeleteOpenAPI(functionID string) error {
	if functionID == "" {
		return fmt.Errorf("function ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.openapiOperations[functionID]; !exists {
		return fmt.Errorf("operation not found: %s", functionID)
	}

	delete(s.openapiOperations, functionID)
	return nil
}

// DeleteOpenAPIProvider removes an OpenAPI provider by provider ID.
func (s *Store) DeleteOpenAPIProvider(providerID string) error {
	if providerID == "" {
		return fmt.Errorf("provider ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.openapiProviders[providerID]; !exists {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	delete(s.openapiProviders, providerID)
	return nil
}

func (s *Store) UpsertRegistrationWarning(ctx context.Context, item FunctionRegistrationWarning) error {
	if item.Message == "" {
		return nil
	}
	key := item.Key
	if key == "" {
		// Use SHA-256 hash for fixed-length key, include game_id and env for isolation
		h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s|%s", item.GameID, item.Env, item.AgentID, item.FunctionID, item.Code, item.Message)))
		key = hex.EncodeToString(h[:16]) // Use first 16 bytes (32 hex chars) for brevity
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.registrationWarnings == nil {
		s.registrationWarnings = make(map[string]*FunctionRegistrationWarning)
	}
	existing, ok := s.registrationWarnings[key]
	if !ok || existing == nil {
		cp := item
		cp.Key = key
		if cp.FirstSeen.IsZero() {
			cp.FirstSeen = now
		}
		if cp.LastSeen.IsZero() {
			cp.LastSeen = cp.FirstSeen
		}
		if cp.Count <= 0 {
			cp.Count = 1
		}
		s.registrationWarnings[key] = &cp
		return nil
	}
	existing.Count++
	existing.LastSeen = now
	if existing.Version == "" && item.Version != "" {
		existing.Version = item.Version
	}
	if existing.AgentID == "" && item.AgentID != "" {
		existing.AgentID = item.AgentID
	}
	if existing.FunctionID == "" && item.FunctionID != "" {
		existing.FunctionID = item.FunctionID
	}
	return nil
}

func (s *Store) ListRegistrationWarnings(filter RegistrationWarningFilter) []FunctionRegistrationWarning {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FunctionRegistrationWarning, 0, len(s.registrationWarnings))
	for _, item := range s.registrationWarnings {
		if item == nil {
			continue
		}
		if filter.GameID != "" && item.GameID != filter.GameID {
			continue
		}
		if filter.Env != "" && item.Env != filter.Env {
			continue
		}
		if filter.AgentID != "" && item.AgentID != filter.AgentID {
			continue
		}
		if filter.FunctionID != "" && item.FunctionID != filter.FunctionID {
			continue
		}
		if filter.Code != "" && item.Code != filter.Code {
			continue
		}
		out = append(out, *item)
	}
	// Sort by most recent warnings first.
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out
}

// AgentSessionLoader defines the interface for loading agent sessions from database.
type AgentSessionLoader interface {
	LoadActiveSessions(ctx context.Context) ([]*AgentSession, error)
}

// LoadFromDB loads active agent sessions from the database and populates the in-memory store.
func (s *Store) LoadFromDB(ctx context.Context, loader AgentSessionLoader) error {
	if s.db == nil {
		return fmt.Errorf("database not enabled")
	}
	if loader == nil {
		return fmt.Errorf("agent session loader is required")
	}

	sessions, err := loader.LoadActiveSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to load sessions: %w", err)
	}

	s.mu.Lock()
	for _, sess := range sessions {
		s.agents[sess.AgentID] = sess
	}
	s.mu.Unlock()

	if err := s.recoverPendingRegistrationOperations(ctx); err != nil {
		return err
	}

	slog.Info("loaded agent sessions from database", "count", len(sessions))
	return nil
}

func (s *Store) recoverPendingRegistrationOperations(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	if s.contractService == nil {
		return nil
	}
	var operations []AgentRegistrationOperationDB
	if err := s.db.WithContext(ctx).
		Where("status IN ?", []string{"pending", "compensation_required"}).
		Order("created_at, id").
		Find(&operations).Error; err != nil {
		return fmt.Errorf("list pending registration recovery operations: %w", err)
	}
	for _, operation := range operations {
		previous, err := decodeRegistrationSession(operation.PreviousSession)
		if err != nil {
			s.markRegistrationOperation(operation.OperationID, "compensation_required", err)
			return fmt.Errorf("decode previous registration session %s: %w", operation.OperationID, err)
		}
		target, err := decodeRegistrationSession(operation.TargetSession)
		if err != nil || target == nil {
			if err == nil {
				err = fmt.Errorf("target session is empty")
			}
			s.markRegistrationOperation(operation.OperationID, "compensation_required", err)
			return fmt.Errorf("decode target registration session %s: %w", operation.OperationID, err)
		}
		if previous == nil {
			previous = &AgentSession{
				AgentID:   target.AgentID,
				GameID:    target.GameID,
				Env:       target.Env,
				Functions: map[string]FunctionMeta{},
			}
		}
		if previous.AgentID == "" {
			previous.AgentID = target.AgentID
		}
		if previous.GameID == "" {
			previous.GameID = target.GameID
		}
		if previous.Env == "" {
			previous.Env = target.Env
		}

		scopeCtx := s.rebuildContext(target.GameID, target.Env)
		reverseDiff := classifyFunctionSnapshot(target.Functions, previous.Functions)
		if err := s.materializeScopedTransaction(scopeCtx, previous, s.materializeAgent, reverseDiff); err != nil {
			s.markRegistrationOperation(operation.OperationID, "compensation_required", err)
			return fmt.Errorf("recover registration operation %s: %w", operation.OperationID, err)
		}
		s.markRegistrationOperation(operation.OperationID, "compensated", nil)
	}
	return nil
}

func decodeRegistrationSession(raw string) (*AgentSession, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var session AgentSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// StartCleanupRoutine 启动后台清理过期 Session 的 goroutine
// 定期从内存中删除过期的 AgentSession，保持内存数据有效
func (s *Store) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Minute // 默认每分钟清理一次
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("Registry cleanup routine stopped")
				return
			case <-ticker.C:
				s.cleanupExpiredSessions()
			}
		}
	}()

	slog.Info("Started registry cleanup routine", "interval", interval)
}

// cleanupExpiredSessions 清理过期的 AgentSession
func (s *Store) cleanupExpiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for agentID, sess := range s.agents {
		if sess == nil {
			continue
		}

		// 检查是否过期
		if sess.ExpireAt.Before(now) {
			delete(s.agents, agentID)
			expiredCount++

			slog.Debug("Cleaned up expired agent session", "agent_id", agentID, "expired_at", sess.ExpireAt.Format(time.RFC3339))
		}
	}

	if expiredCount > 0 {
		slog.Info("Cleaned up expired agent sessions", "expired_count", expiredCount, "remaining", len(s.agents))
	}
}

// RemoveRegistrationWarnings 删除匹配的注册警告（修复闭环：如 provider
// scope 修正后再次注册一致时，清除历史 mismatch 警告——业务层警告的
// 生命周期跟随注册行为，不借运维告警状态机）。
func (s *Store) RemoveRegistrationWarnings(filter RegistrationWarningFilter) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for key, item := range s.registrationWarnings {
		if item == nil {
			continue
		}
		if filter.GameID != "" && item.GameID != filter.GameID {
			continue
		}
		if filter.Env != "" && item.Env != filter.Env {
			continue
		}
		if filter.AgentID != "" && item.AgentID != filter.AgentID {
			continue
		}
		if filter.FunctionID != "" && item.FunctionID != filter.FunctionID {
			continue
		}
		if filter.Code != "" && item.Code != filter.Code {
			continue
		}
		delete(s.registrationWarnings, key)
		removed++
	}
	return removed
}
