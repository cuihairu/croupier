package function

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// openClosedDBV9 opens an in-memory sqlite DB and closes the underlying
// handle so all subsequent queries fail with "database is closed".
func openClosedDBV9(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return db
}

func insertNilAgentSessionV9(store *reg.Store) {
	store.Mu().Lock()
	store.AgentsUnsafe()["v9-nil-agent"] = nil
	store.Mu().Unlock()
}

// ---------------------------------------------------------------------------
// ValidateFunctionID error branches across logic entry points
// ---------------------------------------------------------------------------

func TestFunctionLogicEmptyIDBranchesV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)

	t.Run("analytics", func(t *testing.T) {
		_, err := NewFunctionAnalyticsLogic(ctx, svcCtx).FunctionAnalytics(&FunctionAnalyticsRequest{ID: "   "})
		assert.Error(t, err)
	})

	t.Run("detail", func(t *testing.T) {
		_, err := NewFunctionDetailLogic(ctx, svcCtx).FunctionDetail(&FunctionDetailRequest{ID: ""})
		assert.Error(t, err)
	})

	t.Run("disable", func(t *testing.T) {
		err := NewFunctionDisableLogic(ctx, svcCtx).FunctionDisable(&FunctionActionRequest{ID: "  "})
		assert.Error(t, err)
	})

	t.Run("enable", func(t *testing.T) {
		err := NewFunctionEnableLogic(ctx, svcCtx).FunctionEnable(&FunctionActionRequest{ID: ""})
		assert.Error(t, err)
	})

	t.Run("history", func(t *testing.T) {
		_, _, err := NewFunctionHistoryLogic(ctx, svcCtx).FunctionHistory(&FunctionHistoryRequest{ID: " "})
		assert.Error(t, err)
	})

	t.Run("instances", func(t *testing.T) {
		_, err := NewFunctionInstancesLogic(ctx, svcCtx).FunctionInstances(&FunctionInstancesRequest{ID: ""})
		assert.Error(t, err)
	})

	t.Run("invoke", func(t *testing.T) {
		_, err := NewFunctionInvokeLogic(ctx, svcCtx).FunctionInvoke(&FunctionInvokeRequest{ID: "  "})
		assert.Error(t, err)
	})

	t.Run("permissions", func(t *testing.T) {
		_, err := NewFunctionPermissionsLogic(ctx, svcCtx).FunctionPermissions(&FunctionPermissionsRequest{ID: " "})
		assert.Error(t, err)
	})

	t.Run("permissions update", func(t *testing.T) {
		_, err := NewFunctionPermissionsUpdateLogic(ctx, svcCtx).FunctionPermissionsUpdate(&FunctionPermissionsUpdateRequest{ID: ""})
		assert.Error(t, err)
	})

	t.Run("publish", func(t *testing.T) {
		_, err := NewFunctionPublishLogic(ctx, svcCtx).FunctionPublish(&FunctionPublishRequest{ID: "   "})
		assert.Error(t, err)
	})

	t.Run("copy", func(t *testing.T) {
		_, err := NewFunctionCopyLogic(ctx, svcCtx).FunctionCopy(&FunctionCopyRequest{ID: ""})
		assert.Error(t, err)
	})

	t.Run("delete", func(t *testing.T) {
		err := NewFunctionDeleteLogic(ctx, svcCtx).FunctionDelete(&FunctionActionRequest{ID: ""})
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Batch operations: model-layer error paths
// ---------------------------------------------------------------------------

func TestBatchOperationsModelErrorsV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("functions"))

	_, err := NewBatchUpdateFunctionsLogic(ctx, svcCtx).BatchUpdateFunctions(&BatchUpdateFunctionsRequest{
		FunctionIds: []string{"a.b"}, Enabled: true,
	})
	assert.Error(t, err)

	_, err = NewBatchDeleteFunctionsLogic(ctx, svcCtx).BatchDeleteFunctions(&BatchDeleteFunctionsRequest{
		FunctionIds: []string{"a.b"},
	})
	assert.Error(t, err)

	// BatchCopyFunctions swallows per-item failures into the failed list and
	// never surfaces an error, so only the response shape is asserted here.
	resp, err := NewBatchCopyFunctionsLogic(ctx, svcCtx).BatchCopyFunctions(&BatchCopyFunctionsRequest{
		FunctionIds: []string{"a.b"},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Updated)
	assert.Contains(t, resp.Failed, "a.b")
}

// ---------------------------------------------------------------------------
// FunctionsPending: ListPending error
// ---------------------------------------------------------------------------

func TestFunctionsPendingListErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("pending_functions"))

	_, err := NewFunctionsPendingLogic(ctx, svcCtx).FunctionsPending(&FunctionsPendingRequest{})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionPublish / FunctionDisable / FunctionEnable: Update failure
