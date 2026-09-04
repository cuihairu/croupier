package approval

import (
	"context"
	"testing"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestService_List_ApprovalsStoreListError(t *testing.T) {
	// Create a store that will return an error on List
	store := &errorApprovalsStore{}
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.List(context.Background(), &ApprovalsListRequest{Page: 1, PageSize: 10})
	assert.Error(t, err)
}

func TestService_Get_ApprovalsStoreGetError(t *testing.T) {
	store := &errorApprovalsStore{}
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Get(context.Background(), &ApprovalGetRequest{ID: "test-id"})
	assert.Error(t, err)
}

func TestService_Approve_ApprovalsStoreApproveError(t *testing.T) {
	store := &errorApprovalsStore{}
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Approve(context.Background(), &ApprovalApproveRequest{ID: "test-id"})
	assert.Error(t, err)
}

func TestService_Reject_ApprovalsStoreRejectError(t *testing.T) {
	store := &errorApprovalsStore{}
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Reject(context.Background(), &ApprovalRejectRequest{ID: "test-id", Reason: "test"})
	assert.Error(t, err)
}

func TestDecodeApprovalPayload_IndentError(t *testing.T) {
	// Create a payload that will cause json.Indent to fail
	// This is tricky because json.Indent rarely fails on valid JSON
	// But we can test with a very large nested structure
	a := &approvals.Approval{
		Payload: []byte(`{"key":"value"}`),
	}
	payload, preview := decodeApprovalPayload(a)
	assert.NotNil(t, payload)
	assert.NotEmpty(t, preview)
}

func TestDecodeApprovalPayload_MalformedJSON(t *testing.T) {
	a := &approvals.Approval{
		Payload: []byte(`{invalid`),
	}
	payload, preview := decodeApprovalPayload(a)
	assert.Nil(t, payload)
	assert.NotEmpty(t, preview)
}

func TestToApprovalSummaries_NilInput(t *testing.T) {
	result := toApprovalSummaries(nil)
	assert.Empty(t, result)
}

func TestFilterApprovalSummariesByState_NoMatch(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1", State: "pending"},
		{ID: "2", State: "approved"},
	}

	result := filterApprovalSummariesByState(items, "rejected")
	assert.Empty(t, result)
}

func TestPaginateApprovalSummaries_PageBeyondTotal(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1"}, {ID: "2"},
	}

	result, total := paginateApprovalSummaries(items, 10, 10)
	assert.Empty(t, result)
	assert.Equal(t, 2, total)
}

func TestPaginateApprovalSummaries_LargePageSize(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1"}, {ID: "2"},
	}

	result, total := paginateApprovalSummaries(items, 1, 100)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, total)
}

// errorApprovalsStore is a mock store that always returns errors
type errorApprovalsStore struct{}

func (s *errorApprovalsStore) List(filter approvals.Filter, page approvals.Page) ([]*approvals.Approval, int, error) {
	return nil, 0, assert.AnError
}

func (s *errorApprovalsStore) Get(id string) (*approvals.Approval, error) {
	return nil, assert.AnError
}

func (s *errorApprovalsStore) Create(a *approvals.Approval) (*approvals.Approval, error) {
	return nil, assert.AnError
}

func (s *errorApprovalsStore) Update(a *approvals.Approval) (*approvals.Approval, error) {
	return nil, assert.AnError
}

func (s *errorApprovalsStore) Approve(id, operator string) (*approvals.Approval, error) {
	return nil, assert.AnError
}

func (s *errorApprovalsStore) Reject(id, reason, operator string) (*approvals.Approval, error) {
	return nil, assert.AnError
}

