package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
)

func setupExportDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/audit-export.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, db.AutoMigrate(&audit.AuditModel{}))
	return db
}

func seedAuditRows(t *testing.T, db *gorm.DB, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		require.NoError(t, db.Table("audit_records").Create(map[string]interface{}{
			"audit_id":       fakeAuditID(i),
			"timestamp":      time.Date(2026, 8, 26, 10, i, 0, 0, time.UTC),
			"event_type":     "function.invoke",
			"outcome":        "success",
			"category":       "operation",
			"severity":       "info",
			"chain_hash":     fakeAuditHash(i),
			"chain_sequence": int64(i + 1),
			"created_at":     time.Date(2026, 8, 26, 10, i, 0, 0, time.UTC),
			"actor_id":       "alice",
			"actor_json":     []byte(`{"id":"alice","type":"user"}`),
			"resource_json":  []byte(`{"id":"player.ban"}`),
			"details_json":   []byte(`{"traceId":"t-` + fakeAuditID(i) + `"}`),
			"game_id":        "demo",
			"env":            "prod",
			"ip":             "10.0.0.1",
		}).Error)
	}
}

func fakeAuditID(i int) string {
	return "aud-" + time.Duration(i).String()
}

func fakeAuditHash(i int) string {
	return "hash-" + fakeAuditID(i)
}

func newTestSvcCtxFromDB(db *gorm.DB) *svc.ServiceContext {
	return &svc.ServiceContext{DB: db}
}

func newExportHandler(t *testing.T, db *gorm.DB) (*Handler, *gin.Engine) {
	t.Helper()
	svcCtx := newTestSvcCtxFromDB(db)
	svc := NewService(svcCtx)
	h := NewHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/audit/export", h.Export)
	r.GET("/audit/chain/verify", h.VerifyChain)
	return h, r
}

func TestExport_JSON(t *testing.T) {
	db := setupExportDB(t)
	seedAuditRows(t, db, 3)
	_, r := newExportHandler(t, db)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/audit/export?format=json&actor=alice", nil)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "audit-export-")

	var payload struct {
		Items []AuditItem `json:"items"`
		Count int         `json:"count"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Len(t, payload.Items, 3)
	assert.Equal(t, 3, payload.Count)
	assert.Equal(t, "alice", payload.Items[0].UserID)
}

func TestExport_CSV(t *testing.T) {
	db := setupExportDB(t)
	seedAuditRows(t, db, 2)
	_, r := newExportHandler(t, db)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/audit/export?format=csv", nil)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/csv")
	body := rec.Body.String()
	// BOM + 表头 + 2 行。
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte{0xEF, 0xBB, 0xBF}))
	assert.Contains(t, body, "id,timestamp,action,result")
	assert.Contains(t, body, "function.invoke")
}

func TestExport_Truncation(t *testing.T) {
	db := setupExportDB(t)
	seedAuditRows(t, db, 5)
	svcCtx := newTestSvcCtxFromDB(db)
	svc := NewService(svcCtx)

	items, truncated, err := svc.ExportRows(context.Background(), &AuditRequest{Actor: "alice"}, 3)
	require.NoError(t, err)
	assert.Len(t, items, 3)
	assert.True(t, truncated)
}

func TestExport_ScopeFiltering(t *testing.T) {
	db := setupExportDB(t)
	seedAuditRows(t, db, 2)

	// 无鉴权模型（直连/迁移场景）→ 不限域（与 GetAuditLogs 同语义）。
	svc := NewService(newTestSvcCtxFromDB(db))
	items, truncated, err := svc.ExportRows(context.Background(), &AuditRequest{}, 10)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Len(t, items, 2)

	// 过滤条件生效：不存在的 actor → 空集。
	items, _, err = svc.ExportRows(context.Background(), &AuditRequest{Actor: "nobody"}, 10)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestVerifyChain_Empty(t *testing.T) {
	db := setupExportDB(t)
	auditStore, err := audit.NewSQLAuditStore(db)
	require.NoError(t, err)
	svcCtx := newTestSvcCtxFromDB(db)
	svcCtx.AuditService = audit.NewAuditService(auditStore, nil)

	svc := NewService(svcCtx)
	h := NewHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/audit/chain/verify", h.VerifyChain)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit/chain/verify", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"valid":true`)
	assert.Contains(t, rec.Body.String(), "empty chain")
}

func TestVerifyChain_Break(t *testing.T) {
	db := setupExportDB(t)
	auditStore, err := audit.NewSQLAuditStore(db)
	require.NoError(t, err)
	svcCtx := newTestSvcCtxFromDB(db)
	svcCtx.AuditService = audit.NewAuditService(auditStore, nil)

	// 通过核心服务写入两条合法链接的记录。
	svc1 := svcCtx.AuditService
	_, err = svc1.Log(context.Background(), "function.invoke",
		audit.WithActorID("alice", "user", "alice"),
		audit.WithDetails(map[string]interface{}{"seq": 1}),
	)
	require.NoError(t, err)
	_, err = svc1.Log(context.Background(), "function.invoke",
		audit.WithActorID("alice", "user", "alice"),
		audit.WithDetails(map[string]interface{}{"seq": 2}),
	)
	require.NoError(t, err)

	// 篡改第一条的 hash 制造断链。
	require.NoError(t, db.Exec("UPDATE audit_records SET chain_hash = 'tampered' WHERE chain_sequence = 1").Error)

	result, err := NewService(svcCtx).VerifyChain(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Valid)
	assert.NotZero(t, result.FirstBreakSeq)
}
