package registry

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_UpsertAgent(t *testing.T) {
	store := NewStore()

	t.Run("insert new agent", func(t *testing.T) {
		agent := &AgentSession{
			AgentID: "agent-1",
			GameID:  "game-1",
			Env:     "prod",
			RPCAddr: "localhost:19090",
			Version: "1.0.0",
		}

		store.UpsertAgent(agent)

		retrieved := store.AgentsUnsafe()["agent-1"]
		assert.NotNil(t, retrieved)
		assert.Equal(t, "agent-1", retrieved.AgentID)
		assert.Equal(t, "game-1", retrieved.GameID)
		assert.Equal(t, "localhost:19090", retrieved.RPCAddr)
	})

	t.Run("update existing agent", func(t *testing.T) {
		agent := &AgentSession{
			AgentID: "agent-2",
			GameID:  "game-2",
			Env:     "dev",
		}

		store.UpsertAgent(agent)

		// Update with new version
		agent.Version = "2.0.0"
		agent.Region = "us-west"
		store.UpsertAgent(agent)

		retrieved := store.AgentsUnsafe()["agent-2"]
		assert.Equal(t, "2.0.0", retrieved.Version)
		assert.Equal(t, "us-west", retrieved.Region)
	})

	t.Run("update compatibility rpc addr mirror", func(t *testing.T) {
		agent := &AgentSession{
			AgentID: "agent-3",
			GameID:  "game-3",
			Env:     "test",
			RPCAddr: "legacy-a",
		}
		store.UpsertAgent(agent)

		updated := &AgentSession{
			AgentID: "agent-3",
			GameID:  "game-3",
			Env:     "test",
			RPCAddr: "legacy-b",
		}
		store.UpsertAgent(updated)

		retrieved := store.AgentsUnsafe()["agent-3"]
		assert.Equal(t, "legacy-b", retrieved.RPCAddr)
	})
}

func TestStore_OpenAPIOperations(t *testing.T) {
	store := NewStore()

	t.Run("upsert and get OpenAPI operation", func(t *testing.T) {
		operation := &openapi3.Operation{
			OperationID: "player.ban",
			Summary:     "Ban a player",
		}

		err := store.UpsertOpenAPI("player.ban", operation)
		require.NoError(t, err)

		retrieved, err := store.GetOpenAPI("player.ban")
		require.NoError(t, err)
		assert.Equal(t, "player.ban", retrieved.OperationID)
	})

	t.Run("get non-existent operation", func(t *testing.T) {
		_, err := store.GetOpenAPI("nonexistent")
		assert.Error(t, err)
	})

	t.Run("list all operations", func(t *testing.T) {
		op1 := &openapi3.Operation{OperationID: "op1"}
		op2 := &openapi3.Operation{OperationID: "op2"}

		store.UpsertOpenAPI("op1", op1)
		store.UpsertOpenAPI("op2", op2)

		operations := store.ListOpenAPIOperations()
		assert.Contains(t, operations, "op1")
		assert.Contains(t, operations, "op2")
	})

	t.Run("delete operation", func(t *testing.T) {
		op := &openapi3.Operation{OperationID: "to_delete"}
		store.UpsertOpenAPI("to_delete", op)

		err := store.DeleteOpenAPI("to_delete")
		require.NoError(t, err)

		_, err = store.GetOpenAPI("to_delete")
		assert.Error(t, err)
	})
}

func TestStore_OpenAPIProviders(t *testing.T) {
	store := NewStore()

	t.Run("upsert and get provider", func(t *testing.T) {
		caps := OpenAPIProviderCaps{
			ID:      "provider-test-1",
			Version: "1.0.0",
			Lang:    "go",
			SDK:     "croupier-go-sdk",
		}

		err := store.UpsertOpenAPIProvider(caps)
		require.NoError(t, err)

		retrieved, err := store.GetOpenAPIProvider("provider-test-1")
		require.NoError(t, err)
		assert.Equal(t, "provider-test-1", retrieved.ID)
		assert.Equal(t, "go", retrieved.Lang)
	})

	t.Run("get non-existent provider", func(t *testing.T) {
		_, err := store.GetOpenAPIProvider("nonexistent")
		assert.Error(t, err)
	})

	t.Run("list all providers", func(t *testing.T) {
		caps1 := OpenAPIProviderCaps{ID: "prov-test-1", Lang: "go"}
		caps2 := OpenAPIProviderCaps{ID: "prov-test-2", Lang: "java"}

		store.UpsertOpenAPIProvider(caps1)
		store.UpsertOpenAPIProvider(caps2)

		providers := store.ListOpenAPIProviders()

		// Count only our test providers
		testProviderCount := 0
		for _, p := range providers {
			if p.ID == "prov-test-1" || p.ID == "prov-test-2" {
				testProviderCount++
			}
		}
		assert.Equal(t, 2, testProviderCount)
	})

	t.Run("delete provider", func(t *testing.T) {
		caps := OpenAPIProviderCaps{ID: "provider-to-delete"}
		store.UpsertOpenAPIProvider(caps)

		err := store.DeleteOpenAPIProvider("provider-to-delete")
		require.NoError(t, err)

		_, err = store.GetOpenAPIProvider("provider-to-delete")
		assert.Error(t, err)
	})
}

func TestStore_BuildOpenAPISpec(t *testing.T) {
	store := NewStore()

	t.Run("build spec with operations", func(t *testing.T) {
		op1 := &openapi3.Operation{
			OperationID: "player.ban",
			Summary:     "Ban player",
		}
		op2 := &openapi3.Operation{
			OperationID: "player.kick",
			Summary:     "Kick player",
		}

		store.UpsertOpenAPI("player.ban", op1)
		store.UpsertOpenAPI("player.kick", op2)

		spec, err := store.BuildOpenAPISpec()
		require.NoError(t, err)

		assert.Equal(t, "3.0.3", spec.OpenAPI)
		assert.Equal(t, "Croupier Functions", spec.Info.Title)

		// Check paths using Paths.Map() method
		pathsMap := spec.Paths.Map()
		assert.Contains(t, pathsMap, "/functions/player.ban")
		assert.Contains(t, pathsMap, "/functions/player.kick")
	})

	t.Run("build empty spec", func(t *testing.T) {
		emptyStore := NewStore()
		spec, err := emptyStore.BuildOpenAPISpec()
		require.NoError(t, err)

		assert.Equal(t, "3.0.3", spec.OpenAPI)
		// Check that paths map is empty
		pathsMap := spec.Paths.Map()
		assert.Empty(t, pathsMap)
	})
}

func TestStore_Errors(t *testing.T) {
	store := NewStore()

	t.Run("upsert OpenAPI with empty function ID", func(t *testing.T) {
		err := store.UpsertOpenAPI("", &openapi3.Operation{})
		assert.Error(t, err)
	})

	t.Run("upsert OpenAPI with nil operation", func(t *testing.T) {
		err := store.UpsertOpenAPI("test.id", nil)
		assert.Error(t, err)
	})

	t.Run("upsert provider with empty ID", func(t *testing.T) {
		err := store.UpsertOpenAPIProvider(OpenAPIProviderCaps{})
		assert.Error(t, err)
	})
}
