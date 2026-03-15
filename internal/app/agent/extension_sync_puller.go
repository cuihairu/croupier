package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
)

type extensionSyncAPIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Payload json.RawMessage `json:"payload"`
}

type ExtensionSyncPuller struct {
	baseURL  string
	agentID  string
	client   *http.Client
	interval time.Duration
	runtime  *ExtensionRuntime
}

func NewExtensionSyncPuller(baseURL, agentID string, interval time.Duration, runtime *ExtensionRuntime) *ExtensionSyncPuller {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &ExtensionSyncPuller{
		baseURL:  base,
		agentID:  strings.TrimSpace(agentID),
		client:   &http.Client{Timeout: 8 * time.Second},
		interval: interval,
		runtime:  runtime,
	}
}

func (p *ExtensionSyncPuller) Start(ctx context.Context) {
	if p == nil || p.runtime == nil || p.baseURL == "" || p.agentID == "" {
		return
	}
	go func() {
		_ = p.PullOnce(ctx)
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = p.PullOnce(ctx)
			}
		}
	}()
}

func (p *ExtensionSyncPuller) PullOnce(ctx context.Context) error {
	if p == nil || p.runtime == nil {
		return fmt.Errorf("extension sync puller not initialized")
	}
	url := fmt.Sprintf("%s/api/v1/agents/%s/extensions", p.baseURL, p.agentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("extension sync pull failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var wrapper extensionSyncAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return err
	}
	if len(wrapper.Payload) == 0 || string(wrapper.Payload) == "null" {
		return nil
	}
	var payload extensionsync.AgentSyncPayload
	if err := json.Unmarshal(wrapper.Payload, &payload); err != nil {
		return err
	}
	_, err = p.runtime.ApplyPayload(&payload)
	if err != nil {
		p.runtime.RecordError(err)
	}
	return err
}
