// 覆盖目标：注册警告生命周期 API、PreviousFunctionSchema、writeToDB/
// scoped 事务错误路径、marshal 失败分支、metrics 回调与边界、OpenAPI 边缘分支。
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var invalidRawV9 = json.RawMessage(`{"broken":`)

func TestRegistrationWarningLifecycleV9(t *testing.T) {
	ctx := context.Background()
	s := NewStore()

	for _, key := range []string{"k1", "k2", "k3"} {
		require.NoError(t, s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
			Key:     key,
			GameID:  "g",
			Env:     "e",
			AgentID: "a",
			Message: "m-" + key,
		}))
	}

	// Delete single warning.
	assert.False(t, s.DeleteRegistrationWarning("missing"))
	assert.True(t, s.DeleteRegistrationWarning("k1"))
	assert.False(t, s.DeleteRegistrationWarning("k1"))

	// Mark single warning read.
	assert.False(t, s.MarkRegistrationWarningRead("missing"))
	assert.True(t, s.MarkRegistrationWarningRead("k2"))

	// nil 存量条目：读取/标记必须安全跳过。
	s.registrationWarnings["nil-item"] = nil
	assert.False(t, s.MarkRegistrationWarningRead("nil-item"))

	// 只有 k3 未读；nil 条目被跳过。
	assert.Equal(t, 1, s.MarkAllRegistrationWarningsRead())
	assert.Equal(t, 0, s.MarkAllRegistrationWarningsRead())

	warnings := s.ListRegistrationWarnings(RegistrationWarningFilter{})
	require.Len(t, warnings, 2)
	for _, w := range warnings {
		assert.True(t, w.Read)
	}

	// 清空全部（nil 条目计入 map 长度）。
	assert.Equal(t, 3, s.DeleteAllRegistrationWarnings())
	assert.Empty(t, s.ListRegistrationWarnings(RegistrationWarningFilter{}))
}

func TestPreviousFunctionSchemaV9(t *testing.T) {
	s := NewStore()
	require.NoError(t, s.UpsertAgent(&AgentSession{
		AgentID:   "a1",
		GameID:    "g",
		Env:       "e",
		Functions: map[string]FunctionMeta{"fn": {InputSchema: "in-v1", OutputSchema: "out-v1"}},
	}))

	_, _, ok := s.PreviousFunctionSchema("missing", "g", "e", "fn")
	assert.False(t, ok)
	_, _, ok = s.PreviousFunctionSchema("a1", "other-game", "e", "fn")
	assert.False(t, ok)
	_, _, ok = s.PreviousFunctionSchema("a1", "g", "other-env", "fn")
	assert.False(t, ok)
	_, _, ok = s.PreviousFunctionSchema("a1", "g", "e", "missing-fn")
	assert.False(t, ok)

	in, out, ok := s.PreviousFunctionSchema("a1", "g", "e", "fn")
	assert.True(t, ok)
	assert.Equal(t, "in-v1", in)
	assert.Equal(t, "out-v1", out)
}

func TestProviderSessionSnapshotsNilSessionV9(t *testing.T) {
	s := NewStore()
	s.mu.Lock()
	s.agents["ghost"] = nil
	s.agents["live"] = &AgentSession{
		AgentID: "live",
		GameID:  "g",
		Env:     "e",
		Providers: []ProviderSession{{
			ProviderID:   "p1",
			Addr:         "127.0.0.1:19091",
			Version:      "v1",
			SDKLanguage:  "go",
			SDKVersion:   "1.2.3",
			SDKName:      "croupier-go-sdk",
			LastSeenUnix: 42,
			FunctionIDs:  []string{"fn"},
		}},
	}
	s.mu.Unlock()

	snaps := s.ProviderSessionSnapshots()
	require.Len(t, snaps, 1)
	assert.Equal(t, "live", snaps[0].AgentID)
	assert.Equal(t, "p1", snaps[0].ProviderID)
	assert.Equal(t, "go", snaps[0].SDKLanguage)
	assert.Equal(t, int64(42), snaps[0].LastSeenUnix)
	assert.Equal(t, []string{"fn"}, snaps[0].FunctionIDs)
}

