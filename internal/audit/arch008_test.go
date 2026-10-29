package audit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// arch008FailingStore 只在 Create 上失败，其余行为透传内存存储——
// 驱动 D6 写失败策略的确定性故障注入。
type arch008FailingStore struct{ AuditStore }

func (f *arch008FailingStore) Create(*AuditRecord) error {
	return errors.New("injected audit store failure")
}

func newFailingService(t *testing.T) *AuditService {
	t.Helper()
	return NewAuditService(&arch008FailingStore{AuditStore: NewInMemoryAuditStore()}, nil)
}

// newArch008Service 起一个走 SQL store（真实 JSON 列往返）的审计服务，
// 并写入一条载荷丰富的 v2 记录，供篡改矩阵与版本分发用例复用。
func newArch008Service(t *testing.T) (*SQLAuditStore, *AuditService) {
	t.Helper()
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	service := NewAuditService(store, nil)

	_, err = service.Log(context.Background(), EventFunctionInvoke,
		WithActorID("op-1", "user", "Operator"),
		WithGameID("demo", "prod"),
		WithResourceID("function", "gift.send"),
		WithChanges(
			map[string]interface{}{"count": 10},
			map[string]interface{}{"count": 20},
		),
		WithDetails(map[string]interface{}{"reason": "ops"}),
		WithContext(AuditContext{RequestID: "req-1", TraceID: "trace-1"}),
		WithOutcome("success", ""),
		WithSeverity(SeverityWarning),
	)
	require.NoError(t, err)
	return store, service
}

// tamperSQL 直接改库列——模拟绕过应用的 DB 层篡改，断言 ValidateChain
// 能检出。这是 ARCH-008「逐字段修改被检测」的验收矩阵。
func tamperSQL(t *testing.T, store *SQLAuditStore, column, value string) {
	t.Helper()
	require.NoError(t, store.db.Exec(
		"UPDATE audit_records SET "+column+" = ? WHERE chain_sequence = 1", value).Error)
}

// TestArch008_FullPayloadTamperDetected：v2 全载荷摘要下，此前不参与
// 哈希的字段（action/scope/changes/context/severity/error_message/actor）
// 任何一处改动都必须断链。
func TestArch008_FullPayloadTamperDetected(t *testing.T) {
	cases := []struct {
		name   string
		column string
		value  string
	}{
		{"action", "action", "tampered.action"},
		{"severity", "severity", "critical"},
		{"error_message", "error_message", "tampered"},
		// scope：resource_json 的 gameId/environment
		{"scope_game", "resource_json", `{"type":"function","id":"gift.send","gameId":"other"}`},
		{"changes", "changes_json", `{"before":{"count":999},"after":{"count":20}}`},
		{"context", "context_json", `{"requestId":"forged"}`},
		{"actor", "actor_json", `{"id":"evil","type":"user"}`},
		{"details", "details_json", `{"reason":"forged"}`},
		{"outcome", "outcome", "failure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, service := newArch008Service(t)
			result, err := service.ValidateChain(1, 1)
			require.NoError(t, err)
			require.True(t, result.Valid, "precondition: chain valid before tamper")

			tamperSQL(t, store, tc.column, tc.value)

			result, err = service.ValidateChain(1, 1)
			require.NoError(t, err)
			assert.False(t, result.Valid, "%s tamper must break the chain", tc.column)
			assert.Equal(t, "hash_mismatch", result.Errors[0].Type)
		})
	}
}

