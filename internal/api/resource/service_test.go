package resource

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceListCollectsRegistryDescriptorV2Metadata(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "game-1",
		Env:      "prod",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:      true,
				Version:      "1.0.0",
				Tags:         []string{"player"},
				Summary:      "List players",
				Description:  "List player accounts",
				InputSchema:  `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"items":{"type":"array"}}}`,
				Resource:     "player",
				Risk:         "safe",
				Operation:    "list",
				Permission:   "player:list",
			},
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				Tags:         []string{"player", "moderation"},
				Summary:      "Ban player",
				Description:  "Ban a player account",
				InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
				OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
				Resource:     "player",
				Risk:         "danger",
				Operation:    "ban",
				Permission:   "player:ban",
			},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.List(context.Background(), &ResourceListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	player := resp.Items[0]
	assert.Equal(t, "player", player.Key)
	assert.Equal(t, "player", player.Labels["zh-CN"])
	assert.Equal(t, "player", player.Category.Key)
	assert.Equal(t, "player", player.Category.Labels["zh-CN"])
	require.Len(t, player.Operations, 2)

	ops := map[string]spec.OperationSpec{}
	for _, op := range player.Operations {
		ops[op.FunctionID] = op
	}

	listOp := ops["player.list"]
	assert.Equal(t, "list", listOp.Operation)
	assert.Equal(t, "player:list", listOp.Permission)
	assert.Empty(t, listOp.Diagnostics)

	banOp := ops["player.ban"]
	assert.Equal(t, "ban", banOp.Operation)
	assert.Equal(t, spec.RiskDanger, banOp.Risk)
	assert.Equal(t, "player:ban", banOp.Permission)
	assert.Empty(t, banOp.Diagnostics)
}

func TestServiceGeneratedPagesCreatesConservativeOperationCandidates(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:     true,
				Version:     "1.0.0",
				InputSchema: `{"type":"object"}`,
				Resource:    "player",
				Operation:   "list",
			},
			"player.ban": {
				Enabled:     true,
				Version:     "1.0.0",
				InputSchema: `{"type":"object"}`,
				Resource:    "player",
				Operation:   "ban",
			},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.GeneratedPages(context.Background(), &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, "player.ban", page.PageKey)
	assert.Equal(t, "player", page.Category.Key)
	assert.Contains(t, string(page.Schema), `"x-component":"QueryForm"`)
	assert.Contains(t, string(page.Schema), `"bindingId":"player.main"`)
	assert.Equal(t, "needs_review", page.Quality)
	assert.NotContains(t, string(page.Schema), `"functionId"`)
	assert.NotContains(t, string(page.Schema), `"operation":"update"`)
}

func TestServiceGeneratedPagesDoesNotGuessTableContract(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:     true,
				Version:     "1.0.0",
				InputSchema: `{"type":"object"}`,
				Resource:    "player",
				Operation:   "list",
			},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.GeneratedPages(context.Background(), &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, "needs_review", page.Quality)
	assert.NotContains(t, string(page.Schema), `"x-component":"DataTable"`)
	assert.Contains(t, string(page.Schema), `"x-component":"QueryForm"`)
	require.NotEmpty(t, page.Diagnostics)
	assert.Equal(t, "page_contract_missing", page.Diagnostics[0].Code)
}
