package resource

import (
	"context"
	"encoding/json"
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
				Enabled:          true,
				Version:          "1.0.0",
				Tags:             []string{"player"},
				Summary:          "List players",
				Description:      "List player accounts",
				InputSchema:      `{"type":"object","properties":{"keyword":{"type":"string"}}}`,
				OutputSchema:     `{"type":"object","properties":{"items":{"type":"array"}}}`,
				Category:         "ops",
				Risk:             "safe",
				Entity:           "player",
				Operation:        "list",
				CategoryDisplay:  map[string]string{"zh": "运营", "en": "Operations"},
				EntityDisplay:    map[string]string{"zh": "玩家", "en": "Player"},
				OperationDisplay: map[string]string{"zh": "查询", "en": "List"},
				OperationKind:    "list",
				Placement:        "table_data",
			},
			"player.ban": {
				Enabled:          true,
				Version:          "1.0.0",
				Tags:             []string{"player", "moderation"},
				Summary:          "Ban player",
				Description:      "Ban a player account",
				InputSchema:      `{"type":"object","properties":{"player_id":{"type":"string"}}}`,
				OutputSchema:     `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
				Category:         "ops",
				Risk:             "danger",
				Entity:           "player",
				Operation:        "ban",
				CategoryDisplay:  map[string]string{"zh-CN": "运营", "en-US": "Operations"},
				EntityDisplay:    map[string]string{"zh-CN": "玩家", "en-US": "Player"},
				OperationDisplay: map[string]string{"zh-CN": "封禁", "en-US": "Ban"},
				OperationKind:    "action",
				Placement:        "rowAction",
				PageHint:         "player.manage",
				Extensions:       map[string]string{"x-owner": "gm"},
			},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.List(context.Background(), &ResourceListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	player := resp.Items[0]
	assert.Equal(t, "player", player.Key)
	assert.Equal(t, "玩家", player.Labels["zh-CN"])
	assert.Equal(t, "Player", player.Labels["en-US"])
	assert.Equal(t, "ops", player.Category.Key)
	assert.Equal(t, "运营", player.Category.Labels["zh-CN"])
	require.Len(t, player.Operations, 2)

	ops := map[string]spec.OperationSpec{}
	for _, op := range player.Operations {
		ops[op.FunctionID] = op
	}

	listOp := ops["player.list"]
	assert.Equal(t, "list", listOp.Operation)
	assert.Equal(t, spec.OperationKindList, listOp.Kind)
	assert.Equal(t, spec.PlacementTableData, listOp.Placement)
	assert.Equal(t, "查询", listOp.Labels["zh-CN"])
	assert.Empty(t, listOp.Diagnostics)

	banOp := ops["player.ban"]
	assert.Equal(t, "ban", banOp.Operation)
	assert.Equal(t, spec.OperationKindAction, banOp.Kind)
	assert.Equal(t, spec.PlacementRowAction, banOp.Placement)
	assert.Equal(t, spec.RiskDanger, banOp.Risk)
	assert.Equal(t, "封禁", banOp.Labels["zh-CN"])
	assert.Empty(t, banOp.Diagnostics)
}

func TestServiceGeneratedPagesUsesOperationKindAndPlacement(t *testing.T) {
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.list": {
				Enabled:          true,
				Version:          "1.0.0",
				InputSchema:      `{"type":"object"}`,
				Category:         "ops",
				Entity:           "player",
				Operation:        "list",
				CategoryDisplay:  map[string]string{"zh-CN": "运营"},
				EntityDisplay:    map[string]string{"zh-CN": "玩家"},
				OperationDisplay: map[string]string{"zh-CN": "查询"},
				OperationKind:    "list",
				Placement:        "tableData",
				Extensions: map[string]string{
					"x-page-contract": mustJSON(t, spec.PageContract{
						Version: "v1",
						InputMapping: json.RawMessage(
							`{"page":"$.pagination.page","pageSize":"$.pagination.pageSize"}`,
						),
						OutputMapping: json.RawMessage(
							`{"stateKey":"players","itemsPath":"$.response.items","totalPath":"$.response.total"}`,
						),
						Pagination: &spec.PagePaginationContract{
							PageField:     "page",
							PageSizeField: "pageSize",
							ItemsPath:     "$.response.items",
							TotalPath:     "$.response.total",
						},
						Table: &spec.PageTableContract{
							Columns: []spec.PageTableColumnContract{
								{Key: "id", Title: spec.LocalizedText{"zh-CN": "玩家 ID"}, ValuePath: "id"},
							},
						},
					}),
				},
			},
			"player.ban": {
				Enabled:          true,
				Version:          "1.0.0",
				InputSchema:      `{"type":"object"}`,
				Category:         "ops",
				Entity:           "player",
				Operation:        "ban",
				CategoryDisplay:  map[string]string{"zh-CN": "运营"},
				EntityDisplay:    map[string]string{"zh-CN": "玩家"},
				OperationDisplay: map[string]string{"zh-CN": "封禁"},
				OperationKind:    "action",
				Placement:        "rowAction",
				Extensions: map[string]string{
					"x-page-contract": mustJSON(t, spec.PageContract{
						Version:      "v1",
						InputMapping: json.RawMessage(`{"playerId":"$.row.id"}`),
					}),
				},
			},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.GeneratedPages(context.Background(), &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeEntity, page.Type)
	assert.Equal(t, "player.manage", page.PageKey)
	assert.Equal(t, "ops", page.Category.Key)
	assert.Contains(t, string(page.Schema), `"x-component":"DataTable"`)
	assert.Contains(t, string(page.Schema), `"bindingId":"player.list"`)
	assert.Contains(t, string(page.Schema), `"bindingId":"player.ban"`)
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
				Enabled:          true,
				Version:          "1.0.0",
				InputSchema:      `{"type":"object"}`,
				Category:         "ops",
				Entity:           "player",
				Operation:        "list",
				CategoryDisplay:  map[string]string{"zh-CN": "运营"},
				EntityDisplay:    map[string]string{"zh-CN": "玩家"},
				OperationDisplay: map[string]string{"zh-CN": "查询"},
				OperationKind:    "list",
				Placement:        "tableData",
			},
		},
	})

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.GeneratedPages(context.Background(), &ResourceGeneratedPagesRequest{ResourceKey: "player"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)

	page := resp.Items[0]
	assert.Equal(t, spec.PageTypeEntity, page.Type)
	assert.Equal(t, "needs_review", page.Quality)
	assert.NotContains(t, string(page.Schema), `"x-component":"DataTable"`)
	assert.Contains(t, string(page.Schema), `"x-component":"QueryForm"`)
	require.NotEmpty(t, page.Diagnostics)
	assert.Equal(t, "page_contract_missing", page.Diagnostics[0].Code)
}

func mustJSON[T any](t *testing.T, value T) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}