// ---------------------------------------------------------------------------

func blockFunctionUpdatesV9(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(
		"CREATE TRIGGER v9_block_function_update BEFORE UPDATE ON functions BEGIN SELECT RAISE(ABORT, 'v9 blocked'); END",
	).Error)
}

func TestFunctionPublishUpdateErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	seedFunctionForBatch(t, svcCtx, "publish.fail")
	blockFunctionUpdatesV9(t, svcCtx.DB)

	_, err := NewFunctionPublishLogic(ctx, svcCtx).FunctionPublish(&FunctionPublishRequest{ID: "publish.fail"})
	assert.Error(t, err)
}

func TestFunctionDisableUpdateErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	seedFunctionForBatch(t, svcCtx, "disable.fail")
	blockFunctionUpdatesV9(t, svcCtx.DB)

	err := NewFunctionDisableLogic(ctx, svcCtx).FunctionDisable(&FunctionActionRequest{ID: "disable.fail"})
	assert.Error(t, err)
}

func TestFunctionEnableUpdateErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	seedFunctionForBatch(t, svcCtx, "enable.fail")
	blockFunctionUpdatesV9(t, svcCtx.DB)

	err := NewFunctionEnableLogic(ctx, svcCtx).FunctionEnable(&FunctionActionRequest{ID: "enable.fail"})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionCopy / FunctionDelete: model error paths
// ---------------------------------------------------------------------------

func TestFunctionCopyModelErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	seedFunctionForBatch(t, svcCtx, "copy.fail")
	require.NoError(t, svcCtx.DB.Migrator().DropTable("functions"))

	_, err := NewFunctionCopyLogic(ctx, svcCtx).FunctionCopy(&FunctionCopyRequest{ID: "copy.fail"})
	assert.Error(t, err)
}

func TestFunctionDeleteModelErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("functions"))

	err := NewFunctionDeleteLogic(ctx, svcCtx).FunctionDelete(&FunctionActionRequest{ID: "delete.fail"})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionPermissions: nil model + ListPermissions error
// ---------------------------------------------------------------------------

func TestFunctionPermissionsNilModelV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.FunctionModel = nil

	_, err := NewFunctionPermissionsLogic(ctx, svcCtx).FunctionPermissions(&FunctionPermissionsRequest{ID: "fn.perm"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FunctionModel")
}

func TestFunctionPermissionsListErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("function_permissions"))

	_, err := NewFunctionPermissionsLogic(ctx, svcCtx).FunctionPermissions(&FunctionPermissionsRequest{ID: "fn.perm"})
	assert.Error(t, err)
}

func TestFunctionPermissionsAdminLoadErrorV9(t *testing.T) {
	svcCtx, _ := setupFullTestContext(t)

	_, err := NewFunctionPermissionsLogic(context.Background(), svcCtx).FunctionPermissions(&FunctionPermissionsRequest{ID: "fn.perm"})
	assert.Error(t, err)
}

func TestFunctionPermissionsPermIDErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("role_permissions"))

	_, err := NewFunctionPermissionsLogic(ctx, svcCtx).FunctionPermissions(&FunctionPermissionsRequest{ID: "fn.perm"})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionPermissionsUpdate: error branches
