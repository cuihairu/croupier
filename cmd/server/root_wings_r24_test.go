// 覆盖率巡检第二十四轮（wt-api）：cmd/server root.go 装配面——
// ① runServer 配置缺失翼 + rootCmd.RunE / server 别名闭包（错误路径
//
//	共享同一廉价入口：坏 cfgFile 让 runServer 在 loadConfigFile 即返回）；
//
// ② runServer 全链 ×3 boot（mode/debug/logLevel 组合矩阵全覆盖）：
//
//	真实 NewServiceContext + 路由注册 + 集群 db 协调面 + 遥测服务 +
//	信号等待与优雅停机全序列。就绪锚定 "Starting Croupier Server at "
//	同步打印行（第二十三轮 fixture 用例同款：管道探测 + 持续排空 +
//	SIGINT 幂等重发消除命令内 signal.Notify 注册窗口竞态）；
//	boot A mode=test/logLevel=debug（131 + 146-150 + gin TestMode）
//	boot B mode=prod（129 + gin ReleaseMode）
//	boot C mode=dev/debug=true（133 + 137-143 + gin DebugMode）
//	boot C 额外经全局 port/host 覆盖翼（119-124），A/B 走配置 127.0.0.1:0；
//
// ③ startControlServer 地址归一两翼（""→:19090→0.0.0.0、':'前缀→
//
//	0.0.0.0 拼接）与 TLS 证书加载失败翼（468-471，不存在的证书文件
//	确定性构造，不依赖端口冲突）；
//
// ④ startRegistryCleanup 的 nil store 翼；⑤ 三个 env 覆盖函数的
// nil 配置翼 + 全部环境变量注入体；⑥ wrapHTTPHandler 的遥测中间件
// 翼（离线构造：enabled 但 tracing/metrics 关闭，无 exporter 无出站）；
// ⑦ validateAndAdjustTimeout 矩阵（自定义/缺省间隔 × 调整/通过两翼）。
//
// 登记不可达（不造假用例、不删防御分支）：
//   - root.go Execute/main（65-74）：cmd-1 进程边界豁免（os.Exit）；
//   - HTTP ListenAndServe 错误翼（348-350）：分支体是 os.Exit(1)，进程内
//     任何构造（端口被占即触发）都会直接杀死测试二进制；
//   - 会话 prune ticker 体（486-489）：30s 硬编码间隔 + 5min 陈旧阈值，
//     无 reconcileTickerInterval 式注入点，等待成本与价值不成比例；
//   - tcpListener.Serve 非 Canceled 错误翼（497-498）：Close→nil、
//     cancel→Canceled（被过滤），无确定性第三形态。
package main

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/server"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 坏 cfgFile：runServer 在 loadConfigFile 即返回错误——同时覆盖
// rootCmd.RunE 转发闭包与 "server" 别名命令闭包，无需完整启动。
func TestRunServer_MissingConfigWing(t *testing.T) {
	oldCfg := cfgFile
	cfgFile = filepath.Join(t.TempDir(), "no-such-server.yaml")
	t.Cleanup(func() { cfgFile = oldCfg })

	require.Error(t, rootCmd.RunE(rootCmd, nil), "rootCmd.RunE → runServer 配置缺失报错")

	found := false
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "server" {
			found = true
			require.Error(t, sub.RunE(sub, nil), "server 别名闭包 → runServer 配置缺失报错")
		}
	}
	require.True(t, found, "rootCmd 应注册 server 别名子命令")
}

