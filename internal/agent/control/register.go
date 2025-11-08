package control

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"

	controlv1 "github.com/cuihairu/croupier/pkg/pb/croupier/control/v1"
)

// Client wraps the generated ControlService client with helper methods.
type Client struct {
	c controlv1.ControlServiceClient
}

func NewClient(cc *grpc.ClientConn) *Client { return &Client{c: controlv1.NewControlServiceClient(cc)} }

// RegisterAndHeartbeat performs initial register and keeps sending heartbeats until ctx done.
func (cl *Client) RegisterAndHeartbeat(ctx context.Context, agentID, version, rpcAddr, gameID, env string, fns []*controlv1.FunctionDescriptor) {
	resp, err := cl.c.Register(ctx, &controlv1.RegisterRequest{AgentId: agentID, Version: version, RpcAddr: rpcAddr, GameId: gameID, Env: env, Functions: fns})
	if err != nil {
		slog.Error("agent register failed", "error", err.Error())
		return
	}
	slog.Info("agent registered", "session", resp.GetSessionId())
	// best-effort meta report (region/zone) once and periodically
	go func() {
		reportAgentMeta(agentID)
		tk := time.NewTicker(5 * time.Minute)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				reportAgentMeta(agentID)
			}
		}
	}()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := cl.c.Heartbeat(ctx, &controlv1.HeartbeatRequest{AgentId: agentID, SessionId: resp.GetSessionId()}); err != nil {
				slog.Warn("heartbeat failed", "error", err.Error())
			}
		}
	}
}

// reportAgentMeta posts region/zone to server if AGENT_META_URL and AGENT_META_TOKEN are set.
func reportAgentMeta(agentID string) {
	url := strings.TrimSpace(os.Getenv("AGENT_META_URL"))
	tok := strings.TrimSpace(os.Getenv("AGENT_META_TOKEN"))
	if url == "" || tok == "" {
		return
	}
	reg := strings.TrimSpace(os.Getenv("AGENT_REGION"))
	zon := strings.TrimSpace(os.Getenv("AGENT_ZONE"))
	if reg == "" && zon == "" {
		return
	}
	payload := map[string]string{"AgentID": agentID}
	if reg != "" {
		payload["Region"] = reg
	}
	if zon != "" {
		payload["Zone"] = zon
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", tok)
	cli := &http.Client{Timeout: 2 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		slog.Warn("agent meta report failed", "error", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		slog.Warn("agent meta report non-2xx", "status", resp.StatusCode)
	}
}
