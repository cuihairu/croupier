package approval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cache"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestApproveRecordsExtensionEvent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	installed, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialApprovalID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         map[string]any{},
		Operator:       "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}
	if installed == nil {
		t.Fatal("expected non-nil installation")
	}

	store := approvals.NewMemStore()
	if _, err := store.Create(&approvals.Approval{ID: "ap-1", State: "pending", Actor: "tester"}); err != nil {
		t.Fatalf("create approval failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	if _, err := s.Approve(context.WithValue(context.Background(), "username", "alice"), &ApprovalApproveRequest{ID: "ap-1"}); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	events, _, err := installationSvc.ListEvents(context.Background(), installed.ID, extensioninstallation.EventListQuery{
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	found := false
	for _, event := range events {
		if event.EventType == "approvals_approve" && event.CreatedBy == "alice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected approvals_approve event created by alice, events=%+v", events)
	}
}

func TestRejectRecordsExtensionEvent(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	installed, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialApprovalID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         map[string]any{},
		Operator:       "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}
	if installed == nil {
		t.Fatal("expected non-nil installation")
	}

	store := approvals.NewMemStore()
	if _, err := store.Create(&approvals.Approval{ID: "ap-2", State: "pending", Actor: "tester"}); err != nil {
		t.Fatalf("create approval failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	if _, err := s.Reject(context.WithValue(context.Background(), "username", "bob"), &ApprovalRejectRequest{ID: "ap-2", Reason: "invalid"}); err != nil {
		t.Fatalf("Reject() error = %v", err)
	}

	events, _, err := installationSvc.ListEvents(context.Background(), installed.ID, extensioninstallation.EventListQuery{
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	found := false
	for _, event := range events {
		if event.EventType == "approvals_reject" && event.CreatedBy == "bob" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected approvals_reject event created by bob, events=%+v", events)
	}
}

func TestListPrefersExtensionInstallationConfig(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	_, err = installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialApprovalID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config: map[string]any{
			"approvals": []map[string]any{
				{
					"id":         "ap-ext-1",
					"actor":      "tester",
					"state":      "pending",
					"created_at": "2026-03-15 00:00:00",
					"updated_at": "2026-03-15 00:00:00",
				},
			},
		},
		Operator: "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	resp, err := s.List(context.Background(), &ApprovalsListRequest{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resp.Approvals) != 1 || resp.Approvals[0].ID != "ap-ext-1" {
		t.Fatalf("expected extension approvals list, got %#v", resp.Approvals)
	}
}

func TestApproveSyncsApprovalToExtensionConfig(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	installed, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialApprovalID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         map[string]any{},
		Operator:       "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}
	if installed == nil {
		t.Fatal("expected non-nil installation")
	}

	store := approvals.NewMemStore()
	if _, err := store.Create(&approvals.Approval{ID: "ap-sync-1", State: "pending", Actor: "tester"}); err != nil {
		t.Fatalf("create approval failed: %v", err)
	}

	s := NewService(&svc.ServiceContext{
		ApprovalsStore: store,
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})
	if _, err := s.Approve(context.Background(), &ApprovalApproveRequest{ID: "ap-sync-1"}); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	current, err := installationSvc.Get(context.Background(), installed.ID)
	if err != nil {
		t.Fatalf("installationSvc.Get() error = %v", err)
	}
	config := map[string]any{}
	if err := json.Unmarshal([]byte(current.ConfigJSON), &config); err != nil {
		t.Fatalf("unmarshal config failed: %v", err)
	}
	raw, ok := config["approvals"]
	if !ok || raw == nil {
		t.Fatalf("expected approvals written into extension config, got %#v", config)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal approvals failed: %v", err)
	}
	items := []Approval{}
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal approvals failed: %v", err)
	}
	if len(items) == 0 || items[0].ID != "ap-sync-1" || items[0].State != "approved" {
		t.Fatalf("unexpected approvals config: %+v", items)
	}
}

func TestApproveRejectsPageContinuationWhenPublishedBindingStale(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	store := reg.NewStore()
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				Resource:     "player",
				Operation:    "ban",
				Risk:         "danger",
				Permission:   "player:ban",
				InputSchema:  `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`,
				OutputSchema: `{"type":"object","properties":{"ok":{"type":"boolean"}}}`,
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
	inputSchema := `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`
	outputSchema := `{"type":"object","properties":{"ok":{"type":"boolean"}}}`
	if err := seedApprovalPublishedPage(svcCtx, context.Background(), inputSchema, outputSchema); err != nil {
		t.Fatalf("seed published page failed: %v", err)
	}
	if _, err := svcCtx.ApprovalsStore.Create(&approvals.Approval{
		ID:         "page-player-ban-1",
		State:      "pending",
		FunctionID: "player.ban",
		GameID:     "demo-game",
		Env:        "development",
		Actor:      "tester",
		Payload:    []byte(`{"playerId":"p1"}`),
		Metadata: map[string]string{
			"page_snapshot_governance": "validated",
			"page_key":                 "player.manage",
			"publish_version":          "1",
			"binding_id":               "player.ban",
		},
	}); err != nil {
		t.Fatalf("create approval failed: %v", err)
	}
	store.UpsertAgent(&reg.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo-game",
		Env:      "development",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]reg.FunctionMeta{
			"player.ban": {
				Enabled:      true,
				Version:      "1.0.0",
				Resource:     "player",
				Operation:    "ban",
				Risk:         "danger",
				Permission:   "player:ban.admin",
				InputSchema:  inputSchema,
				OutputSchema: outputSchema,
			},
		},
	})

	_, err = NewService(svcCtx).Approve(context.Background(), &ApprovalApproveRequest{ID: "page-player-ban-1"})

	if err == nil {
		t.Fatal("expected stale published binding to block approval continuation")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "stale") {
		t.Fatalf("expected stale error, got %q", got)
	}
}

