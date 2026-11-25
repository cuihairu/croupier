package alert

// Extra coverage for inbound.go (failure-injection / pure-function arm conventions):
// the request-body read failure arm, ingestInbound's model-not-initialized and Upsert
// failure pass-through arms, the generic payload severity default and title/body
// combination arms, the alertmanager payload malformed-JSON and empty-list arms,
// mapAMFields type/status mapping, the description fallback, the kind and firing
// fallback arms, and the inbound rejection audit chain (Log failure -> ReportWriteGap).
//
// Honest skips: none.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// errBodyReader always returns a read error (GetRawData failure arm).
type errBodyReader struct{}

func (errBodyReader) Read([]byte) (int, error) { return 0, errors.New("body pipe broken") }

// TestInbound_ReadBodyError: request body read failure (68-70).
func TestInbound_ReadBodyError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupAlertTestDB(t)
	svcCtx := setupAlertServiceContext(t, db)
	svcCtx.Config.AlertInbound.Sources = map[string]config.AlertInboundSourceConfig{
		"generic": {SecretEnv: "INBOUND_TEST_SECRET"},
	}
	h := NewHandler(NewService(svcCtx))
	e := gin.New()
	e.POST("/api/v1/alerts/inbound/:source", h.Inbound)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/inbound/generic", errBodyReader{})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	require.GreaterOrEqual(t, w.Code, 400)
	require.Contains(t, w.Body.String(), "read body failed")
}

// ingestInbound's alert-model-not-initialized arm (112-113).
func TestIngestInbound_NilAlertModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	_, _, err := s.ingestInbound(context.Background(), "generic", []byte(`{"eventId":"e1"}`),
		config.AlertInboundSourceConfig{SecretEnv: "X"})
	require.ErrorContains(t, err, "告警模型未初始化")
}

// ingestInbound's UpsertByAlertID failure pass-through arm (128-129): injected Create failure.
func TestIngestInbound_UpsertError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1) // :memory: single connection
	}
	require.NoError(t, db.AutoMigrate(&model.Alert{}))

	var hits int
	require.NoError(t, db.Callback().Create().After("gorm:create").Register("test:fail_alerts", func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "alerts" {
			return
		}
		hits++
		if hits == 1 {
			_ = tx.AddError(errors.New("failpoint: alerts down"))
		}
	}))

	s := NewService(&svc.ServiceContext{AlertModel: model.NewAlertModel(db)})
	_, _, err = s.ingestInbound(context.Background(), "generic", []byte(`{"eventId":"e1"}`),
		config.AlertInboundSourceConfig{SecretEnv: "X"})
	require.ErrorContains(t, err, "alerts down")
}

// Generic payload mapping default and combination arms: empty severity -> warning (162),
// title+body combination (168) and body-only (169-170).
func TestMapGenericPayload_Fallbacks(t *testing.T) {
	a, err := mapGenericPayload([]byte(`{"eventId":"e1"}`), "generic")
	require.NoError(t, err)
	require.Equal(t, "warning", a.Level)
	require.Empty(t, a.Message)

	a, err = mapGenericPayload([]byte(`{"eventId":"e1","severity":"critical","title":"T","body":"B"}`), "generic")
	require.NoError(t, err)
	require.Equal(t, "T: B", a.Message)

	a, err = mapGenericPayload([]byte(`{"eventId":"e1","body":"only body"}`), "generic")
	require.NoError(t, err)
	require.Equal(t, "only body", a.Message)
}

// Alertmanager payload mapping malformed-JSON (220-221) and empty-list (223-224) arms.
func TestMapAlertmanagerPayload_InvalidAndEmpty(t *testing.T) {
	_, err := mapAlertmanagerPayload([]byte(`not-json`), nil)
	require.ErrorContains(t, err, "invalid alertmanager payload")

	_, err = mapAlertmanagerPayload([]byte(`{"alerts":[]}`), nil)
	require.ErrorContains(t, err, "alerts is empty")
}

// mapAMFields mapping and fallback arms: instance->type (default mapping, 253),
// custom status mapping (257), description fallback (266-268), kind fallback
// (271-272), firing fallback (278-279).
func TestMapAMFields_Branches(t *testing.T) {
	// Default mapping: severity->level, summary->message, instance->type; unmapped keys -> details
	level, message, alertType, status, details := mapAMFields(inboundAMAlert{
		Labels: map[string]string{"instance": "host-1", "extra": "v"},
	}, defaultAlertmanagerLabelMapping)
	require.Empty(t, level)
	require.Empty(t, message)
	require.Equal(t, "host-1", alertType)

	// summary->message mapping (253)
	level, message, _, _, _ = mapAMFields(inboundAMAlert{
		Labels: map[string]string{"severity": "critical", "summary": "disk full"},
	}, defaultAlertmanagerLabelMapping)
	require.Equal(t, "critical", level)
	require.Equal(t, "disk full", message)
	require.Equal(t, "firing", status)
	require.Equal(t, "v", details["label.extra"])

	// Custom status mapping
	_, _, _, status, _ = mapAMFields(inboundAMAlert{
		Status: "ignored-by-mapping",
		Labels: map[string]string{"status": "resolved"},
	}, map[string]string{"status": "status"})
	require.Equal(t, "resolved", status)

	// annotations.description fallback when message is empty; unmapped type -> the fixed kind
	_, message, alertType, _, _ = mapAMFields(inboundAMAlert{
		Annotations: map[string]string{"description": "desc only"},
	}, nil)
	require.Equal(t, "desc only", message)
	require.Equal(t, inboundKindAlertManager, alertType)
}

// failAuditStore is an audit store that only fails on Create (other methods delegate to memory).
type failAuditStore struct {
	*audit.InMemoryAuditStore
	createErr error
}

func (s *failAuditStore) Create(record *audit.AuditRecord) error { return s.createErr }

// Inbound rejection audit chain (336-344): signature rejection -> AuditService.Log
// failure -> ReportWriteGap fallback branch.
func TestAuditInboundRejected_ReportWriteGap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := NewService(&svc.ServiceContext{AuditService: audit.NewAuditService(
		&failAuditStore{InMemoryAuditStore: audit.NewInMemoryAuditStore(), createErr: errors.New("audit down")}, nil)})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/alerts/inbound/generic", nil)

	require.NotPanics(t, func() {
		s.auditInboundRejected(c, "generic", "signature_invalid")
	})
}

// Control group: the successful audit write path (Log does not fail -> no ReportWriteGap).
func TestAuditInboundRejected_LogSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := NewService(&svc.ServiceContext{AuditService: audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/alerts/inbound/generic", nil)

	require.NotPanics(t, func() {
		s.auditInboundRejected(c, "generic", "timestamp_replay")
	})
}
