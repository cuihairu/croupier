package openapi

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_ValidateSpec(t *testing.T) {
	validator := NewValidator()

	t.Run("valid minimal spec", func(t *testing.T) {
		spec := []byte(`{
			"openapi": "3.0.3",
			"info": {
				"title": "Test API",
				"version": "1.0.0"
			},
			"paths": {
				"/test": {
					"get": {
						"operationId": "testOp",
						"responses": {
							"200": {
								"description": "OK"
							}
						}
					}
				}
			}
		}`)

		doc, err := validator.ValidateSpec(spec)
		require.NoError(t, err)
		assert.Equal(t, "3.0.3", doc.OpenAPI)
		assert.NotNil(t, doc.Info)
	})

	t.Run("missing openapi version", func(t *testing.T) {
		spec := []byte(`{
			"info": {
				"title": "Test API",
				"version": "1.0.0"
			},
			"paths": {}
		}`)

		_, err := validator.ValidateSpec(spec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing openapi version")
	})

	t.Run("unsupported version", func(t *testing.T) {
		spec := []byte(`{
			"openapi": "3.1.0",
			"info": {
				"title": "Test API",
				"version": "1.0.0"
			},
			"paths": {}
		}`)

		_, err := validator.ValidateSpec(spec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported OpenAPI version")
	})

	t.Run("missing info", func(t *testing.T) {
		spec := []byte(`{
			"openapi": "3.0.3",
			"paths": {}
		}`)

		_, err := validator.ValidateSpec(spec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing info")
	})
}

func TestValidator_ValidateOperation(t *testing.T) {
	validator := NewValidator()

	t.Run("valid operation", func(t *testing.T) {
		op := &openapi3.Operation{
			OperationID: "testOp",
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{},
					},
				},
			},
			Responses: openapi3.NewResponses(),
		}
		op.Responses.Set("200", &openapi3.ResponseRef{
			Value: &openapi3.Response{},
		})

		err := validator.ValidateOperation(op)
		assert.NoError(t, err)
	})

	t.Run("missing operationId", func(t *testing.T) {
		op := &openapi3.Operation{}
		err := validator.ValidateOperation(op)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operationId is required")
	})

	t.Run("missing 200 response", func(t *testing.T) {
		op := &openapi3.Operation{
			OperationID: "testOp",
			Responses:   openapi3.NewResponses(),
		}
		err := validator.ValidateOperation(op)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have a 200 response")
	})
}

func TestEntityManager_RegisterEntity(t *testing.T) {
	manager := NewEntityManager()

	t.Run("register valid entity", func(t *testing.T) {
		entity := &Entity{
			ID:          "player",
			Name:        "Player",
			Description: "Game player entity",
			Operations:  make(map[string]*openapi3.Operation),
		}

		err := manager.RegisterEntity(entity)
		assert.NoError(t, err)

		retrieved, exists := manager.GetEntity("player")
		assert.True(t, exists)
		assert.Equal(t, "player", retrieved.ID)
		assert.Equal(t, "Player", retrieved.Name)
	})

	t.Run("register nil entity", func(t *testing.T) {
		err := manager.RegisterEntity(nil)
		assert.Error(t, err)
	})

	t.Run("register entity with empty ID", func(t *testing.T) {
		entity := &Entity{
			ID: "",
		}
		err := manager.RegisterEntity(entity)
		assert.Error(t, err)
	})
}

func TestEntityManager_AddOperationToEntity(t *testing.T) {
	manager := NewEntityManager()

	// Setup: register an entity
	entity := &Entity{
		ID:         "session",
		Name:       "Session",
		Operations: make(map[string]*openapi3.Operation),
	}
	err := manager.RegisterEntity(entity)
	require.NoError(t, err)

	t.Run("add operation to existing entity", func(t *testing.T) {
		op := &openapi3.Operation{
			OperationID: "session.create",
			Summary:     "Create session",
		}

		err := manager.AddOperationToEntity("session", "create", op)
		assert.NoError(t, err)

		retrievedOp, err := manager.GetEntityOperation("session", "create")
		assert.NoError(t, err)
		assert.Equal(t, "session.create", retrievedOp.OperationID)
	})

	t.Run("add operation to non-existent entity", func(t *testing.T) {
		op := &openapi3.Operation{
			OperationID: "test",
		}
		err := manager.AddOperationToEntity("nonexistent", "test", op)
		assert.Error(t, err)
	})
}

func TestEntityManager_ListEntities(t *testing.T) {
	manager := NewEntityManager()

	// Register multiple entities
	entity1 := &Entity{ID: "player", Name: "Player", Operations: make(map[string]*openapi3.Operation)}
	entity2 := &Entity{ID: "session", Name: "Session", Operations: make(map[string]*openapi3.Operation)}

	err := manager.RegisterEntity(entity1)
	require.NoError(t, err)
	err = manager.RegisterEntity(entity2)
	require.NoError(t, err)

	entities := manager.ListEntities()
	assert.Len(t, entities, 2)

	ids := make([]string, 0, 2)
	for _, e := range entities {
		ids = append(ids, e.ID)
	}
	assert.ElementsMatch(t, []string{"player", "session"}, ids)
}

func TestProviderConverter_ToOpenAPIOperation(t *testing.T) {
	converter := NewProviderConverter()

	t.Run("convert basic descriptor", func(t *testing.T) {
		desc := &FunctionDescriptor{
			ID:          "player.ban",
			Summary:     "Ban player",
			Description: "Ban a player from the game",
			Category:    "player",
			Risk:        "high",
			Operation:   "update",
		}

		op, err := converter.ToOpenAPIOperation(desc)
		require.NoError(t, err)
		assert.Equal(t, "player.ban", op.OperationID)
		assert.Equal(t, "Ban player", op.Summary)
		assert.Equal(t, "player", op.Extensions["x-category"])
		assert.Equal(t, "high", op.Extensions["x-risk"])
		assert.Equal(t, "update", op.Extensions["x-operation"])
	})

	t.Run("convert nil descriptor", func(t *testing.T) {
		_, err := converter.ToOpenAPIOperation(nil)
		assert.Error(t, err)
	})

	t.Run("convert descriptor with entity", func(t *testing.T) {
		desc := &FunctionDescriptor{
			ID:        "session.create",
			Summary:   "Create session",
			Entity:    "session",
			Operation: "create",
		}

		op, err := converter.ToOpenAPIOperation(desc)
		require.NoError(t, err)
		assert.Equal(t, "session", op.Extensions["x-entity"])
		assert.Equal(t, "create", op.Extensions["x-operation"])
	})
}