// TestArch008_LegacyV1StillVerifies：v1 行（旧窄字段 djb2 摘要）按冻结的
// legacyHash 复算仍可验；其未覆盖的字段（action）保持旧检出边界不变，
// 覆盖的字段（details）仍可检出。hash_version=0（滚动部署早于列出现的行）
// 按 v1 兜底。
func TestArch008_LegacyV1StillVerifies(t *testing.T) {
	newLegacyRecord := func(id string) *AuditRecord {
		rec := &AuditRecord{
			ID:        id,
			Timestamp: time.Now().UTC().Truncate(time.Millisecond),
			EventType: EventLogin,
			Category:  CategorySecurity,
			Severity:  SeverityInfo,
			Actor:     ActorInfo{ID: "u1", Type: "user"},
			Action:    "login",
			Outcome:   "success",
			Details:   map[string]interface{}{"ip": "10.0.0.1"},
			ChainInfo: ChainInfo{Sequence: 1, HashVersion: HashVersionLegacy},
		}
		hash, err := legacyHash(rec)
		require.NoError(t, err)
		rec.ChainInfo.Hash = hash
		return rec
	}

	t.Run("v1 row verifies", func(t *testing.T) {
		db := newTestDB(t)
		store, err := NewSQLAuditStore(db)
		require.NoError(t, err)
		require.NoError(t, store.Create(newLegacyRecord("legacy-1")))

		service := NewAuditService(store, nil)
		result, err := service.ValidateChain(1, 1)
		require.NoError(t, err)
		assert.True(t, result.Valid, "legacy v1 record must verify, errors: %v", result.Errors)
	})

	t.Run("v1 uncovered field stays uncovered (documented boundary)", func(t *testing.T) {
		db := newTestDB(t)
		store, err := NewSQLAuditStore(db)
		require.NoError(t, err)
		require.NoError(t, store.Create(newLegacyRecord("legacy-2")))
		// v1 摘要从未覆盖 action——篡改 action 在 v1 行上不 断链（与升级前
		// 行为一致）；这是旧记录的既有暴露面，明示而非静默。
		require.NoError(t, db.Exec("UPDATE audit_records SET action = 'forged' WHERE chain_sequence = 1").Error)

		service := NewAuditService(store, nil)
		result, err := service.ValidateChain(1, 1)
		require.NoError(t, err)
		assert.True(t, result.Valid)
	})

	t.Run("v1 covered field still detected", func(t *testing.T) {
		db := newTestDB(t)
		store, err := NewSQLAuditStore(db)
		require.NoError(t, err)
		require.NoError(t, store.Create(newLegacyRecord("legacy-3")))
		require.NoError(t, db.Exec(`UPDATE audit_records SET details_json = '{"ip":"1.2.3.4"}' WHERE chain_sequence = 1`).Error)

		service := NewAuditService(store, nil)
		result, err := service.ValidateChain(1, 1)
		require.NoError(t, err)
		assert.False(t, result.Valid, "v1 covered-field tamper must still break the chain")
	})

	t.Run("zero version treated as legacy", func(t *testing.T) {
		db := newTestDB(t)
		store, err := NewSQLAuditStore(db)
		require.NoError(t, err)
		rec := newLegacyRecord("legacy-0")
		rec.ChainInfo.HashVersion = 0
		require.NoError(t, store.Create(rec))

		service := NewAuditService(store, nil)
		result, err := service.ValidateChain(1, 1)
		require.NoError(t, err)
		assert.True(t, result.Valid, "hash_version=0 rows must verify under the legacy digest")
	})
}

// TestArch008_MixedVersionChain：v1 链尾上接 v2 记录，两段按各自版本
// 复算、链接完整——「旧链可验证」的混合形态验收。
func TestArch008_MixedVersionChain(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)

	legacy := &AuditRecord{
		ID:        "legacy-head",
		Timestamp: time.Now().UTC().Truncate(time.Millisecond),
		EventType: EventLogin,
		Category:  CategorySecurity,
		Severity:  SeverityInfo,
		Actor:     ActorInfo{ID: "u1", Type: "user"},
		Action:    "login",
		Outcome:   "success",
		ChainInfo: ChainInfo{Sequence: 1, HashVersion: HashVersionLegacy},
	}
	hash, err := legacyHash(legacy)
	require.NoError(t, err)
	legacy.ChainInfo.Hash = hash
	require.NoError(t, store.Create(legacy))

	service := NewAuditService(store, nil)
	rec, err := service.Log(context.Background(), EventPagePublish, WithActorID("op-1", "user", "Op"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), rec.ChainInfo.Sequence, "v2 record must link onto the v1 chain tail")
	assert.Equal(t, HashVersionCurrent, rec.ChainInfo.HashVersion)
	assert.Equal(t, hash, rec.ChainInfo.PrevHash, "prevHash must be the v1 tail hash")

	result, err := service.ValidateChain(1, 2)
	require.NoError(t, err)
	assert.True(t, result.Valid, "mixed-version chain must validate, errors: %v", result.Errors)
}

// TestArch008_LargeIntegerRoundTrip：details 含 >2^53 整数时，写时
// （int64）与读时（UseNumber 解码）的规范化字节必须一致——旧 float64
// 解码会在此产生完好链条的假 hash_mismatch（ARCH-007 D1 修复点）。
func TestArch008_LargeIntegerRoundTrip(t *testing.T) {
	db := newTestDB(t)
	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	service := NewAuditService(store, nil)

	_, err = service.Log(context.Background(), EventDataExport,
		WithActorID("op-1", "user", "Op"),
		WithDetails(map[string]interface{}{
			"bigId":     int64(9007199254740993), // 2^53+1，float64 必丢精度
			"orderNo":   int64(12345678901234567),
			"ratio":     0.1,
			"plainText": "1", // 数字形字符串不受影响
		}),
	)
	require.NoError(t, err)

	result, err := service.ValidateChain(1, 1)
	require.NoError(t, err)
	assert.True(t, result.Valid, "large-integer details must round-trip without false mismatch, errors: %v", result.Errors)
}

