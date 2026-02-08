package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FunctionMeta describes a function capability on an agent.
type FunctionMeta struct {
	Enabled bool
	Version string
}

// ProviderSession represents a single provider registered to an agent (via SDK->Agent local registry).
type ProviderSession struct {
	ProviderID   string
	GameID       string
	Env          string
	Addr         string
	Version      string
	LastSeenUnix int64
	FunctionIDs  []string
	OpenAPIDoc   json.RawMessage
}

// AgentSession represents a registered agent instance in the registry.
type AgentSession struct {
	AgentID   string
	GameID    string
	Env       string
	RPCAddr   string
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
	openapiOperations map[string]*openapi3.Operation  // function_id -> OpenAPI operation
	openapiProviders  map[string]*OpenAPIProviderCaps // provider_id -> OpenAPI caps
	// Optional database for dual-write persistence
	db *gorm.DB
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
		agents:            map[string]*AgentSession{},
		openapiOperations: make(map[string]*openapi3.Operation),
		openapiProviders:  make(map[string]*OpenAPIProviderCaps),
		db:                nil,
	}
}

// NewStoreWithDB creates a new Store with database dual-write enabled.
func NewStoreWithDB(db *gorm.DB) *Store {
	return &Store{
		agents:            map[string]*AgentSession{},
		openapiOperations: make(map[string]*openapi3.Operation),
		openapiProviders:  make(map[string]*OpenAPIProviderCaps),
		db:                db,
	}
}

// Mu exposes the lock for read/update operations when callers need batch views.
func (s *Store) Mu() *sync.RWMutex { return &s.mu }

// AgentsUnsafe returns the internal agents map without copying. Callers MUST hold Mu().RLock/Lock.
func (s *Store) AgentsUnsafe() map[string]*AgentSession { return s.agents }

// UpsertAgent inserts or updates an agent session by AgentID.
// Implements dual-write pattern: writes to database first (if enabled), then to memory.
// Database failures are logged but don't block memory operations.
func (s *Store) UpsertAgent(a *AgentSession) {
	if a == nil || a.AgentID == "" {
		return
	}

	// Dual-write: database first (if enabled)
	if s.db != nil {
		if err := s.writeToDB(context.Background(), a); err != nil {
			// Log error but continue - memory store is the primary source
			logx.Errorf("failed to write agent session to database (agent_id=%s): %v", a.AgentID, err)
		}
	}

	// Always write to memory (primary store)
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.agents[a.AgentID]
	if cur == nil {
		s.agents[a.AgentID] = a
		return
	}
	// merge minimal fields
	cur.GameID, cur.Env, cur.RPCAddr, cur.Version = a.GameID, a.Env, a.RPCAddr, a.Version
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
}

// writeToDB persists agent session to database for recovery purposes.
// Only called when db is enabled.
func (s *Store) writeToDB(ctx context.Context, a *AgentSession) error {
	if s.db == nil {
		return nil
	}

	// Convert to database model
	dbSess := struct {
		AgentID   string
		GameID    string
		Env       string
		RPCAddr   string
		Version   string
		Region    string
		Zone      string
		Labels    string
		Functions string
		Providers string
		ExpireAt  time.Time
		LastSeen  time.Time
	}{
		AgentID:  a.AgentID,
		GameID:   a.GameID,
		Env:      a.Env,
		RPCAddr:  a.RPCAddr,
		Version:  a.Version,
		Region:   a.Region,
		Zone:     a.Zone,
		ExpireAt: a.ExpireAt,
		LastSeen: a.LastSeen,
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

	s.mu.Lock()
	defer s.mu.Unlock()
	s.openapiOperations[functionID] = operation
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

	return op, nil
}

// ListOpenAPIOperations returns all OpenAPI operations.
func (s *Store) ListOpenAPIOperations() map[string]*openapi3.Operation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*openapi3.Operation, len(s.openapiOperations))
	for id, op := range s.openapiOperations {
		result[id] = op
	}

	return result
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

	logx.Infof("loaded %d agent sessions from database", len(sessions))
	return nil
}