// ---------------------------------------------------------------------------

func TestFunctionPermissionsUpdateAdminLoadErrorV9(t *testing.T) {
	svcCtx, _ := setupFullTestContext(t)
	_, err := NewFunctionPermissionsUpdateLogic(context.Background(), svcCtx).
		FunctionPermissionsUpdate(&FunctionPermissionsUpdateRequest{ID: "fn.perm"})
	assert.Error(t, err)
}

func TestFunctionPermissionsUpdatePermIDErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("role_permissions"))

	_, err := NewFunctionPermissionsUpdateLogic(ctx, svcCtx).
		FunctionPermissionsUpdate(&FunctionPermissionsUpdateRequest{ID: "fn.perm"})
	assert.Error(t, err)
}

func TestFunctionPermissionsUpdateInvalidResourceV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)

	_, err := NewFunctionPermissionsUpdateLogic(ctx, svcCtx).FunctionPermissionsUpdate(&FunctionPermissionsUpdateRequest{
		ID:          "fn.perm",
		Permissions: []FunctionPermission{{Resource: "   "}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "权限资源名称不能为空")
}

func TestFunctionPermissionsUpdateReplaceErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("function_permissions"))

	_, err := NewFunctionPermissionsUpdateLogic(ctx, svcCtx).FunctionPermissionsUpdate(&FunctionPermissionsUpdateRequest{
		ID:          "fn.perm",
		Permissions: []FunctionPermission{{Resource: "prom.query", Actions: []string{"read"}}},
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionAnalytics: getOrCreate failure + ConfigVersionModel failure
// ---------------------------------------------------------------------------

func TestFunctionAnalyticsRecordErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.FunctionModel = nil

	_, err := NewFunctionAnalyticsLogic(ctx, svcCtx).FunctionAnalytics(&FunctionAnalyticsRequest{ID: "fn.an"})
	assert.Error(t, err)
}

func TestFunctionAnalyticsConfigVersionErrorV9(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	closedDB := openClosedDBV9(t)

	svcCtx := &svc.ServiceContext{
		DB:                 db,
		FunctionModel:      model.NewFunctionModel(db),
		ConfigVersionModel: model.NewConfigVersionModel(closedDB),
	}
	ctx := context.Background()

	_, err = NewFunctionAnalyticsLogic(ctx, svcCtx).FunctionAnalytics(&FunctionAnalyticsRequest{ID: "fn.an2"})
	assert.Error(t, err)
}

func TestFunctionAnalyticsConfigVersionSuccessV9(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	svcCtx := &svc.ServiceContext{
		DB:                 db,
		FunctionModel:      model.NewFunctionModel(db),
		ConfigVersionModel: model.NewConfigVersionModel(db),
	}
	ctx := context.Background()
	require.NoError(t, svcCtx.FunctionModel.Create(ctx, &model.Function{FunctionID: "fn.an3", Name: "fn.an3", Status: 1}))
	_, err = svcCtx.ConfigVersionModel.Create(ctx, "function_form:fn.an3", `{"name":"x"}`, "unit")
	require.NoError(t, err)

	resp, err := NewFunctionAnalyticsLogic(ctx, svcCtx).FunctionAnalytics(&FunctionAnalyticsRequest{ID: "fn.an3"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.TotalCalls)
	assert.Equal(t, int64(1), resp.CallsToday)
}

// ---------------------------------------------------------------------------
// FunctionHistory: getOrCreate failure + offset slicing
// ---------------------------------------------------------------------------

func TestFunctionHistoryRecordErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.FunctionModel = nil

	_, _, err := NewFunctionHistoryLogic(ctx, svcCtx).FunctionHistory(&FunctionHistoryRequest{ID: "fn.hist"})
	assert.Error(t, err)
}

func TestFunctionHistoryOffsetSlicingV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.ConfigVersionModel = model.NewConfigVersionModel(svcCtx.DB)
	for i := 1; i <= 2; i++ {
		require.NoError(t, svcCtx.DB.Create(&model.ConfigVersion{
			Key:       "function_form:fn.hist2",
			Version:   i,
			Value:     `{"name":"x"}`,
			CreatedBy: "tester",
		}).Error)
	}

	items, total, err := NewFunctionHistoryLogic(ctx, svcCtx).FunctionHistory(&FunctionHistoryRequest{ID: "fn.hist2", Offset: 1})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, items, 2)
}

