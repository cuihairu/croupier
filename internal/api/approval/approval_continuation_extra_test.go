// 覆盖目标：service.go 的 continueApprovedFunction / ensurePageApprovalStillFresh
// 各分支（快照缺失、inactive、binding/contract 缺失、fresh 通过）、
// findApprovalBinding / findApprovalContract / loadApprovalFunctionSpecs、
// findActiveApprovalInstallation 的错误与 uninstalled 分支、
// loadApprovalsFromExtensionInstallation 的 null/类型错误、
// Get/Approve/Reject 的 scope 越权与 store 错误分支。
package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cache"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ---- page 快照 continuation 环境 ----

const (
	pageEnvGame       = "demo-game"
	pageEnvEnv        = "development"
	pageEnvInputJSON  = `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`
	pageEnvOutputJSON = `{"type":"object","properties":{"ok":{"type":"boolean"}}}`
)

type approvalPageEnv struct {
	svcCtx    *svc.ServiceContext
	db        *gorm.DB
	inputJSON string
}

// newApprovalPageEnv 构造带已发布页面快照的完整环境；
// mutate 可改写 page/contracts/published 或向 db 追加契约记录。
func newApprovalPageEnv(t *testing.T, mutate func(page *spec.PageSpec, contracts *[]spec.BindingContractSnapshot, published *model.PublishedPageSpec, db *gorm.DB)) *approvalPageEnv {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-1", GameID: pageEnvGame, Env: pageEnvEnv,
		ExpireAt: time.Now().Add(time.Minute), LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled: true, Version: "1.0.0", Resource: "player", Operation: "ban",
				Risk: "danger", Permission: "player:ban",
				InputSchema: pageEnvInputJSON, OutputSchema: pageEnvOutputJSON,
			},
		},
	})
	svcCtx := &svc.ServiceContext{
		DB:                     db,
		PageSpecModel:          model.NewPageSpecModel(db),
		PublishedPageSpecModel: model.NewPublishedPageSpecModel(db),
		PageVersionModel:       model.NewPageVersionModel(db),
		RegistryStore:          store,
		Dispatcher:             dispatch.NewDispatcher(store),
		ApprovalsStore:         approvals.NewMemStore(),
		Cache:                  cache.NewNullCache(),
		CacheHelper:            cache.NewCacheHelper(cache.NewNullCache()),
	}

	page := spec.PageSpec{
		PageKey: "player.manage", Type: spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				IdentityKey: "playerId",
				Columns: []spec.ColumnSpec{
					{Key: "playerId", Title: spec.LocalizedText{"zh-CN": "玩家ID"}, DataType: "string", Visible: true},
				},
			},
		},
		Bindings: []spec.PageFunctionBinding{{
			ID: "player.ban", FunctionID: "player.ban", Usage: spec.BindingUsageAction,
			Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
			Selectors: &spec.BindingSelectors{
				Input: spec.SelectorAST{Assignments: []spec.InputAssignment{{
					Target: "/playerId", Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/playerId"},
				}}},
			},
		}},
	}
	contracts := []spec.BindingContractSnapshot{{
		BindingID: "player.ban", FunctionID: "player.ban", FunctionVersion: "1.0.0",
		InputSchemaDigest:  freshness.CanonicalDigest([]byte(pageEnvInputJSON)),
		OutputSchemaDigest: freshness.CanonicalDigest([]byte(pageEnvOutputJSON)),
		Risk:               spec.RiskDanger, Permission: "player:ban",
		Approval:      spec.ApprovalPolicy{Required: true, PolicyKey: "two_person"},
		ExecutionMode: spec.PageExecutionModeSync, RendererSchemaVersion: "page-spec:1",
	}}
	published := &model.PublishedPageSpec{
		GameID: pageEnvGame, Env: pageEnvEnv, PageKey: "player.manage", Version: 1,
		RendererSchemaVersion: "page-spec:1", Active: true,
		PublishedAt: time.Now(), PublishedBy: "tester",
	}
	if mutate != nil {
		mutate(&page, &contracts, published, db)
	}
	wantInactive := !published.Active
	specJSON, err := json.Marshal(page)
	require.NoError(t, err)
	contractsJSON, err := json.Marshal(contracts)
	require.NoError(t, err)
	published.SpecJSON = string(specJSON)
	published.BindingContractsJSON = string(contractsJSON)
	require.NoError(t, svcCtx.PublishedPageSpecModel.Create(context.Background(), published))
	// gorm 对 bool 零值 + default:true 标签会跳过列写入（且 RETURNING 会把
	// 默认值回填进 struct），Active=false 需要显式 SQL 覆盖。
	if wantInactive {
		require.NoError(t, db.Exec("UPDATE published_page_specs SET active = false WHERE id = ?", published.ID).Error)
	}
	return &approvalPageEnv{svcCtx: svcCtx, db: db, inputJSON: pageEnvInputJSON}
}

