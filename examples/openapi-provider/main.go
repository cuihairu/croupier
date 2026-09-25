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
	"errors"
	"flag"
	"fmt"
	"io"
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

// exit 是 os.Exit 的测试注入点：main 只经由它退出，测试替换后可直接调用
// main 断言退出码而不终止测试进程；生产路径等价于 os.Exit(code)。
var exit = os.Exit

// embeddedAgent 是 run 对内嵌 Agent 的最小依赖面（生产实现即 *agentcore.App）。
type embeddedAgent interface {
	SetLocalAddr(addr string)
	SetUpstreamTransportKind(kind string)
	WithUpstreamMetadata(meta agentcore.UpstreamMetadata)
	Run(ctx context.Context) error
}

var _ embeddedAgent = (*agentcore.App)(nil)

// agentFactory 构造内嵌 Agent：run 的注入点，测试传入假实现，
// 避免绑定端口与上游重连等进程级副作用。
type agentFactory func(serverAddr, agentID, configDir string) embeddedAgent

// runConfig 收集 run 所需的全部 CLI 输入（字段与 newFlagSet 中的 flag 一一对应）。
type runConfig struct {
	serverAddr string
	httpAddr   string
	gameID     string
	env        string
	agentID    string
	localAddr  string
	configDir  string
	noAgent    bool
}

func main() {
	exit(runMain(os.Args[0], os.Args[1:], os.Stderr))
}

// runMain 解析命令行 flag 并运行 demo，返回进程退出码：
// 0=正常结束或 -h，2=flag 解析错误，1=运行期错误（错误日志已由 run 输出）。
func runMain(name string, args []string, output io.Writer) int {
	fs, cfg := newFlagSet(name, output)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return exitCode(run(ctx, *cfg, defaultNewAgent))
}

// exitCode 把 run 的结果映射为进程退出码。
func exitCode(err error) int {
	if err != nil {
		return 1
	}
	return 0
}

// newFlagSet 定义全部 CLI flag：名称、默认值、帮助文本与原 flag.Parse 版本完全一致。
// 错误处理用 ContinueOnError，由 runMain 自行映射退出码，对齐原 flag.ExitOnError
// 语义（-h 打印 usage 后退出 0，其余 flag 错误打印错误+usage 后退出 2）。
// 返回 *runConfig：flag 包写入的是字段地址，调用方必须持有同一份结构体。
func newFlagSet(name string, output io.Writer) (*flag.FlagSet, *runConfig) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	cfg := &runConfig{}
	fs.StringVar(&cfg.serverAddr, "server", "127.0.0.1:19090", "croupier-server 控制面 TCP 地址")
	fs.StringVar(&cfg.httpAddr, "http", "127.0.0.1:8091", "本 demo HTTP API 监听地址")
	fs.StringVar(&cfg.gameID, "game-id", "default", "注册函数归属的游戏 ID")
	fs.StringVar(&cfg.env, "env", "dev", "注册函数归属的环境")
	fs.StringVar(&cfg.agentID, "agent-id", "openapi-demo-agent", "Agent 实例 ID")
	fs.StringVar(&cfg.localAddr, "local-addr", "127.0.0.1:19091", "Agent 本地 SDK 监听地址（供 SDK 连接，demo 中不使用）")
	fs.StringVar(&cfg.configDir, "config-dir", "", "Agent 配置目录（默认使用临时目录，自动写入 providers.yaml）")
	fs.BoolVar(&cfg.noAgent, "no-agent", false, "只启动 HTTP API，不内嵌 Agent（供外部 Agent 接入）")
	return fs, cfg
}