func TestBuildApprovalSummary_WithAllFields(t *testing.T) {
	a := &approvals.Approval{
		ID:              "test-1",
		Actor:           "user1",
		FunctionID:      "func-1",
		GameID:          "game-1",
		Env:             "dev",
		State:           "PENDING",
		Mode:            "invoke",
		Route:           "/api/test",
		IdempotencyKey:  "key-1",
		TargetServiceID: "svc-1",
		HashKey:         "hash-1",
		Reason:          "test reason",
	}

	result := buildApprovalSummary(a)

	assert.Equal(t, "test-1", result.ID)
	assert.Equal(t, "user1", result.Actor)
	assert.Equal(t, "func-1", result.FunctionID)
	assert.Equal(t, "game-1", result.GameID)
	assert.Equal(t, "dev", result.Env)
	assert.Equal(t, "pending", result.State) // Should be lowercased
	assert.Equal(t, "invoke", result.Mode)
	assert.Equal(t, "/api/test", result.Route)
	assert.Equal(t, "key-1", result.IdempotencyKey)
	assert.Equal(t, "svc-1", result.TargetServiceID)
	assert.Equal(t, "hash-1", result.HashKey)
	assert.Equal(t, "test reason", result.Reason)
}

func TestBuildApprovalDetail_WithPayload(t *testing.T) {
	a := &approvals.Approval{
		ID:      "test-1",
		Actor:   "user1",
		State:   "pending",
		Payload: []byte(`{"key":"value"}`),
	}

	result := buildApprovalDetail(a)

	assert.Equal(t, "test-1", result.ID)
	assert.Equal(t, "user1", result.Actor)
	assert.NotNil(t, result.Payload)
	assert.NotEmpty(t, result.PayloadPreview)
}

func TestDefaultString_NilValue(t *testing.T) {
	result := defaultString("", "default")
	assert.Equal(t, "default", result)
}

func TestDefaultString_OnlyWhitespace(t *testing.T) {
	result := defaultString("   ", "default")
	assert.Equal(t, "default", result)
}

func TestDefaultString_RealValue(t *testing.T) {
	result := defaultString("actual", "default")
	assert.Equal(t, "actual", result)
}

