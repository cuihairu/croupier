package function

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// failQueryOnTableGN registers a Query callback that fails the Nth (1-based)
// SELECT executed against the given table. SQLite triggers cannot intercept
// SELECT statements, so the callback seam is the only deterministic way to
// inject List errors for a table whose writes must keep succeeding.
func failQueryOnTableGN(t *testing.T, db *gorm.DB, table string, failAt int) *int {
	t.Helper()
	counter := new(int)
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(
		fmt.Sprintf("groupn_fail_%s_%d", table, failAt),
		func(tx *gorm.DB) {
			if tx.Statement == nil {
				return
			}
			name := tx.Statement.Table
			if name == "" && tx.Statement.Schema != nil {
				name = tx.Statement.Schema.Table
			}
			if name != table {
				return
			}
			*counter++
			if *counter == failAt {
				_ = tx.AddError(errors.New("groupn injected query failure: " + table))
			}
		},
	))
	return counter
}

// ---------------------------------------------------------------------------
// FunctionAnalytics: countInRange error branches
//
// FunctionAnalytics issues four config_versions queries in a fixed order:
// Q1 outer List (line 55), Q2 day range, Q3 week range, Q4 month range.
// Failing Q2/Q3/Q4 exercises the three countInRange error returns.
// ---------------------------------------------------------------------------

func TestGroupN_FunctionAnalyticsCountInRangeErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		failAt int
	}{
		{"day range fails", 2},
		{"week range fails", 3},
		{"month range fails", 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, model.AutoMigrate(db))
			failQueryOnTableGN(t, db, "config_versions", tc.failAt)

			svcCtx := &svc.ServiceContext{
				DB:                 db,
				FunctionModel:      model.NewFunctionModel(db),
				ConfigVersionModel: model.NewConfigVersionModel(db),
			}

			_, err = NewFunctionAnalyticsLogic(context.Background(), svcCtx).
				FunctionAnalytics(&FunctionAnalyticsRequest{ID: "fn.gn.count"})
			assert.Error(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// FunctionPermissionsUpdate: ListPermissions error after successful replace
//
// ReplacePermissions only issues DELETE/INSERT (no SELECT on
// function_permissions), so failing the first SELECT on that table hits
// exactly the ListPermissions call on line 56.
// ---------------------------------------------------------------------------

func TestGroupN_FunctionPermissionsUpdateListError(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	failQueryOnTableGN(t, svcCtx.DB, "function_permissions", 1)

	_, err := NewFunctionPermissionsUpdateLogic(ctx, svcCtx).FunctionPermissionsUpdate(
		&FunctionPermissionsUpdateRequest{ID: "fn.gn.perm"},
	)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// getOrCreateFunctionRecordWithRisk: duplicate-key retry branch
//
// A soft-deleted row keeps occupying the function_id unique index while the
// default query scope hides it, so:
//   - FindByFunctionID -> ErrRecordNotFound
//   - Create -> SQLITE_CONSTRAINT_UNIQUE -> (TranslateError) ErrDuplicatedKey
//   - retry FindByFunctionID -> still ErrRecordNotFound
// ---------------------------------------------------------------------------

func TestGroupN_GetOrCreateFunctionRecordDuplicateKey(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	seed := &model.Function{FunctionID: "fn.gn.dup", Name: "fn.gn.dup", Status: 1}
	require.NoError(t, db.Create(seed).Error)
	require.NoError(t, db.Delete(seed).Error)

	svcCtx := &svc.ServiceContext{DB: db, FunctionModel: model.NewFunctionModel(db)}
	fn, err := getOrCreateFunctionRecordWithRisk(context.Background(), svcCtx, "fn.gn.dup", "")
	assert.Error(t, err)
	assert.Nil(t, fn)
}

// ---------------------------------------------------------------------------
// runtimeFunctions: version override branch
//
// The branch `meta.Version > item.Version` only fires when a lower-version
// agent is iterated before a higher-version one. Map iteration order is
// randomized per run, so registering ten agents with strictly increasing
// versions makes the miss probability 1/10! (~2.8e-7): the branch is missed
// only if agents are visited in exact descending order.
// ---------------------------------------------------------------------------

func TestGroupN_FunctionsListRuntimeVersionOverride(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	versions := []string{
		"0.0.1", "0.0.2", "0.0.3", "0.0.4", "0.0.5",
		"0.0.6", "0.0.7", "0.0.8", "0.0.9", "0.1.0",
	}
	for i, version := range versions {
		require.NoError(t, svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
			AgentID:  fmt.Sprintf("agent-gn-%02d", i),
			GameID:   "demo",
			Env:      "prod",
			Addr:     "127.0.0.1:9400",
			ExpireAt: time.Now().Add(time.Hour),
			Functions: map[string]reg.FunctionMeta{
				"ver.gn": {Enabled: true, Version: version},
			},
		}))
	}

	resp, err := NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&FunctionsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "0.1.0", resp.Items[0].Version)
	assert.Equal(t, 10, resp.Items[0].Instances)
}

// ---------------------------------------------------------------------------
// buildFallbackRequestSchema: required-field branch
//
// Production callers only pass fallbackFields() whose single payload field
// has Required=false; the required-append branch is exercised by feeding the
// private helper a synthetic field list with Required=true directly.
// ---------------------------------------------------------------------------

func TestGroupN_BuildFallbackRequestSchemaRequiredFields(t *testing.T) {
	schema := buildFallbackRequestSchema([]fallbackField{
		{Name: "payload", Type: "object", Description: "Invocation payload", Required: true},
		{Name: "note", Type: "string", Description: "Optional note", Required: false},
	})
	require.NotNil(t, schema)
	assert.Equal(t, []string{"payload"}, schema.Required)
	require.Contains(t, schema.Properties, "payload")
	require.Contains(t, schema.Properties, "note")
	if v, ok := schema.Properties["payload"]; ok && v.Value != nil {
		assert.Equal(t, "Invocation payload", v.Value.Description)
	}
}

// ---------------------------------------------------------------------------
// buildFallbackInputSchema: required-field branch
//
// 与上方 buildFallbackRequestSchema 的 required 分支同理：生产数据源
// fallbackFields() 的唯一字段 Required=false，追加分支由合成字段清单直接
// 驱动。同时锁定 BuildFallbackInputJSONSchema 对生产数据源的恒定投影。
// ---------------------------------------------------------------------------

func TestGroupN_BuildFallbackInputSchemaRequiredFields(t *testing.T) {
	schema := buildFallbackInputSchema([]fallbackField{
		{Name: "payload", Type: "object", Description: "Invocation payload", Required: true},
		{Name: "note", Type: "string", Description: "Optional note", Required: false},
	})
	require.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"])
	assert.Equal(t, []string{"payload"}, schema["required"])
	props, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok, "properties must be a map")
	require.Contains(t, props, "payload")
	require.Contains(t, props, "note")
	payload, ok := props["payload"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", payload["type"])
	assert.Equal(t, "payload", payload["title"])
	assert.Equal(t, "Invocation payload", payload["description"])
}

func TestGroupN_BuildFallbackInputJSONSchemaProductionShape(t *testing.T) {
	schema := BuildFallbackInputJSONSchema("player.update")
	require.NotNil(t, schema)
	// fallbackFields() 当前唯一字段 payload 为可选：required 恒为空数组。
	assert.Equal(t, []string{}, schema["required"])
	props, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok, "properties must be a map")
	require.Contains(t, props, "payload")
	require.NotContains(t, props, "playerId")
}
