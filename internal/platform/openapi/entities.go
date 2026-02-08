package openapi

import (
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
)

// EntityManager manages entity definitions and their operations
type EntityManager struct {
	mu       sync.RWMutex
	entities map[string]*Entity
}

// Entity represents a business entity with its operations
type Entity struct {
	ID          string
	Name        string
	Description string
	Operations  map[string]*openapi3.Operation
}

// NewEntityManager creates a new entity manager
func NewEntityManager() *EntityManager {
	return &EntityManager{
		entities: make(map[string]*Entity),
	}
}

// RegisterEntity registers or updates an entity definition
func (m *EntityManager) RegisterEntity(entity *Entity) error {
	if entity == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.entities[entity.ID] == nil {
		m.entities[entity.ID] = &Entity{
			ID:         entity.ID,
			Operations: make(map[string]*openapi3.Operation),
		}
	}

	// Update entity metadata
	if entity.Name != "" {
		m.entities[entity.ID].Name = entity.Name
	}
	if entity.Description != "" {
		m.entities[entity.ID].Description = entity.Description
	}

	// Add operations
	for opID, op := range entity.Operations {
		m.entities[entity.ID].Operations[opID] = op
	}

	return nil
}

// GetEntity retrieves an entity by ID
func (m *EntityManager) GetEntity(id string) (*Entity, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entity, exists := m.entities[id]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid concurrent modifications
	copy := &Entity{
		ID:          entity.ID,
		Name:        entity.Name,
		Description: entity.Description,
		Operations:  make(map[string]*openapi3.Operation),
	}

	for opID, op := range entity.Operations {
		copy.Operations[opID] = op
	}

	return copy, true
}

// ListEntities returns all registered entities
func (m *EntityManager) ListEntities() []*Entity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Entity, 0, len(m.entities))
	for _, entity := range m.entities {
		result = append(result, entity)
	}

	return result
}

// DeleteEntity removes an entity
func (m *EntityManager) DeleteEntity(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.entities, id)
}

// GetEntityOperation retrieves a specific operation from an entity
func (m *EntityManager) GetEntityOperation(entityID, operationID string) (*openapi3.Operation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entity, exists := m.entities[entityID]
	if !exists {
		return nil, false
	}

	op, exists := entity.Operations[operationID]
	return op, exists
}
