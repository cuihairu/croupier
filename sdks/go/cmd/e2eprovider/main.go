// Command e2eprovider is a tiny game-server fixture used exclusively by the
// real-dashboard E2E environment. It uses the genuine Go SDK to register a
// deterministic function set with a real Croupier Agent and serve invocations.
//
// Function descriptors are supplied as JSON (see -functions). Each invocation
// is reported to the fixture control API (-report-url) so browser E2E tests
// can assert call counts and payloads without mocks.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

var (
	agentAddr = flag.String("agent-addr", "127.0.0.1:19091", "agent local SDK gateway address")
	gameID    = flag.String("game-id", "", "game scope")
	env       = flag.String("env", "", "environment scope")
	serviceID = flag.String("service-id", "real-dashboard-sdk", "service id")
	functions = flag.String("functions", "", "JSON array of croupier.FunctionDescriptor")
	reportURL = flag.String("report-url", "", "fixture control API base URL for invocation reports")
)

func main() {
	flag.Parse()

	var descs []croupier.FunctionDescriptor
	if *functions == "" {
		fmt.Fprintln(os.Stderr, "missing -functions")
		os.Exit(2)
	}
	if err := json.Unmarshal([]byte(*functions), &descs); err != nil {
		fmt.Fprintf(os.Stderr, "invalid -functions: %v\n", err)
		os.Exit(2)
	}

	cfg := croupier.DefaultClientConfig()
	cfg.AgentAddr = *agentAddr
	cfg.GameID = *gameID
	cfg.Env = *env
	cfg.ServiceID = *serviceID
	cfg.Insecure = true
	cfg.DisableLogging = true
	cfg.Reconnect = &croupier.ReconnectConfig{Enabled: false}

	client := croupier.NewClient(cfg)
	for _, desc := range descs {
		desc := desc
		if err := client.RegisterFunction(desc, func(ctx context.Context, payload []byte) ([]byte, error) {
			return handleInvoke(ctx, *reportURL, desc.ID, payload)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "register %s failed: %v\n", desc.ID, err)
			os.Exit(1)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("E2E_PROVIDER_READY")

	if err := client.Serve(ctx); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
		os.Exit(1)
	}
	_ = client.Close()
}

// handleInvoke produces deterministic structured results and reports the
// invocation to the fixture control API when configured. mail.wait is a
// cancellation canary: it blocks only until its invocation context is
// cancelled, so the real Agent -> SDK cancellation path is exercised.
func handleInvoke(ctx context.Context, reportURL, functionID string, payload []byte) ([]byte, error) {
	if reportURL != "" {
		report := map[string]interface{}{
			"functionId": functionID,
			"payload":    json.RawMessage(payload),
		}
		raw, _ := json.Marshal(report)
		req, err := http.NewRequest(http.MethodPost, reportURL+"/__fixture__/sdk/calls", bytes.NewReader(raw))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 3 * time.Second}
			if resp, err := client.Do(req); err == nil {
				resp.Body.Close()
			}
		}
	}

	switch functionID {
	case "mail.send":
		var req struct {
			PlayerID string `json:"playerId"`
			Title    string `json:"title"`
			Content  string `json:"content"`
		}
		_ = json.Unmarshal(payload, &req)
		return json.Marshal(map[string]interface{}{
			"success": true,
			"mail_id": "mail-0001",
			"title":   req.Title,
		})
	case "mail.wait":
		var req struct {
			WaitMS int `json:"waitMs"`
		}
		_ = json.Unmarshal(payload, &req)
		wait := time.Duration(req.WaitMS) * time.Millisecond
		if wait <= 0 {
			wait = 30 * time.Second
		}
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return json.Marshal(map[string]interface{}{"success": true, "waitedMs": req.WaitMS})
		}
	default:
		return json.Marshal(map[string]interface{}{"success": true})
	}
}
