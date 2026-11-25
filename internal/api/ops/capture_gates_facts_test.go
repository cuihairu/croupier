package ops

// Extra error-surface coverage for capture_gates.go service/handler: the service
// error branches of the five handlers (semantically invalid after lint, and
// passthrough of Get/Put/Rollback/Versions when the config_versions table
// fails), captureGatesAuthor's nil-ctx fallback, captureAgentFacts' scope/
// role/expiry/two-contributor intersection branches, and
// parseCapabilityLabel's empty-key and capability keys.
//
// Reuses validGatesContent from capture_gates_test.go; the DB failure surface
// uses a local fixture (exposes the db handle so the table can be dropped to
// simulate a non-NotFound error).

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

// captureGatesSvcDB is like newCaptureGatesService but exposes db (so the table
// can be dropped to simulate failures).
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

	// Lint passes but Gates unmarshalling fails (wrong array element type) ->
	// validate 422
	c, w := withScopeRequest(t, http.MethodPost, "/ops/capture/gates/validate",
		`{"content":"{\"reconcile\":{\"rules\":[42]}}"}`)
	h.CaptureGatesValidate(c)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Body.String(), "parse gates json")

	// Publish with the same content shape -> 422 (same truth)
	c, w = withScopeRequest(t, http.MethodPut, "/ops/capture/gates",
		`{"content":"{\"reconcile\":{\"rules\":[42]}}"}`)
	h.CaptureGatesPut(c)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Body.String(), "parse gates json")

	// config_versions table missing -> the non-NotFound error passes through on
	// every endpoint
	require.NoError(t, db.Migrator().DropTable(&model.ConfigVersion{}))

	// Get: FindLatestByScope error -> 500
	c, w = withScopeRequest(t, http.MethodGet, "/ops/capture/gates", "")
	h.CaptureGatesGet(c)
	require.Equal(t, http.StatusInternalServerError, w.Code)

	// Put: CreateWithMeta error -> 500
	c, w = withScopeRequest(t, http.MethodPut, "/ops/capture/gates",
		`{"content":"{\"reconcile\":{\"rules\":[]}}"}`)
	h.CaptureGatesPut(c)
	require.Equal(t, http.StatusInternalServerError, w.Code)

	// Rollback: FindByScope error -> 500
	c, w = withScopeRequest(t, http.MethodPost, "/ops/capture/gates/rollback", `{"version":1}`)
	h.CaptureGatesRollback(c)
	require.Equal(t, http.StatusInternalServerError, w.Code)

	// Versions: ListByScope error -> 500
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

	// Unknown capability key is rejected
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

	// Other scope (continue at 402) / non-capture role, expired session
	// (continue at 405)
	upsert("a-other", "other", "prod", labelRoleCapture, "game.items", "beforeImage", fresh)
	upsert("a-idle", "demo", "prod", "agent", "game.items", "beforeImage", fresh)
	upsert("a-expired", "demo", "prod", labelRoleCapture, "game.items", "beforeImage", time.Now().Add(-time.Hour))

	// Two valid capture agents: tables union, capabilities intersection (strict)
	// The trailing comma covers the empty-key skip branch
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