// TestArch008_V2DigestIsSha256FullPayload：新记录摘要为 64 位 hex
// （SHA-256），且与 legacyHash 显著不同（非 8 位 djb2）。
func TestArch008_V2DigestIsSha256FullPayload(t *testing.T) {
	store, _ := newArch008Service(t)
	records, err := store.GetChainRange(1, 1)
	require.NoError(t, err)
	require.Len(t, records, 1)
	hash := records[0].ChainInfo.Hash
	assert.Len(t, hash, 64, "v2 digest must be SHA-256 hex")
	assert.Equal(t, HashVersionCurrent, records[0].ChainInfo.HashVersion)
}

// --- ARCH-007 D6 写失败策略 ---

// TestArch008_LogOrDegrade_FailOpenProceeds：非白名单事件写失败时
// 失败开放——返回 (nil, nil)（操作照常进行），计数器 +1，请求标记降级。
func TestArch008_LogOrDegrade_FailOpenProceeds(t *testing.T) {
	service := newFailingService(t)
	ctx := WithDegradedFlag(context.Background())

	record, err := service.LogOrDegrade(ctx, EventFunctionInvoke, WithActorID("op-1", "user", "Op"))
	require.NoError(t, err, "fail-open events must proceed; failure is surfaced, not propagated")
	assert.Nil(t, record)
	assert.Equal(t, int64(1), service.WriteFailures(), "write failure must be counted")
	assert.True(t, WasDegraded(ctx), "request must be marked degraded for the response header")
}

// TestArch008_LogOrDegrade_FailClosedAborts：白名单事件写失败时失败
// 关闭——返回 ErrAuditWriteFailed，调用方据此中止操作。
func TestArch008_LogOrDegrade_FailClosedAborts(t *testing.T) {
	for _, event := range []AuditEventType{
		EventUserDelete, EventUserLock, EventUserUnlock, EventPasswordChange,
		EventMFAEnabled, EventMFADisabled, EventBackupRestore,
	} {
		t.Run(string(event), func(t *testing.T) {
			service := newFailingService(t)
			ctx := WithDegradedFlag(context.Background())

			record, err := service.LogOrDegrade(ctx, event)
			assert.Nil(t, record)
			assert.ErrorIs(t, err, ErrAuditWriteFailed, "allowlist events must abort when the audit store is down")
		})
	}
}

// TestArch008_LogOrDegrade_SuccessPassthrough：存储健康时策略层透传。
func TestArch008_LogOrDegrade_SuccessPassthrough(t *testing.T) {
	store, service := newArch008Service(t)
	ctx := WithDegradedFlag(context.Background())

	record, err := service.LogOrDegrade(ctx, EventFunctionInvoke, WithActorID("op-2", "user", "Op"))
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, int64(0), service.WriteFailures())
	assert.False(t, WasDegraded(ctx))
	_, err = store.Get(record.ID)
	require.NoError(t, err)
}

// TestArch008_DegradedMiddleware_Header：中间件把降级标志带到响应头；
// 无降级时不写头。
func TestArch008_DegradedMiddleware_Header(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("degraded writes header", func(t *testing.T) {
		service := newFailingService(t)
		r := gin.New()
		r.Use(DegradedMiddleware())
		r.GET("/op", func(c *gin.Context) {
			_, err := service.LogOrDegrade(c.Request.Context(), EventPageExecute)
			require.NoError(t, err)
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/op", nil))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "true", w.Header().Get(DegradedHeader))
	})

	t.Run("clean request has no header", func(t *testing.T) {
		store, service := newArch008Service(t)
		_ = store
		r := gin.New()
		r.Use(DegradedMiddleware())
		r.GET("/op", func(c *gin.Context) {
			_, err := service.LogOrDegrade(c.Request.Context(), EventPageExecute)
			require.NoError(t, err)
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/op", nil))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Header().Get(DegradedHeader))
	})

	t.Run("non-http context is safe no-op", func(t *testing.T) {
		service := newFailingService(t)
		// 无标志植入（context.Background）：markDegraded 安全 no-op，
		// 计数与策略照常。
		_, err := service.LogOrDegrade(context.Background(), EventPageExecute)
		require.NoError(t, err)
		assert.Equal(t, int64(1), service.WriteFailures())
		assert.False(t, WasDegraded(context.Background()))
	})
}
