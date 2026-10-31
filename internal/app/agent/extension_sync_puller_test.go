package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/security/jwtutil"
)

func TestExtensionSyncPullerPullOnce(t *testing.T) {
	rt := NewExtensionRuntime()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/v1/agents/agent-1/extensions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"code":200,
			"message":"success",
			"payload":{
				"agent_id":"agent-1",
				"generated_at":200,
				"version":"v200",
				"installations":[
					{"installationId":12,"extensionId":"official.analytics","configJson":"{}"}
				]
			}
		}`))
	}))
	defer srv.Close()

	p := NewExtensionSyncPuller(srv.URL, "agent-1", "", 0, rt)
	if err := p.PullOnce(context.Background()); err != nil {
		t.Fatalf("pull once failed: %v", err)
	}
	snap := rt.Snapshot()
	if len(snap.Installations) != 1 || snap.Installations[0].InstallationID != 12 {
		t.Fatalf("unexpected runtime snapshot: %+v", snap.Installations)
	}
}

// TestExtensionSyncPuller_MachineTokenHeader 验证 ARCH-021：配置了机器身份
// token 的拉取器在请求中携带 X-Croupier-Agent-Token 专用头；未配置则不带
// （服务端仍会 401 拒绝，但拉取器不得在无凭据时静默放行）。
func TestExtensionSyncPuller_MachineTokenHeader(t *testing.T) {
	const token = "machine-token-abc"
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get(jwtutil.MachineTokenHeader)
		// 服务端拒绝（无有效 secret），但记录头即可。
		http.Error(w, `{"error":"machine_token_invalid","message":"bad"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	rt := NewExtensionRuntime()

	// 带 token：头必须出现且等于配置值。
	withToken := NewExtensionSyncPuller(srv.URL, "agent-1", token, 0, rt)
	_ = withToken.PullOnce(context.Background())
	if gotHeader != token {
		t.Fatalf("expected header %q, got %q", token, gotHeader)
	}

	// 不带 token：头必须为空。
	gotHeader = "sentinel"
	noToken := NewExtensionSyncPuller(srv.URL, "agent-1", "", 0, rt)
	_ = noToken.PullOnce(context.Background())
	if gotHeader != "" {
		t.Fatalf("expected empty header, got %q", gotHeader)
	}
}
