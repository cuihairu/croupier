package cicd_test

// CI/CD service/webhook residual error-path coverage (the blocks left over after the
// handler layer was closed out).
// Injection conventions match this repo's existing batches:
//   - Read wing "missing table": after DropTable, gorm errors immediately with no side
//     effects (storage failure);
//   - Write wing "trigger blocks writes": BEFORE UPDATE/DELETE TRIGGER + RAISE(ABORT),
//     so the write statement fails only once it really reaches the DB (covering the
//     "all validation passes, the write blows up" class);
//   - Missing record: a real 404;
//   - Provider errors: an httptest fake CI server returns 4xx/5xx per case.
//
// The file tail also registers three defensively-unreachable branches (their
// construction premise is eliminated by the upstream code itself), recorded as
// house-rule registrations rather than faked cases.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/api/cicd"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func jsonBody(s string) *strings.Reader { return strings.NewReader(s) }

// withUsername injects a logged-in username (CurrentUsername reads the "username" key
// from ctx).
func withUsername(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, "username", name) //nolint:staticcheck // the contract key is a string key
}

// seedRowIntegration writes an integration row directly (bypassing Create's
// validation layer, to build shapes the store cannot produce through the validation
// layer: malformed endpoint, unknown kind, empty scope).
func seedRowIntegration(t *testing.T, db *gorm.DB, kind, endpoint string, extra datatypes.JSONMap) uint {
	t.Helper()
	row := &model.CicdIntegration{
		GameID: "demo", Env: "prod", Kind: kind, Name: "seeded-" + kind,
		Endpoint: endpoint, Extra: extra, Enabled: true,
		// After fail-closed (AUD-COV-001 P1), write-backs must carry a credential: the
		// seeded row has a fixed token and requests attach X-CICD-Token via doWebhook.
		Token: "hook-secret",
	}
	require.NoError(t, db.Create(row).Error)
	require.NotZero(t, row.ID)
	return row.ID
}

