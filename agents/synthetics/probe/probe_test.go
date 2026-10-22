package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
)

func TestRunHTTPGetStatusAndBodyAssertion(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method=%s", r.Method)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("healthy"))
	}))
	defer s.Close()
	r := Run(context.Background(), config.Target{Name: "local", URL: s.URL, Method: "GET", Timeout: time.Second, ExpectedStatus: 200, BodyContains: "healthy"})
	if !r.OK || r.Status != 200 || r.Latency < 0 {
		t.Fatalf("result=%+v", r)
	}
}