func TestUpsertAgentPersistsViaDBV9(t *testing.T) {
	db := setupTestDB(t)
	s := NewStoreWithDB(db)

	session := &AgentSession{
		AgentID:   "a-db",
		GameID:    "g",
		Env:       "e",
		Addr:      "1.2.3.4:5",
		Region:    "r",
		Zone:      "z",
		Labels:    map[string]string{"k": "v"},
		Functions: map[string]FunctionMeta{"fn": {Enabled: true}},
		Providers: []ProviderSession{{ProviderID: "p", FunctionIDs: []string{"fn"}, OpenAPIDoc: json.RawMessage(`{}`)}},
		ExpireAt:  time.Now().Add(time.Hour),
		LastSeen:  time.Now(),
	}
	require.NoError(t, s.UpsertAgent(session))

	var row AgentSessionDB
	require.NoError(t, db.Where("agent_id = ?", "a-db").First(&row).Error)
	assert.Equal(t, `{"k":"v"}`, row.Labels)
	assert.Contains(t, row.Functions, `"fn"`)
	assert.Contains(t, row.Providers, `"p"`)

	// 二次注册：cur 无 labels 时补建 map；Providers 快照整体替换。
	require.NoError(t, s.UpsertAgent(&AgentSession{AgentID: "a-db2", GameID: "g"}))
	require.NoError(t, s.UpsertAgent(&AgentSession{
		AgentID:   "a-db2",
		GameID:    "g",
		Labels:    map[string]string{"a": "1"},
		Providers: []ProviderSession{{ProviderID: "p2"}},
	}))

	s.mu.RLock()
	a2 := s.agents["a-db2"]
	s.mu.RUnlock()
	require.NotNil(t, a2)
	assert.Equal(t, map[string]string{"a": "1"}, a2.Labels)
	require.NotNil(t, a2.Providers)
	assert.Equal(t, "p2", a2.Providers[0].ProviderID)
}

func TestCloneAgentSessionMarshalFailureV9(t *testing.T) {
	assert.Nil(t, cloneAgentSession(nil))
	bad := &AgentSession{Providers: []ProviderSession{{OpenAPIDoc: invalidRawV9}}}
	assert.Nil(t, cloneAgentSession(bad))
}

