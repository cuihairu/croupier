package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/agents/devops/config"
)

func testConfig(enabled bool) config.Config {
	return config.Config{
		Agent: config.Agent{ID: "devops-test", ServerAddr: "127.0.0.1:1", GameID: "demo", Env: "prod"},
		Devops: config.Devops{
			Enabled: enabled,
			Allowlist: []config.AllowHost{
				{Host: "10.0.1.20", Ports: []int{22}, Protocols: []string{"ssh"}},
			},
		},
	}
}

func TestBuildPayloadCarriesDevopsRole(t *testing.T) {
	r := New(testConfig(false), nil)
	req, err := r.buildPayload(context.Background())
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if req.AgentId != "devops-test" || req.GameId != "demo" || req.Env != "prod" {
		t.Fatalf("payload identity = %s/%s/%s", req.AgentId, req.GameId, req.Env)
	}
	if req.Labels["role"] != "devops" || req.Labels["hostname"] == "" {
		t.Fatalf("labels = %v", req.Labels)
	}
}

func TestRunRegistrationOnlyWhenDisabled(t *testing.T) {
	r := New(testConfig(false), nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestRunRegistrationOnlyWhenEnabled(t *testing.T) {
	// enabled=true 装配真实 SSH 通道（替代骨架 noop）+ Hub + configsync
	// 白名单拉取；会话建立下行通道另批接线。审计/存档目录落 TempDir。
	cfg := testConfig(true)
	cfg.Devops.SessionDir = t.TempDir()
	cfg.Devops.ExeclogDir = t.TempDir()
	// SSH 凭据（测试用口令文件 + insecureHostKey 逃生门）。
	pwFile := filepath.Join(t.TempDir(), "pw")
	if err := os.WriteFile(pwFile, []byte("testpass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Devops.SSH = config.SSH{
		User: "ops", PasswordFile: pwFile, InsecureHostKey: true,
	}
	r := New(cfg, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}
	if r.Broker() == nil {
		t.Fatal("broker not wired when enabled")
	}
}
