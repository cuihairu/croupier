package probe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
)

type Result struct {
	Target  string
	OK      bool
	Status  int
	Latency time.Duration
	Error   string
	At      time.Time
}

func Run(ctx context.Context, t config.Target) Result {
	r := Result{Target: t.Name, At: time.Now()}
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, t.Method, t.URL, nil)
	if err != nil {
		r.Error = "configuration: " + err.Error()
		return r
	}
	resp, err := http.DefaultClient.Do(req)
	r.Latency = time.Since(start)
	if err != nil {
		r.Error = classify(err)
		return r
	}
	defer resp.Body.Close()
	r.Status = resp.StatusCode
	if resp.StatusCode != t.ExpectedStatus {
		r.Error = fmt.Sprintf("http_status: got %d want %d", resp.StatusCode, t.ExpectedStatus)
		return r
	}
	if t.BodyContains != "" && t.Method != "HEAD" {
		body, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if e != nil {
			r.Error = "response_read: " + e.Error()
			return r
		}
		if !strings.Contains(string(body), t.BodyContains) {
			r.Error = "content_assertion: expected configured text not found"
			return r
		}
	}
	r.OK = true
	return r
}
func classify(err error) string {
	if e, ok := err.(interface{ Timeout() bool }); ok && e.Timeout() {
		return "timeout: " + err.Error()
	}
	if strings.Contains(strings.ToLower(err.Error()), "no such host") {
		return "dns: " + err.Error()
	}
	return "network: " + err.Error()
}