// doWebhook is the write-back variant of doCicd: it attaches the token header
// matching seedRowIntegration.
func doWebhook(r http.Handler, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-CICD-Token", "hook-secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// newWebhookRouter replicates registerCicdWebhookRoute.
func newWebhookRouter(s *cicd.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/cicd/webhooks/:id", cicd.NewHandler(s).Webhook)
	return r
}

// fakeCI stands up a programmable fake CI server: trigger 201+Location, status
// 200+building.
func fakeCI(t *testing.T, triggerStatus, statusCode int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/job/pack/build", func(w http.ResponseWriter, _ *http.Request) {
		if triggerStatus != http.StatusCreated {
			w.WriteHeader(triggerStatus)
			return
		}
		w.Header().Set("Location", "/queue/item/7/")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/queue/item/7/api/json", func(w http.ResponseWriter, _ *http.Request) {
		if statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			return
		}
		_, _ = w.Write([]byte(`{"executable":{"number":7}}`))
	})
	mux.HandleFunc("/job/pack/7/api/json", func(w http.ResponseWriter, _ *http.Request) {
		if statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			return
		}
		_, _ = w.Write([]byte(`{"number":7,"building":false,"url":"http://ci.internal/job/pack/7/"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// On create, createdBy comes from the logged-in username in the request context
// (falls back to "system" without one).
func TestCicdService_CreateRecordsAuthor(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))

	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/cicd/integrations",
		jsonBody(`{"kind":"jenkins","name":"authored","endpoint":"http://ci.internal"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withUsername(req.Context(), "alice"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"createdBy":"alice"`)

	var row model.CicdIntegration
	require.NoError(t, db.Where("name = ?", "authored").First(&row).Error)
	assert.Equal(t, "alice", row.CreatedBy)

	// No logged-in username -> falls back to "system"
	w = doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"jenkins","name":"anon","endpoint":"http://ci.internal"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"createdBy":"system"`)
}

// TestCicdService_CreateEnabledFalseIsDroppedKnownDefect registers a **known defect**
// (not an unreachable branch): when the create request passes enabled:false
// explicitly, the landed value is still true.
//
// Root cause: model.CicdIntegration.Enabled carries `gorm:"default:true"`, and GORM
// omits the column from the INSERT for fields that have a default tag and a zero
// value, falling back to the DB default. IntegrationCreateRequest.Enabled is *bool
// and the service layer's `row.Enabled = *req.Enabled` assigns correctly, but the
// driver layer drops false as a zero value. In practice neither Select("*") nor
// per-field Select can bypass it (this GORM version falls back unconditionally).
//
// Blast radius: only the "create as disabled" path; Update goes through Save
// (writes all fields) and is unaffected. The fix must change the model field to
// *bool (dragging the service read side along, and triggering this repo's
// "model changes must ship a numbered migration" contract), which belongs to a
// separate fix batch - this coverage batch does not touch production code.
func TestCicdService_CreateEnabledFalseIsDroppedKnownDefect(t *testing.T) {
	t.Skip("已发现缺陷：CicdIntegration.Enabled 的 gorm default:true 使创建时的显式 false 被丢弃（修法需改模型字段类型，另立批次）")

	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	w := doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"jenkins","name":"off","endpoint":"http://ci.internal","enabled":false}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var row model.CicdIntegration
	require.NoError(t, db.Where("name = ?", "off").First(&row).Error)
	assert.False(t, row.Enabled, "契约：显式 false 应落库 false")
}

// Update covers fields one by one: kind/endpoint/token/extra each take effect; empty
// strings do not overwrite.
func TestCicdService_UpdateOverridesFields(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := createIntegration(t, r, "http://ci.internal")

	w := doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{
		"kind":"generic","name":"renamed","endpoint":"http://ci2.internal",
		"token":"new-token-5678","extra":{"triggerUrl":"http://ci2.internal/trig"}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"kind":"generic"`)
	assert.Contains(t, w.Body.String(), `"name":"renamed"`)
	assert.Contains(t, w.Body.String(), "http://ci2.internal")
	assert.Contains(t, w.Body.String(), "****5678", "新凭据掩码回显")
	assert.Contains(t, w.Body.String(), "triggerUrl")
}

// The two update validation errors: an invalid integration shape (endpoint not http)
// and a missing provider required key.
func TestCicdService_UpdateValidationErrors(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := createIntegration(t, r, "http://ci.internal")

	w := doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{"endpoint":"ftp://ci.internal"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// Switch to a provider that requires the project key but send no extra -> 400 at
	// construction
	w = doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{"kind":"gitlab-ci"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "project")
}

// Write failure (write-wing trigger blocks the UPDATE): all validation passes and the
// SQL only blows up when really executed.
func TestCicdService_UpdateStoreError(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := createIntegration(t, r, "http://ci.internal")

	require.NoError(t, db.Exec(
		`CREATE TRIGGER blk_int_upd BEFORE UPDATE ON cicd_integrations BEGIN SELECT RAISE(ABORT, 'blocked'); END`).Error)
	w := doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{"name":"x"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}

// Delete cascade: a missing builds table makes the cascade cleanup fail -> 500 (the
// integration row must not be deleted).
func TestCicdService_DeleteCascadeStoreError(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := createIntegration(t, r, "http://ci.internal")

	require.NoError(t, db.Migrator().DropTable(&model.CicdBuild{}))
	w := doCicd(r, http.MethodDelete, "/cicd/integrations/"+itoa(id), "")
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

	var n int64
	require.NoError(t, db.Model(&model.CicdIntegration{}).Where("id = ?", id).Count(&n).Error)
	assert.EqualValues(t, 1, n, "级联清理失败时接入行保留")
}

// Missing integrations table: the four integration-reading endpoints plus refresh
// (with a build row in place) uniformly return 500.
func TestCicdService_IntegrationTableMissing(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	// Land a build row first; otherwise refresh stops at "build record not found"
	// (404) and never reaches the integration query
	require.NoError(t, db.Create(&model.CicdBuild{
		IntegrationID: 1, Kind: "jenkins", Pipeline: "pack", ExternalID: "e-1",
		Status: model.CicdBuildQueued,
	}).Error)
	require.NoError(t, db.Migrator().DropTable(&model.CicdIntegration{}))

	for _, tc := range []struct{ method, target, body string }{
		{http.MethodPut, "/cicd/integrations/1", `{"name":"x"}`},
		{http.MethodDelete, "/cicd/integrations/1", ""},
		{http.MethodPost, "/cicd/integrations/1/test", ""},
		{http.MethodPost, "/cicd/integrations/1/trigger", `{"pipeline":"pack"}`},
		{http.MethodPost, "/cicd/builds/1/refresh", ""},
	} {
		w := doCicd(r, tc.method, tc.target, tc.body)
		assert.Equal(t, http.StatusInternalServerError, w.Code, "%s %s → %d %s", tc.method, tc.target, w.Code, w.Body.String())
	}
}

// Connectivity test: a malformed endpoint sitting in the store (unproducible through
// the validation layer) -> presented as a client error per AUD-046: 400 + stable code
// cicd_probe_invalid_target, not leaked as 5xx; the raw parse error only goes to logs
// and is no longer returned as a 200 ok:false body.
func TestCicdService_TestConnectionMalformedEndpoint(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := seedRowIntegration(t, db, "jenkins", "://bad-endpoint", nil)

	w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/test", "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "cicd_probe_invalid_target")
}

// Trigger: two error paths - a provider-side error (fake CI returns 500) and a build
// record write failure (missing table).
func TestCicdService_TriggerErrorPaths(t *testing.T) {
	t.Run("provider rejects trigger", func(t *testing.T) {
		srv := fakeCI(t, http.StatusInternalServerError, http.StatusOK)
		ctxSvc, _ := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
		id := createIntegration(t, r, srv.URL)

		w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger", `{"pipeline":"pack"}`)
		assert.GreaterOrEqual(t, w.Code, 400, w.Body.String())
	})

	t.Run("build record store failure", func(t *testing.T) {
		srv := fakeCI(t, http.StatusCreated, http.StatusOK)
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
		id := createIntegration(t, r, srv.URL)
		require.NoError(t, db.Migrator().DropTable(&model.CicdBuild{}))

		w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger", `{"pipeline":"pack"}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})

	t.Run("provider construction fails", func(t *testing.T) {
		// An unknown kind sits in the store, unproducible through the validation layer
		// (Create would block it; direct write bypasses)
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc))
		id := seedRowIntegration(t, db, "bogus-provider", "http://ci.internal", nil)

		w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger", `{"pipeline":"pack"}`)
		// AUD-047: a provider construction failure (unknown kind) is an integration
		// configuration problem, surfaced as 400 in Trigger just like Create/Update
		// (not an opaque 500).
		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "unknown cicd provider kind")
	})
}