// run 是 demo 主体：先起「游戏方 HTTP API」，再按 noAgent 决定是否内嵌 Agent，
// 最后阻塞等待 ctx 结束（生产为 SIGINT/SIGTERM）并优雅关闭 HTTP 服务。
// 失败路径保持原实现的错误日志与退出码语义（返回 error → runMain → exit(1)）。
func run(ctx context.Context, cfg runConfig, newAgent agentFactory) error {
	// ── 角色 1：游戏方 HTTP API ─────────────────────────────────────────────
	api := newPlayersAPI()
	mux := http.NewServeMux()
	api.Register(mux)
	ln, err := net.Listen("tcp", cfg.httpAddr)
	if err != nil {
		slog.Error("http listen failed", "addr", cfg.httpAddr, "error", err)
		return fmt.Errorf("http listen %s: %w", cfg.httpAddr, err)
	}
	httpSrv := &http.Server{Handler: mux}
	go serveHTTP(httpSrv, ln)
	baseURL := "http://" + ln.Addr().String()
	slog.Info("players demo API ready",
		"baseUrl", baseURL,
		"openapi", baseURL+"/openapi.json",
		"functions", "players.player.{list,get,create,update,delete,kick}")

	if cfg.noAgent {
		<-ctx.Done()
		shutdownHTTP(httpSrv)
		return nil
	}

	// ── 角色 2：内嵌 Agent（providers.yaml → openapi provider）─────────────
	dir, err := resolveConfigDir(cfg.configDir)
	if err != nil {
		shutdownHTTP(httpSrv)
		return err
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
`, cfg.gameID, cfg.env, baseURL, baseURL+"/openapi.json")
	providersPath := filepath.Join(dir, "providers.yaml")
	if err := os.WriteFile(providersPath, []byte(providersYAML), 0o644); err != nil {
		slog.Error("write providers.yaml failed", "path", providersPath, "error", err)
		shutdownHTTP(httpSrv)
		return fmt.Errorf("write providers.yaml: %w", err)
	}
	slog.Info("providers.yaml written", "path", providersPath)

	agent := newAgent(cfg.serverAddr, cfg.agentID, dir)
	agent.SetLocalAddr(cfg.localAddr)
	agent.SetUpstreamTransportKind("tcp")
	agent.WithUpstreamMetadata(agentcore.UpstreamMetadata{
		GameID:            cfg.gameID,
		Env:               cfg.env,
		Version:           "openapi-demo-1.0",
		DialTimeout:       5 * time.Second,
		RequestTimeout:    10 * time.Second,
		HeartbeatInterval: 2,
	})
	go runAgent(ctx, agent)

	<-ctx.Done()
	shutdownHTTP(httpSrv)
	return nil
}

// resolveConfigDir 规范化 Agent 配置目录：空值时建临时目录（demo 默认行为），
// 随后确保存在。失败时按原实现输出同样的错误日志并返回 error。
func resolveConfigDir(dir string) (string, error) {
	if dir == "" {
		tmp, err := os.MkdirTemp("", "openapi-provider-demo-*")
		if err != nil {
			slog.Error("create temp config dir failed", "error", err)
			return "", err
		}
		dir = tmp
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Error("prepare config dir failed", "dir", dir, "error", err)
		return "", err
	}
	return dir, nil
}

// serveHTTP 运行 http.Server.Serve：正常关停（Shutdown/Close）返回的
// http.ErrServerClosed 属预期，其余错误（如 Accept 持续失败）记日志。
// Serve 不会返回 nil（net/http/server.go 只有 ErrServerClosed 或错误返回），
// 因此无需再判 err != nil。
func serveHTTP(srv *http.Server, ln net.Listener) {
	if err := srv.Serve(ln); err != http.ErrServerClosed {
		slog.Error("http server stopped", "error", err)
	}
}

// runAgent 运行内嵌 Agent：ctx 主动取消是正常关停，不记日志；
// Agent 在 ctx 仍有效时提前失败才是异常，记错误日志。
func runAgent(ctx context.Context, agent embeddedAgent) {
	if err := agent.Run(ctx); err != nil && ctx.Err() == nil {
		slog.Error("agent stopped", "error", err)
	}
}

// defaultNewAgent 是生产路径的 Agent 构造（仅构造对象，不发起连接）。
func defaultNewAgent(serverAddr, agentID, configDir string) embeddedAgent {
	return agentcore.NewWithConfigDir(serverAddr, agentID, configDir)
}

func shutdownHTTP(srv *http.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
