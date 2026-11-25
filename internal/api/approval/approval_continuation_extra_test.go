// Coverage targets: the continueApprovedFunction /
// ensurePageApprovalStillFresh branches in service.go (snapshot missing,
// inactive, binding/contract missing, fresh passes),
// findApprovalBinding / findApprovalContract / loadApprovalFunctionSpecs,
// the error and uninstalled branches of findActiveApprovalInstallation,
// the null/type errors of loadApprovalsFromExtensionInstallation,
// and the scope overreach and store error branches of Get/Approve/Reject.
package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/cuihairu/croupier/internal/security/otp"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ---- Page snapshot continuation environment ----

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

// newApprovalPageEnv builds a complete environment with a published page
// snapshot; mutate may alter page/contracts/published or append contract
// records to the db.
func newApprovalPageEnv(t *testing.T, mutate func(page *spec.PageSpec, contracts *[]spec.BindingContractSnapshot, published *model.PublishedPageSpec, db *gorm.DB)) *approvalPageEnv {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:approval_page_%d?mode=memory&cache=shared", covSharedDBSeq.Add(1))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	store := reg.NewStore()
	_ = store.UpsertAgent(&reg.AgentSession{
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
		// OPEN-ISSUES #75: Approve on the high-risk tier goes through
		// step-up TOTP verification, which needs to look up the operator's
		// binding state; without it everything is treated as unbound.
		AdminModel: model.NewAdminModel(db),
	}

	page := spec.PageSpec{
		PageKey: "player.manage", Type: spec.PageTypeResource,
		Title:    spec.LocalizedText{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{Key: "player"},
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
	// gorm skips the column write for a bool zero value with a default:true
	// tag (and RETURNING would backfill the default into the struct), so
	// Active=false needs an explicit SQL override.
	if wantInactive {
		require.NoError(t, db.Exec("UPDATE published_page_specs SET active = false WHERE id = ?", published.ID).Error)
	}
	return &approvalPageEnv{svcCtx: svcCtx, db: db, inputJSON: pageEnvInputJSON}
}

// seedPageFunctionContract writes a function contract whose digests match the
// snapshot (the prerequisite for freshness).
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
	// metadata lacks pageSnapshotGovernance -> snapshot validation is
	// skipped and it goes straight to FunctionInvoke; the ctx has no scope,
	// so FunctionInvoke reports a missing game/env scope.
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
		// Snapshot validation passes -> reaches FunctionInvoke -> errors out
		// because the ctx has no scope.
		_, err := s.continueApprovedFunction(ctx, pageGovernedRecord())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scope")
	})
}

func TestContinueApprovedFunction_FreshButContractDBUnavailable(t *testing.T) {
	env := newApprovalPageEnv(t, func(_ *spec.PageSpec, _ *[]spec.BindingContractSnapshot, _ *model.PublishedPageSpec, db *gorm.DB) {
		seedPageFunctionContract(t, db)
	})
	env.svcCtx.DB = nil // PublishedPageSpecModel stays usable; the contract DB is unavailable.
	s := NewService(env.svcCtx)
	_, err := s.continueApprovedFunction(context.Background(), pageGovernedRecord())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function contract database unavailable")
}

// ---- find/load helper functions ----

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

	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:approval_specs_%d?mode=memory&cache=shared", covSharedDBSeq.Add(1))), &gorm.Config{})
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

// ---- findActiveApprovalInstallation / extension loading ----

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

// ---- Scope overreach and store errors of Get/Approve/Reject ----

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

// stubStore: Get succeeds and returns a fixed record; every other operation
// keeps errorApprovalsStore's blanket failures.
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

// ---- Approve main flow with a fresh snapshot (covers the continuation error
// write-back path) ----

func TestApprove_FreshPageSnapshotContinuationFailureRecordsReason(t *testing.T) {
	env := newApprovalPageEnv(t, func(_ *spec.PageSpec, _ *[]spec.BindingContractSnapshot, _ *model.PublishedPageSpec, db *gorm.DB) {
		seedPageFunctionContract(t, db)
	})
	s := NewService(env.svcCtx)
	// OPEN-ISSUES #75: the page-governed record is on the danger tier, and
	// Approve now demands step-up TOTP -- the operator must be bound and the
	// request must carry a valid code, otherwise it is refused at the
	// step-up gate.
	secret, err := otp.GenerateSecret()
	require.NoError(t, err)
	require.NoError(t, env.db.Create(&model.Admin{
		Username: "approver-1", PasswordHash: "x", OTPSecret: secret, OTPEnabled: true,
	}).Error)
	code, ok := otp.CodeAt(secret, time.Now())
	require.True(t, ok)
	ctx := context.WithValue(context.Background(), "username", "approver-1")
	_, err = env.svcCtx.ApprovalsStore.Create(pageGovernedRecord())
	require.NoError(t, err)

	// Snapshot validation passes, then FunctionInvoke fails for lack of a
	// scope -> the error is returned and the reason is written back.
	_, err = s.Approve(ctx, &ApprovalApproveRequest{ID: "ap-page", OTP: code})
	require.Error(t, err)

	stored, err := env.svcCtx.ApprovalsStore.Get("ap-page")
	require.NoError(t, err)
	assert.Equal(t, "approved", stored.State)
	assert.Contains(t, stored.Reason, "approved but continuation failed")
}

// ---- Misc: odd shapes when loading from the extension ----

func TestLoadApprovalsFromExtension_FieldTypeMismatch(t *testing.T) {
	s, _, _ := newApprovalExtensionEnv(t, map[string]any{
		approvalRecordsKey: []map[string]any{{"id": 123}},
	})
	// The numeric id cannot be decoded into the string field -> an error is
	// returned.
	_, _, err := s.loadApprovalsFromExtensionInstallation(context.Background())
	require.Error(t, err)
}
