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

	"github.com/cuihairu/croupier/core/configsync"
)

// ExtensionSyncPuller 扩展同步拉取器：轮询节奏、版本跳过与错误回调已上收
// core/configsync（agent-core K4），本类型只保留业务面——HTTP wire 契约
// （docs/architecture/extensions-api-contract-baseline.md §3.4）与 Apply 热生效。
type ExtensionSyncPuller struct {
	baseURL string
	agentID string
	runtime *ExtensionRuntime
	puller  *configsync.Puller[*extensionsync.AgentSyncPayload]
}

// extensionSyncAPIResponse 只解 payload 包装；历史响应中的 code/message 字段
// 已随契约收口移除，decode 对未知键宽容，不在此声明。
type extensionSyncAPIResponse struct {
	Payload json.RawMessage `json:"payload"`
}

func NewExtensionSyncPuller(baseURL, agentID string, interval time.Duration, runtime *ExtensionRuntime) *ExtensionSyncPuller {
	p := &ExtensionSyncPuller{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		agentID: strings.TrimSpace(agentID),
		runtime: runtime,
	}
	p.puller = configsync.New[*extensionsync.AgentSyncPayload](interval,
		func(ctx context.Context) (*configsync.Versioned[*extensionsync.AgentSyncPayload], error) {
			payload, err := p.fetch(ctx)
			if err != nil {
				return nil, err
			}
			if payload == nil {
				return nil, nil
			}
			return &configsync.Versioned[*extensionsync.AgentSyncPayload]{Version: payload.Version, Payload: payload}, nil
		},
		func(ctx context.Context, payload *extensionsync.AgentSyncPayload) error {
			// ApplyPayload 仅有的错误路径是 nil receiver / nil payload，二者已被
			// 上游判断排除，err 分支为死代码。
			_, _ = p.runtime.ApplyPayload(payload)
			return nil
		},
	)
	return p
}

// Start 启动轮询（立即一轮 + 周期循环，ctx 取消退出）。装配不全时空转返回。
func (p *ExtensionSyncPuller) Start(ctx context.Context) {
	if p == nil || p.runtime == nil || p.baseURL == "" || p.agentID == "" {
		return
	}
	p.puller.Start(ctx)
}

// PullOnce 执行一轮拉取与热生效；返回本轮是否实际应用了新版本与错误。
func (p *ExtensionSyncPuller) PullOnce(ctx context.Context) error {
	if p == nil || p.runtime == nil {
		return fmt.Errorf("extension sync puller not initialized")
	}
	_, err := p.puller.PullOnce(ctx)
	return err
}

func (p *ExtensionSyncPuller) fetch(ctx context.Context) (*extensionsync.AgentSyncPayload, error) {
	if p.baseURL == "" || p.agentID == "" {
		return nil, fmt.Errorf("extension sync puller not initialized")
	}
	url := fmt.Sprintf("%s/api/v1/agents/%s/extensions", p.baseURL, p.agentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("extension sync pull failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var wrapper extensionSyncAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.Payload) == 0 || string(wrapper.Payload) == "null" {
		return nil, nil
	}
	var payload extensionsync.AgentSyncPayload
	if err := json.Unmarshal(wrapper.Payload, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