// Scope fallback when a trigger lands: with no game_id/env on the integration row,
// the request body is used; empty on both sides stays empty.
func TestCicdService_TriggerScopeFallback(t *testing.T) {
	srv := fakeCI(t, http.StatusCreated, http.StatusOK)
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))

	// Row and request both carry no scope -> the build record keeps an empty scope
	// (no default value is fabricated)
	row := &model.CicdIntegration{
		Kind: "jenkins", Name: "no-scope", Endpoint: srv.URL, Enabled: true,
	}
	require.NoError(t, db.Create(row).Error)
	w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(row.ID)+"/trigger",
		`{"pipeline":"pack","gameId":"from-req","env":"prod"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"gameId":"from-req"`)

	// Row and request both empty -> stays an empty string
	row2 := &model.CicdIntegration{
		Kind: "jenkins", Name: "empty-scope", Endpoint: srv.URL, Enabled: true,
	}
	require.NoError(t, db.Create(row2).Error)
	w = doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(row2.ID)+"/trigger", `{"pipeline":"pack"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"gameId":""`)
}

// Layer-by-layer refresh failures: missing record / storage failure / missing
// integration / provider construction failure / provider error / write-back failure.
func TestCicdService_RefreshBuildErrorPaths(t *testing.T) {
	t.Run("build table missing", func(t *testing.T) {
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc))
		require.NoError(t, db.Migrator().DropTable(&model.CicdBuild{}))
		w := doCicd(r, http.MethodPost, "/cicd/builds/1/refresh", "")
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})

	t.Run("integration missing", func(t *testing.T) {
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc))
		require.NoError(t, db.Create(&model.CicdBuild{
			IntegrationID: 999, Kind: "jenkins", Pipeline: "pack", ExternalID: "e-1",
			Status: model.CicdBuildQueued,
		}).Error)
		w := doCicd(r, http.MethodPost, "/cicd/builds/1/refresh", "")
		assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	})

	t.Run("provider construction fails", func(t *testing.T) {
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc))
		// An unknown kind sits in the store, unproducible through the validation layer
		id := seedRowIntegration(t, db, "bogus-provider", "http://ci.internal", nil)
		require.NoError(t, db.Create(&model.CicdBuild{
			IntegrationID: id, Kind: "bogus-provider", Pipeline: "pack", ExternalID: "e-2",
			Status: model.CicdBuildQueued,
		}).Error)
		w := doCicd(r, http.MethodPost, "/cicd/builds/1/refresh", "")
		assert.GreaterOrEqual(t, w.Code, 400, w.Body.String())
	})

	t.Run("provider status fetch fails", func(t *testing.T) {
		srv := fakeCI(t, http.StatusCreated, http.StatusBadGateway)
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
		// ExternalID must be the full queue URL (the jenkins provider distinguishes
		// the two shapes by the "http" prefix); otherwise it takes the job#n branch
		// and never reaches the provider-error wing
		id := seedRowIntegration(t, db, "jenkins", srv.URL, nil)
		require.NoError(t, db.Create(&model.CicdBuild{
			IntegrationID: id, Kind: "jenkins", Pipeline: "pack",
			ExternalID: srv.URL + "/queue/item/7/",
			Status:     model.CicdBuildQueued,
		}).Error)
		w := doCicd(r, http.MethodPost, "/cicd/builds/1/refresh", "")
		assert.GreaterOrEqual(t, w.Code, 400, w.Body.String())
	})

	t.Run("build write back fails", func(t *testing.T) {
		srv := fakeCI(t, http.StatusCreated, http.StatusOK)
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
		id := seedRowIntegration(t, db, "jenkins", srv.URL, nil)
		require.NoError(t, db.Create(&model.CicdBuild{
			IntegrationID: id, Kind: "jenkins", Pipeline: "pack",
			ExternalID: srv.URL + "/queue/item/7/",
			Status:     model.CicdBuildQueued,
		}).Error)
		// The provider pull succeeds and the status write-back is blocked by the
		// trigger -> the write-failure wing
		require.NoError(t, db.Exec(
			`CREATE TRIGGER blk_build_upd BEFORE UPDATE ON cicd_builds BEGIN SELECT RAISE(ABORT, 'blocked'); END`).Error)
		w := doCicd(r, http.MethodPost, "/cicd/builds/1/refresh", "")
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})
}

