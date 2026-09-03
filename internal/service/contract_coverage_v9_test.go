package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDBFileV9 与 setupTestDB 相同迁移集，但使用临时文件数据库：
// 连接池中多个连接共享同一物理库，触发器/表改动对后续查询稳定可见
// （":memory:" 每个连接是独立库，触发器类用例会随机失效）。
func setupTestDBFileV9(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/cov9.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.FunctionContract{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.BlockedProposalIssue{},
		&model.PageSpec{},
		&model.PublishedPageSpec{},
		&model.PageVersion{},
		&model.TermDictionary{},
		&model.Alert{},
	))
	return db
}

// tableNameV9 解析 gorm 模型对应的物理表名。
func tableNameV9(t *testing.T, db *gorm.DB, mdl interface{}) string {
	t.Helper()
	stmt := &gorm.Statement{DB: db}
	require.NoError(t, stmt.Parse(mdl))
	return stmt.Table
}

// stubTableV9 用只含 id 列的空表替换原表：SELECT 具名列会报
// "no such column"，用于触发查询错误分支（非 missing-table）。
func stubTableV9(t *testing.T, db *gorm.DB, mdl interface{}) {
	t.Helper()
	table := tableNameV9(t, db, mdl)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table).Error)
	require.NoError(t, db.Exec("CREATE TABLE "+table+" (id integer primary key)").Error)
}

// abortWritesV9 在表上创建 RAISE(ABORT) 触发器：SELECT 正常、写失败。
func abortWritesV9(t *testing.T, db *gorm.DB, mdl interface{}, events ...string) {
	t.Helper()
	table := tableNameV9(t, db, mdl)
	for _, event := range events {
		sql := "CREATE TRIGGER abort_" + table + "_" + event + " BEFORE " + event +
			" ON " + table + " BEGIN SELECT RAISE(ABORT, '" + event + " blocked'); END"
		require.NoError(t, db.Exec(sql).Error)
	}
}

// abortUpdatesWhenV9 创建带 WHEN 条件的 UPDATE 阻断触发器，
// 用于只让特定行的更新失败（如仅 standalone 提案的软删除）。
func abortUpdatesWhenV9(t *testing.T, db *gorm.DB, mdl interface{}, when string) {
	t.Helper()
	table := tableNameV9(t, db, mdl)
	sql := "CREATE TRIGGER abort_" + table + "_update_when BEFORE UPDATE ON " + table +
		" WHEN " + when + " BEGIN SELECT RAISE(ABORT, 'UPDATE blocked'); END"
	require.NoError(t, db.Exec(sql).Error)
}

func registerPlayerListV9(t *testing.T, svc *ContractService, ctx context.Context, gameID, env, resource, functionID string) {
	t.Helper()
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, gameID, env, "sdk", FunctionMetaInput{
		ID:           functionID,
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     resource,
		Operation:    "list",
		Capability:   "collection_query",
		Execution:    "sync",
		InputSchema:  `{"type":"object","properties":{"page":{"type":"integer"},"page_size":{"type":"integer"}}}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}}}},"total":{"type":"integer"}}}`,
	}))
}

// WithAlertModel（0% → 注入/链式）与 mustParseRisk 兜底分支。
func TestWithAlertModelAndMustParseRiskV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	svc := NewContractService(db)
	alerts := model.NewAlertModel(db)
	returned := svc.WithAlertModel(alerts)
	assert.Same(t, svc, returned)
	svc.WithAlertModel(nil)

	assert.Equal(t, dbenum.RiskSafe, mustParseRisk("totally-unknown"))
	assert.Equal(t, dbenum.RiskHigh, mustParseRisk("high"))
}