// ---------------------------------------------------------------------------
// DescriptorsV2: scope / permission / query error branches
// ---------------------------------------------------------------------------

func TestDescriptorsV2MissingEnvV9(t *testing.T) {
	svcCtx, _ := setupNoAuthTestContext(t)
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo"})
	_, err := NewDescriptorsLogic(ctx, svcCtx).DescriptorsV2(&DescriptorsRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "X-Env is required")
}

func TestDescriptorsV2MissingGameV9(t *testing.T) {
	svcCtx, _ := setupNoAuthTestContext(t)
	_, err := NewDescriptorsLogic(context.Background(), svcCtx).DescriptorsV2(&DescriptorsRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")
}

func TestDescriptorsV2QueryErrorV9(t *testing.T) {
	svcCtx, _ := setupNoAuthTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("function_contracts"))

	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	_, err := NewDescriptorsLogic(ctx, svcCtx).DescriptorsV2(&DescriptorsRequest{})
	assert.Error(t, err)
}

func TestDescriptorsV2NilDBV9(t *testing.T) {
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	_, err := NewDescriptorsLogic(ctx, &svc.ServiceContext{}).DescriptorsV2(&DescriptorsRequest{})
	assert.Error(t, err)
}

func TestDescriptorsV2AdminLoadErrorV9(t *testing.T) {
	svcCtx, _ := setupNoAuthTestContext(t)
	// Username present in ctx but no AdminModel wired: LoadCurrentAdmin fails.
	ctx := context.WithValue(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"}),
		"username", "ghost",
	)
	_, err := NewDescriptorsLogic(ctx, svcCtx).DescriptorsV2(&DescriptorsRequest{})
	assert.Error(t, err)
}

func TestDescriptorsV2PermIDQueryErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("role_permissions"))

	scopedCtx := svc.WithGameScope(ctx, svc.GameScope{GameID: "demo", Env: "prod"})
	_, err := NewDescriptorsLogic(scopedCtx, svcCtx).DescriptorsV2(&DescriptorsRequest{})
	assert.Error(t, err)
}

func TestDescriptorsV2ResourceFilterAndSortV9(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FunctionContract{}))

	seedContractV9(t, db, "demo", "prod", "b.second", "player")
	seedContractV9(t, db, "demo", "prod", "a.first", "economy")
	seedContractV9(t, db, "demo", "prod", "c.other", "player")

	svcCtx := &svc.ServiceContext{DB: db}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	logic := NewDescriptorsLogic(ctx, svcCtx)

	// Resource filter drops non-matching contracts.
	result, err := logic.DescriptorsV2(&DescriptorsRequest{Resource: "player"})
	require.NoError(t, err)
	require.Len(t, result.Functions, 2)
	// Sort body executes with >1 item.
	assert.Equal(t, "b.second", result.Functions[0].ID)
	assert.Equal(t, "c.other", result.Functions[1].ID)

	// No filter returns everything sorted.
	result, err = logic.DescriptorsV2(&DescriptorsRequest{})
	require.NoError(t, err)
	assert.Len(t, result.Functions, 3)
	assert.Equal(t, "a.first", result.Functions[0].ID)
}

func seedContractV9(t *testing.T, db *gorm.DB, gameID, env, functionID, resource string) {
	t.Helper()
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID:      gameID,
		Env:         env,
		FunctionID:  functionID,
		ResourceKey: resource,
		Version:     "1.0.0",
		Enabled:     true,
	}).Error)
}

// ---------------------------------------------------------------------------
// schemaRefToMap: marshal failure via unmarshalable extension
// ---------------------------------------------------------------------------