// seedPageFunctionContract 写入与快照 digest 一致的函数契约（fresh 前提）。
func seedPageFunctionContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: pageEnvGame, Env: pageEnvEnv, FunctionID: "player.ban", Version: "1.0.0",
		Enabled: true, Capability: dbenum.CapabilityAction, Execution: "sync",
		Risk: dbenum.RiskDanger, Permission: "player:ban",
		Approval:     datatypes.JSONMap{"required": true, "policyKey": "two_person"},
		InputSchema:  model.JSON(pageEnvInputJSON),
		OutputSchema: model.JSON(pageEnvOutputJSON),
	}).Error)
}

func pageGovernedRecord() *approvals.Approval {
	return &approvals.Approval{
		ID: "ap-page", State: "pending", FunctionID: "player.ban",
		GameID: pageEnvGame, Env: pageEnvEnv, Actor: "tester",
		Payload: []byte(`{"playerId":"p1"}`),
		Metadata: map[string]string{
			"pageSnapshotGovernance": "validated",
			"page_key":               "player.manage",
			"publish_version":        "1",
			"binding_id":             "player.ban",
		},
	}
}

func TestContinueApprovedFunction_NilRecordAndBlankFunctionID(t *testing.T) {
	s := NewService(&svc.ServiceContext{ApprovalsStore: approvals.NewMemStore()})
	ctx := context.Background()

	_, err := s.continueApprovedFunction(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approval record is required")

	res, err := s.continueApprovedFunction(ctx, &approvals.Approval{ID: "x"})
	require.NoError(t, err)
	assert.False(t, res.Triggered)
}

func TestContinueApprovedFunction_NonGovernedHitsFunctionInvoke(t *testing.T) {
	// metadata 无 pageSnapshotGovernance → 跳过快照校验直接 FunctionInvoke；
	// ctx 无 scope → FunctionInvoke 报“游戏环境 scope 缺失”。
	s := NewService(&svc.ServiceContext{ApprovalsStore: approvals.NewMemStore()})
	_, err := s.continueApprovedFunction(context.Background(), &approvals.Approval{
		ID: "ap-1", FunctionID: "player.ban", Actor: "tester",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scope")
}

func TestContinueApprovedFunction_PageSnapshotGuards(t *testing.T) {
	ctx := context.Background()

	t.Run("published page model unavailable", func(t *testing.T) {
		s := NewService(&svc.ServiceContext{ApprovalsStore: approvals.NewMemStore()})
		_, err := s.continueApprovedFunction(ctx, pageGovernedRecord())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "published page model unavailable")
	})

	t.Run("incomplete metadata", func(t *testing.T) {
		env := newApprovalPageEnv(t, nil)
		s := NewService(env.svcCtx)
		rec := pageGovernedRecord()
		rec.Metadata["page_key"] = ""
		_, err := s.continueApprovedFunction(ctx, rec)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metadata is incomplete")
	})

	t.Run("snapshot not found", func(t *testing.T) {
		env := newApprovalPageEnv(t, nil)
		s := NewService(env.svcCtx)
		rec := pageGovernedRecord()
		rec.Metadata["publish_version"] = "99"
		_, err := s.continueApprovedFunction(ctx, rec)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "published page snapshot not found")
	})

	t.Run("snapshot no longer active", func(t *testing.T) {
		env := newApprovalPageEnv(t, func(_ *spec.PageSpec, _ *[]spec.BindingContractSnapshot, published *model.PublishedPageSpec, _ *gorm.DB) {
			published.Active = false
		})
		s := NewService(env.svcCtx)
		_, err := s.continueApprovedFunction(ctx, pageGovernedRecord())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no longer active")
	})

	t.Run("binding missing from snapshot", func(t *testing.T) {
		env := newApprovalPageEnv(t, func(page *spec.PageSpec, _ *[]spec.BindingContractSnapshot, _ *model.PublishedPageSpec, _ *gorm.DB) {
			page.Bindings = nil
		})
		s := NewService(env.svcCtx)
		_, err := s.continueApprovedFunction(ctx, pageGovernedRecord())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "binding not found")
	})

	t.Run("contract snapshot missing", func(t *testing.T) {
		env := newApprovalPageEnv(t, func(_ *spec.PageSpec, contracts *[]spec.BindingContractSnapshot, _ *model.PublishedPageSpec, _ *gorm.DB) {
			*contracts = nil
		})
		s := NewService(env.svcCtx)
		_, err := s.continueApprovedFunction(ctx, pageGovernedRecord())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "contract snapshot missing")
	})

	t.Run("fresh snapshot passes then invoke rejected without scope", func(t *testing.T) {
		env := newApprovalPageEnv(t, func(_ *spec.PageSpec, _ *[]spec.BindingContractSnapshot, _ *model.PublishedPageSpec, db *gorm.DB) {
			seedPageFunctionContract(t, db)
		})
		s := NewService(env.svcCtx)
		// 快照校验通过 → 走到 FunctionInvoke → ctx 无 scope 报错。
		_, err := s.continueApprovedFunction(ctx, pageGovernedRecord())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scope")
	})
}

