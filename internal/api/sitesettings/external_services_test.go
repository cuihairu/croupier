package sitesettings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/gin-gonic/gin"
)

func findService(t *testing.T, services []ExternalService, name string) ExternalService {
	t.Helper()
	for _, s := range services {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("service %q not found in %+v", name, services)
	return ExternalService{}
}

func serveExternalServices(t *testing.T, cfg config.Config) ([]ExternalService, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewExternalServicesHandler(cfg).Register(r.Group("/"))
	req := httptest.NewRequest(http.MethodGet, "/site/external-services", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body struct {
		Services []ExternalService `json:"services"`
		Source   string            `json:"source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, w.Body.String())
	}
	return body.Services, w.Code
}

// Zero config: all three report configured=false / active=false, missing items
// name their config keys; the response never echoes any value.
func TestExternalServices_ZeroConfig(t *testing.T) {
	t.Setenv("HERALD_TRIGGER_TOKEN", "")
	services, code := serveExternalServices(t, config.Config{})
	if code != http.StatusOK {
		t.Fatalf("http %d", code)
	}
	if len(services) != 4 {
		t.Fatalf("services = %d, want 4 (herald + 2 inbound + serverStatus)", len(services))
	}
	herald := findService(t, services, "herald")
	if herald.Configured || herald.Active || len(herald.Missing) != 3 {
		t.Fatalf("herald = %+v", herald)
	}
	if herald.SecretEnv != "HERALD_TRIGGER_TOKEN" {
		t.Fatalf("secretEnv = %q, want default", herald.SecretEnv)
	}
	am := findService(t, services, "alertInbound.alertmanager")
	if am.Configured || am.Active || am.Missing[0] != "alertInbound.sources.alertmanager.secretEnv" {
		t.Fatalf("alertmanager = %+v", am)
	}
	st := findService(t, services, "serverStatus.atlas")
	if st.Active || st.Missing[0] != "serverStatus.enabled" {
		t.Fatalf("serverStatus = %+v", st)
	}
}

// Everything present: enabled + baseUrl + a valued token env var -> herald active=true.
func TestExternalServices_HeraldActive(t *testing.T) {
	t.Setenv("HERALD_TRIGGER_TOKEN", "tok")
	cfg := config.Config{}
	cfg.Herald.Enabled = true
	cfg.Herald.BaseURL = "http://herald:8080"
	services, _ := serveExternalServices(t, cfg)
	herald := findService(t, services, "herald")
	if !herald.Active || !herald.Configured || !herald.SecretSet || len(herald.Missing) != 0 {
		t.Fatalf("herald = %+v", herald)
	}
}

// enabled is set but the token env var is missing: active=false, and the missing
// item names the env var (same criteria as the svc noop-fallback assembly decision).
func TestExternalServices_HeraldTokenMissing(t *testing.T) {
	t.Setenv("MY_HERALD_TOKEN", "")
	cfg := config.Config{}
	cfg.Herald.Enabled = true
	cfg.Herald.BaseURL = "http://herald:8080"
	cfg.Herald.TokenEnv = "MY_HERALD_TOKEN"
	services, _ := serveExternalServices(t, cfg)
	herald := findService(t, services, "herald")
	if herald.Active || herald.Missing[0] != "MY_HERALD_TOKEN" || herald.SecretSet {
		t.Fatalf("herald = %+v", herald)
	}
}

// alertInbound: the source is configured and the secretEnv env var has a value -> active.
func TestExternalServices_AlertInboundActive(t *testing.T) {
	t.Setenv("AM_SECRET", "s3cret")
	cfg := config.Config{}
	if cfg.AlertInbound.Sources == nil {
		cfg.AlertInbound.Sources = map[string]config.AlertInboundSourceConfig{}
	}
	cfg.AlertInbound.Sources["alertmanager"] = config.AlertInboundSourceConfig{
		SecretEnv:    "AM_SECRET",
		LabelMapping: map[string]string{"severity": "level"},
	}
	services, _ := serveExternalServices(t, cfg)
	am := findService(t, services, "alertInbound.alertmanager")
	if !am.Active || am.Detail != "labelMapping=1" {
		t.Fatalf("alertmanager = %+v", am)
	}
	// The unconfigured generic source is still listed (the whole closed set is presented)
	gen := findService(t, services, "alertInbound.generic")
	if gen.Active || gen.Configured {
		t.Fatalf("generic = %+v", gen)
	}
}

// serverStatus: enabled + a registered provider + baseUrl all present -> active;
// an unregistered provider and a missing baseUrl are each named.
func TestExternalServices_ServerStatus(t *testing.T) {
	cfg := config.Config{}
	cfg.ServerStatus.Enabled = true
	cfg.ServerStatus.Provider = "atlas"
	cfg.ServerStatus.Providers = map[string]config.ServerStatusSourceConfig{
		"atlas": {BaseURL: "http://atlas:8080"},
	}
	services, _ := serveExternalServices(t, cfg)
	st := findService(t, services, "serverStatus.atlas")
	if !st.Active || st.Detail == "" {
		t.Fatalf("atlas = %+v", st)
	}

	// Missing baseUrl -> active=false and named (an actionable hint under the
	// fail-open annotation policy)
	cfg.ServerStatus.Providers["atlas"] = config.ServerStatusSourceConfig{}
	services, _ = serveExternalServices(t, cfg)
	st = findService(t, services, "serverStatus.atlas")
	if st.Active || st.Missing[0] != "serverStatus.providers.atlas.baseUrl" {
		t.Fatalf("atlas no-baseURL = %+v", st)
	}

	// Unregistered provider -> named
	cfg.ServerStatus.Provider = "ghost"
	services, _ = serveExternalServices(t, cfg)
	st = findService(t, services, "serverStatus.ghost")
	if st.Active || st.Missing[0] != "serverStatus.provider:ghost" {
		t.Fatalf("ghost = %+v", st)
	}

	// noop: enabled alone makes it active (no baseUrl/secret requirement)
	cfg.ServerStatus.Provider = "noop"
	services, _ = serveExternalServices(t, cfg)
	st = findService(t, services, "serverStatus.noop")
	if !st.Active || len(st.Missing) != 0 {
		t.Fatalf("noop = %+v", st)
	}

	// An empty provider = the default atlas (batch S4): the missing baseUrl is named,
	// with the missing items on the same criteria as an explicit atlas (the
	// assembly layer likewise bypasses mounting the gate).
	cfg.ServerStatus.Provider = ""
	services, _ = serveExternalServices(t, cfg)
	st = findService(t, services, "serverStatus.atlas")
	if st.Active || st.Missing[0] != "serverStatus.providers.atlas.baseUrl" {
		t.Fatalf("default atlas no-baseURL = %+v", st)
	}
	// The same naming applies when the whole providers section is absent
	// (zero-config enabled != active)
	cfg.ServerStatus.Providers = nil
	services, _ = serveExternalServices(t, cfg)
	st = findService(t, services, "serverStatus.atlas")
	if st.Active || st.Missing[0] != "serverStatus.providers.atlas.baseUrl" {
		t.Fatalf("default atlas no-providers = %+v", st)
	}
}