func TestSchemaRefToMapMarshalErrorV9(t *testing.T) {
	schema := &openapi3.Schema{
		Extensions: map[string]interface{}{"x-bad": make(chan int)},
	}
	out := schemaRefToMap(&openapi3.SchemaRef{Value: schema})
	assert.Nil(t, out)
}

// ---------------------------------------------------------------------------
// getOrCreateFunctionRecordWithRisk: create error, policy branches
// ---------------------------------------------------------------------------

func TestGetOrCreateFunctionRecordQueryErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("functions"))

	_, err := getOrCreateFunctionRecord(ctx, svcCtx, "fn.rec")
	assert.Error(t, err)
}

func TestGetOrCreateFunctionRecordCreateErrorV9(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	// Table exists but inserts are blocked, so Find returns RecordNotFound
	// while Create fails.
	require.NoError(t, svcCtx.DB.Exec(
		"CREATE TRIGGER v9_block_function_insert BEFORE INSERT ON functions BEGIN SELECT RAISE(ABORT, 'v9 blocked'); END",
	).Error)

	_, err := getOrCreateFunctionRecord(ctx, svcCtx, "fn.rec2")
	assert.Error(t, err)
}

func TestGetOrCreateFunctionRecordPolicyDefaultRiskV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	pm, err := policymgr.NewManager(svcCtx.DB, "")
	require.NoError(t, err)
	svcCtx.PolicyManager = pm

	fn, err := getOrCreateFunctionRecordWithRisk(ctx, svcCtx, "fn.risk.default", "")
	require.NoError(t, err)
	assert.Equal(t, "fn.risk.default", fn.FunctionID)
}

func TestGetOrCreateFunctionRecordPolicyErrorV9(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	closedDB := openClosedDBV9(t)
	pm, err := policymgr.NewManager(closedDB, "")
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
		PolicyManager: pm,
	}

	// The record is still created even when the default policy fails.
	fn, err := getOrCreateFunctionRecordWithRisk(context.Background(), svcCtx, "fn.risk.err", "high")
	require.NoError(t, err)
	assert.Equal(t, "fn.risk.err", fn.FunctionID)
}

func TestBackfillFromRegistryNilSessionV9(t *testing.T) {
	svcCtx, _ := setupNoAuthTestContext(t)
	insertNilAgentSessionV9(svcCtx.RegistryStore)

	fn := &model.Function{FunctionID: "fn.nilsess"}
	backfillFromRegistry(svcCtx, "fn.nilsess", fn)
	assert.Equal(t, "fn.nilsess", fn.FunctionID)
}

// ---------------------------------------------------------------------------
// FunctionDetail: runtime record + create failure; nil session in store
// ---------------------------------------------------------------------------

func TestFunctionDetailRuntimeCreateFailsV9(t *testing.T) {
	svcCtx, _ := setupNoAuthTestContext(t)
	require.NoError(t, svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-detail",
		GameID:    "demo",
		Env:       "prod",
		Addr:      "127.0.0.1:9100",
		ExpireAt:  time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{"detail.fail": {Enabled: true}},
	}))
	require.NoError(t, svcCtx.DB.Migrator().DropTable("functions"))

	_, err := NewFunctionDetailLogic(context.Background(), svcCtx).FunctionDetail(&FunctionDetailRequest{ID: "detail.fail"})
	assert.Error(t, err)
}

func TestFunctionDetailNilAgentSessionV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	seedFunctionForBatch(t, svcCtx, "detail.nil")
	insertNilAgentSessionV9(svcCtx.RegistryStore)

	resp, err := NewFunctionDetailLogic(ctx, svcCtx).FunctionDetail(&FunctionDetailRequest{ID: "detail.nil"})
	require.NoError(t, err)
	assert.Equal(t, "detail.nil", resp.Function.ID)
}

// ---------------------------------------------------------------------------
// FunctionInstances / FunctionInstancesAll: registry edge cases + DB fallback
// ---------------------------------------------------------------------------