// runServerBootR24：单次完整启动 + SIGINT 优雅停机。全局参数由调用方
// 预先布置（mode/debug/logLevel/port/host 组合即矩阵轴）。
func runServerBootR24(t *testing.T) {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	readyCh := make(chan struct{}, 1)
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "Starting Croupier Server at ") {
				readyCh <- struct{}{}
				break
			}
		}
		// 持续排空：gin 调试路由表等大量输出写满管道会阻塞启动路径
		io.Copy(io.Discard, r)
	}()

	done := make(chan error, 1)
	go func() { done <- runServer() }()

	select {
	case <-readyCh:
	case <-time.After(120 * time.Second):
		os.Stdout = oldStdout
		t.Fatal("runServer 未在期限内打出就绪行")
	}
	os.Stdout = oldStdout
	time.Sleep(200 * time.Millisecond) // 同步打印与 signal.Notify 之间的窗口

	for i := 0; i < 20; i++ {
		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
		select {
		case err := <-done:
			require.NoError(t, err, "SIGINT 后 runServer 须走完优雅停机并返回 nil")
			// 注意：不能 Close(w)——runServer 的 SetupLoggerWithFile 把
			// 全局 slog handler 指向该管道 *os.File，关闭后 isTerminal
			// 对 closed file Stat 得 nil FileInfo 再解引用即 panic
			//（后续任意 slog.Info 都会炸）。留开 + 排空 goroutine 持续
			// 消费即可；slog.Default 由矩阵用例统一恢复。
			return
		case <-time.After(3 * time.Second):
		}
	}
	t.Fatal("runServer 未随 SIGINT 退出")
}

func TestRunServer_FullBootMatrix(t *testing.T) {
	// 预注册 SIGINT：任何时刻的 Kill 都不会落入默认处置（杀测试进程）
	mine := make(chan os.Signal, 1)
	signal.Notify(mine, syscall.SIGINT)
	t.Cleanup(func() { signal.Stop(mine) })

	// SetupLoggerWithFile 替换进程级 slog 默认 logger（指向启动管道），
	// 矩阵结束后恢复，避免污染后续用例的日志输出路径
	oldLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	// 相对路径（data/uploads、日志）隔离
	t.Chdir(t.TempDir())

	dbPath := filepath.Join(t.TempDir(), "r24-fullboot.db")
	cfg := writeCfgR22(t, "server:\n  host: 127.0.0.1\n  port: 0\n"+
		"database:\n  driver: sqlite\n  dataSource: "+dbPath+"\n"+
		"auth:\n  jwtSecret: r24-fullboot-secret-0123456789\n"+
		"control:\n  addr: 127.0.0.1:0\n"+
		"cluster:\n  enabled: true\n  instanceId: server-r24-fullboot\n"+
		"  store: db\n  advertiseAddr: 127.0.0.1:0\n  heartbeatInterval: 500ms\n"+
		"telemetry:\n  enabled: true\n")

	// 预选空闲 HTTP 端口（boot C 的全局 port 覆盖翼需要确定端口）
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	httpPort := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	oldCfg, oldMode, oldPort, oldDebug := cfgFile, mode, port, debug
	oldHost, oldLevel, oldBoot := host, logLevel, bootstrapDataDir
	t.Cleanup(func() {
		cfgFile, mode, port, debug = oldCfg, oldMode, oldPort, oldDebug
		host, logLevel, bootstrapDataDir = oldHost, oldLevel, oldBoot
	})

	boots := []struct {
		name     string
		modeVal  string
		portVal  int
		hostVal  string
		debugVal bool
		levelVal string
	}{
		// test 模式 + 显式 logLevel：131 + 146-150 + gin TestMode(306)
		{"test-mode", "test", 0, "", false, "debug"},
		// prod 模式：129 + gin ReleaseMode(304)
		{"prod-mode", "prod", 0, "", false, ""},
		// 默认模式 + debug：133 + 137-143 + gin DebugMode(308)
		{"dev-debug", "dev", httpPort, "127.0.0.1", true, ""},
	}
	for _, b := range boots {
		t.Run(b.name, func(t *testing.T) {
			cfgFile, mode, port, debug = cfg, b.modeVal, b.portVal, b.debugVal
			host, logLevel = b.hostVal, b.levelVal
			bootstrapDataDir = filepath.Join(t.TempDir(), "boot")
			runServerBootR24(t)
		})
	}
}

