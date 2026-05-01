package assignment

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_List_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockAssignmentService{
		listFunc: func(ctx context.Context, req *AssignmentsListRequest) (*AssignmentsListResponse, error) {
			return &AssignmentsListResponse{
				Assignments: map[string][]string{
					"game1|prod": {"func1", "func2"},
				},
				Total:    1,
				Page:     1,
				PageSize: 20,
			}, nil
		},
	}

	handler := &Handler{service: service}
	router := gin.New()
	router.GET("/assignments", handler.List)

	req, _ := http.NewRequest("GET", "/assignments?game_id=game1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "assignments")
}

func TestHandler_List_InvalidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &Handler{service: &mockAssignmentService{}}
	router := gin.New()
	router.GET("/assignments", handler.List)

	// Test with invalid pageSize (not an integer)
	req, _ := http.NewRequest("GET", "/assignments?page=invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Gin's ShouldBindQuery returns error for invalid type conversion
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHandler_List_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockAssignmentService{
		listFunc: func(ctx context.Context, req *AssignmentsListRequest) (*AssignmentsListResponse, error) {
			return nil, errorx.NewInternalError("服务错误")
		},
	}

	handler := &Handler{service: service}
	router := gin.New()
	router.GET("/assignments", handler.List)

	req, _ := http.NewRequest("GET", "/assignments", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_History_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockAssignmentService{
		historyFunc: func(ctx context.Context, req *AssignmentsHistoryRequest) (*AssignmentsHistoryResponse, error) {
			return &AssignmentsHistoryResponse{
				Items: []assignmentHistoryEntry{
					{ID: "1", FunctionID: "func1", Action: "add"},
				},
				Total:    1,
				Page:     1,
				PageSize: 20,
			}, nil
		},
	}

	handler := &Handler{service: service}
	router := gin.New()
	router.GET("/assignments/history", handler.History)

	req, _ := http.NewRequest("GET", "/assignments/history?game_id=game1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "items")
}

func TestHandler_History_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockAssignmentService{
		historyFunc: func(ctx context.Context, req *AssignmentsHistoryRequest) (*AssignmentsHistoryResponse, error) {
			return nil, errorx.NewNotFound("未找到记录")
		},
	}

	handler := &Handler{service: service}
	router := gin.New()
	router.GET("/assignments/history", handler.History)

	req, _ := http.NewRequest("GET", "/assignments/history", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_Update_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockAssignmentService{
		updateFunc: func(ctx context.Context, req *AssignmentsUpdateRequest) (*AssignmentsUpdateResponse, error) {
			return &AssignmentsUpdateResponse{
				OK:      true,
				Unknown: []string{"unknown_func"},
				Assignments: map[string][]string{
					"game1|prod": {"func1", "func2"},
				},
			}, nil
		},
	}

	handler := &Handler{service: service}
	router := gin.New()
	router.PUT("/assignments", handler.Update)

	body, _ := json.Marshal(AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Action:    "assign",
		Functions: []string{"func1", "func2"},
	})

	req, _ := http.NewRequest("PUT", "/assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp["ok"].(bool))
	assert.Contains(t, resp, "unknown")
}

func TestHandler_Update_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &Handler{service: &mockAssignmentService{}}
	router := gin.New()
	router.PUT("/assignments", handler.Update)

	req, _ := http.NewRequest("PUT", "/assignments", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Update_EmptyAssignments(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &mockAssignmentService{
		updateFunc: func(ctx context.Context, req *AssignmentsUpdateRequest) (*AssignmentsUpdateResponse, error) {
			return &AssignmentsUpdateResponse{
				OK:          true,
				Unknown:     nil,
				Assignments: map[string][]string{},
			}, nil
		},
	}

	handler := &Handler{service: service}
	router := gin.New()
	router.PUT("/assignments", handler.Update)

	body, _ := json.Marshal(AssignmentsUpdateRequest{
		GameId:    "game1",
		Env:       "prod",
		Functions: []string{},
	})

	req, _ := http.NewRequest("PUT", "/assignments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Mock service
type mockAssignmentService struct {
	listFunc    func(ctx context.Context, req *AssignmentsListRequest) (*AssignmentsListResponse, error)
	historyFunc func(ctx context.Context, req *AssignmentsHistoryRequest) (*AssignmentsHistoryResponse, error)
	updateFunc  func(ctx context.Context, req *AssignmentsUpdateRequest) (*AssignmentsUpdateResponse, error)
}

func (m *mockAssignmentService) List(ctx context.Context, req *AssignmentsListRequest) (*AssignmentsListResponse, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, req)
	}
	return &AssignmentsListResponse{}, nil
}

func (m *mockAssignmentService) History(ctx context.Context, req *AssignmentsHistoryRequest) (*AssignmentsHistoryResponse, error) {
	if m.historyFunc != nil {
		return m.historyFunc(ctx, req)
	}
	return &AssignmentsHistoryResponse{}, nil
}

func (m *mockAssignmentService) Update(ctx context.Context, req *AssignmentsUpdateRequest) (*AssignmentsUpdateResponse, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, req)
	}
	return &AssignmentsUpdateResponse{}, nil
}