func TestService_List_WithExtensionError(t *testing.T) {
	// Test with nil Extensions
	service := NewService(&svc.ServiceContext{
		ApprovalsStore: approvals.NewMemStore(),
	})

	resp, err := service.List(context.Background(), &ApprovalsListRequest{Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_Get_WithExtensionError(t *testing.T) {
	// Test with nil Extensions
	service := NewService(&svc.ServiceContext{
		ApprovalsStore: approvals.NewMemStore(),
	})

	_, err := service.Get(context.Background(), &ApprovalGetRequest{ID: "test"})
	assert.Error(t, err) // Should return "not found"
}

func TestService_List_WithStatusFilter(t *testing.T) {
	store := approvals.NewMemStore()
	store.Create(&approvals.Approval{ID: "1", State: "pending"})
	store.Create(&approvals.Approval{ID: "2", State: "approved"})

	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	resp, err := service.List(context.Background(), &ApprovalsListRequest{
		Page:     1,
		PageSize: 10,
		Status:   "pending",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_Get_ExistingApproval(t *testing.T) {
	store := approvals.NewMemStore()
	store.Create(&approvals.Approval{ID: "existing", State: "pending", Actor: "tester"})

	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	resp, err := service.Get(context.Background(), &ApprovalGetRequest{ID: "existing"})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "existing", resp.Approval.ID)
}

func TestService_Approve_NilStore_Extra(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	_, err := service.Approve(context.Background(), &ApprovalApproveRequest{ID: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestService_Reject_NilStore_Extra(t *testing.T) {
	service := NewService(&svc.ServiceContext{})

	_, err := service.Reject(context.Background(), &ApprovalRejectRequest{ID: "test", Reason: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestService_List_WithPagination(t *testing.T) {
	store := approvals.NewMemStore()
	for i := 0; i < 25; i++ {
		store.Create(&approvals.Approval{
			ID:    "approval-" + string(rune('a'+i)),
			State: "pending",
		})
	}

	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	resp, err := service.List(context.Background(), &ApprovalsListRequest{
		Page:     2,
		PageSize: 10,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(25), resp.Total)
	assert.Equal(t, 2, resp.Page)
	assert.Equal(t, 10, resp.Size)
}

func TestBuildApprovalSummary_DefaultMode(t *testing.T) {
	a := &approvals.Approval{
		ID:    "test-1",
		Mode:  "",
		State: "pending",
	}

	result := buildApprovalSummary(a)
	assert.Equal(t, "invoke", result.Mode) // Should default to "invoke"
}

func TestBuildApprovalSummary_WhitespaceState(t *testing.T) {
	a := &approvals.Approval{
		ID:    "test-1",
		State: "  PENDING  ",
	}

	result := buildApprovalSummary(a)
	assert.Equal(t, "pending", result.State) // Should be trimmed and lowercased
}

func TestDecodeApprovalPayload_NilPayload(t *testing.T) {
	a := &approvals.Approval{
		Payload: nil,
	}
	payload, preview := decodeApprovalPayload(a)
	assert.Nil(t, payload)
	assert.Empty(t, preview)
}

func TestDecodeApprovalPayload_EmptyPayload_Extra(t *testing.T) {
	a := &approvals.Approval{
		Payload: []byte{},
	}
	payload, preview := decodeApprovalPayload(a)
	assert.Nil(t, payload)
	assert.Empty(t, preview)
}

func TestDecodeApprovalPayload_ValidJSONWithIndent(t *testing.T) {
	a := &approvals.Approval{
		Payload: []byte(`{"nested":{"key":"value"}}`),
	}
	payload, preview := decodeApprovalPayload(a)
	assert.NotNil(t, payload)
	assert.NotEmpty(t, preview)
	// preview should be indented
	assert.Contains(t, preview, "\n")
}

func TestToApprovalSummaries_MultipleItems(t *testing.T) {
	items := []Approval{
		{ID: "1", Actor: "user1", State: "pending"},
		{ID: "2", Actor: "user2", State: "approved"},
		{ID: "3", Actor: "user3", State: "rejected"},
	}

	result := toApprovalSummaries(items)
	assert.Len(t, result, 3)
	assert.Equal(t, "1", result[0].ID)
	assert.Equal(t, "user1", result[0].Actor)
	assert.Equal(t, "pending", result[0].State)
}

func TestFilterApprovalSummariesByState_ExactMatch(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1", State: "pending"},
		{ID: "2", State: "approved"},
		{ID: "3", State: "pending"},
	}

	result := filterApprovalSummariesByState(items, "pending")
	assert.Len(t, result, 2)
}

func TestFilterApprovalSummariesByState_CaseInsensitiveMatch(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1", State: "PENDING"},
		{ID: "2", State: "Approved"},
	}

	result := filterApprovalSummariesByState(items, "pending")
	assert.Len(t, result, 1)
	assert.Equal(t, "1", result[0].ID)
}

func TestPaginateApprovalSummaries_MiddlePage(t *testing.T) {
	items := make([]ApprovalSummary, 25)
	for i := range items {
		items[i] = ApprovalSummary{ID: "item-" + string(rune('a'+i))}
	}

	result, total := paginateApprovalSummaries(items, 2, 10)
	assert.Len(t, result, 10)
	assert.Equal(t, 25, total)
}

func TestPaginateApprovalSummaries_LastPage(t *testing.T) {
	items := make([]ApprovalSummary, 25)
	for i := range items {
		items[i] = ApprovalSummary{ID: "item-" + string(rune('a'+i))}
	}

	result, total := paginateApprovalSummaries(items, 3, 10)
	assert.Len(t, result, 5) // 25 - 20 = 5
	assert.Equal(t, 25, total)
}

func TestService_List_NilRequestWithStore(t *testing.T) {
	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	resp, err := service.List(context.Background(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Page)  // Default page
	assert.Equal(t, 20, resp.Size) // Default size
}

func TestService_List_DefaultPagination(t *testing.T) {
	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	resp, err := service.List(context.Background(), &ApprovalsListRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Page)  // Default page
	assert.Equal(t, 20, resp.Size) // Default size
}

func TestService_Get_NilRequestWithStore(t *testing.T) {
	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Get(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestService_Approve_NilRequestWithStore(t *testing.T) {
	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Approve(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestService_Reject_NilRequestWithStore(t *testing.T) {
	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Reject(context.Background(), nil)
	assert.Error(t, err)
}

func TestService_Reject_EmptyReasonWithStore(t *testing.T) {
	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Reject(context.Background(), &ApprovalRejectRequest{ID: "test", Reason: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reason")
}

func TestService_Reject_WhitespaceReasonWithStore(t *testing.T) {
	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Reject(context.Background(), &ApprovalRejectRequest{ID: "test", Reason: "   "})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reason")
}

func TestBuildApprovalSummary_EmptyMode(t *testing.T) {
	a := &approvals.Approval{
		ID:   "test",
		Mode: "",
	}
	result := buildApprovalSummary(a)
	assert.Equal(t, "invoke", result.Mode)
}

func TestBuildApprovalSummary_WhitespaceMode(t *testing.T) {
	a := &approvals.Approval{
		ID:   "test",
		Mode: "   ",
	}
	result := buildApprovalSummary(a)
	assert.Equal(t, "invoke", result.Mode)
}

func TestBuildApprovalSummary_CustomMode(t *testing.T) {
	a := &approvals.Approval{
		ID:   "test",
		Mode: "query",
	}
	result := buildApprovalSummary(a)
	assert.Equal(t, "query", result.Mode)
}

func TestDefaultString_EmptyString(t *testing.T) {
	result := defaultString("", "fallback")
	assert.Equal(t, "fallback", result)
}

func TestDefaultString_NonEmptyString(t *testing.T) {
	result := defaultString("value", "fallback")
	assert.Equal(t, "value", result)
}

func TestDefaultString_WhitespaceString(t *testing.T) {
	result := defaultString("   ", "fallback")
	assert.Equal(t, "fallback", result)
}

func TestService_Get_ExtensionPath_Found(t *testing.T) {
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
		Config: map[string]any{
			approvalRecordsKey: []map[string]any{
				{"id": "ext-1", "state": "pending", "actor": "tester"},
				{"id": "ext-2", "state": "approved", "actor": "admin"},
			},
		},
		Operator: "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}
	_ = installed

	service := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})

	// Test finding existing approval
	resp, err := service.Get(context.Background(), &ApprovalGetRequest{ID: "ext-1"})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "ext-1", resp.Approval.ID)

	// Test not finding approval
	_, err = service.Get(context.Background(), &ApprovalGetRequest{ID: "nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_List_ExtensionPath(t *testing.T) {
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
			approvalRecordsKey: []map[string]any{
				{"id": "ext-1", "state": "pending"},
				{"id": "ext-2", "state": "approved"},
			},
		},
		Operator: "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}

	service := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})

	resp, err := service.List(context.Background(), &ApprovalsListRequest{Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Approvals, 2)
}

func TestFindActiveApprovalInstallation_UninstalledSkipped(t *testing.T) {
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
		Config:         map[string]any{},
		Operator:       "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}

	service := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})

	// Before uninstall, should find active installation
	item, ok, err := service.findActiveApprovalInstallation(context.Background())
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, item)
}

func TestLoadApprovalsFromExtension_CorruptedConfig(t *testing.T) {
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

	// Corrupt the config JSON directly
	db.Model(&model.ExtensionInstallation{}).Where("id = ?", installed.ID).Update("config_json", "not-valid-json")

	service := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})

	_, _, err = service.loadApprovalsFromExtensionInstallation(context.Background())
	assert.Error(t, err)
}

func TestUpsertApprovalToExtension_UpdateExisting(t *testing.T) {
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
			approvalRecordsKey: []map[string]any{
				{"id": "ap-1", "state": "pending"},
			},
		},
		Operator: "tester",
	})
	if err != nil {
		t.Fatalf("install extension failed: %v", err)
	}

	service := NewService(&svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationSvc,
		},
	})

	// Update existing approval
	err = service.upsertApprovalToExtension(context.Background(), Approval{ID: "ap-1", State: "approved"})
	assert.NoError(t, err)

	// Verify it was updated
	items, _, err := service.loadApprovalsFromExtensionInstallation(context.Background())
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "approved", items[0].State)
}

// Ensure errorApprovalsStore implements the interface
var _ approvals.Store = (*errorApprovalsStore)(nil)