// 纯函数分支：requiredSchemaFields 非法 JSON、inferCollectionFields nil 契约、
// convChain 空、pageShapeMatchesType 组合页两分支。
func TestPureHelpersV9(t *testing.T) {
	assert.Nil(t, requiredSchemaFields(map[string]json.RawMessage{"required": json.RawMessage(`[1,2]`)}))
	assert.Nil(t, requiredSchemaFields(nil))

	inferCollectionFields(&model.CapabilitySemantics{}, nil)

	assert.Nil(t, convChain(nil))

	assert.True(t, pageShapeMatchesType(spec.PageSpec{
		Type:      spec.PageTypeComposite,
		Composite: &spec.CompositePageSpec{Sections: []spec.CompositeSection{{}}},
	}))
	assert.False(t, pageShapeMatchesType(spec.PageSpec{
		Type:      spec.PageTypeComposite,
		Composite: &spec.CompositePageSpec{},
	}))
}

// inferIdentityField：候选字段非标量（跳过→无候选）与多候选（歧义）分支。
func TestInferIdentityFieldBranchesV9(t *testing.T) {
	sem := &model.CapabilitySemantics{ResourceKey: "player", CollectionQueryID: 1}
	contract := &model.FunctionContract{
		FunctionID:   "player.list",
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"object"}}}}}}`),
	}
	contract.ID = 1
	inferIdentityField(sem, []*model.FunctionContract{contract})
	assert.Empty(t, sem.IdentityField)
	assert.Contains(t, string(sem.Diagnostics), "resource_identity_not_verifiable")

	semAmbiguous := &model.CapabilitySemantics{ResourceKey: "player", CollectionQueryID: 1}
	ambiguous := &model.FunctionContract{
		FunctionID:   "player.list",
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"player_id":{"type":"string"}}}}}}`),
	}
	ambiguous.ID = 1
	inferIdentityField(semAmbiguous, []*model.FunctionContract{ambiguous})
	assert.Empty(t, semAmbiguous.IdentityField)
	assert.Contains(t, string(semAmbiguous.Diagnostics), "resource_identity_ambiguous")
}

// schema diff：混合 findings（breaking + compatible）→ 非 breaking 条目被 continue 跳过；
// 同一破坏性变更重复出现 → 告警按指纹去重跳过。
func TestSchemaDiffMixedFindingsAndAlertDedupV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)

	base := FunctionMetaInput{
		ID:           "mail.purge",
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     "mail",
		Operation:    "purge",
		Capability:   "action",
		Execution:    "sync",
		InputSchema:  `{"type":"object","properties":{"keep":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"}}}`,
	}
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g9", "e9", "sdk", base))

	// v2：删 b（breaking）+ 加 c（compatible）→ 混合 findings + 告警落库。
	v2 := base
	v2.OutputSchema = `{"type":"object","properties":{"a":{"type":"string"},"c":{"type":"string"}}}`
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g9", "e9", "sdk", v2))

	var alertCount int64
	require.NoError(t, db.Model(&model.Alert{}).Count(&alertCount).Error)
	require.Equal(t, int64(1), alertCount)

	// 回退 v1（删 c → 不同指纹的新告警）。
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g9", "e9", "sdk", base))
	require.NoError(t, db.Model(&model.Alert{}).Count(&alertCount).Error)
	require.Equal(t, int64(2), alertCount)

	// 再次 v2：与第一次完全相同的 breaking findings → 命中已有告警，跳过创建。
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g9", "e9", "sdk", v2))
	require.NoError(t, db.Model(&model.Alert{}).Count(&alertCount).Error)
	assert.Equal(t, int64(2), alertCount)
}

// 告警写入失败只记日志；审计服务写失败同样不影响注册主流程。
func TestWarnAndAuditFailurePathsV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()

	// 审计存储指向已关闭的数据库：Log 必失败 → slog.Warn 分支。
	auditStore, err := audit.NewSQLAuditStore(newClosedServiceDB(t))
	require.NoError(t, err)
	svc := NewContractService(db).WithAuditService(audit.NewAuditService(auditStore, nil))

	base := FunctionMetaInput{
		ID:           "gift.clear",
		Version:      "1.0.0",
		Enabled:      true,
		Resource:     "gift",
		Operation:    "clear",
		Capability:   "action",
		Execution:    "sync",
		OutputSchema: `{"type":"object","properties":{"x":{"type":"string"}}}`,
	}
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g9", "e9", "sdk", base))

	// 告警插入被触发器阻断：Create 失败 → warn 分支。
	abortWritesV9(t, db, &model.Alert{}, "INSERT")
	base.OutputSchema = `{"type":"object","properties":{"y":{"type":"string"}}}`
	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g9", "e9", "sdk", base))
}

