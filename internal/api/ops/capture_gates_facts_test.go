package ops

// capture_gates.go service/handler 错误面补测：五个 handler 的 service 错误
// 分支（lint 后语义不合法、config_versions 表故障的 Get/Put/Rollback/Versions
// 透传）、captureGatesAuthor 的 nil ctx 兜底、captureAgentFacts 的 scope/
// 角色/过期/双 contributor 交集分支、parseCapabilityLabel 的空键与能力键。
//
// 复用 capture_gates_test.go 的 validGatesContent；DB 故障面用本地 fixture
// （暴露 db 句柄以便 drop 表模拟非 NotFound 错误）。

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// captureGatesSvcDB 同 newCaptureGatesService 但暴露 db（drop 表模拟故障）。
func captureGatesSvcDB(t *testing.T) (*Service, context.Context, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/cg_extra.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ConfigVersion{}))
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	return NewService(&svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}), ctx, db
}

func TestCaptureGatesErrorPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s, _, db := captureGatesSvcDB(t)
	h := NewHandler(s)

	// lint 通过但 Gates 反序列化失败（数组元素类型错）→ validate 422
	c, w := withScopeRequest(t, http.MethodPost, "/ops/capture/gates/validate",
		`{"content":"{\"reconcile\":{\"rules\":[42]}}"}`)
	h.CaptureGatesValidate(c)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Body.String(), "parse gates json")

	// publish 同型内容 → 422（同一真值）
	c, w = withScopeRequest(t, http.MethodPut, "/ops/capture/gates",
		`{"content":"{\"reconcile\":{\"rules\":[42]}}"}`)
	h.CaptureGatesPut(c)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Body.String(), "parse gates json")

	// config_versions 表缺失 → 非 NotFound 错误逐端点透传
	require.NoError(t, db.Migrator().DropTable(&model.ConfigVersion{}))

	// Get：FindLatestByScope 错误 → 500
	c, w = withScopeRequest(t, http.MethodGet, "/ops/capture/gates", "")
	h.CaptureGatesGet(c)
	require.Equal(t, http.StatusInternalServerError, w.Code)

	// Put：CreateWithMeta 错误 → 500
	c, w = withScopeRequest(t, http.MethodPut, "/ops/capture/gates",
		`{"content":"{\"reconcile\":{\"rules\":[]}}"}`)
	h.CaptureGatesPut(c)
	require.Equal(t, http.StatusInternalServerError, w.Code)

	// Rollback：FindByScope 错误 → 500
	c, w = withScopeRequest(t, http.MethodPost, "/ops/capture/gates/rollback", `{"version":1}`)
	h.CaptureGatesRollback(c)
	require.Equal(t, http.StatusInternalServerError, w.Code)

	// Versions：ListByScope 错误 → 500
	c, w = withScopeRequest(t, http.MethodGet, "/ops/capture/gates/versions", "")
	h.CaptureGatesVersionsGet(c)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCaptureGatesAuthorNilContext(t *testing.T) {
	require.Equal(t, "system", captureGatesAuthor(nil))
}

func TestParseCapabilityLabel(t *testing.T) {
	caps, err := parseCapabilityLabel("beforeImage")
	require.NoError(t, err)
	require.True(t, caps.BeforeImage)
	require.False(t, caps.AfterImage)

	caps, err = parseCapabilityLabel("snapshot, pushdown")
	require.NoError(t, err)
	require.True(t, caps.Snapshot)
	require.True(t, caps.Pushdown)

	// 未知能力键拒绝
	_, err = parseCapabilityLabel("beforeImage,warp-drive")
	require.ErrorContains(t, err, "unknown connector capability")
}

func TestCaptureAgentFactsScopeAndIntersect(t *testing.T) {
	store := registry.NewStore()
	upsert := func(id, gameID, env, role, tables, caps string, expire time.Time) {
		t.Helper()
		labels := map[string]string{}
		if role != "" {
			labels[labelRole] = role
		}
		if tables != "" {
			labels[labelCaptureTables] = tables
		}
		if caps != "" {
			labels[labelCaptureConnectorCapabilities] = caps
		}
		require.NoError(t, store.UpsertAgent(&registry.AgentSession{
			AgentID: id, GameID: gameID, Env: env, Labels: labels, ExpireAt: expire,
		}))
	}
	fresh := time.Now().Add(time.Hour)

	// 其他 scope（402 continue）/ 非 capture 角色、过期会话（405 continue）
	upsert("a-other", "other", "prod", labelRoleCapture, "game.items", "beforeImage", fresh)
	upsert("a-idle", "demo", "prod", "agent", "game.items", "beforeImage", fresh)
	upsert("a-expired", "demo", "prod", labelRoleCapture, "game.items", "beforeImage", time.Now().Add(-time.Hour))

	// 两个有效 capture agent：tables 并集、capabilities 交集（从严）
	// 尾随逗号覆盖空键跳过分支
	upsert("a-cap1", "demo", "prod", labelRoleCapture, "game.items, game.ledger",
		"beforeImage,afterImage,snapshot,pushdown,", fresh)
	upsert("a-cap2", "demo", "prod", labelRoleCapture, "game.items",
		"beforeImage,snapshot", fresh)

	s := NewService(&svc.ServiceContext{RegistryStore: store})
	facts := s.captureAgentFacts(svc.GameScope{GameID: "demo", Env: "prod"})
	require.Equal(t, 2, facts.contributors)
	require.True(t, facts.tables["game.items"])
	require.True(t, facts.tables["game.ledger"], "tables label must be a union across agents")
	require.True(t, facts.capabilities.BeforeImage)
	require.True(t, facts.capabilities.Snapshot)
	require.False(t, facts.capabilities.AfterImage, "weakest connector must win the intersection")
	require.False(t, facts.capabilities.Pushdown, "weakest connector must win the intersection")
}