func TestContinueApprovedFunction_FreshButContractDBUnavailable(t *testing.T) {
	env := newApprovalPageEnv(t, func(_ *spec.PageSpec, _ *[]spec.BindingContractSnapshot, _ *model.PublishedPageSpec, db *gorm.DB) {
		seedPageFunctionContract(t, db)
	})
	env.svcCtx.DB = nil // PublishedPageSpecModel 仍可用，契约库不可用。
	s := NewService(env.svcCtx)
	_, err := s.continueApprovedFunction(context.Background(), pageGovernedRecord())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function contract database unavailable")
}

// ---- find/load 辅助函数 ----

func TestFindApprovalBindingAndContract(t *testing.T) {
	binding := spec.PageFunctionBinding{ID: " b1 "}
	found, ok := findApprovalBinding([]spec.PageFunctionBinding{binding}, "b1")
	require.True(t, ok)
	assert.Equal(t, " b1 ", found.ID, "命中项按原样返回（查找侧做 trim 比较）")
	_, ok = findApprovalBinding(nil, "b1")
	assert.False(t, ok)

	contract := spec.BindingContractSnapshot{BindingID: "c1"}
	got, ok := findApprovalContract([]spec.BindingContractSnapshot{contract}, "c1")
	require.True(t, ok)
	assert.Equal(t, "c1", got.BindingID)
	_, ok = findApprovalContract(nil, "c1")
	assert.False(t, ok)
}

func TestLoadApprovalFunctionSpecs(t *testing.T) {
	ctx := context.Background()
	record := &approvals.Approval{GameID: pageEnvGame, Env: pageEnvEnv}

	_, err := loadApprovalFunctionSpecs(ctx, nil, record)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database unavailable")

	_, err = loadApprovalFunctionSpecs(ctx, &svc.ServiceContext{}, record)
	require.Error(t, err)

	_, err = loadApprovalFunctionSpecs(ctx, &svc.ServiceContext{DB: nil}, record)
	require.Error(t, err)

	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	_, err = loadApprovalFunctionSpecs(ctx, &svc.ServiceContext{DB: db}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approval record unavailable")

	_, err = loadApprovalFunctionSpecs(ctx, &svc.ServiceContext{DB: db}, &approvals.Approval{GameID: " ", Env: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approval scope is incomplete")

	seedPageFunctionContract(t, db)
	specs, err := loadApprovalFunctionSpecs(ctx, &svc.ServiceContext{DB: db}, record)
	require.NoError(t, err)
	require.Contains(t, specs, "player.ban")
	assert.Equal(t, "1.0.0", specs["player.ban"].Version)
}

// ---- findActiveApprovalInstallation / extension 装载 ----

func newApprovalExtensionEnv(t *testing.T, config map[string]any) (*Service, *extensioninstallation.Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}))
	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	if config != nil {
		_, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
			ExtensionID:    officialApprovalID,
			ReleaseVersion: "1.0.0",
			ScopeType:      "system", ScopeID: "global",
			TargetType: "agent_group", TargetID: "default",
			Config:   config,
			Operator: "tester",
		})
		require.NoError(t, err)
	}
	return NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{Installation: installationSvc},
	}), installationSvc, db
}

func TestFindActiveApprovalInstallation_ListError(t *testing.T) {
	s, _, db := newApprovalExtensionEnv(t, map[string]any{})
	require.NoError(t, db.Migrator().DropTable("extension_installations"))

	_, _, err := s.findActiveApprovalInstallation(context.Background())
	require.Error(t, err)
}

func TestFindActiveApprovalInstallation_AllUninstalledFallsThrough(t *testing.T) {
	s, installationSvc, _ := newApprovalExtensionEnv(t, map[string]any{})
	items, _, err := installationSvc.List(context.Background(), extensioninstallation.ListQuery{ExtensionID: officialApprovalID})
	require.NoError(t, err)
	require.NotEmpty(t, items)
	require.NoError(t, installationSvc.Uninstall(context.Background(), items[0].ID, "tester"))

	item, ok, err := s.findActiveApprovalInstallation(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, item)
}