// 遥测服务初始化失败翼（198-199）登记不可达：经探针实证（第二十四轮，
// internal/telemetry 侧临时用例六形态扫描后移除），NewGameTelemetryService
// 对畸形输入全部 EAGER 返回 nil error——OTLP HTTP 客户端不预先解析
// endpoint（首次上传才失败）、resource 属性解析宽松、headers 坏 JSON
// 不在构造期校验。错误仅在 exporter 运行期上传时产生，构造路径无可
// 注入失败面，不造假用例。

// startControlServer 的地址归一两翼 + TLS 证书加载失败翼。
func TestStartControlServer_AddrAndTLSWings(t *testing.T) {
	newCfg := func(addr, cert, key string) *config.Config {
		c := &config.Config{}
		c.Control.Addr = addr
		c.Control.Cert = cert
		c.Control.Key = key
		return c
	}

	// ① addr "": 默认 :19090 → 0.0.0.0:19090。绑定成败均合法
	//（端口被真实 server 占用时落到错误翼），断言只钉返回结构。
	svcCtx, _ := newControlServerSvcCtx(t)
	ctx, cancel := context.WithCancel(context.Background())
	rt := startControlServer(ctx, newCfg("", "", ""), svcCtx, server.NewAgentSessionStore())
	require.NotNil(t, rt.controlService)
	if rt.tcpListener != nil {
		require.NoError(t, rt.tcpListener.Close())
	}
	rt.controlService.Stop()
	cancel()

	// ② ':' 前缀：拼接 0.0.0.0 → 随机端口绑定成功，句柄齐备。
	svcCtx2, _ := newControlServerSvcCtx(t)
	ctx2, cancel2 := context.WithCancel(context.Background())
	rt2 := startControlServer(ctx2, newCfg(":0", "", ""), svcCtx2, server.NewAgentSessionStore())
	require.NotNil(t, rt2.controlService)
	require.NotNil(t, rt2.tcpListener, "0.0.0.0:0 绑定应成功")
	require.NoError(t, rt2.tcpListener.Close())
	rt2.controlService.Stop()
	cancel2()

	// ③ TLS 证书缺失：NewTCPListener 加载失败 → 只返回 controlService。
	svcCtx3, _ := newControlServerSvcCtx(t)
	ctx3, cancel3 := context.WithCancel(context.Background())
	rt3 := startControlServer(ctx3, newCfg("127.0.0.1:0", "no-such-cert.pem", "no-such-key.pem"), svcCtx3, server.NewAgentSessionStore())
	require.NotNil(t, rt3.controlService)
	assert.Nil(t, rt3.tcpListener, "证书加载失败 → 无监听句柄")
	rt3.controlService.Stop()
	cancel3()
}

// startRegistryCleanup 的 nil store 翼（打印后跳过清理例程）。
func TestStartRegistryCleanup_NilStoreWing(t *testing.T) {
	out := captureServerStdout(t, func() {
		startRegistryCleanup(context.Background(), &svc.ServiceContext{})
	})
	assert.Contains(t, out, "RegistryStore is nil")
}

