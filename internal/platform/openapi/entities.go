package openapi

import (
	"fmt"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
)

// EntityManager manages entity definitions and their operations
type EntityManager struct {
	mu       sync.RWMutex
	entities map[string]*Entity // entity_id -> Entity
}

// Entity represents a business entity with operations
type Entity struct {
	ID          string                         `json:"id"`
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	Operations  map[string]*openapi3.Operation `json:"operations"` // operation_id -> operation
}

// NewEntityManager creates a new Entity manager
func NewEntityManager() *EntityManager {
	return &EntityManager{
		entities: make(map[string]*Entity),
	}
}

// RegisterEntity registers a new entity
func (m *EntityManager) RegisterEntity(entity *Entity) error {
	if entity == nil {
		return fmt.Errorf("entity is nil")
	}

	if entity.ID == "" {
		return fmt.Errorf("entity ID is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.entities[entity.ID] = entity
	return nil
}

// GetEntity retrieves an entity by ID
func (m *EntityManager) GetEntity(id string) (*Entity, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entity, exists := m.entities[id]
	return entity, exists
}

// ListEntities returns all registered entities
func (m *EntityManager) ListEntities() []*Entity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entities := make([]*Entity, 0, len(m.entities))
	for _, entity := range m.entities {
		entities = append(entities, entity)
	}
	return entities
}

// AddOperationToEntity adds an operation to an entity
func (m *EntityManager) AddOperationToEntity(entityID string, operationID string, op *openapi3.Operation) error {
	if operationID == "" {
		return fmt.Errorf("operation ID is required")
	}

	if op == nil {
		return fmt.Errorf("operation is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entity, exists := m.entities[entityID]
	if !exists {
		return fmt.Errorf("entity not found: %s", entityID)
	}

	if entity.Operations == nil {
		entity.Operations = make(map[string]*openapi3.Operation)
	}

	entity.Operations[operationID] = op
	return nil
}

// GetEntityOperation retrieves a specific operation from an entity
func (m *EntityManager) GetEntityOperation(entityID, operationID string) (*openapi3.Operation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entity, exists := m.entities[entityID]
	if !exists {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	if entity.Operations == nil {
		return nil, fmt.Errorf("entity has no operations")
	}

	op, exists := entity.Operations[operationID]
	if !exists {
		return nil, fmt.Errorf("operation not found: %s", operationID)
	}

	return op, nil
}

// ListEntityOperations returns all operations for an entity
func (m *EntityManager) ListEntityOperations(entityID string) (map[string]*openapi3.Operation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entity, exists := m.entities[entityID]
	if !exists {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	if entity.Operations == nil {
		return make(map[string]*openapi3.Operation), nil
	}

	// Return a copy to avoid race conditions
	ops := make(map[string]*openapi3.Operation, len(entity.Operations))
	for k, v := range entity.Operations {
		ops[k] = v
	}

	return ops, nil
}

// DeleteEntity removes an entity
func (m *EntityManager) DeleteEntity(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.entities[id]; !exists {
		return fmt.Errorf("entity not found: %s", id)
	}

	delete(m.entities, id)
	return nil
}
