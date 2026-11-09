package alert

// inbound.go 补测（失败注入/纯函数臂口径）：请求体读取失败臂、
// ingestInbound 的模型未初始化与 Upsert 失败透传臂、generic 载荷的
// severity 缺省与 title/body 组合臂、alertmanager 载荷的非法 JSON 与
// 空列表臂、mapAMFields 的 type/status 映射、description 回退、kind 与
// firing 兜底臂、入站拒绝审计链（Log 失败 → ReportWriteGap）。
//
// 诚实跳过：无。

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

// errBodyReader 恒返回读取错误（GetRawData 失败臂）。
type errBodyReader struct{}

func (errBodyReader) Read([]byte) (int, error) { return 0, errors.New("body pipe broken") }

// TestInbound_ReadBodyError：请求体读取失败（68-70）。
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

// ingestInbound 的告警模型未初始化臂（112-113）。
func TestIngestInbound_NilAlertModel(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	_, _, err := s.ingestInbound(context.Background(), "generic", []byte(`{"eventId":"e1"}`),
		config.AlertInboundSourceConfig{SecretEnv: "X"})
	require.ErrorContains(t, err, "告警模型未初始化")
}

// ingestInbound 的 UpsertByAlertID 失败透传臂（128-129）：Create 注入失败。
func TestIngestInbound_UpsertError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1) // :memory: 单连接
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

// generic 载荷映射的缺省与组合臂：severity 空 → warning（162）、
// title+body 组合（168）与仅 body（169-170）。
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

// alertmanager 载荷映射的非法 JSON（220-221）与空列表（223-224）臂。
func TestMapAlertmanagerPayload_InvalidAndEmpty(t *testing.T) {
	_, err := mapAlertmanagerPayload([]byte(`not-json`), nil)
	require.ErrorContains(t, err, "invalid alertmanager payload")

	_, err = mapAlertmanagerPayload([]byte(`{"alerts":[]}`), nil)
	require.ErrorContains(t, err, "alerts is empty")
}

// mapAMFields 的映射与兜底臂：instance→type（默认映射，253）、
// 自定义 status 映射（257）、description 回退（266-268）、kind 兜底
// （271-272）、firing 兜底（278-279）。
func TestMapAMFields_Branches(t *testing.T) {
	// 默认映射：severity→level、summary→message、instance→type；无映射键 → details
	level, message, alertType, status, details := mapAMFields(inboundAMAlert{
		Labels: map[string]string{"instance": "host-1", "extra": "v"},
	}, defaultAlertmanagerLabelMapping)
	require.Empty(t, level)
	require.Empty(t, message)
	require.Equal(t, "host-1", alertType)

	// summary→message 映射（253）
	level, message, _, _, _ = mapAMFields(inboundAMAlert{
		Labels: map[string]string{"severity": "critical", "summary": "disk full"},
	}, defaultAlertmanagerLabelMapping)
	require.Equal(t, "critical", level)
	require.Equal(t, "disk full", message)
	require.Equal(t, "firing", status)
	require.Equal(t, "v", details["label.extra"])

	// 自定义 status 映射
	_, _, _, status, _ = mapAMFields(inboundAMAlert{
		Status: "ignored-by-mapping",
		Labels: map[string]string{"status": "resolved"},
	}, map[string]string{"status": "status"})
	require.Equal(t, "resolved", status)

	// message 空时 annotations.description 回退；无映射 type → 固定 kind
	_, message, alertType, _, _ = mapAMFields(inboundAMAlert{
		Annotations: map[string]string{"description": "desc only"},
	}, nil)
	require.Equal(t, "desc only", message)
	require.Equal(t, inboundKindAlertManager, alertType)
}

// failAuditStore 只让 Create 失败的审计存储（其余方法透传内存实现）。
type failAuditStore struct {
	*audit.InMemoryAuditStore
	createErr error
}

func (s *failAuditStore) Create(record *audit.AuditRecord) error { return s.createErr }

// 入站拒绝审计链（336-344）：签名拒绝 → AuditService.Log 失败 →
// ReportWriteGap 兜底分支。
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

// 对照组：审计写入成功路径（Log 不失败 → 不触发 ReportWriteGap）。
func TestAuditInboundRejected_LogSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := NewService(&svc.ServiceContext{AuditService: audit.NewAuditService(audit.NewInMemoryAuditStore(), nil)})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/alerts/inbound/generic", nil)

	require.NotPanics(t, func() {
		s.auditInboundRejected(c, "generic", "timestamp_replay")
	})
}
