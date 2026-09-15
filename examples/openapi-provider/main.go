// Command openapi-provider 演示「游戏方 HTTP 服务 + Agent openapi provider」
// 的完整接入：本进程同时扮演两个角色——
//
//  1. 游戏方 HTTP API（/players CRUD + /openapi.json 契约导出）；
//  2. 内嵌 Croupier Agent：用自动生成的 providers.yaml（type: openapi）把
//     上面的 API 注册进 Server，函数 ID 为 players.<operationId>。
//
// 快速开始（先启动 croupier-server）：
//
//	go run ./examples/openapi-provider -server 127.0.0.1:19090 -http 127.0.0.1:8091
//
// 然后在控制台「函数目录」中查看 players.player.list / player.get /
// player.create / player.update / player.delete / player.kick。
//
// 只想跑 HTTP API（用自己的 Agent / 生产 Agent 接入）时加 -no-agent，
// providers.yaml 的写法见 README.md 与 docs/guide/integrations/agent-providers.md。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
)

func main() {
	serverAddr := flag.String("server", "127.0.0.1:19090", "croupier-server 控制面 TCP 地址")
	httpAddr := flag.String("http", "127.0.0.1:8091", "本 demo HTTP API 监听地址")
	gameID := flag.String("game-id", "default", "注册函数归属的游戏 ID")
	env := flag.String("env", "dev", "注册函数归属的环境")
	agentID := flag.String("agent-id", "openapi-demo-agent", "Agent 实例 ID")
	localAddr := flag.String("local-addr", "127.0.0.1:19091", "Agent 本地 SDK 监听地址（供 SDK 连接，demo 中不使用）")
	configDir := flag.String("config-dir", "", "Agent 配置目录（默认使用临时目录，自动写入 providers.yaml）")
	noAgent := flag.Bool("no-agent", false, "只启动 HTTP API，不内嵌 Agent（供外部 Agent 接入）")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── 角色 1：游戏方 HTTP API ─────────────────────────────────────────────
	api := newPlayersAPI()
	mux := http.NewServeMux()
	api.Register(mux)
	ln, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		slog.Error("http listen failed", "addr", *httpAddr, "error", err)
		os.Exit(1)
	}
	httpSrv := &http.Server{Handler: mux}
	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("http server stopped", "error", err)
		}
	}()
	baseURL := "http://" + ln.Addr().String()
	slog.Info("players demo API ready",
		"baseUrl", baseURL,
		"openapi", baseURL+"/openapi.json",
		"functions", "players.player.{list,get,create,update,delete,kick}")

	if *noAgent {
		<-ctx.Done()
		shutdownHTTP(httpSrv)
		return
	}

	// ── 角色 2：内嵌 Agent（providers.yaml → openapi provider）─────────────
	dir := *configDir
	if dir == "" {
		tmp, err := os.MkdirTemp("", "openapi-provider-demo-*")
		if err != nil {
			slog.Error("create temp config dir failed", "error", err)
			os.Exit(1)
		}
		dir = tmp
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Error("prepare config dir failed", "dir", dir, "error", err)
		os.Exit(1)
	}
	providersYAML := fmt.Sprintf(`# 由 demo 自动生成；生产部署请放在 Agent 配置目录并自行维护。
# 字段说明见 docs/guide/integrations/agent-providers.md
providers:
  players:
    enabled: true
    type: openapi
    game_id: %q
    env: %q
    config:
      baseUrl: %q
      openapiSpec: %q
      timeout: "5s"
`, *gameID, *env, baseURL, baseURL+"/openapi.json")
	providersPath := filepath.Join(dir, "providers.yaml")
	if err := os.WriteFile(providersPath, []byte(providersYAML), 0o644); err != nil {
		slog.Error("write providers.yaml failed", "path", providersPath, "error", err)
		os.Exit(1)
	}
	slog.Info("providers.yaml written", "path", providersPath)

	agent := agentcore.NewWithConfigDir(*serverAddr, *agentID, dir)
	agent.SetLocalAddr(*localAddr)
	agent.SetUpstreamTransportKind("tcp")
	agent.WithUpstreamMetadata(agentcore.UpstreamMetadata{
		GameID:            *gameID,
		Env:               *env,
		Version:           "openapi-demo-1.0",
		DialTimeout:       5 * time.Second,
		RequestTimeout:    10 * time.Second,
		HeartbeatInterval: 2,
	})
	go func() {
		if err := agent.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("agent stopped", "error", err)
		}
	}()

	<-ctx.Done()
	shutdownHTTP(httpSrv)
}

func shutdownHTTP(srv *http.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