// RebuildResourceCapability 各写入/查询错误分支。
func TestRebuildResourceCapabilityErrorBranchesV9(t *testing.T) {
	ctx := context.Background()

	t.Run("capability find error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		registerPlayerListV9(t, svc, ctx, "g9", "e9", "player", "player.list")
		require.NoError(t, svc.RebuildResourceCapability(ctx, "g9", "e9", "player"))
		stubTableV9(t, db, &model.ResourceCapability{})
		err := svc.RebuildResourceCapability(ctx, "g9", "e9", "player")
		assert.ErrorContains(t, err, "find existing resource capability")
	})

	t.Run("capability upsert error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		registerPlayerListV9(t, svc, ctx, "g9", "e9", "player", "player.list")
		abortWritesV9(t, db, &model.ResourceCapability{}, "INSERT")
		err := svc.RebuildResourceCapability(ctx, "g9", "e9", "player")
		assert.ErrorContains(t, err, "upsert resource capability")
	})

	t.Run("semantics find error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		registerPlayerListV9(t, svc, ctx, "g9", "e9", "player", "player.list")
		stubTableV9(t, db, &model.CapabilitySemantics{})
		err := svc.RebuildResourceCapability(ctx, "g9", "e9", "player")
		assert.ErrorContains(t, err, "find existing capability semantics")
	})

	t.Run("semantics upsert error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		registerPlayerListV9(t, svc, ctx, "g9", "e9", "player", "player.list")
		abortWritesV9(t, db, &model.CapabilitySemantics{}, "INSERT")
		err := svc.RebuildResourceCapability(ctx, "g9", "e9", "player")
		assert.ErrorContains(t, err, "upsert capability semantics")
	})

	t.Run("semantic version create error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		registerPlayerListV9(t, svc, ctx, "g9", "e9", "player", "player.list")
		abortWritesV9(t, db, &model.CapabilitySemanticVersion{}, "INSERT")
		err := svc.RebuildResourceCapability(ctx, "g9", "e9", "player")
		assert.ErrorContains(t, err, "create semantic version")
	})

	t.Run("semantics delete error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		registerPlayerListV9(t, svc, ctx, "g9", "e9", "player", "player.list")
		require.NoError(t, svc.RebuildResourceCapability(ctx, "g9", "e9", "player"))
		require.NoError(t, svc.contractModel.DeleteByScopeAndFunctionID(ctx, "g9", "e9", "player.list"))
		// 软删除走 UPDATE：阻断 UPDATE 即可让删除失败。
		abortWritesV9(t, db, &model.CapabilitySemantics{}, "UPDATE")
		err := svc.RebuildResourceCapability(ctx, "g9", "e9", "player")
		assert.ErrorContains(t, err, "delete capability semantics")
	})
}