func TestPrepareRegistrationOperationMarshalFailuresV9(t *testing.T) {
	s := NewStoreWithDB(setupTestDB(t))
	good := &AgentSession{AgentID: "a"}
	bad := &AgentSession{Providers: []ProviderSession{{OpenAPIDoc: invalidRawV9}}}

	_, err := s.prepareRegistrationOperation(bad, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal target registration session")

	_, err = s.prepareRegistrationOperation(good, bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal previous registration session")
}

func TestToDBSessionProvidersMarshalFailureV9(t *testing.T) {
	_, err := toDBSession(&AgentSession{Providers: []ProviderSession{{OpenAPIDoc: invalidRawV9}}})
	require.Error(t, err)
}

func TestAgentSessionModelUpsertInvalidSessionV9(t *testing.T) {
	m := NewAgentSessionModel(setupTestDB(t))
	err := m.Upsert(context.Background(), &AgentSession{
		Providers: []ProviderSession{{OpenAPIDoc: invalidRawV9}},
	})
	require.Error(t, err)
}

func TestAgentSessionModelClosedDBErrorsV9(t *testing.T) {
	db := setupTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	require.Error(t, MigrateAgentSessions(db))

	m := NewAgentSessionModel(db)
	_, err = m.LoadActiveSessions(context.Background())
	require.Error(t, err)
}

func TestMaterializeScopedTransactionWithoutDBV9(t *testing.T) {
	s := NewStore()
	err := s.materializeScopedTransaction(context.Background(), &AgentSession{}, s.materializeAgent, functionSnapshotDiff{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registration projection database is not initialized")
}

var errV9Compensation = errors.New("v9 remove contract failure")

// removeFailingContractServiceV9 只在 RemoveFunctionContract 上失败：
// 正向物化成功、meta 写失败后的逆向补偿失败 → compensation_required。
type removeFailingContractServiceV9 struct{}

func (removeFailingContractServiceV9) RebuildContractFromFunctionMeta(context.Context, string, string, string, spec.FunctionContractInput) error {
	return nil
}

func (removeFailingContractServiceV9) RemoveFunctionContract(context.Context, string, string, string) (string, error) {
	return "", errV9Compensation
}

func (removeFailingContractServiceV9) RebuildResourceCapability(context.Context, string, string, string) error {
	return nil
}

func (removeFailingContractServiceV9) RebuildProposalsForResource(context.Context, string, string, string) error {
	return nil
}

func (removeFailingContractServiceV9) RebuildProposalForFunction(context.Context, string, string, string) error {
	return nil
}

func TestUpsertAgentScopedTransactionFailurePathsV9(t *testing.T) {
	scopedSession := func() *AgentSession {
		return &AgentSession{
			AgentID:   "a-scope",
			GameID:    "g",
			Env:       "e",
			Functions: map[string]FunctionMeta{"fn": {Enabled: true, Resource: "res"}},
		}
	}

	// newScopedStoreV9 返回 store 与 meta 库句柄（操作记录落在 meta 库）。
	newScopedStoreV9 := func(t *testing.T) (*Store, *gorm.DB) {
		t.Helper()
		metaDB := setupTestDB(t)
		gameDB := setupTestDB(t)
		s := NewStoreWithDB(metaDB)
		s.SetScopeContextResolver(func(gameID, env string) context.Context {
			return dbctx.WithDB(context.Background(), gameDB)
		})
		return s, metaDB
	}

	t.Run("materialization failure marks operation aborted", func(t *testing.T) {
		s, metaDB := newScopedStoreV9(t)
		s.SetContractService(failingContractService{err: assert.AnError})

		err := s.UpsertAgent(scopedSession())
		require.Error(t, err)

		var op AgentRegistrationOperationDB
		require.NoError(t, metaDB.Where("agent_id = ?", "a-scope").First(&op).Error)
		assert.Equal(t, "aborted", op.Status)
	})

	t.Run("operation create failure aborts before materialize", func(t *testing.T) {
		s, metaDB := newScopedStoreV9(t)
		require.NoError(t, metaDB.Migrator().DropTable(&AgentRegistrationOperationDB{}))
		s.SetContractService(&recordingContractService{})

		err := s.UpsertAgent(scopedSession())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "create registration recovery operation")
	})

	t.Run("failed compensation marks compensation required", func(t *testing.T) {
		s, metaDB := newScopedStoreV9(t)
		require.NoError(t, metaDB.Migrator().DropTable(&AgentSessionDB{}))
		s.SetContractService(removeFailingContractServiceV9{})

		err := s.UpsertAgent(scopedSession())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registration compensation failed")

		var op AgentRegistrationOperationDB
		require.NoError(t, metaDB.Where("agent_id = ?", "a-scope").First(&op).Error)
		assert.Equal(t, "compensation_required", op.Status)
	})

	t.Run("single database write failure rolls back", func(t *testing.T) {
		db := setupTestDB(t)
		require.NoError(t, db.Migrator().DropTable(&AgentSessionDB{}))
		s := NewStoreWithDB(db)

		err := s.UpsertAgent(scopedSession())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "write agent session to database")
	})
}

func TestRecoverPendingRegistrationOperationsMorePathsV9(t *testing.T) {
	t.Run("nil db no-op", func(t *testing.T) {
		require.NoError(t, NewStore().recoverPendingRegistrationOperations(context.Background()))
	})

	t.Run("list failure surfaces", func(t *testing.T) {
		db := setupTestDB(t)
		s := NewStoreWithDB(db)
		s.SetContractService(&recordingContractService{})
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		err = s.recoverPendingRegistrationOperations(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list pending registration recovery operations")
	})

	t.Run("previous session fields backfilled from target", func(t *testing.T) {
		db := setupTestDB(t)
		s := NewStoreWithDB(db)
		s.SetContractService(&recordingContractService{})
		require.NoError(t, db.Create(&AgentRegistrationOperationDB{
			OperationID:     "op-backfill",
			AgentID:         "a",
			GameID:          "g",
			Env:             "e",
			PreviousSession: `{"Functions":{"old.fn":{"Resource":"res"}}}`,
			TargetSession:   `{"AgentID":"a","GameID":"g","Env":"e","Functions":{"new.fn":{"Resource":"res"}}}`,
			Status:          "pending",
		}).Error)

		require.NoError(t, s.recoverPendingRegistrationOperations(context.Background()))

		var op AgentRegistrationOperationDB
		require.NoError(t, db.Where("operation_id = ?", "op-backfill").First(&op).Error)
		assert.Equal(t, "compensated", op.Status)
	})

	t.Run("LoadFromDB propagates recovery failure", func(t *testing.T) {
		db := setupTestDB(t)
		s := NewStoreWithDB(db)
		s.SetContractService(&recordingContractService{})
		require.NoError(t, db.Create(&AgentRegistrationOperationDB{
			OperationID:     "op-bad",
			AgentID:         "a",
			GameID:          "g",
			Env:             "e",
			PreviousSession: "{bad",
			TargetSession:   `{}`,
			Status:          "pending",
		}).Error)

		err := s.LoadFromDB(context.Background(), &fakeSessionLoader{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode previous registration session")
	})
}

func TestRegistrationWarningMapGuardsV9(t *testing.T) {
	ctx := context.Background()

	t.Run("nil warning map initialized lazily", func(t *testing.T) {
		s := &Store{}
		require.NoError(t, s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{Key: "k", Message: "m"}))
		require.Len(t, s.ListRegistrationWarnings(RegistrationWarningFilter{}), 1)
	})

	s := NewStore()
	require.NoError(t, s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
		Key: "w1", GameID: "g1", Env: "e1", AgentID: "a1", FunctionID: "f1", Code: "c1", Message: "m",
	}))
	require.NoError(t, s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
		Key: "w2", GameID: "g2", Env: "e2", AgentID: "a2", FunctionID: "f2", Code: "c2", Message: "m",
	}))
	s.registrationWarnings["nil-item"] = nil

	// 每个过滤器字段的不匹配分支 + nil 条目跳过。
	for _, filter := range []RegistrationWarningFilter{
		{GameID: "nope"},
		{Env: "nope"},
		{AgentID: "nope"},
		{FunctionID: "nope"},
		{Code: "nope"},
	} {
		assert.Empty(t, s.ListRegistrationWarnings(filter))
	}

	assert.Len(t, s.ListRegistrationWarnings(RegistrationWarningFilter{GameID: "g1"}), 1)
	assert.Len(t, s.ListRegistrationWarnings(RegistrationWarningFilter{Env: "e1"}), 1)
	assert.Len(t, s.ListRegistrationWarnings(RegistrationWarningFilter{AgentID: "a1"}), 1)
	assert.Len(t, s.ListRegistrationWarnings(RegistrationWarningFilter{FunctionID: "f1"}), 1)
	assert.Len(t, s.ListRegistrationWarnings(RegistrationWarningFilter{Code: "c1"}), 1)

	assert.Equal(t, 0, s.RemoveRegistrationWarnings(RegistrationWarningFilter{GameID: "nope"}))
	assert.Equal(t, 0, s.RemoveRegistrationWarnings(RegistrationWarningFilter{AgentID: "nope"}))
	assert.Equal(t, 0, s.RemoveRegistrationWarnings(RegistrationWarningFilter{FunctionID: "nope"}))
	assert.Equal(t, 1, s.RemoveRegistrationWarnings(RegistrationWarningFilter{Code: "c1"}))
	// 只剩 w2（nil 条目不参与删除）。
	assert.Equal(t, 1, s.RemoveRegistrationWarnings(RegistrationWarningFilter{}))
}