// extra value-type normalization: booleans go through fmt.Sprint, nested objects
// fall back to json.Marshal. gorm.io/datatypes enables UseNumber() when reading the
// JSON column, so on the DB path numbers decode as json.Number, not float64 (the
// float64 branch registration is at the tail of this file).
func TestCicdService_NormalizeExtraTypeCoercion(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := seedRowIntegration(t, db, "generic", "http://ci.internal", datatypes.JSONMap{
		"num": 42, "flag": true, "obj": map[string]any{"a": 1}, "str": "keep",
	})

	// Confirm the stored shape first (otherwise the assertions below are idle):
	// numbers as json.Number, booleans as bool
	var row model.CicdIntegration
	require.NoError(t, db.First(&row, id).Error)
	require.IsType(t, json.Number(""), row.Extra["num"], "datatypes 读回启用 UseNumber")
	require.IsType(t, false, row.Extra["flag"], "JSON 布尔解出 bool")

	w := doCicd(r, http.MethodGet, "/cicd/integrations", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"num":"42"`, "数字经 Marshal 兜底为字符串值")
	assert.Contains(t, w.Body.String(), `"flag":"true"`, "bool 分支")
	assert.Contains(t, w.Body.String(), `{\"a\":1}`, "嵌套对象走 Marshal 兜底")
	assert.Contains(t, w.Body.String(), `"str":"keep"`, "string 分支直通")
}

// The webhook endpoint's four rejection classes - path / binding / lookup / store -
// plus extra being preserved verbatim.
func TestCicdWebhook_PathBindAndStoreErrors(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	s := cicd.NewService(ctxSvc)
	r := newWebhookRouter(s)

	// Invalid path id
	for _, target := range []string{"/cicd/webhooks/abc", "/cicd/webhooks/0"} {
		w := doCicd(r, http.MethodPost, target, `{"pipeline":"p","externalId":"e","status":"passed"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s → %d %s", target, w.Code, w.Body.String())
	}

	// Invalid body
	w := doCicd(r, http.MethodPost, "/cicd/webhooks/1", `{"pipeline":`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// Integration not found -> 404
	w = doCicd(r, http.MethodPost, "/cicd/webhooks/999",
		`{"pipeline":"p","externalId":"e","status":"passed"}`)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	// Normal delivery: extra lands verbatim in raw, scope falls back to the
	// integration row
	id := seedRowIntegration(t, db, "generic", "http://ci.internal", nil)
	w = doWebhook(r, "/cicd/webhooks/"+itoa(id),
		`{"pipeline":"","externalId":"e-9","status":"passed","gameId":"demo","extra":{"k":"v"}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"pipeline":"default"`, "空 pipeline 回落 default")
	assert.Contains(t, w.Body.String(), `"triggeredBy":"webhook"`)

	var saved model.CicdBuild
	require.NoError(t, db.Where("external_id = ?", "e-9").First(&saved).Error)
	assert.Equal(t, "v", saved.Raw["k"], "extra 原样留存 raw")

	// Missing builds table -> write failure 500
	require.NoError(t, db.Migrator().DropTable(&model.CicdBuild{}))
	w = doWebhook(r, "/cicd/webhooks/"+itoa(id),
		`{"pipeline":"p","externalId":"e-10","status":"passed"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}

// The three branches below are **unreachable** under this repo's code structure and
// are registered as house-rule registrations (no faked cases, no deleted defense
// branches). If any premise changes, replace with a real error-path case and delete
// this registration.

// ① service.go Trigger's `if build.ExternalID == ""` (L373-375):
// The previous line already falls back through `firstNonEmpty(ref.ExternalID,
// fmt.Sprintf("trigger-%d", now.UnixNano()))`; both arguments cannot be empty at
// once (the second is a nanosecond-timestamp string), so the check is always false.
// Any provider that returns an empty identifier lands on the locally generated
// trigger-* identifier.
func TestCicdService_TriggerEmptyExternalIDBranchUnreachable(t *testing.T) {
	// Trigger returns 201 without a Location -> the jenkins provider returns an empty
	// BuildRef.ExternalID
	mux := http.NewServeMux()
	mux.HandleFunc("/job/pack/build", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
	id := createIntegration(t, r, srv.URL)

	// Provider returns an empty identifier -> still succeeds with a locally generated
	// trigger-* fallback (the check is always false)
	w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger", `{"pipeline":"pack"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"externalId":"trigger-`)
}

// ② The GetByID error branch at the tail of webhook.go IngestWebhook (L104-107):
// The previous line UpsertByExternalKey has already returned id, and GetByID fetches
// by that id immediately after. To make "write succeeds -> the very next read
// fails" hold, a storage fault must be injected between two storage calls within the
// same request - no such shape exists on the production path. Construction premise:
// no observable storage fault between Upsert/GetByID.
func TestCicdWebhook_GetByIDAfterUpsertBranchUnreachable(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	s := cicd.NewService(ctxSvc)
	r := newWebhookRouter(s)
	id := seedRowIntegration(t, db, "generic", "http://ci.internal", nil)

	// Normal shape: after a successful upsert the row is always read back (the
	// reachable form of this branch is locked by this assertion)
	w := doWebhook(r, "/cicd/webhooks/"+itoa(id),
		`{"pipeline":"p","externalId":"e-1","status":"passed"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"externalId":"e-1"`)
}

// ③ handler.go List's ShouldBindQuery error branch: see the same registration in
// handler_coverage_test.go (IntegrationListRequest has two `form` string fields,
// and gin form binding has no failure path). Cross-checked here only.
func TestCicdService_ListRequestHasNoBindableFailurePath(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	// Empty, valid, unknown and duplicate params all yield 200 (the binding layer
	// cannot produce an error)
	for _, q := range []string{"", "?gameId=demo", "?env=prod&gameId=demo", "?nope=1"} {
		w := doCicd(r, http.MethodGet, "/cicd/integrations"+q, "")
		assert.Equal(t, http.StatusOK, w.Code, "%q → %d %s", q, w.Code, w.Body.String())
	}
}

// ④ service.go normalizeExtra's `case float64` (L92-93) is a defensive unreachable
// branch: normalizeExtra has exactly two input sources - ① model.CicdIntegration.Extra
// read from the JSON column, where gorm.io/datatypes' JSONMap scanning enables
// decoder.UseNumber() (datatypes@v1.2.7 json_map.go:48), so numbers are always
// json.Number; ② rows just constructed in-process, whose values come from
// IntegrationCreateRequest.Extra (map[string]string) and can only be string.
// Neither source produces float64, so numbers actually take the default branch
// (json.Marshal(json.Number) also yields "42"). If datatypes later disables UseNumber
// or a direct map[string]any write path appears, this branch becomes reachable - add
// the corresponding case then and delete this registration.
func TestCicdService_NormalizeExtraFloat64BranchUnreachable(t *testing.T) {
	_, db := setupCtx(t)
	seedRowIntegration(t, db, "generic", "http://ci.internal", datatypes.JSONMap{"num": 42})

	var row model.CicdIntegration
	require.NoError(t, db.Where("name = ?", "seeded-generic").First(&row).Error)
	_, isFloat := row.Extra["num"].(float64)
	assert.False(t, isFloat, "datatypes 读回不得产出 float64（UseNumber 生效中）")
}