func seedApprovalPublishedPage(svcCtx *svc.ServiceContext, ctx context.Context, inputSchema string, outputSchema string) error {
	page := spec.PageSpec{
		PageKey: "player.manage",
		Type:    spec.PageTypeResource,
		Title:   spec.LocalizedText{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Resource: &spec.ResourcePageSpec{
			ListView: &spec.ListViewSpec{
				IdentityKey: "playerId",
				Columns: []spec.ColumnSpec{
					{Key: "playerId", Title: spec.LocalizedText{"zh-CN": "玩家ID"}, DataType: "string", Visible: true},
				},
			},
		},
		Bindings: []spec.PageFunctionBinding{
			{
				ID:         "player.ban",
				FunctionID: "player.ban",
				Usage:      spec.BindingUsageAction,
				Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
				Selectors: &spec.BindingSelectors{
					Input: spec.SelectorAST{Assignments: []spec.InputAssignment{
						{
							Target: "/playerId",
							Source: spec.ValueSource{Kind: spec.SourceForm, Path: "/playerId"},
						},
					}},
				},
			},
		},
	}
	specJSON, err := json.Marshal(page)
	if err != nil {
		return err
	}
	contractsJSON, err := json.Marshal([]spec.BindingContractSnapshot{
		{
			BindingID:             "player.ban",
			FunctionID:            "player.ban",
			FunctionVersion:       "1.0.0",
			InputSchemaDigest:     digestApprovalRaw([]byte(inputSchema)),
			OutputSchemaDigest:    digestApprovalRaw([]byte(outputSchema)),
			Risk:                  spec.RiskDanger,
			Permission:            "player:ban",
			Approval:              spec.ApprovalPolicy{Required: true, PolicyKey: "two_person"},
			ExecutionMode:         spec.PageExecutionModeSync,
			RendererSchemaVersion: "page-spec:1",
		},
	})
	if err != nil {
		return err
	}
	return svcCtx.PublishedPageSpecModel.Create(ctx, &model.PublishedPageSpec{
		GameID:                "demo-game",
		Env:                   "development",
		PageKey:               "player.manage",
		Version:               1,
		SpecJSON:              string(specJSON),
		BindingContractsJSON:  string(contractsJSON),
		RendererSchemaVersion: "page-spec:1",
		Active:                true,
		PublishedAt:           time.Now(),
		PublishedBy:           "tester",
	})
}

func digestApprovalRaw(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// requireApprovalScope tests
// ---------------------------------------------------------------------------

func TestRequireApprovalScope_EmptyScope(t *testing.T) {
	scope := svc.GameScope{}
	err := requireApprovalScope(scope, "game", "env")
	assert.NoError(t, err)
}

func TestRequireApprovalScope_MatchingScope(t *testing.T) {
	scope := svc.GameScope{GameID: "game", Env: "env"}
	err := requireApprovalScope(scope, "game", "env")
	assert.NoError(t, err)
}

func TestRequireApprovalScope_MatchingCaseInsensitive(t *testing.T) {
	scope := svc.GameScope{GameID: "GAME", Env: "ENV"}
	err := requireApprovalScope(scope, "game", "env")
	assert.NoError(t, err)
}

func TestRequireApprovalScope_MismatchedGameID(t *testing.T) {
	scope := svc.GameScope{GameID: "other", Env: "env"}
	err := requireApprovalScope(scope, "game", "env")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该审批")
}

func TestRequireApprovalScope_MismatchedEnv(t *testing.T) {
	scope := svc.GameScope{GameID: "game", Env: "prod"}
	err := requireApprovalScope(scope, "game", "env")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该审批")
}

func TestRequireApprovalScope_WhitespaceTrimmed(t *testing.T) {
	scope := svc.GameScope{GameID: "game", Env: "env"}
	err := requireApprovalScope(scope, "  game  ", "  env  ")
	assert.NoError(t, err)
}