func TestMetricsStoreOnReportCallbackV9(t *testing.T) {
	s := NewMetricsStore()
	called := make(chan struct{}, 4)
	s.SetOnReport(func(ctx context.Context, agentID string, report *opsv1.MetricsReport) {
		called <- struct{}{}
	})
	s.Add("a-cb", &opsv1.MetricsReport{})
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("onReport callback not invoked")
	}

	// 回调 panic 被 recover，存储链路不受影响。
	s.SetOnReport(func(context.Context, string, *opsv1.MetricsReport) { panic("boom") })
	s.Add("a-panic", &opsv1.MetricsReport{})
	assert.Len(t, s.ListAgents(), 2)
}

func TestMetricsStoreGetLatestStaleIndexV9(t *testing.T) {
	s := NewMetricsStore()
	s.mu.Lock()
	s.byAgent["ghost"] = []int{7}
	s.mu.Unlock()

	_, ok := s.GetLatest("ghost")
	assert.False(t, ok)
}

func TestMetricsStoreGetFromMemoryCapsAllocationV9(t *testing.T) {
	s := NewMetricsStore()
	s.mu.Lock()
	s.byAgent["big"] = make([]int, 10050)
	s.mu.Unlock()

	assert.Empty(t, s.getFromMemory("big", time.Time{}, 0))
}

func TestMetricsStorePruneDatabaseV9(t *testing.T) {
	db := openMetricsTestDB(t)
	s := NewMetricsStore()
	s.SetDB(db)
	s.Add("a-prune", &opsv1.MetricsReport{})

	require.Eventually(t, func() bool {
		var n int64
		require.NoError(t, db.Model(&AgentMetricsHistory{}).Count(&n).Error)
		return n == 1
	}, 2*time.Second, 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)
	s.Prune(time.Millisecond)

	var n int64
	require.NoError(t, db.Model(&AgentMetricsHistory{}).Count(&n).Error)
	assert.Zero(t, n)
}

func TestInferResourceAndCapabilityNilMetaV9(t *testing.T) {
	assert.NotPanics(t, func() { InferResourceAndCapability("player.list", nil) })
}

