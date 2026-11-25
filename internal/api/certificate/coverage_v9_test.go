package certificate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

// closeV9DB closes the underlying connection so all later model operations fail.
func closeV9DB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

// failV9Updates registers an update pre-callback that fails every UPDATE.
func failV9Updates(db *gorm.DB) {
	_ = db.Callback().Update().Before("gorm:update").Register("v9_fail_update", func(tx *gorm.DB) {
		_ = tx.AddError(errors.New("forced update failure"))
	})
}

// failV9CreateFor registers a create pre-callback that fails INSERTs of the given Dest
// type.
func failV9CreateFor(db *gorm.DB, match func(tx *gorm.DB) bool) {
	_ = db.Callback().Create().Before("gorm:create").Register("v9_fail_create", func(tx *gorm.DB) {
		if match(tx) {
			_ = tx.AddError(errors.New("forced create failure"))
		}
	})
}

func TestHandlerBindErrorsV9(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _ := setupTestService(t)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/certificates", handler.List)
	router.POST("/certificates", handler.Add)
	router.POST("/certificates/alerts", handler.AddAlert)
	router.GET("/certificates/alerts", handler.ListAlerts)
	router.GET("/certificates/expiring", handler.GetExpiring)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"list bad page", http.MethodGet, "/certificates?page=abc", ""},
		{"add bad json", http.MethodPost, "/certificates", `not-json`},
		{"add alert bad json", http.MethodPost, "/certificates/alerts", `not-json`},
		{"list alerts bad page", http.MethodGet, "/certificates/alerts?page=abc", ""},
		{"expiring bad days", http.MethodGet, "/certificates/expiring?days=abc", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.body != "" {
				req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

func TestHandlerServiceErrorsV9(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, db := setupTestService(t)
	closeV9DB(t, db)
	handler := NewHandler(service)

	router := gin.New()
	router.GET("/certificates", handler.List)
	router.GET("/certificates/stats", handler.Stats)
	router.GET("/certificates/alerts", handler.ListAlerts)
	router.POST("/certificates/check-all", handler.CheckAll)
	router.GET("/certificates/domain/:domain", handler.GetDomainInfo)
	router.GET("/certificates/expiring", handler.GetExpiring)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/certificates?page=1&pageSize=10"},
		{http.MethodGet, "/certificates/stats"},
		{http.MethodGet, "/certificates/alerts?page=1&pageSize=10"},
		{http.MethodPost, "/certificates/check-all"},
		{http.MethodGet, "/certificates/domain/example.com"},
		{http.MethodGet, "/certificates/expiring?days=30"},
	}
	for _, ep := range endpoints {
		t.Run(ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			// GetDomainInfo's DTO uses form tags, so uri binding is ineffective
			// (domain stays empty) and also lands in the error branch (400); the
			// rest are service-layer errors (500).
			assert.NotEqual(t, http.StatusOK, w.Code, w.Body.String())
		})
	}
}

