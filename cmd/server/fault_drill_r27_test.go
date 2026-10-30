// ARCH-018 执行故障钻机：在真实 runServer 进程内注入 断链 / drain / SIGTERM /
// 看门狗四类执行中断，逐项记录新请求拒绝、终态/未知态收敛、截止时间与资源
// 释放。
//
// 边界诚实：探针（e2e-agent-probe -hang / -step-ms）只是计数 fixture——
// 探针侧的任务/副作用计数不构成游戏恢复凭据。本文件断言的只有平台收敛行为
// （状态机终态/未知态/拒绝语义/资源释放），不断言游戏 exactly-once
// （GAME-010 仍 NOT RUN，见 docs/design/task-retry-recovery-protocol.md 已知边界）。
//
// 各场景共启一个 runServer + 一个探针子进程，包变量共享故顺序执行（不 Parallel）。
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/cuihairu/croupier/internal/tasks"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const (
	drillFunctionID = "drill.echo"
	drillGameID     = "default"
	drillEnv        = "dev"
	drillAdminUser  = "admin"
	drillAdminPass  = "admin123"
	drillJWTSecret  = "drill-secret-0123456789"
	// 看门狗预算：drill 配置 taskLog.reaperTimeout=4s，短预算驱动 2s 收割 tick。
	drillReaperBudget = 4 * time.Second
)

// 探针二进制只构建一次（sync.Once），4 场景共用。
var (
	drillProbeOnce sync.Once
	drillProbePath string
	drillProbeErr  error
)

func drillRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// fault_drill_r27_test.go → cmd/server → cmd → 仓库根。
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func drillBuildProbe(t *testing.T) string {
	t.Helper()
	drillProbeOnce.Do(func() {
		root := drillRepoRoot(t)
		dir, err := os.MkdirTemp("", "drill-probe-*")
		if err != nil {
			drillProbeErr = err
			return
		}
		bin := filepath.Join(dir, "e2e-agent-probe")
		cmd := exec.Command("go", "build", "-o", bin, "./examples/cmd/e2e-agent-probe")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			drillProbeErr = fmt.Errorf("build probe: %v\n%s", err, out)
			return
		}
		drillProbePath = bin
	})
	require.NoError(t, drillProbeErr)
	return drillProbePath
}

// drillServer 持有一次 runServer boot 的句柄与资源。
type drillServer struct {
	base     string // http://127.0.0.1:<port>
	ctrlAddr string // 127.0.0.1:<control-port>（探针拨号目标）
	dbPath   string
	probeBin string
	done     chan error
	exited   bool
	runErr   error
}

func drillBoot(t *testing.T) *drillServer {
	// 预注册 SIGINT/SIGTERM：任何时刻的 Kill 都不落入默认处置（杀测试进程）。
	mine := make(chan os.Signal, 4)
	signal.Notify(mine, syscall.SIGINT, syscall.SIGTERM)
	t.Cleanup(func() { signal.Stop(mine) })

	oldLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	// 相对路径（data/uploads、日志）隔离。
	t.Chdir(t.TempDir())

	dbPath := filepath.Join(t.TempDir(), "drill.db")
	// 预选空闲 HTTP/控制端口（确定性拨号目标；HTTP 服务器必然在就绪行后监听）。
	httpPort := drillFreePort(t)
	ctrlPort := drillFreePort(t)

	cfg := writeCfgR22(t, "server:\n  host: 127.0.0.1\n  port: "+strconv.Itoa(httpPort)+"\n"+
		"database:\n  driver: sqlite\n  dataSource: "+dbPath+"\n"+
		"auth:\n  jwtSecret: "+drillJWTSecret+"\n"+
		"control:\n  addr: 127.0.0.1:"+strconv.Itoa(ctrlPort)+"\n"+
		"cluster:\n  enabled: false\n"+
		"telemetry:\n  enabled: false\n"+
		"taskLog:\n  reaperTimeout: "+strconv.Itoa(int(drillReaperBudget/time.Second))+"s\n")

	oldCfg, oldMode, oldPort, oldDebug := cfgFile, mode, port, debug
	oldHost, oldLevel, oldBoot := host, logLevel, bootstrapDataDir
	t.Cleanup(func() {
		cfgFile, mode, port, debug = oldCfg, oldMode, oldPort, oldDebug
		host, logLevel, bootstrapDataDir = oldHost, oldLevel, oldBoot
	})
	cfgFile = cfg
	// 真实 configs 引导：seed admin（admin/admin123）与 default 游戏。
	bootstrapDataDir = filepath.Join(drillRepoRoot(t), "configs")

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
		// 持续排空：gin 调试路由表等大量输出写满管道会阻塞启动路径。
		io.Copy(io.Discard, r)
	}()

	// 进程级单例重置：jwtutil 全局密钥是 sync.Once，哪个测试先初始化谁生效。
	// 全量跑时前面的测试已初始化过——drill 的 runServer 里 InitGlobalSecret
	// 会静默 no-op，导致 login 用配置密钥签名而 authority 用旧全局密钥验证
	// → 401（全量跑实测）。重置使 drill 的配置密钥成为当前全局（cleanup 还原）。
	jwtRestore := jwtutil.ResetGlobalSecretForTesting(drillJWTSecret)
	t.Cleanup(jwtRestore)

	done := make(chan error, 1)
	go func() { done <- runServer() }()

	select {
	case <-readyCh:
	case <-time.After(120 * time.Second):
		os.Stdout = oldStdout
		t.Fatal("runServer 未在期限内打出就绪行")
	}

	s := &drillServer{
		base:     fmt.Sprintf("http://127.0.0.1:%d", httpPort),
		ctrlAddr: fmt.Sprintf("127.0.0.1:%d", ctrlPort),
		dbPath:   dbPath,
		probeBin: drillBuildProbe(t),
		done:     done,
	}
	t.Cleanup(func() {
		s.stop(t)
		os.Stdout = oldStdout
		_ = w.Close()
	})
	return s
}

func drillFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}

// waitExit 等待 runServer 返回，记录终局并返回其错误（超时则失败测试）。
func (s *drillServer) waitExit(t *testing.T, timeout time.Duration) error {
	t.Helper()
	select {
	case err := <-s.done:
		s.exited = true
		s.runErr = err
		return err
	case <-time.After(timeout):
		t.Fatalf("runServer 未在 %s 内退出", timeout)
		return nil
	}
}

// stop 以 SIGINT 触发优雅停机并等待 runServer 返回（已在停机测试中退出则 no-op）。
func (s *drillServer) stop(t *testing.T) {
	t.Helper()
	if s.exited {
		return
	}
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	s.waitExit(t, 30*time.Second)
}

// drillProbe 是探针子进程句柄。
type drillProbe struct {
	cmd     *exec.Cmd
	logPath string
}

// spawnProbe 启动探针 serve 模式并等待其打出「serving function」。
func (s *drillServer) spawnProbe(t *testing.T, agentID string, extra ...string) *drillProbe {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "probe.log")
	logFile, err := os.Create(logPath)
	require.NoError(t, err)
	args := []string{
		"-addr", s.ctrlAddr,
		"-agent-id", agentID,
		"-game-id", drillGameID,
		"-env", drillEnv,
		"-mock-task", drillFunctionID,
		"-serve-duration", "120s",
	}
	args = append(args, extra...)
	cmd := exec.Command(s.probeBin, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = logFile.Close()
	})
	p := &drillProbe{cmd: cmd, logPath: logPath}
	p.waitServing(t)
	return p
}

func (p *drillProbe) waitServing(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(p.logPath)
		if err == nil && bytes.Contains(raw, []byte("serving function")) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	raw, _ := os.ReadFile(p.logPath)
	t.Fatalf("probe 未在期限内打出 serving function（log: %s）", raw)
}

// drillAPI 是带 Bearer + game scope 头的 HTTP 客户端。
type drillAPI struct {
	base   string
	token  string
	client *http.Client
}