func TestStoreOpenAPIEdgeCasesV9(t *testing.T) {
	s := NewStore()

	require.Error(t, s.UpsertOpenAPI("", nil))
	require.Error(t, s.UpsertOpenAPI("fn-nil", nil))
	_, err := s.GetOpenAPI("")
	require.Error(t, err)
	_, err = s.GetOpenAPIProvider("")
	require.Error(t, err)

	_, err = s.GetOpenAPIProvider("ghost")
	require.Error(t, err)

	// map 中的 nil 操作：列表与 spec 构建都必须跳过。
	s.mu.Lock()
	s.openapiOperations["nil-op"] = nil
	s.mu.Unlock()

	assert.Empty(t, s.ListOpenAPIOperations())
	doc, err := s.BuildOpenAPISpec()
	require.NoError(t, err)
	require.NotNil(t, doc)
	assert.Equal(t, 0, doc.Paths.Len())
}

func TestNormalizeOpenAPISchemaUnmarshalErrorV9(t *testing.T) {
	n := NewSchemaNormalizer()
	_, err := n.normalizeOpenAPISchema(map[string]interface{}{"type": 1.5})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal schema")
}

func TestSurvivingFunctionMetaSkipsNilSessionsV9(t *testing.T) {
	s := NewStore()
	s.mu.Lock()
	s.agents["ghost"] = nil
	s.agents["holder"] = &AgentSession{
		AgentID:   "holder",
		GameID:    "g",
		Env:       "e",
		Functions: map[string]FunctionMeta{"fn": {Version: "9"}},
	}
	s.mu.Unlock()

	meta, ok := s.survivingFunctionMeta("someone", "g", "e", "fn")
	assert.True(t, ok)
	assert.Equal(t, "9", meta.Version)

	_, ok = s.survivingFunctionMeta("someone", "other", "e", "fn")
	assert.False(t, ok)
}

func TestCleanupRoutineDefaultIntervalV9(t *testing.T) {
	s := NewStore()
	s.mu.Lock()
	s.agents["ghost"] = nil
	s.mu.Unlock()
	assert.NotPanics(t, s.cleanupExpiredSessions)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.StartCleanupRoutine(ctx, 0)
}

func TestMarkRegistrationOperationUpdateFailureV9(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&AgentRegistrationOperationDB{}))
	s := NewStoreWithDB(db)

	assert.NotPanics(t, func() {
		s.markRegistrationOperation("op-v9", "committed", nil)
	})
}

// proposalsFailingServiceV9 只在 RebuildProposalsForResource 上失败。
type proposalsFailingServiceV9 struct{}

func (proposalsFailingServiceV9) RebuildContractFromFunctionMeta(context.Context, string, string, string, spec.FunctionContractInput) error {
	return nil
}

func (proposalsFailingServiceV9) RemoveFunctionContract(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (proposalsFailingServiceV9) RebuildResourceCapability(context.Context, string, string, string) error {
	return nil
}

func (proposalsFailingServiceV9) RebuildProposalsForResource(context.Context, string, string, string) error {
	return errors.New("v9 proposal rebuild failure")
}

func (proposalsFailingServiceV9) RebuildProposalForFunction(context.Context, string, string, string) error {
	return nil
}

func TestUpsertAgentRemovedSurvivorRebuildFailsV9(t *testing.T) {
	s := NewStore()
	// 两个 agent 同时持有 fn.x；a1 随后下线 fn.x，幸存者重建路径必须触发。
	require.NoError(t, s.UpsertAgent(&AgentSession{
		AgentID:   "a1",
		GameID:    "g",
		Env:       "e",
		Functions: map[string]FunctionMeta{"fn.x": {Enabled: true}},
	}))
	require.NoError(t, s.UpsertAgent(&AgentSession{
		AgentID:   "a2",
		GameID:    "g",
		Env:       "e",
		Functions: map[string]FunctionMeta{"fn.x": {Enabled: true}},
	}))

	s.SetContractService(failingContractService{err: assert.AnError})
	err := s.UpsertAgent(&AgentSession{
		AgentID:   "a1",
		GameID:    "g",
		Env:       "e",
		Functions: map[string]FunctionMeta{"fn.y": {Enabled: true}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent registration contract rebuild failed")
}

func TestUpsertAgentProposalRebuildFailureV9(t *testing.T) {
	s := NewStore()
	s.SetContractService(proposalsFailingServiceV9{})

	err := s.UpsertAgent(&AgentSession{
		AgentID:   "a-p",
		GameID:    "g",
		Env:       "e",
		Functions: map[string]FunctionMeta{"fn.q": {Enabled: true, Resource: "res-q"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rebuild page proposals")
}
