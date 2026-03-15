package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
					{"installation_id":12,"extension_id":"official.analytics","config_json":"{}"}
				]
			}
		}`))
	}))
	defer srv.Close()

	p := NewExtensionSyncPuller(srv.URL, "agent-1", 0, rt)
	if err := p.PullOnce(context.Background()); err != nil {
		t.Fatalf("pull once failed: %v", err)
	}
	snap := rt.Snapshot()
	if len(snap.Installations) != 1 || snap.Installations[0].InstallationID != 12 {
		t.Fatalf("unexpected runtime snapshot: %+v", snap.Installations)
	}
}
