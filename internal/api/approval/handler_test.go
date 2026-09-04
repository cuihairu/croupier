package approval

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewHandler(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}

func TestHandler_List_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals", handler.List)

	req := httptest.NewRequest("GET", "/approvals?page=1&pageSize=20", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_EmptyResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals", handler.List)

	req := httptest.NewRequest("GET", "/approvals", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_BindingError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals", handler.List)

	req := httptest.NewRequest("GET", "/approvals?page=invalid", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Get_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	approval := &approvals.Approval{ID: "test-1", State: "pending", Actor: "tester"}
	store.Create(approval)

	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals/:id", handler.Get)

	req := httptest.NewRequest("GET", "/approvals/test-1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Get_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals/:id", handler.Get)

	req := httptest.NewRequest("GET", "/approvals/nonexistent", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Get_InvalidURI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals/:id", handler.Get)

	// Empty ID should fail binding
	req := httptest.NewRequest("GET", "/approvals/", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Approve_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	approval := &approvals.Approval{ID: "test-approve-1", State: "pending", Actor: "tester"}
	store.Create(approval)

	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/approvals/:id/approve", handler.Approve)

	ctx := context.WithValue(context.Background(), "username", "admin")
	req := httptest.NewRequest("POST", "/approvals/test-approve-1/approve", nil).WithContext(ctx)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Approve_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/approvals/:id/approve", handler.Approve)

	req := httptest.NewRequest("POST", "/approvals/nonexistent/approve", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Reject_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	approval := &approvals.Approval{ID: "test-reject-1", State: "pending", Actor: "tester"}
	store.Create(approval)

	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/approvals/:id/reject", func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "username", "admin"))
		handler.Reject(c)
	})

	reqBody := `{"reason":"test reason"}`
	req := httptest.NewRequest("POST", "/approvals/test-reject-1/reject", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Reject_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/approvals/:id/reject", handler.Reject)

	reqBody := `{"reason":"not found"}`
	req := httptest.NewRequest("POST", "/approvals/nonexistent/reject", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Reject_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/approvals/:id/reject", handler.Reject)

	req := httptest.NewRequest("POST", "/approvals/test-id/reject", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Reject_InvalidURI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/approvals/:id/reject", handler.Reject)

	reqBody := `{"reason":"test"}`
	req := httptest.NewRequest("POST", "/approvals//reject", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_List_WithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals", handler.List)

	req := httptest.NewRequest("GET", "/approvals?state=pending&page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_WithActorFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals", handler.List)

	req := httptest.NewRequest("GET", "/approvals?actor=testuser&page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Approve_InvalidURI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/approvals/:id/approve", handler.Approve)

	// Empty ID should fail binding
	req := httptest.NewRequest("POST", "/approvals//approve", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

// Additional service tests for coverage

func TestService_Get_NilRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	_, err := service.Get(context.Background(), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestService_Get_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	_, err := service.Get(context.Background(), &ApprovalGetRequest{ID: ""})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestService_Get_WhitespaceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	_, err := service.Get(context.Background(), &ApprovalGetRequest{ID: "  "})

	assert.Error(t, err)
}

func TestService_Get_NilStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	_, err := service.Get(context.Background(), &ApprovalGetRequest{ID: "test-id"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestService_List_NilRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	resp, err := service.List(context.Background(), nil)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_List_NilStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	_, err := service.List(context.Background(), &ApprovalsListRequest{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestService_Approve_NilRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Approve(context.Background(), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestService_Approve_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Approve(context.Background(), &ApprovalApproveRequest{ID: ""})

	assert.Error(t, err)
}

func TestService_Approve_NilStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})

	_, err := service.Approve(context.Background(), &ApprovalApproveRequest{ID: "test-id"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestService_Reject_NilRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Reject(context.Background(), nil)

	assert.Error(t, err)
}

func TestService_Reject_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Reject(context.Background(), &ApprovalRejectRequest{ID: "", Reason: "test"})

	assert.Error(t, err)
}

func TestService_Reject_EmptyReason(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Reject(context.Background(), &ApprovalRejectRequest{ID: "test-id", Reason: ""})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reason")
}

func TestService_Reject_WhitespaceReason(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	_, err := service.Reject(context.Background(), &ApprovalRejectRequest{ID: "test-id", Reason: "  "})

	assert.Error(t, err)
}

func TestService_Reject_NilStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})

	_, err := service.Reject(context.Background(), &ApprovalRejectRequest{ID: "test-id", Reason: "test"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

// Helper function tests

func TestBuildApprovalSummary_NilApproval(t *testing.T) {
	result := buildApprovalSummary(nil)

	assert.Empty(t, result.ID)
	assert.Empty(t, result.Actor)
}

func TestBuildApprovalDetail_NilApproval(t *testing.T) {
	result := buildApprovalDetail(nil)

	assert.Empty(t, result.ID)
	assert.Empty(t, result.Actor)
	assert.Nil(t, result.Payload)
}

func TestDecodeApprovalPayload_NilApproval(t *testing.T) {
	payload, preview := decodeApprovalPayload(nil)

	assert.Nil(t, payload)
	assert.Empty(t, preview)
}

func TestDecodeApprovalPayload_EmptyPayload(t *testing.T) {
	a := &approvals.Approval{}
	payload, preview := decodeApprovalPayload(a)

	assert.Nil(t, payload)
	assert.Empty(t, preview)
}

func TestDecodeApprovalPayload_InvalidJSON(t *testing.T) {
	a := &approvals.Approval{Payload: []byte("{invalid json")}
	payload, preview := decodeApprovalPayload(a)

	assert.Nil(t, payload)
	assert.NotEmpty(t, preview)
}

func TestDecodeApprovalPayload_ValidJSON(t *testing.T) {
	a := &approvals.Approval{Payload: []byte(`{"key":"value"}`)}
	payload, preview := decodeApprovalPayload(a)

	assert.NotNil(t, payload)
	assert.NotEmpty(t, preview)
	assert.Equal(t, "value", payload["key"])
}

func TestDefaultString_Empty(t *testing.T) {
	result := defaultString("", "fallback")

	assert.Equal(t, "fallback", result)
}

func TestDefaultString_Whitespace(t *testing.T) {
	result := defaultString("  ", "fallback")

	assert.Equal(t, "fallback", result)
}

func TestDefaultString_NonEmpty(t *testing.T) {
	result := defaultString("value", "fallback")

	assert.Equal(t, "value", result)
}

func TestFindActiveApprovalInstallation_NilService(t *testing.T) {
	var service *Service
	_, ok, err := service.findActiveApprovalInstallation(context.Background())

	assert.False(t, ok)
	assert.NoError(t, err)
}

func TestFindActiveApprovalInstallation_NilContext(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	_, ok, err := service.findActiveApprovalInstallation(context.Background())

	assert.False(t, ok)
	assert.NoError(t, err)
}

func TestLoadApprovalsFromExtensionInstallation_NilService(t *testing.T) {
	var service *Service
	_, ok, err := service.loadApprovalsFromExtensionInstallation(context.Background())

	assert.False(t, ok)
	assert.NoError(t, err)
}

func TestRecordApprovalEvent_NilService(t *testing.T) {
	var service *Service
	err := service.recordApprovalEvent(context.Background(), "test", "message", "{}")

	assert.NoError(t, err)
}

func TestSaveApprovalsToExtensionInstallation_NilService(t *testing.T) {
	var service *Service
	err := service.saveApprovalsToExtensionInstallation(context.Background(), []Approval{})

	assert.NoError(t, err)
}

func TestUpsertApprovalToExtension_NilService(t *testing.T) {
	var service *Service
	err := service.upsertApprovalToExtension(context.Background(), Approval{ID: "test"})

	assert.NoError(t, err)
}

func TestUpsertApprovalToExtension_EmptyID(t *testing.T) {
	service := NewService(&svc.ServiceContext{})
	err := service.upsertApprovalToExtension(context.Background(), Approval{})

	assert.NoError(t, err)
}

func TestFilterApprovalSummariesByState_EmptyState(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1", State: "pending"},
		{ID: "2", State: "approved"},
	}

	result := filterApprovalSummariesByState(items, "")

	assert.Len(t, result, 2)
}

func TestFilterApprovalSummariesByState_CaseInsensitive(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1", State: "PENDING"},
		{ID: "2", State: "approved"},
	}

	result := filterApprovalSummariesByState(items, "pending")

	assert.Len(t, result, 1)
}

func TestPaginateApprovalSummaries_EmptyList(t *testing.T) {
	result, total := paginateApprovalSummaries([]ApprovalSummary{}, 1, 10)

	assert.Empty(t, result)
	assert.Equal(t, 0, total)
}

func TestPaginateApprovalSummaries_ZeroPage(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1"}, {ID: "2"},
	}

	result, total := paginateApprovalSummaries(items, 0, 1)

	assert.Len(t, result, 1)
	assert.Equal(t, 2, total)
}

func TestPaginateApprovalSummaries_ZeroSize(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1"}, {ID: "2"},
	}

	result, total := paginateApprovalSummaries(items, 1, 0)

	assert.Len(t, result, 2)
	assert.Equal(t, 2, total)
}

func TestToApprovalSummaries_EmptyList(t *testing.T) {
	result := toApprovalSummaries([]Approval{})

	assert.Empty(t, result)
}

func TestToApprovalSummaries_PreserveFields(t *testing.T) {
	items := []Approval{
		{
			ID:         "test-1",
			Actor:      "user1",
			State:      "pending",
			FunctionID: "func-1",
			GameID:     "game-1",
		},
	}

	result := toApprovalSummaries(items)

	assert.Len(t, result, 1)
	assert.Equal(t, "test-1", result[0].ID)
	assert.Equal(t, "user1", result[0].Actor)
}

func TestHandler_Approve_NilStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/approvals/:id/approve", handler.Approve)

	req := httptest.NewRequest("POST", "/approvals/test-1/approve", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Reject_NilStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/approvals/:id/reject", handler.Reject)

	reqBody := `{"reason":"test"}`
	req := httptest.NewRequest("POST", "/approvals/test-1/reject", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Get_NilStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals/:id", handler.Get)

	req := httptest.NewRequest("GET", "/approvals/test-1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_List_NilStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals", handler.List)

	req := httptest.NewRequest("GET", "/approvals", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_Get_WithPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := approvals.NewMemStore()
	approval := &approvals.Approval{
		ID:      "test-payload",
		State:   "pending",
		Actor:   "tester",
		Payload: []byte(`{"key":"value","nested":{"a":1}}`),
	}
	store.Create(approval)

	service := NewService(&svc.ServiceContext{ApprovalsStore: store})
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/approvals/:id", handler.Get)

	req := httptest.NewRequest("GET", "/approvals/test-payload", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestService_Get_WithValidStore(t *testing.T) {
	store := approvals.NewMemStore()
	approval := &approvals.Approval{
		ID:    "test-valid",
		State: "pending",
		Actor: "tester",
	}
	store.Create(approval)

	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	resp, err := service.Get(context.Background(), &ApprovalGetRequest{ID: "test-valid"})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-valid", resp.Approval.ID)
}

func TestService_Approve_WithValidStore(t *testing.T) {
	store := approvals.NewMemStore()
	approval := &approvals.Approval{
		ID:    "test-approve-valid",
		State: "pending",
		Actor: "tester",
	}
	store.Create(approval)

	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	ctx := context.WithValue(context.Background(), "username", "admin")
	resp, err := service.Approve(ctx, &ApprovalApproveRequest{ID: "test-approve-valid"})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "approved", resp.State)
}

func TestService_Reject_WithValidStore(t *testing.T) {
	store := approvals.NewMemStore()
	approval := &approvals.Approval{
		ID:    "test-reject-valid",
		State: "pending",
		Actor: "tester",
	}
	store.Create(approval)

	service := NewService(&svc.ServiceContext{ApprovalsStore: store})

	ctx := context.WithValue(context.Background(), "username", "admin")
	resp, err := service.Reject(ctx, &ApprovalRejectRequest{ID: "test-reject-valid", Reason: "test reason"})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "rejected", resp.State)
}

func TestPaginateApprovalSummaries_NegativePage(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1"}, {ID: "2"},
	}

	result, total := paginateApprovalSummaries(items, -1, 10)

	assert.Len(t, result, 2)
	assert.Equal(t, 2, total)
}

func TestPaginateApprovalSummaries_NegativeSize(t *testing.T) {
	items := []ApprovalSummary{
		{ID: "1"}, {ID: "2"},
	}

	result, total := paginateApprovalSummaries(items, 1, -1)

	assert.Len(t, result, 2)
	assert.Equal(t, 2, total)
}