func TestFunctionInstancesNilAgentSessionV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	insertNilAgentSessionV9(svcCtx.RegistryStore)

	resp, err := NewFunctionInstancesLogic(ctx, svcCtx).FunctionInstances(&FunctionInstancesRequest{ID: "fn.inst"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestFunctionInstancesAllNilAgentSessionV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	insertNilAgentSessionV9(svcCtx.RegistryStore)

	resp, err := NewFunctionInstancesAllLogic(ctx, svcCtx).FunctionInstancesAll()
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestFunctionInstancesDBFallbackV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.RegistryStore = nil
	require.NoError(t, svcCtx.DB.Create(&model.FunctionInstance{
		FunctionID: "fn.dbinst",
		AgentID:    "agent-db",
		AgentName:  "DB Agent",
		Status:     "online",
	}).Error)

	resp, err := NewFunctionInstancesLogic(ctx, svcCtx).FunctionInstances(&FunctionInstancesRequest{ID: "fn.dbinst"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "agent-db", resp.Items[0].AgentId)
	assert.Equal(t, "DB Agent", resp.Items[0].AgentName)
}

func TestFunctionInstancesDBFallbackErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	svcCtx.RegistryStore = nil
	require.NoError(t, svcCtx.DB.Migrator().DropTable("function_instances"))

	_, err := NewFunctionInstancesLogic(ctx, svcCtx).FunctionInstances(&FunctionInstancesRequest{ID: "fn.dbinst"})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionsList: admin branch, db error, runtime merge, filters
// ---------------------------------------------------------------------------

func TestFunctionsListAdminBranchV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	seedFunctionForBatch(t, svcCtx, "list.admin.fn")

	resp, err := NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&FunctionsListRequest{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Total, int64(1))
}

func TestFunctionsListDBErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("functions"))

	_, err := NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&FunctionsListRequest{})
	assert.Error(t, err)
}

func TestFunctionsListRuntimeMergeV9(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	seedFunctionForBatch(t, svcCtx, "merge.fn")
	require.NoError(t, svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-merge",
		GameID:   "demo",
		Env:      "prod",
		Addr:     "127.0.0.1:9200",
		ExpireAt: time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{
			"merge.fn": {Enabled: true, Version: "9.0.0", Tags: []string{"tagged"}, Summary: "merged summary"},
			"rt.only":  {Enabled: true, Version: "1.0.0", Summary: "runtime only"},
		},
	}))

	resp, err := NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&FunctionsListRequest{})
	require.NoError(t, err)

	byID := map[string]Function{}
	for _, item := range resp.Items {
		byID[item.ID] = item
	}

	merged := byID["merge.fn"]
	assert.Equal(t, "9.0.0", merged.Version, "runtime version should override empty DB version")
	assert.Equal(t, []string{"tagged"}, merged.Tags, "runtime tags should backfill DB record")
	assert.Equal(t, map[string]string{"zh": "merged summary", "en": "merged summary"}, merged.Summary)
	assert.Equal(t, 1, merged.Instances)

	runtimeOnly := byID["rt.only"]
	assert.Equal(t, "demo", runtimeOnly.GameId)
	assert.Equal(t, map[string]string{"zh": "runtime only", "en": "runtime only"}, runtimeOnly.Summary)
	assert.Equal(t, []string{}, runtimeOnly.Tags)
}

func TestFunctionsListRuntimeVersionMergeV9(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	for _, agent := range []struct {
		id      string
		version string
	}{
		{"agent-ver-1", "1.0.0"},
		{"agent-ver-2", "2.0.0"},
	} {
		require.NoError(t, svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
			AgentID:  agent.id,
			GameID:   "demo",
			Env:      "prod",
			Addr:     "127.0.0.1:9300",
			ExpireAt: time.Now().Add(time.Hour),
			Functions: map[string]reg.FunctionMeta{
				"ver.fn": {Enabled: true, Version: agent.version},
			},
		}))
	}

	resp, err := NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&FunctionsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "2.0.0", resp.Items[0].Version)
	assert.Equal(t, 2, resp.Items[0].Instances)
}

