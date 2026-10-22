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

// 零配置：三项全部 configured=false / active=false，缺项点名配置键；
// 响应永不回显任何值。
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

// 全齐：enabled + baseUrl + token 环境变量有值 → herald active=true。
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

// 配了 enabled 但缺 token 环境变量：active=false，缺项点名环境变量（与
// svc 回落 noop 装配判定同口径）。
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

// alertInbound 来源已配置且 secretEnv 环境变量有值 → active。
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
	// 未配置的 generic 来源仍列出（呈现全闭集状态）
	gen := findService(t, services, "alertInbound.generic")
	if gen.Active || gen.Configured {
		t.Fatalf("generic = %+v", gen)
	}
}

// serverStatus：enabled + provider 已注册 + baseUrl 齐全 → active；
// 未注册 provider 与缺 baseUrl 各自点名。
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

	// 缺 baseUrl → active=false 且点名（fail-open 标注口径的可操作提示）
	cfg.ServerStatus.Providers["atlas"] = config.ServerStatusSourceConfig{}
	services, _ = serveExternalServices(t, cfg)
	st = findService(t, services, "serverStatus.atlas")
	if st.Active || st.Missing[0] != "serverStatus.providers.atlas.baseUrl" {
		t.Fatalf("atlas no-baseURL = %+v", st)
	}

	// 未注册 provider → 点名
	cfg.ServerStatus.Provider = "ghost"
	services, _ = serveExternalServices(t, cfg)
	st = findService(t, services, "serverStatus.ghost")
	if st.Active || st.Missing[0] != "serverStatus.provider:ghost" {
		t.Fatalf("ghost = %+v", st)
	}

	// noop：enabled 即 active（无 baseUrl/密钥要求）
	cfg.ServerStatus.Provider = "noop"
	services, _ = serveExternalServices(t, cfg)
	st = findService(t, services, "serverStatus.noop")
	if !st.Active || len(st.Missing) != 0 {
		t.Fatalf("noop = %+v", st)
	}

	// provider 空=缺省 atlas（批 S4）：缺 baseUrl 点名，缺项与显式 atlas
	// 同口径（装配层同样旁路不挂 gate）。
	cfg.ServerStatus.Provider = ""
	services, _ = serveExternalServices(t, cfg)
	st = findService(t, services, "serverStatus.atlas")
	if st.Active || st.Missing[0] != "serverStatus.providers.atlas.baseUrl" {
		t.Fatalf("default atlas no-baseURL = %+v", st)
	}
	// providers 段整个不存在时同样点名（零配置启用≠active）
	cfg.ServerStatus.Providers = nil
	services, _ = serveExternalServices(t, cfg)
	st = findService(t, services, "serverStatus.atlas")
	if st.Active || st.Missing[0] != "serverStatus.providers.atlas.baseUrl" {
		t.Fatalf("default atlas no-providers = %+v", st)
	}
}
