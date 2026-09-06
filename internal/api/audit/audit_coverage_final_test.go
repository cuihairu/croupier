// 补齐 audit 包剩余可覆盖分支：Export 截断响应头、VerifyChain 非空合法链、
// GetAuditLogs 的 Scan 失败与 LoadCurrentAdmin 二次加载失败。
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	auditcore "github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAuditFinalDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("audit_final_%s", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, db.AutoMigrate(&auditcore.AuditModel{}))
	return db
}

// bulkSeedAuditRows 批量写入 n 行审计记录（内联 INSERT，避免逐行 Create 过慢）。
func bulkSeedAuditRows(t *testing.T, db *gorm.DB, n int) {
	t.Helper()
	const batchSize = 500
	base := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	for start := 0; start < n; start += batchSize {
		end := start + batchSize
		if end > n {
			end = n
		}
		var sb strings.Builder
		sb.WriteString("INSERT INTO audit_records (audit_id, timestamp, event_type, outcome, category, severity, chain_hash, chain_sequence, created_at, actor_id, actor_json, resource_json, details_json, game_id, env, ip) VALUES ")
		for i := start; i < end; i++ {
			if i > start {
				sb.WriteString(",")
			}
			ts := base.Add(time.Duration(i) * time.Second)
			fmt.Fprintf(&sb, "('bulk-%d', '%s', 'function.invoke', 'success', 'operation', 'info', 'hash-%d', %d, '%s', 'alice', '{}', '{}', '{}', 'demo', 'prod', '10.0.0.1')",
				i, ts.UTC().Format("2006-01-02 15:04:05.000"), i, i+1, ts.UTC().Format("2006-01-02 15:04:05.000"))
		}
		require.NoError(t, db.Exec(sb.String()).Error)
	}
}

// Export 行数超过 exportRowLimit → 响应携带 X-Truncated: true。
func TestHandler_Export_TruncatedHeader(t *testing.T) {
	db := newAuditFinalDB(t)
	bulkSeedAuditRows(t, db, exportRowLimit+1)

	svcCtx := &svc.ServiceContext{DB: db}
	h := NewHandler(NewService(svcCtx))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/audit/export", h.Export)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audit/export?format=json", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "true", rec.Header().Get("X-Truncated"))

	var payload struct {
		Items     []AuditItem `json:"items"`
		Truncated bool        `json:"truncated"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Len(t, payload.Items, exportRowLimit)
	assert.True(t, payload.Truncated)
}

// VerifyChain：非空且完整合法的链 → valid=true 且返回 checked 数。
func TestVerifyChain_NonEmptyValid(t *testing.T) {
	db := newAuditFinalDB(t)
	auditStore, err := auditcore.NewSQLAuditStore(db)
	require.NoError(t, err)
	svcCtx := &svc.ServiceContext{DB: db, AuditService: auditcore.NewAuditService(auditStore, nil)}

	for i := 0; i < 2; i++ {
		_, err := svcCtx.AuditService.Log(context.Background(), "function.invoke",
			auditcore.WithActorID("alice", "user", "alice"),
			auditcore.WithDetails(map[string]interface{}{"seq": i + 1}),
		)
		require.NoError(t, err)
	}

	result, err := NewService(svcCtx).VerifyChain(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, int64(2), result.Checked)
	assert.Empty(t, result.Message)
}

// GetAuditLogs：Count 成功后行 Scan 失败（Row 处理器注入错误）。
func TestGetAuditLogs_ScanError(t *testing.T) {
	db := newAuditFinalDB(t)
	seedAuditEntryForFinal(t, db, "auth.login", "u1", "g1", "prod")

	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("test/fail_audit_row_scan", func(tx *gorm.DB) {
		_ = tx.AddError(fmt.Errorf("forced scan failure"))
	}))
	t.Cleanup(func() { db.Callback().Row().Remove("test/fail_audit_row_scan") })

	svc := NewService(&svc.ServiceContext{DB: db})
	_, err := svc.GetAuditLogs(context.Background(), &AuditRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced scan failure")
}

// GetAuditLogs：权限校验通过后，第二次 LoadCurrentAdmin（admins 查询）失败。
func TestGetAuditLogs_SecondAdminLoadFails(t *testing.T) {
	db := newAuditFinalDB(t)
	_, username := seedAuditViewer(t, db, "auditor")

	adminQueries := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test/fail_second_admin_load", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*model.Admin); ok {
			adminQueries++
			if adminQueries == 2 {
				_ = tx.AddError(fmt.Errorf("forced admin load failure"))
			}
		}
	}))
	t.Cleanup(func() { db.Callback().Query().Remove("test/fail_second_admin_load") })

	svc := NewService(&svc.ServiceContext{
		DB: db, AdminModel: model.NewAdminModel(db), RoleModel: model.NewRoleModel(db),
		GameModel: model.NewGameModel(db),
	})
	_, err := svc.GetAuditLogs(context.WithValue(context.Background(), "username", username), &AuditRequest{})
	require.Error(t, err)
	// LoadCurrentAdmin 将底层查询错误包装为「查询管理员失败」
	assert.Contains(t, err.Error(), "查询管理员失败")
}

func seedAuditEntryForFinal(t *testing.T, db *gorm.DB, eventType, actor, gameID, env string) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, db.Table("audit_records").Create(map[string]interface{}{
		"audit_id":       "fin-1",
		"timestamp":      now,
		"event_type":     eventType,
		"outcome":        "success",
		"category":       "operation",
		"severity":       "info",
		"chain_hash":     "fin-hash-1",
		"chain_sequence": int64(1),
		"created_at":     now,
		"actor_id":       actor,
		"actor_json":     []byte(`{"id":"` + actor + `","type":"user"}`),
		"resource_json":  []byte(`{}`),
		"details_json":   []byte(`{}`),
		"game_id":        gameID,
		"env":            env,
		"ip":             "10.0.0.1",
	}).Error)
}