func TestLoadApprovalsFromExtension_NullValueAndTypeMismatch(t *testing.T) {
	s, _, _ := newApprovalExtensionEnv(t, map[string]any{approvalRecordsKey: nil})
	items, ok, err := s.loadApprovalsFromExtensionInstallation(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, items)

	s2, _, _ := newApprovalExtensionEnv(t, map[string]any{approvalRecordsKey: "not-an-array"})
	_, _, err = s2.loadApprovalsFromExtensionInstallation(context.Background())
	require.Error(t, err)
}

func TestUpsertApprovalToExtension_LoadError(t *testing.T) {
	s, _, db := newApprovalExtensionEnv(t, map[string]any{})
	require.NoError(t, db.Migrator().DropTable("extension_installations"))

	err := s.upsertApprovalToExtension(context.Background(), Approval{ID: "ap-1"})
	require.Error(t, err)
}

// ---- Get/Approve/Reject 的 scope 越权与 store 错误 ----

func TestService_ScopeMismatchOnStorePath(t *testing.T) {
	store := approvals.NewMemStore()
	require.NoError(t, func() error {
		_, err := store.Create(&approvals.Approval{
			ID: "ap-scope", State: "pending", GameID: "game-a", Env: "prod", Actor: "tester",
		})
		return err
	}())
	s := NewService(&svc.ServiceContext{ApprovalsStore: store})
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-b", Env: "prod"})

	_, err := s.Get(ctx, &ApprovalGetRequest{ID: "ap-scope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该审批")

	_, err = s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-scope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该审批")

	_, err = s.Reject(ctx, &ApprovalRejectRequest{ID: "ap-scope", Reason: "no"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该审批")
}

func TestService_GetExtensionScopeMismatchAndLoadError(t *testing.T) {
	s, _, _ := newApprovalExtensionEnv(t, map[string]any{
		approvalRecordsKey: []map[string]any{
			{"id": "ext-1", "state": "pending", "gameId": "game-a", "env": "prod"},
		},
	})
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-b", Env: "prod"})

	_, err := s.Get(ctx, &ApprovalGetRequest{ID: "ext-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该审批")

	s2, _, db := newApprovalExtensionEnv(t, nil)
	require.NoError(t, db.Migrator().DropTable("extension_installations"))
	_, err = s2.Get(context.Background(), &ApprovalGetRequest{ID: "any"})
	require.Error(t, err)
	_, err = s2.List(context.Background(), &ApprovalsListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
}

// stubStore：Get 成功返回固定记录，其余操作沿用 errorApprovalsStore 全报错。
type stubStore struct {
	errorApprovalsStore
	record *approvals.Approval
}

func (s *stubStore) Get(id string) (*approvals.Approval, error) {
	if s.record != nil && s.record.ID == id {
		return s.record, nil
	}
	return nil, errors.New("not found")
}

func TestService_ApproveRejectStoreOperationError(t *testing.T) {
	record := &approvals.Approval{ID: "ap-flaky", State: "pending", Actor: "tester"}
	s := NewService(&svc.ServiceContext{ApprovalsStore: &stubStore{record: record}})

	_, err := s.Approve(context.WithValue(context.Background(), "username", "admin"), &ApprovalApproveRequest{ID: "ap-flaky"})
	require.Error(t, err)

	_, err = s.Reject(context.Background(), &ApprovalRejectRequest{ID: "ap-flaky", Reason: "no"})
	require.Error(t, err)
}

// ---- Approve 主流程带 fresh 快照（覆盖 continuation 错误回写路径） ----

func TestApprove_FreshPageSnapshotContinuationFailureRecordsReason(t *testing.T) {
	env := newApprovalPageEnv(t, func(_ *spec.PageSpec, _ *[]spec.BindingContractSnapshot, _ *model.PublishedPageSpec, db *gorm.DB) {
		seedPageFunctionContract(t, db)
	})
	s := NewService(env.svcCtx)
	ctx := context.WithValue(context.Background(), "username", "approver-1")
	_, err := env.svcCtx.ApprovalsStore.Create(pageGovernedRecord())
	require.NoError(t, err)

	// 快照校验通过，FunctionInvoke 因无 scope 失败 → 返回错误并回写 reason。
	_, err = s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-page"})
	require.Error(t, err)

	stored, err := env.svcCtx.ApprovalsStore.Get("ap-page")
	require.NoError(t, err)
	assert.Equal(t, "approved", stored.State)
	assert.Contains(t, stored.Reason, "approved but continuation failed")
}

// ---- 杂项：extension 装载异常形态 ----

func TestLoadApprovalsFromExtension_FieldTypeMismatch(t *testing.T) {
	s, _, _ := newApprovalExtensionEnv(t, map[string]any{
		approvalRecordsKey: []map[string]any{{"id": 123}},
	})
	// id 数值无法解码进 string 字段 → 返回错误。
	_, _, err := s.loadApprovalsFromExtensionInstallation(context.Background())
	require.Error(t, err)
}
