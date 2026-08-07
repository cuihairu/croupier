package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
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

	Resource   string
	Operation  string
	Capability string
	Execution  string
	Risk       string
	Permission string
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
	mu     sync.RWMutex
	agents map[string]*AgentSession // agent_id -> session
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
	contractService interface {
		RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta interface{}) error
		RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error
		RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error
		RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error
	}
}

type FunctionRegistrationWarning struct {
	Key        string
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
	AgentID    string
	FunctionID string
	Code       string
	Limit      int
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
func (s *Store) SetContractService(svc interface {
	RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta interface{}) error
	RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error
	RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error
	RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error
}) {
	s.contractService = svc
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

	// Dual-write: database first (if enabled)
	if s.db != nil {
		if err := s.writeToDB(context.Background(), a); err != nil {
			return fmt.Errorf("write agent session to database: %w", err)
		}
	}

	// Rebuild FunctionContracts if contract service is available
	if s.contractService != nil && a.Functions != nil {
		scopeCtx := s.rebuildContext(a.GameID, a.Env)
		var rebuildErrors []error
		for funcID, meta := range a.Functions {
			input := struct {
				ID           string
				Version      string
				Enabled      bool
				Summary      string
				Description  string
				InputSchema  string
				OutputSchema string
				Resource     string
				Operation    string
				Capability   string
				Execution    string
				Risk         string
				Permission   string
				Tags         []string
			}{
				ID:           funcID,
				Version:      meta.Version,
				Enabled:      meta.Enabled,
				Summary:      meta.Summary,
				Description:  meta.Description,
				InputSchema:  meta.InputSchema,
				OutputSchema: meta.OutputSchema,
				Resource:     meta.Resource,
				Operation:    meta.Operation,
				Capability:   meta.Capability,
				Execution:    meta.Execution,
				Risk:         meta.Risk,
				Permission:   meta.Permission,
				Tags:         meta.Tags,
			}
			if err := s.contractService.RebuildContractFromFunctionMeta(scopeCtx, a.GameID, a.Env, "sdk", input); err != nil {
				rebuildErrors = append(rebuildErrors, fmt.Errorf("rebuild function contract %s: %w", funcID, err))
			}
		}

		// Rebuild resource capabilities for unique resources
		resources := make(map[string]bool)
		standaloneFunctions := make(map[string]bool)
		for functionID, meta := range a.Functions {
			if meta.Resource != "" {
				resources[meta.Resource] = true
			} else {
				standaloneFunctions[functionID] = true
			}
		}
		for resource := range resources {
			if err := s.contractService.RebuildResourceCapability(scopeCtx, a.GameID, a.Env, resource); err != nil {
				rebuildErrors = append(rebuildErrors, fmt.Errorf("rebuild resource capability %s: %w", resource, err))
				continue
			}
			if err := s.contractService.RebuildProposalsForResource(scopeCtx, a.GameID, a.Env, resource); err != nil {
				rebuildErrors = append(rebuildErrors, fmt.Errorf("rebuild page proposals for %s: %w", resource, err))
			}
		}
		for functionID := range standaloneFunctions {
			if err := s.contractService.RebuildProposalForFunction(scopeCtx, a.GameID, a.Env, functionID); err != nil {
				rebuildErrors = append(rebuildErrors, fmt.Errorf("rebuild standalone page proposal %s: %w", functionID, err))
			}
		}
		if len(rebuildErrors) > 0 {
			return fmt.Errorf("agent registration contract rebuild failed: %w", errors.Join(rebuildErrors...))
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
		Labels    string `gorm:"type:json"`
		Functions string `gorm:"type:json"`
		Providers string `gorm:"type:json"`
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
	return s.db.WithContext(ctx).
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

func (s *Store) UpsertRegistrationWarning(item FunctionRegistrationWarning) {
	if item.Message == "" {
		return
	}
	key := item.Key
	if key == "" {
		key = fmt.Sprintf("%s|%s|%s|%s", item.AgentID, item.FunctionID, item.Code, item.Message)
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
		return
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
}

func (s *Store) ListRegistrationWarnings(filter RegistrationWarningFilter) []FunctionRegistrationWarning {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FunctionRegistrationWarning, 0, len(s.registrationWarnings))
	for _, item := range s.registrationWarnings {
		if item == nil {
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

	sessions, err := loader.LoadActiveSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to load sessions: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sess := range sessions {
		s.agents[sess.AgentID] = sess
	}

	slog.Info("loaded agent sessions from database", "count", len(sessions))
	return nil
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