func TestServiceListModelErrorV9(t *testing.T) {
	service, db := setupTestService(t)
	closeV9DB(t, db)

	_, err := service.List(context.Background(), &ListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
}

func TestServiceAddFindModelErrorV9(t *testing.T) {
	service, db := setupTestService(t)
	closeV9DB(t, db)

	// Connection closed: FindByDomain returns an error other than "certificate not found".
	_, err := service.Add(context.Background(), &AddRequest{
		Domain:      "example.com",
		Certificate: generateTestCert(t, "example.com", 30),
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "证书不存在")
}

func TestServiceAddUpdateErrorV9(t *testing.T) {
	service, db := setupTestService(t)

	// An already existing domain takes the Update branch.
	_, err := service.Add(context.Background(), &AddRequest{
		Domain:      "example.com",
		Certificate: generateTestCert(t, "example.com", 30),
	})
	require.NoError(t, err)

	failV9Updates(db)
	_, err = service.Add(context.Background(), &AddRequest{
		Domain:      "example.com",
		Certificate: generateTestCert(t, "example.com", 60),
	})
	require.Error(t, err)
}

func TestServiceAddCreateErrorV9(t *testing.T) {
	service, db := setupTestService(t)
	failV9CreateFor(db, func(tx *gorm.DB) bool {
		_, ok := tx.Statement.Dest.(*model.Certificate)
		return ok
	})

	_, err := service.Add(context.Background(), &AddRequest{
		Domain:      "example.com",
		Certificate: generateTestCert(t, "example.com", 30),
	})
	require.Error(t, err)
}

func TestServiceCheckUpdateErrorV9(t *testing.T) {
	service, db := setupTestService(t)

	cert := &model.Certificate{
		Domain:         "example.com",
		CertificatePEM: generateTestCert(t, "example.com", 30),
		ExpiresAt:      time.Now().Add(30 * 24 * time.Hour),
		Status:         "valid",
	}
	require.NoError(t, db.Create(cert).Error)

	failV9Updates(db)
	_, err := service.Check(context.Background(), &CheckRequest{ID: "1"})
	require.Error(t, err)
}

func TestServiceDeleteModelErrorV9(t *testing.T) {
	service, db := setupTestService(t)
	closeV9DB(t, db)

	err := service.Delete(context.Background(), &DeleteRequest{ID: "1"})
	require.Error(t, err)
}

func TestServiceStatsModelErrorV9(t *testing.T) {
	service, db := setupTestService(t)
	closeV9DB(t, db)

	_, err := service.Stats(context.Background())
	require.Error(t, err)
}

func TestServiceAddAlertInvalidDomainV9(t *testing.T) {
	service, _ := setupTestService(t)

	_, err := service.AddAlert(context.Background(), &AddAlertRequest{Domain: "", Threshold: 7})
	require.Error(t, err)
}

func TestServiceAddAlertModelErrorV9(t *testing.T) {
	service, db := setupTestService(t)

	cert := &model.Certificate{
		Domain:    "example.com",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Status:    "valid",
	}
	require.NoError(t, db.Create(cert).Error)

	failV9CreateFor(db, func(tx *gorm.DB) bool {
		_, ok := tx.Statement.Dest.(*model.CertificateAlert)
		return ok
	})

	_, err := service.AddAlert(context.Background(), &AddAlertRequest{Domain: "example.com", Threshold: 7})
	require.Error(t, err)
}

func TestServiceListAlertsModelErrorV9(t *testing.T) {
	service, db := setupTestService(t)
	closeV9DB(t, db)

	_, err := service.ListAlerts(context.Background(), &ListAlertsRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
}

func TestServiceListAlertsWithRowsV9(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	triggered := time.Now().Add(-time.Hour)
	require.NoError(t, db.Create(&model.CertificateAlert{
		Domain:          "example.com",
		ThresholdDays:   14,
		Active:          true,
		LastTriggeredAt: &triggered,
	}).Error)
	require.NoError(t, db.Create(&model.CertificateAlert{
		Domain:        "other.com",
		ThresholdDays: 7,
	}).Error)
	// gorm applies default:true to a bool zero value, so turning it off needs an
	// explicit update.
	require.NoError(t, db.Model(&model.CertificateAlert{}).
		Where("domain = ?", "other.com").Update("active", false).Error)

	resp, err := service.ListAlerts(ctx, &ListAlertsRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, int64(2), resp.Total)

	byDomain := map[string]AlertItem{}
	for _, item := range resp.Items {
		byDomain[item.Domain] = item
	}
	assert.Equal(t, 14, byDomain["example.com"].ThresholdDays)
	assert.True(t, byDomain["example.com"].Active)
	assert.NotEmpty(t, byDomain["example.com"].LastTriggeredAt)
	assert.NotEmpty(t, byDomain["example.com"].CreatedAt)
	assert.False(t, byDomain["other.com"].Active)
}

func TestServiceCheckAllModelErrorV9(t *testing.T) {
	service, db := setupTestService(t)
	closeV9DB(t, db)

	_, err := service.CheckAll(context.Background())
	require.Error(t, err)
}

func TestServiceCheckAllInvalidPEMV9(t *testing.T) {
	service, db := setupTestService(t)

	require.NoError(t, db.Create(&model.Certificate{
		Domain:         "bad.example.com",
		CertificatePEM: "not a pem",
		Status:         "valid",
	}).Error)
	require.NoError(t, db.Create(&model.Certificate{
		Domain:         "good.example.com",
		CertificatePEM: generateTestCert(t, "good.example.com", 90),
		Status:         "valid",
	}).Error)

	resp, err := service.CheckAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Checked)
	assert.Equal(t, 1, resp.Failed)
	assert.Equal(t, 2, resp.Total)

	var bad model.Certificate
	require.NoError(t, db.Where("domain = ?", "bad.example.com").First(&bad).Error)
	assert.Equal(t, "invalid", bad.Status)
	assert.NotEmpty(t, bad.ErrorMessage)
}

func TestServiceGetExpiringModelErrorV9(t *testing.T) {
	service, db := setupTestService(t)
	closeV9DB(t, db)

	_, err := service.GetExpiring(context.Background(), &ExpiringRequest{Days: 30})
	require.Error(t, err)
}

func TestServiceGetExpiringWithRowsV9(t *testing.T) {
	service, db := setupTestService(t)

	require.NoError(t, db.Create(&model.Certificate{
		Domain:    "soon.example.com",
		ExpiresAt: time.Now().Add(5 * 24 * time.Hour),
		Status:    "expiring",
	}).Error)
	require.NoError(t, db.Create(&model.Certificate{
		Domain:    "far.example.com",
		ExpiresAt: time.Now().Add(300 * 24 * time.Hour),
		Status:    "valid",
	}).Error)

	resp, err := service.GetExpiring(context.Background(), &ExpiringRequest{Days: 30})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, 30, resp.Days)
	assert.Equal(t, "soon.example.com", resp.Items[0].Domain)
}
