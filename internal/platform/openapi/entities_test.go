package openapi

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityManager_RegisterEntity(t *testing.T) {
	manager := NewEntityManager()

	t.Run("register new entity", func(t *testing.T) {
		entity := &Entity{
			ID:   "player",
			Name: "Player",
		}

		err := manager.RegisterEntity(entity)
		require.NoError(t, err)

		retrieved, exists := manager.GetEntity("player")
		assert.True(t, exists)
		assert.Equal(t, "Player", retrieved.Name)
	})

	t.Run("update existing entity", func(t *testing.T) {
		entity := &Entity{
			ID:   "player",
			Name: "Player",
		}

		err := manager.RegisterEntity(entity)
		require.NoError(t, err)

		// Update with new name
		entity.Name = "Game Player"
		err = manager.RegisterEntity(entity)
		require.NoError(t, err)

		retrieved, exists := manager.GetEntity("player")
		assert.True(t, exists)
		assert.Equal(t, "Game Player", retrieved.Name)
	})
}

func TestEntityManager_GetEntity(t *testing.T) {
	manager := NewEntityManager()

	t.Run("get non-existent entity", func(t *testing.T) {
		_, exists := manager.GetEntity("nonexistent")
		assert.False(t, exists)
	})

	t.Run("get existing entity", func(t *testing.T) {
		entity := &Entity{
			ID:          "item",
			Name:        "Item",
			Description: "Game items",
		}

		err := manager.RegisterEntity(entity)
		require.NoError(t, err)

		retrieved, exists := manager.GetEntity("item")
		assert.True(t, exists)
		assert.Equal(t, "Item", retrieved.Name)
		assert.Equal(t, "Game items", retrieved.Description)
	})
}

func TestEntityManager_ListEntities(t *testing.T) {
	manager := NewEntityManager()

	t.Run("empty list", func(t *testing.T) {
		entities := manager.ListEntities()
		assert.Equal(t, 0, len(entities))
	})

	t.Run("list multiple entities", func(t *testing.T) {
		manager.RegisterEntity(&Entity{ID: "player", Name: "Player"})
		manager.RegisterEntity(&Entity{ID: "item", Name: "Item"})
		manager.RegisterEntity(&Entity{ID: "guild", Name: "Guild"})

		entities := manager.ListEntities()
		assert.Equal(t, 3, len(entities))
	})
}

func TestEntityManager_DeleteEntity(t *testing.T) {
	manager := NewEntityManager()

	t.Run("delete existing entity", func(t *testing.T) {
		manager.RegisterEntity(&Entity{ID: "player", Name: "Player"})

		// Verify exists
		_, exists := manager.GetEntity("player")
		assert.True(t, exists)

		// Delete
		manager.DeleteEntity("player")

		// Verify deleted
		_, exists = manager.GetEntity("player")
		assert.False(t, exists)
	})

	t.Run("delete non-existent entity", func(t *testing.T) {
		// Should not panic
		manager.DeleteEntity("nonexistent")
	})
}

func TestEntityManager_GetEntityOperation(t *testing.T) {
	manager := NewEntityManager()

	t.Run("get operation from entity", func(t *testing.T) {
		operation := &openapi3.Operation{
			OperationID: "player.create",
		}

		entity := &Entity{
			ID:   "player",
			Name: "Player",
			Operations: map[string]*openapi3.Operation{
				"create": operation,
			},
		}

		err := manager.RegisterEntity(entity)
		require.NoError(t, err)

		retrieved, exists := manager.GetEntityOperation("player", "create")
		assert.True(t, exists)
		assert.Equal(t, "player.create", retrieved.OperationID)
	})

	t.Run("get operation from non-existent entity", func(t *testing.T) {
		_, exists := manager.GetEntityOperation("nonexistent", "create")
		assert.False(t, exists)
	})

	t.Run("get non-existent operation", func(t *testing.T) {
		manager.RegisterEntity(&Entity{ID: "player", Name: "Player"})

		_, exists := manager.GetEntityOperation("player", "nonexistent")
		assert.False(t, exists)
	})
}
