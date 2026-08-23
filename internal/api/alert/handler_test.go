package alert

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	alertTestDB      *gorm.DB
	alertTestDBOnce  sync.Once
	alertTestDBMutex sync.Mutex
)

func setupAlertTestDB(t *testing.T) *gorm.DB {
	alertTestDBMutex.Lock()
	defer alertTestDBMutex.Unlock()

	alertTestDBOnce.Do(func() {
		var err error
		alertTestDB, err = gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		err = model.AutoMigrate(alertTestDB)
		if err != nil {
			panic(err)
		}
	})

	// Clean up before each test
	alertTestDB.Exec("DELETE FROM alert_silences")
	alertTestDB.Exec("DELETE FROM alerts")

	return alertTestDB
}

func setupAlertServiceContext(t *testing.T, db *gorm.DB) *svc.ServiceContext {
	return &svc.ServiceContext{
		DB:         db,
		AlertModel: model.NewAlertModel(db),
	}
}

func TestNewHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})
	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}

func TestHandler_List_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/alerts", handler.List)

	req := httptest.NewRequest("GET", "/alerts?page=1&pageSize=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_DefaultPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/alerts", handler.List)

	req := httptest.NewRequest("GET", "/alerts", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_WithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/alerts", handler.List)

	req := httptest.NewRequest("GET", "/alerts?level=error&status=active", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_InvalidPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/alerts", handler.List)

	req := httptest.NewRequest("GET", "/alerts?page=invalid", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle invalid page gracefully
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_Silence_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	// Create a test alert
	alertModel := model.NewAlertModel(db)
	alert := &model.Alert{
		AlertID: "test-alert-1",
		Type:    "test_type",
		Level:   "error",
		Message: "Test alert",
		Status:  "active",
		Details: datatypes.JSONMap{"key": "value"},
	}
	require.NoError(t, alertModel.Create(context.Background(), alert))

	router := gin.New()
	router.POST("/alerts/:id/silence", handler.Silence)

	reqBody := `{"reason": "test reason", "duration": 60}`
	req := httptest.NewRequest("POST", "/alerts/test-alert-1/silence", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Silence_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/alerts/:id/silence", handler.Silence)

	reqBody := `{"reason": "test", "duration": 60}`
	req := httptest.NewRequest("POST", "/alerts//silence", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle empty ID gracefully
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_Silence_DefaultDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	// Create a test alert
	alertModel := model.NewAlertModel(db)
	alert := &model.Alert{
		AlertID: "test-alert-2",
		Type:    "test",
		Level:   "info",
		Message: "Test",
		Status:  "active",
	}
	require.NoError(t, alertModel.Create(context.Background(), alert))

	router := gin.New()
	router.POST("/alerts/:id/silence", handler.Silence)

	// Request without duration - should default to 60
	reqBody := `{"reason": "test"}`
	req := httptest.NewRequest("POST", "/alerts/test-alert-2/silence", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Silence_MissingAlert(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/alerts/:id/silence", handler.Silence)

	reqBody := `{"reason": "test", "duration": 60}`
	req := httptest.NewRequest("POST", "/alerts/nonexistent/silence", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should return error for non-existent alert
	assert.NotEqual(t, http.StatusOK, resp.Code)
}

func TestHandler_SilencesList_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/silences", handler.SilencesList)

	req := httptest.NewRequest("GET", "/silences", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_SilenceDelete_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	// Create a test alert and silence
	alertModel := model.NewAlertModel(db)
	alert := &model.Alert{
		AlertID: "test-alert-delete",
		Type:    "test",
		Level:   "error",
		Message: "Test alert",
		Status:  "active",
	}
	require.NoError(t, alertModel.Create(context.Background(), alert))
	silence := &model.AlertSilence{
		AlertID:        alert.ID,
		Reason:         "test silence",
		DurationMinute: 60,
		CreatedBy:      "test",
	}
	require.NoError(t, alertModel.CreateSilence(context.Background(), silence))

	router := gin.New()
	router.DELETE("/silences/:id", handler.SilenceDelete)

	silenceID := fmt.Sprintf("%d", silence.ID)
	req := httptest.NewRequest("DELETE", "/silences/"+silenceID, nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_SilenceDelete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.DELETE("/silences/:id", handler.SilenceDelete)

	req := httptest.NewRequest("DELETE", "/silences/invalid", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle invalid ID gracefully
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_SilenceDelete_Nonexistent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.DELETE("/silences/:id", handler.SilenceDelete)

	req := httptest.NewRequest("DELETE", "/silences/999", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Silence_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/alerts/:id/silence", handler.Silence)

	req := httptest.NewRequest("POST", "/alerts/test-id/silence", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle invalid JSON gracefully - binding error
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_Silence_MissingBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.POST("/alerts/:id/silence", handler.Silence)

	req := httptest.NewRequest("POST", "/alerts/test-id/silence", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle missing body gracefully
	assert.NotEqual(t, http.StatusNotFound, resp.Code)
}

func TestHandler_List_WithLargePage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/alerts", handler.List)

	req := httptest.NewRequest("GET", "/alerts?page=999&pageSize=1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle out of range page gracefully
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_NegativePage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/alerts", handler.List)

	req := httptest.NewRequest("GET", "/alerts?page=-1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle negative page gracefully
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_NegativePageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/alerts", handler.List)

	req := httptest.NewRequest("GET", "/alerts?pageSize=-10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle negative page size gracefully
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_WithAllFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/alerts", handler.List)

	req := httptest.NewRequest("GET", "/alerts?level=warning&status=resolved&page=1&pageSize=20", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_List_InvalidFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/alerts", handler.List)

	req := httptest.NewRequest("GET", "/alerts?level=invalid&status=invalid", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	// Should handle invalid filter values gracefully
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestHandler_Silence_WithExistingAlerts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	service := NewService(svcCtx)
	handler := NewHandler(service)

	// Create multiple test alerts
	alertModel := model.NewAlertModel(db)
	for i := 1; i <= 3; i++ {
		alert := &model.Alert{
			AlertID: fmt.Sprintf("test-alert-%d", i),
			Type:    "test_type",
			Level:   "error",
			Message: fmt.Sprintf("Test alert %d", i),
			Status:  "active",
		}
		require.NoError(t, alertModel.Create(context.Background(), alert))
	}

	router := gin.New()
	router.POST("/alerts/:id/silence", handler.Silence)

	reqBody := `{"reason": "bulk test", "duration": 30}`
	req := httptest.NewRequest("POST", "/alerts/test-alert-2/silence", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}