func TestFunctionsListRuntimeFiltersV9(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	require.NoError(t, svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-filter",
		GameID:   "demo",
		Env:      "prod",
		Addr:     "127.0.0.1:9201",
		ExpireAt: time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{
			"filter.fn": {Enabled: true, Version: "1.0.0", Resource: "player"},
		},
	}))
	insertNilAgentSessionV9(svcCtx.RegistryStore)

	// Status filter: runtime items are always status 1, so requesting 2 filters them out.
	resp, err := NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&FunctionsListRequest{Status: 2})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	// Resource filter mismatch drops the runtime item.
	resp, err = NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&FunctionsListRequest{Resource: "economy"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	// Matching resource keeps it.
	resp, err = NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&FunctionsListRequest{Resource: "player"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "filter.fn", resp.Items[0].ID)
}

// ---------------------------------------------------------------------------
// FunctionInvoke: permission-load error, task-mode errors, broadcast error
// ---------------------------------------------------------------------------

func TestFunctionInvokePermIDLoadErrorV9(t *testing.T) {
	svcCtx, ctx := setupFullTestContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("role_permissions"))

	_, err := NewFunctionInvokeLogic(ctx, svcCtx).FunctionInvoke(&FunctionInvokeRequest{ID: "player.ban"})
	assert.Error(t, err)
}

func TestFunctionInvokeTaskModeScopeErrorV9(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.setupNonAdminScope(t)
	seedNonAdminUser(t, f.svcCtx, "v9-task-user", "function:invoke")

	logic := NewFunctionInvokeLogic(context.WithValue(f.ctx, "username", "v9-task-user"), f.svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{ID: "player.ban", Mode: "task"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "game_id is required")
}

func TestFunctionInvokeTaskModePermissionErrorV9(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.setupNonAdminScope(t)
	seedNonAdminUser(t, f.svcCtx, "v9-task-denied", "function:none")

	logic := NewFunctionInvokeLogic(context.WithValue(f.ctx, "username", "v9-task-denied"), f.svcCtx)
	f.grantGameScope(t, "v9-task-denied")
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Mode: "task", GameID: "demo", Env: "prod",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权调用该函数")
}

func TestFunctionInvokeBroadcastNoAgentErrorV9(t *testing.T) {
	f := newInvokeLogicFixture(t)

	logic := NewFunctionInvokeLogic(f.ctxUser, f.svcCtx)
	_, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID: "player.ban", Route: "broadcast", GameID: "ghost-game", Env: "prod",
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// FunctionWarnings: seeded warnings pass through
// ---------------------------------------------------------------------------

func TestFunctionWarningsWithItemsV9(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	require.NoError(t, svcCtx.RegistryStore.UpsertRegistrationWarning(context.Background(), reg.FunctionRegistrationWarning{
		Key:        "v9-warning-key",
		AgentID:    "agent-w",
		FunctionID: "fn.warn",
		Version:    "1.0.0",
		Code:       "schema_drift",
		Message:    "schema drifted",
	}))

	resp, err := NewFunctionWarningsLogic(ctx, svcCtx).FunctionWarnings(&FunctionWarningsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "v9-warning-key", resp.Items[0].Key)
	assert.Equal(t, "fn.warn", resp.Items[0].FunctionID)
	assert.Equal(t, "schema_drift", resp.Items[0].Code)
	assert.Equal(t, 1, resp.Items[0].Count)
	assert.NotEmpty(t, resp.Items[0].FirstSeen)
	assert.NotEmpty(t, resp.Items[0].LastSeen)
}

func TestFunctionWarningsNilStoreV9(t *testing.T) {
	svcCtx, ctx := setupNoAuthTestContext(t)
	svcCtx.RegistryStore = nil

	resp, err := NewFunctionWarningsLogic(ctx, svcCtx).FunctionWarnings(&FunctionWarningsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}