func drillLogin(t *testing.T, base string) *drillAPI {
	t.Helper()
	client := &http.Client{Timeout: 15 * time.Second}
	body, _ := json.Marshal(map[string]string{"username": drillAdminUser, "password": drillAdminPass})
	deadline := time.Now().Add(20 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("login status %d: %s", resp.StatusCode, raw)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		var out struct {
			Token string `json:"token"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.NotEmpty(t, out.Token, "login 未返回 token: %s", raw)
		return &drillAPI{base: base, token: out.Token, client: client}
	}
	t.Fatalf("login 从未成功: %v", lastErr)
	return nil
}

func (a *drillAPI) do(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, a.base+path, rd)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("X-Game-ID", drillGameID)
	req.Header.Set("X-Env", drillEnv)
	resp, err := a.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}

func (a *drillAPI) startTask(t *testing.T) string {
	t.Helper()
	code, raw := a.do(t, http.MethodPost, "/api/v1/tasks", map[string]any{
		"functionId": drillFunctionID,
		"gameId":     drillGameID,
		"env":        drillEnv,
	})
	require.Equal(t, http.StatusOK, code, "startTask: %s", raw)
	var out struct {
		TaskID string `json:"taskId"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotEmpty(t, out.TaskID, "startTask 未返回 taskId: %s", raw)
	return out.TaskID
}

type drillDetail struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	FinishedAt string `json:"finishedAt"`
}

// waitStatus 轮询任务详情直到 status==want（返回终值详情）。
func (a *drillAPI) waitStatus(t *testing.T, taskID string, timeout time.Duration, want string) drillDetail {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last drillDetail
	for time.Now().Before(deadline) {
		code, raw := a.do(t, http.MethodGet, "/api/v1/tasks/"+taskID, nil)
		if code == http.StatusOK {
			require.NoError(t, json.Unmarshal(raw, &last))
			if last.Status == want {
				return last
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("task %s 未在 %s 内到 %s（最后 status=%q msg=%q finishedAt=%q）",
		taskID, timeout, want, last.Status, last.Message, last.FinishedAt)
	return last
}

// waitEvent 轮询任务事件直到出现指定 type。
func (a *drillAPI) waitEvent(t *testing.T, taskID, eventType string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var seen []string
	for time.Now().Before(deadline) {
		code, raw := a.do(t, http.MethodGet, "/api/v1/tasks/"+taskID+"/events", nil)
		if code == http.StatusOK {
			var out struct {
				Items []struct {
					Type string `json:"type"`
				} `json:"items"`
			}
			require.NoError(t, json.Unmarshal(raw, &out))
			seen = seen[:0]
			for _, it := range out.Items {
				seen = append(seen, it.Type)
				if it.Type == eventType {
					return
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("task %s: 事件 %q 未在 %s 内出现（已见 %v）", taskID, eventType, timeout, seen)
}

// drillSeedFunction 直插函数元数据行（REST 入口 FindByFunctionID 需要）。
// 在就绪行之后调用（表已由服务端 AutoMigrate 建出）。
func drillSeedFunction(t *testing.T, dbPath string) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(dbPath+"?_pragma=busy_timeout(5000)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	fn := model.Function{
		FunctionID: drillFunctionID,
		Name:       "Drill Echo",
		GameID:     drillGameID,
		Version:    "1.0.0",
		Status:     1,
	}
	require.NoError(t, db.Clauses(clause.OnConflict{DoNothing: true}).Create(&fn).Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

// drillReadStatusDB 在停机（连接池已关）后以独立连接读任务终值。
func drillReadStatusDB(t *testing.T, dbPath, taskID string) model.TaskRun {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(dbPath+"?_pragma=busy_timeout(5000)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	var run model.TaskRun
	require.NoError(t, db.Where("task_id = ?", taskID).First(&run).Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return run
}

// --- 场景 1：断链（kill 探针）→ running 收敛 unknown，新请求 503 ---

func TestDrill_AgentKilled_MarksUnknown(t *testing.T) {
	s := drillBoot(t)
	api := drillLogin(t, s.base)
	drillSeedFunction(t, s.dbPath)

	p := s.spawnProbe(t, "drill-agent-kill", "-step-ms", "500")
	taskID := api.startTask(t)
	api.waitEvent(t, taskID, "started", 15*time.Second)

	// 断链注入：杀探针进程 → 服务端 serveConn EOF → 会话移除 → 断连收割。
	require.NoError(t, p.cmd.Process.Kill())

	// 收敛：running → unknown + 断连消息（执行状态未确立，不得自动重跑）。
	detail := api.waitStatus(t, taskID, 15*time.Second, tasks.StatusUnknown)
	require.Equal(t, "agent session lost; execution state unknown", detail.Message)
	require.NotEmpty(t, detail.FinishedAt, "unknown 收割须置 finished_at")

	// 新请求拒绝：唯一 agent 已死，路由无候选 → 503（新请求拒绝记录）。
	code, raw := api.do(t, http.MethodPost, "/api/v1/tasks", map[string]any{
		"functionId": drillFunctionID, "gameId": drillGameID, "env": drillEnv,
	})
	require.Equal(t, http.StatusServiceUnavailable, code, "断链后新请求应 503: %s", raw)
}

// --- 场景 2：drain → 在途完成 + 新请求拒绝；undrain 后恢复 ---

func TestDrill_Drain_InflightCompletes_NewRejected(t *testing.T) {
	s := drillBoot(t)
	api := drillLogin(t, s.base)
	drillSeedFunction(t, s.dbPath)

	s.spawnProbe(t, "drill-agent-drain", "-step-ms", "300")
	t1 := api.startTask(t)
	api.waitEvent(t, t1, "started", 15*time.Second)

	// drain 注入：注册表路由门先停新请求（在途由探针继续完成）。
	code, raw := api.do(t, http.MethodPost, "/api/v1/ops/nodes/drill-agent-drain/drain",
		map[string]string{"nodeId": "drill-agent-drain"})
	require.Equal(t, http.StatusOK, code, "drain: %s", raw)

	// 新请求拒绝：唯一节点已 draining → 无候选 → 503。
	code, raw = api.do(t, http.MethodPost, "/api/v1/tasks", map[string]any{
		"functionId": drillFunctionID, "gameId": drillGameID, "env": drillEnv,
	})
	require.Equal(t, http.StatusServiceUnavailable, code, "drain 期间新请求应 503: %s", raw)

	// 在途不打断：探针继续流事件 → 终态 succeeded。
	detail := api.waitStatus(t, t1, 15*time.Second, tasks.StatusSucceeded)
	require.NotEmpty(t, detail.FinishedAt)

	// undrain → 新请求恢复，成功完成。
	code, raw = api.do(t, http.MethodPost, "/api/v1/ops/nodes/drill-agent-drain/undrain",
		map[string]string{"nodeId": "drill-agent-drain"})
	require.Equal(t, http.StatusOK, code, "undrain: %s", raw)
	t2 := api.startTask(t)
	api.waitStatus(t, t2, 15*time.Second, tasks.StatusSucceeded)
}

// --- 场景 3：看门狗收割 hang 任务 → timed_out（agent 存活，非断连所致） ---

func TestDrill_Watchdog_ReapsHangingToTimedOut(t *testing.T) {
	s := drillBoot(t)
	api := drillLogin(t, s.base)
	drillSeedFunction(t, s.dbPath)

	// -hang：started 后永不发 terminal，会话保持存活（无断连收割），
	// 只能由 D3 看门狗按 reaperTimeout=4s（tick=2s）收割为 timed_out。
	s.spawnProbe(t, "drill-agent-hang", "-hang")
	taskID := api.startTask(t)
	anchor := time.Now() // 锚点：started 事件写入（bump updated_at）前后。
	api.waitEvent(t, taskID, "started", 15*time.Second)

	detail := api.waitStatus(t, taskID, 20*time.Second, tasks.StatusTimedOut)
	elapsed := time.Since(anchor)

	// 截止时间：不早于预算（无事件满预算才转）且不显著晚于预算+tick+轮询。
	// 下界 3s 容忍锚点 slop（started 写入可能先于 anchor 毫秒级），仍区分
	// 立即收割的病态（≤1s）；上界 12s 远大于预算+tick（4+2s）留调度裕量。
	require.GreaterOrEqual(t, elapsed, 3*time.Second,
		"timed_out 早于看门狗预算：收割谓词失效（预算未生效）")
	require.LessOrEqual(t, elapsed, 12*time.Second,
		"timed_out 显著晚于预算+tick：收割循环失灵")
	require.NotEmpty(t, detail.FinishedAt, "timed_out 必须置 finished_at")

	// agent 仍存活：timed_out 是「无证据」终态，非断连所致——新请求照常派发。
	code, raw := api.do(t, http.MethodPost, "/api/v1/tasks", map[string]any{
		"functionId": drillFunctionID, "gameId": drillGameID, "env": drillEnv,
	})
	require.Equal(t, http.StatusOK, code, "hang 探针存活时新请求应 200: %s", raw)
}

// --- 场景 4：SIGTERM → 优雅停机 + 在途收敛 unknown + 资源释放 ---

func TestDrill_Sigterm_InFlightUnknown_ResourcesReleased(t *testing.T) {
	s := drillBoot(t)
	api := drillLogin(t, s.base)
	drillSeedFunction(t, s.dbPath)

	s.spawnProbe(t, "drill-agent-sigterm", "-step-ms", "500")
	taskID := api.startTask(t)
	api.waitEvent(t, taskID, "started", 15*time.Second)

	// SIGTERM 注入：runServer 自己的 Notify 捕获后走 ARCH-017 优雅停机。
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
	runErr := s.waitExit(t, 30*time.Second)
	require.NoError(t, runErr, "SIGTERM 后 runServer 须走完优雅停机并返回 nil")

	// 资源释放：HTTP 监听已关（无新请求可接入）。
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(s.base, "http://"), 2*time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("停机后 HTTP 端口仍可连接：监听未释放")
	}

	// 停机路径断连收割已落库：在途任务 → unknown（独立连接读终值，池已关）。
	run := drillReadStatusDB(t, s.dbPath, taskID)
	require.Equal(t, tasks.StatusUnknown, run.Status)
	require.Equal(t, "agent session lost; execution state unknown", run.Message)
	require.NotNil(t, run.FinishedAt)
}