// RebuildProposalsForResource：digest 漂移回写、列表错误、提案写入错误、
// standalone 提案写入错误、resolveBlockedIssue 错误、consumed 回收与回收失败。
func TestRebuildProposalsErrorBranchesV9(t *testing.T) {
	ctx := context.Background()
	gameID, env := "g9", "e9"

	buildResource := func(t *testing.T) (*gorm.DB, *ContractService) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		registerPlayerListV9(t, svc, ctx, gameID, env, "player", "player.list")
		require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, gameID, env, "sdk", FunctionMetaInput{
			ID:           "player.create",
			Version:      "1.0.0",
			Enabled:      true,
			Resource:     "player",
			Operation:    "create",
			Capability:   "create",
			Execution:    "sync",
			InputSchema:  `{"type":"object","properties":{"name":{"type":"string"}}}`,
			OutputSchema: `{"type":"object","properties":{"id":{"type":"string"}}}`,
		}))
		require.NoError(t, svc.RebuildResourceCapability(ctx, gameID, env, "player"))
		require.NoError(t, svc.RebuildProposalsForResource(ctx, gameID, env, "player"))
		return db, svc
	}

	t.Run("digest drift triggers semantics update", func(t *testing.T) {
		db, svc := buildResource(t)
		require.NoError(t, db.Model(&model.CapabilitySemantics{}).
			Where("game_id = ? AND env = ? AND resource_key = ?", gameID, env, "player").
			Update("source_digest", "stale-digest").Error)
		require.NoError(t, svc.RebuildProposalsForResource(ctx, gameID, env, "player"))
		semantics, err := model.NewCapabilitySemanticsModel(db).FindByScopeAndResourceKey(ctx, gameID, env, "player")
		require.NoError(t, err)
		assert.NotEqual(t, "stale-digest", semantics.SourceDigest)
	})

	t.Run("list contracts error", func(t *testing.T) {
		db, svc := buildResource(t)
		stubTableV9(t, db, &model.FunctionContract{})
		err := svc.RebuildProposalsForResource(ctx, gameID, env, "player")
		assert.ErrorContains(t, err, "list contracts")
	})

	t.Run("rebuild all continues on per-resource error", func(t *testing.T) {
		db, svc := buildResource(t)
		stubTableV9(t, db, &model.FunctionContract{})
		assert.NoError(t, svc.RebuildAllProposals(ctx, gameID, env))
	})

	t.Run("resource proposal upsert error", func(t *testing.T) {
		db, svc := buildResource(t)
		// 资源页提案已存在：upsert 走 UPDATE；新建场景走 INSERT，两者都阻断。
		abortWritesV9(t, db, &model.PageProposal{}, "INSERT", "UPDATE")
		err := svc.RebuildProposalsForResource(ctx, gameID, env, "player")
		assert.Error(t, err)
	})

	t.Run("standalone proposal upsert error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, gameID, env, "sdk", FunctionMetaInput{
			ID:           "player.get",
			Version:      "1.0.0",
			Enabled:      true,
			Resource:     "player",
			Operation:    "get",
			Capability:   "item_query",
			Execution:    "sync",
			InputSchema:  `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
			OutputSchema: `{"type":"object","properties":{"player":{"type":"object"}}}`,
		}))
		require.NoError(t, svc.RebuildResourceCapability(ctx, gameID, env, "player"))
		abortWritesV9(t, db, &model.PageProposal{}, "INSERT")
		err := svc.RebuildProposalsForResource(ctx, gameID, env, "player")
		assert.Error(t, err)
	})

	t.Run("resolve blocked issue error", func(t *testing.T) {
		db, svc := buildResource(t)
		// 预置一条 open 状态的 blocked issue，让 resolve 的 UPDATE 能命中行。
		require.NoError(t, db.Create(&model.BlockedProposalIssue{
			GameID: gameID, Env: env, ResourceKey: "player", Status: "open", UpdatedBy: "system",
		}).Error)
		abortWritesV9(t, db, &model.BlockedProposalIssue{}, "UPDATE")
		err := svc.RebuildProposalsForResource(ctx, gameID, env, "player")
		assert.ErrorContains(t, err, "resolve blocked proposal issue")
	})

	t.Run("consumed standalone proposal reclaimed", func(t *testing.T) {
		_, svc := buildResource(t)
		// 预置一条遗留 standalone 提案：资源页吞并 player.list 后应被回收。
		require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
			GameID: gameID, Env: env, ProposalKey: "operation:player.list",
			PageKey: "operation--player.list", PageType: "operation", ResourceKey: "player",
			Quality: "basic", Status: dbenum.ProposalStatusPending,
			PageSpec: model.JSON(`{"pageKey":"operation--player.list","type":"operation"}`),
		}))
		require.NoError(t, svc.RebuildProposalsForResource(ctx, gameID, env, "player"))
		_, err := svc.proposalModel.FindByScopeAndKey(ctx, gameID, env, "operation:player.list")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("consumed standalone proposal delete error", func(t *testing.T) {
		db, svc := buildResource(t)
		require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
			GameID: gameID, Env: env, ProposalKey: "operation:player.list",
			PageKey: "operation--player.list", PageType: "operation", ResourceKey: "player",
			Quality: "basic", Status: dbenum.ProposalStatusPending,
			PageSpec: model.JSON(`{"pageKey":"operation--player.list","type":"operation"}`),
		}))
		// 仅阻断 standalone 提案行的软删除，资源页提案的 upsert 不受影响。
		abortUpdatesWhenV9(t, db, &model.PageProposal{}, "NEW.proposal_key = 'operation:player.list'")
		err := svc.RebuildProposalsForResource(ctx, gameID, env, "player")
		assert.ErrorContains(t, err, "delete standalone proposal")
	})
}

// RebuildProposalForFunction：语义存在（task/report 语义装配）与语义查询错误分支。
func TestRebuildProposalForFunctionSemanticsBranchesV9(t *testing.T) {
	ctx := context.Background()
	gameID, env := "g9", "e9"

	register := func(t *testing.T, svc *ContractService) {
		registerPlayerListV9(t, svc, ctx, gameID, env, "player", "player.list")
		require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, gameID, env, "sdk", FunctionMetaInput{
			ID:           "player.kick",
			Version:      "1.0.0",
			Enabled:      true,
			Resource:     "player",
			Operation:    "kick",
			Capability:   "action",
			Execution:    "sync",
			InputSchema:  `{"type":"object","properties":{"uid":{"type":"string"},"reason":{"type":"string"}},"required":["uid","reason"]}`,
			OutputSchema: `{"type":"object","properties":{"ok":{"type":"boolean"}}}`,
		}))
	}

	t.Run("semantics found", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		register(t, svc)
		require.NoError(t, svc.RebuildResourceCapability(ctx, gameID, env, "player"))
		require.NoError(t, svc.RebuildProposalForFunction(ctx, gameID, env, "player.kick"))
		_, err := svc.proposalModel.FindByScopeAndKey(ctx, gameID, env, "operation:player.kick")
		assert.NoError(t, err)
	})

	t.Run("semantics find error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		register(t, svc)
		require.NoError(t, svc.RebuildResourceCapability(ctx, gameID, env, "player"))
		stubTableV9(t, db, &model.CapabilitySemantics{})
		err := svc.RebuildProposalForFunction(ctx, gameID, env, "player.kick")
		assert.ErrorContains(t, err, "find capability semantics")
	})
}

// RemoveFunctionContract：契约删除、standalone 回收、blocked issue resolve 的失败分支。
func TestRemoveFunctionContractErrorBranchesV9(t *testing.T) {
	ctx := context.Background()
	gameID, env := "g9", "e9"

	seed := func(t *testing.T) (*gorm.DB, *ContractService) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		registerPlayerListV9(t, svc, ctx, gameID, env, "player", "player.list")
		require.NoError(t, svc.RebuildResourceCapability(ctx, gameID, env, "player"))
		return db, svc
	}

	t.Run("contract delete error", func(t *testing.T) {
		db, svc := seed(t)
		abortWritesV9(t, db, &model.FunctionContract{}, "UPDATE")
		_, err := svc.RemoveFunctionContract(ctx, gameID, env, "player.list")
		assert.ErrorContains(t, err, "delete function contract")
	})

	t.Run("standalone delete error", func(t *testing.T) {
		db, svc := seed(t)
		// 预置 standalone 提案：软删除 UPDATE 需命中行才触发 RAISE。
		require.NoError(t, svc.proposalModel.UpsertProposal(ctx, &model.PageProposal{
			GameID: gameID, Env: env, ProposalKey: "operation:player.list",
			PageKey: "operation--player.list", PageType: "operation", ResourceKey: "player",
			Quality: "basic", Status: dbenum.ProposalStatusPending,
			PageSpec: model.JSON(`{"pageKey":"operation--player.list","type":"operation"}`),
		}))
		abortWritesV9(t, db, &model.PageProposal{}, "UPDATE")
		_, err := svc.RemoveFunctionContract(ctx, gameID, env, "player.list")
		assert.ErrorContains(t, err, "delete standalone proposal")
	})

	t.Run("blocked issue resolve error", func(t *testing.T) {
		db, svc := seed(t)
		require.NoError(t, db.Create(&model.BlockedProposalIssue{
			GameID: gameID, Env: env, ResourceKey: "player", FunctionID: "player.list", Status: "open", UpdatedBy: "system",
		}).Error)
		abortWritesV9(t, db, &model.BlockedProposalIssue{}, "UPDATE")
		_, err := svc.RemoveFunctionContract(ctx, gameID, env, "player.list")
		assert.ErrorContains(t, err, "resolve blocked proposal issue")
	})
}

// upsertGeneratedProposal：空 key 短路、查询已有提案失败、写提案失败、快照失败。
func TestUpsertGeneratedProposalDirectPathsV9(t *testing.T) {
	ctx := context.Background()

	t.Run("empty keys are no-ops", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		require.NoError(t, svc.upsertGeneratedProposal(ctx, "g9", "e9", "", nil, nil, spec.GeneratedPageSpec{PageSpec: spec.PageSpec{PageKey: "k"}}))
		require.NoError(t, svc.upsertGeneratedProposal(ctx, "g9", "e9", "operation:k", nil, nil, spec.GeneratedPageSpec{}))
	})

	t.Run("find existing error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		stubTableV9(t, db, &model.PageProposal{})
		err := svc.upsertGeneratedProposal(ctx, "g9", "e9", "operation:direct", nil, nil,
			spec.GeneratedPageSpec{PageSpec: spec.PageSpec{PageKey: "operation--direct", Type: spec.PageTypeOperation}})
		assert.ErrorContains(t, err, "find existing page proposal")
	})

	t.Run("upsert error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		abortWritesV9(t, db, &model.PageProposal{}, "INSERT")
		err := svc.upsertGeneratedProposal(ctx, "g9", "e9", "operation:direct", nil, nil,
			spec.GeneratedPageSpec{PageSpec: spec.PageSpec{PageKey: "operation--direct", Type: spec.PageTypeOperation}})
		assert.ErrorContains(t, err, "upsert page proposal")
	})

	t.Run("snapshot error", func(t *testing.T) {
		db := setupTestDBFileV9(t)
		svc := NewContractService(db)
		require.NoError(t, db.Migrator().DropTable(&model.PageProposalVersion{}))
		err := svc.upsertGeneratedProposal(ctx, "g9", "e9", "operation:direct", nil, nil,
			spec.GeneratedPageSpec{PageSpec: spec.PageSpec{PageKey: "operation--direct", Type: spec.PageTypeOperation}})
		assert.ErrorContains(t, err, "snapshot page proposal")
	})
}

// createProposalVersionSnapshot：GetNextVersion 成功但 CreateVersion 失败。
func TestCreateProposalVersionSnapshotCreateErrorV9(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	proposal := &model.PageProposal{
		GameID: "g9", Env: "e9", ProposalKey: "operation:x",
		PageKey: "operation--x", PageType: "operation", Quality: "basic",
		Status: dbenum.ProposalStatusPending,
	}
	require.NoError(t, db.Create(proposal).Error)
	abortWritesV9(t, db, &model.PageProposalVersion{}, "INSERT")
	_, err := createProposalVersionSnapshot(ctx, model.NewPageProposalVersionModel(db), proposal, "reason", "actor")
	assert.Error(t, err)
}