// 三个环境变量覆盖函数：nil 配置翼 + 全部注入体。
func TestApplyEnvOverrides_Wings(t *testing.T) {
	t.Run("nil-config", func(t *testing.T) {
		applyStorageEnvironmentOverrides(nil)
		applyClusterEnvironmentOverrides(nil)
		applyAuthSecretEnvironmentOverrides(nil)
	})

	t.Run("storage", func(t *testing.T) {
		t.Setenv("STORAGE_DRIVER", "s3")
		t.Setenv("STORAGE_BASE_DIR", "/tmp/r24-uploads")
		c := &config.Config{}
		applyStorageEnvironmentOverrides(c)
		assert.Equal(t, "s3", c.Storage.Driver)
		assert.Equal(t, "/tmp/r24-uploads", c.Storage.BaseDir)
	})

	t.Run("cluster", func(t *testing.T) {
		t.Setenv("CROUPIER_CLUSTER_ENABLED", "yes")
		t.Setenv("CROUPIER_CLUSTER_INSTANCE_ID", "server-r24-env")
		t.Setenv("CROUPIER_CLUSTER_STORE", "redis")
		t.Setenv("CROUPIER_CLUSTER_REDIS_ADDR", "127.0.0.1:6379")
		t.Setenv("CROUPIER_CLUSTER_ADVERTISE_ADDR", "127.0.0.1:19091")
		t.Setenv("CROUPIER_LB_PROMETHEUS_URL", "http://prom:9090")
		c := &config.Config{}
		applyClusterEnvironmentOverrides(c)
		assert.True(t, c.Cluster.Enabled, "yes 形态应解析为启用")
		assert.Equal(t, "server-r24-env", c.Cluster.InstanceID)
		assert.Equal(t, "redis", c.Cluster.Store)
		assert.Equal(t, "127.0.0.1:6379", c.Cluster.RedisAddr)
		assert.Equal(t, "127.0.0.1:19091", c.Cluster.AdvertiseAddr)
		assert.Equal(t, "http://prom:9090", c.Cluster.LbPrometheusUrl)
	})

	t.Run("auth-secrets", func(t *testing.T) {
		t.Setenv("CROUPIER_AUTH_LDAP_BIND_PASSWORD", "ldap-pw")
		t.Setenv("CROUPIER_AUTH_OIDC_CLIENT_SECRET", "oidc-secret")
		c := &config.Config{}
		applyAuthSecretEnvironmentOverrides(c)
		assert.Equal(t, "ldap-pw", c.Auth.Providers.LDAP.BindPassword)
		assert.Equal(t, "oidc-secret", c.Auth.Providers.OIDC.ClientSecret)
	})
}

// wrapHTTPHandler 的遥测中间件翼：离线构造（tracing/metrics 关闭，
// 无 exporter 无出站连接）。
func TestWrapHTTPHandler_TelemetryWing(t *testing.T) {
	telSvc, err := telemetry.NewGameTelemetryService(telemetry.TelemetryConfig{
		Enabled:     true,
		ServiceName: "r24-telemetry",
	}, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, telSvc)

	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	wrapped := wrapHTTPHandler(&svc.ServiceContext{Telemetry: telSvc}, h)
	// 原句柄直返形态已由 NilSvcCtx/NilTelemetry 既有用例锁定；本翼以
	// 行为证明：返回的包装仍委托内层 handler（otelhttp.NewHandler 本身
	// 也返回 http.HandlerFunc，类型断言无法区分，不走恒等比较）。
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.True(t, called, "包装后仍委托内层 handler")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// validateAndAdjustTimeout：自定义间隔 × 调整翼 / 缺省间隔 × 通过翼。
func TestValidateAndAdjustTimeout_Matrix(t *testing.T) {
	// 超时小于 3×keep-alive → 警告并自动调整
	c := &config.Config{}
	c.SSE.UpdateInterval = 5
	c.SSE.KeepAliveInterval = 30
	c.Server.Timeout = 10000
	out := captureServerStdout(t, func() { validateAndAdjustTimeout(c) })
	assert.Contains(t, out, "自动调整")
	assert.Equal(t, int64(90000), c.Server.Timeout)

	// 缺省间隔（2/30）+ 足够超时 → 验证通过原样保留
	c2 := &config.Config{}
	c2.Server.Timeout = 600000
	out2 := captureServerStdout(t, func() { validateAndAdjustTimeout(c2) })
	assert.Contains(t, out2, "SSE 配置验证通过")
	assert.Equal(t, int64(600000), c2.Server.Timeout)
}
