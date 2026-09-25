package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/approvals"
)

// captureStdout 把 fn 期间写到 os.Stdout 的内容收回来做断言。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	defer func() { os.Stdout = orig }()

	fn()

	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// faultStore 在真实 MemStore 之上按「方法名 -> 第 N 次调用」注入一次错误，
// 用于逐条覆盖 demonstrateStore 的错误分支。
type faultStore struct {
	approvals.Store
	failAt map[string]int
	calls  map[string]int
}

func newFaultStore(failAt map[string]int) *faultStore {
	return &faultStore{
		Store:  approvals.NewMemStore(),
		failAt: failAt,
		calls:  map[string]int{},
	}
}

func (f *faultStore) step(name string) error {
	f.calls[name]++
	if n, ok := f.failAt[name]; ok && f.calls[name] == n {
		return fmt.Errorf("injected %s failure", name)
	}
	return nil
}

func (f *faultStore) Create(a *approvals.Approval) (*approvals.Approval, error) {
	if err := f.step("Create"); err != nil {
		return nil, err
	}
	return f.Store.Create(a)
}

func (f *faultStore) Get(id string) (*approvals.Approval, error) {
	if err := f.step("Get"); err != nil {
		return nil, err
	}
	return f.Store.Get(id)
}

func (f *faultStore) List(filter approvals.Filter, page approvals.Page) ([]*approvals.Approval, int, error) {
	if err := f.step("List"); err != nil {
		return nil, 0, err
	}
	return f.Store.List(filter, page)
}

func (f *faultStore) Approve(id, operator string) (*approvals.Approval, error) {
	if err := f.step("Approve"); err != nil {
		return nil, err
	}
	return f.Store.Approve(id, operator)
}

func (f *faultStore) Reject(id, reason, operator string) (*approvals.Approval, error) {
	if err := f.step("Reject"); err != nil {
		return nil, err
	}
	return f.Store.Reject(id, reason, operator)
}

func TestRunMissingArgsPrintsUsage(t *testing.T) {
	cases := [][]string{
		{"approvals-storage"},
		nil,
	}
	for _, args := range cases {
		var err error
		out := captureStdout(t, func() { err = run(args) })
		if !errors.Is(err, errUsage) {
			t.Fatalf("run(%v) err = %v, want errUsage", args, err)
		}
		for _, want := range []string{
			"用法:",
			"<存储类型> [DSN]",
			"  mem     - 内存存储（默认）",
			"  sqlite  - SQLite 存储",
			"  pg      - PostgreSQL 存储",
			"mem\n",
			"sqlite data/approvals.db",
			"pg postgres://user:pass@localhost:5432/db?sslmode=disable",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("usage output missing %q, got:\n%s", want, out)
			}
		}
	}
}

func TestRunMemDemonstratesStore(t *testing.T) {
	var err error
	out := captureStdout(t, func() { err = run([]string{"approvals-storage", "mem"}) })
	if err != nil {
		t.Fatalf("run(mem) = %v", err)
	}
	for _, want := range []string{
		"使用内存存储",
		"=== 演示 Approvals 存储操作 ===",
		"1. 创建审批请求:",
		"   状态: pending",
		"   功能: player.ban",
		"2. 获取审批请求",
		"3. 列出所有待审批请求",
		"   总数: 1",
		"   当前页: 1",
		"5. 过滤查询演示",
		"   game-001 的审批: 2",
		"   已批准的审批: 1",
		"6. 执行审批操作",
		"   新状态: approved",
		"   新状态: rejected",
		"   拒绝原因: 缺少必要信息",
		"7. 最终统计",
		"   待审批: 0",
		"   已批准: 1",
		"   已拒绝: 1",
		"   总计: 3",
		"=== 演示完成 ===",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("mem demo output missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunSQLiteExplicitDSN(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "approvals.db")
	var err error
	out := captureStdout(t, func() { err = run([]string{"approvals-storage", "sqlite", dsn}) })
	if err != nil {
		t.Fatalf("run(sqlite) = %v", err)
	}
	if !strings.Contains(out, "使用 SQLite 存储: "+dsn) {
		t.Fatalf("missing sqlite banner, got:\n%s", out)
	}
	if !strings.Contains(out, "=== 演示完成 ===") {
		t.Fatalf("demo did not finish, got:\n%s", out)
	}
	if _, statErr := os.Stat(dsn); statErr != nil {
		t.Fatalf("sqlite db not created: %v", statErr)
	}
}

func TestRunSQLiteDefaultDSN(t *testing.T) {
	t.Chdir(t.TempDir())
	var err error
	out := captureStdout(t, func() { err = run([]string{"approvals-storage", "sqlite"}) })
	if err != nil {
		t.Fatalf("run(sqlite without dsn) = %v", err)
	}
	if !strings.Contains(out, "使用 SQLite 存储: "+filepath.Join("data", "approvals.db")) {
		t.Fatalf("missing default dsn banner, got:\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join("data", "approvals.db")); statErr != nil {
		t.Fatalf("default sqlite db not created under cwd: %v", statErr)
	}
}

func TestRunSQLiteOpenError(t *testing.T) {
	dsn := t.TempDir() // 目录不能当数据库文件
	err := run([]string{"approvals-storage", "sqlite", dsn})
	if err == nil {
		t.Fatal("expected sqlite open error, got nil")
	}
	if !strings.Contains(err.Error(), "创建 SQLite 存储失败") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUnknownStoreType(t *testing.T) {
	err := run([]string{"approvals-storage", "redis"})
	if err == nil {
		t.Fatal("expected unsupported store type error, got nil")
	}
	if !strings.Contains(err.Error(), "不支持的存储类型: redis") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPGRequiresDSN(t *testing.T) {
	err := run([]string{"approvals-storage", "pg"})
	if err == nil {
		t.Fatal("expected missing dsn error, got nil")
	}
	if errors.Is(err, errUsage) {
		t.Fatalf("pg without dsn must not be treated as usage: %v", err)
	}
	if err.Error() != "PostgreSQL 需要提供 DSN" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPGBadDSN(t *testing.T) {
	dsn := "postgres://croupier:croupier@127.0.0.1:1/croupier?sslmode=disable&connect_timeout=1"
	start := time.Now()
	err := run([]string{"approvals-storage", "pg", dsn})
	if err == nil {
		t.Fatal("expected postgres open error, got nil")
	}
	if !strings.Contains(err.Error(), "创建 PostgreSQL 存储失败") {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("postgres failure took too long: %v", elapsed)
	}
}

func TestDemonstrateStoreFaults(t *testing.T) {
	cases := []struct {
		name    string
		failAt  map[string]int
		wantErr string
	}{
		{"create", map[string]int{"Create": 1}, "创建失败"},
		{"get", map[string]int{"Get": 1}, "获取失败"},
		{"list pending", map[string]int{"List": 1}, "列表查询失败"},
		{"list by game", map[string]int{"List": 2}, "按游戏过滤失败"},
		{"list by state", map[string]int{"List": 3}, "按状态过滤失败"},
		{"approve", map[string]int{"Approve": 1}, "批准失败"},
		{"reject", map[string]int{"Reject": 1}, "拒绝失败"},
		{"list total", map[string]int{"List": 7}, "获取总数失败"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := demonstrateStore(newFaultStore(tc.failAt))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestDemonstrateStoreTestDataCreateFailure 覆盖第 4 步造测试数据失败的
// 非致命分支（log.Printf 后继续），演示整体仍应成功。第 3 次 Create 即
// testApprovals[1]：跳过它只影响「已批准」计数，不阻断后续 Reject/统计。
func TestDemonstrateStoreTestDataCreateFailure(t *testing.T) {
	var logBuf strings.Builder
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	var err error
	captureStdout(t, func() {
		err = demonstrateStore(newFaultStore(map[string]int{"Create": 3}))
	})
	if err != nil {
		t.Fatalf("demonstrateStore = %v, want nil (non-fatal branch)", err)
	}
	if !strings.Contains(logBuf.String(), "创建测试数据失败") {
		t.Fatalf("missing non-fatal log, got: %q", logBuf.String())
	}
}

// ── 进程内最小 PostgreSQL wire 桩（仅回环、无外部依赖）────────────────
// NewPGStore 的 openDB 只需握手 + gorm 自动 ping 成功即可走到
// 「使用 PostgreSQL 存储」打印与 NewSQLStore 构造；后续真实 SQL 走扩展
// 协议时桩直接断开，demonstrateStore 因此返回错误（本用例只断言 pg
// 成功分支的输出语义，不要求演示跑完）。

func fakePGWireServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveFakePGConn(conn)
		}
	}()

	return "postgres://croupier:croupier@127.0.0.1:" + strconv.Itoa(ln.Addr().(*net.TCPAddr).Port) +
		"/croupier?sslmode=disable&connect_timeout=2"
}

func serveFakePGConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	startup, err := readPGUntypedFrame(conn)
	if err != nil {
		return
	}
	if len(startup) >= 4 && binary.BigEndian.Uint32(startup[0:4]) == 80877103 {
		if _, err := conn.Write([]byte{'N'}); err != nil {
			return
		}
		if _, err := readPGUntypedFrame(conn); err != nil {
			return
		}
	}

	writePGMessage(conn, 'R', func(b []byte) []byte {
		return append(b, 0, 0, 0, 0)
	})
	writePGParameterStatus(conn, "server_version", "16.2")
	writePGParameterStatus(conn, "client_encoding", "UTF8")
	writePGMessage(conn, 'K', func(b []byte) []byte {
		return append(b, 0, 0, 0, 1, 0, 0, 0, 1)
	})
	writePGMessage(conn, 'Z', func(b []byte) []byte { return append(b, 'I') })

	for {
		msgType, _, err := readPGTypedFrame(conn)
		if err != nil {
			return
		}
		switch msgType {
		case 'Q':
			writePGMessage(conn, 'C', func(b []byte) []byte { return append(b, 0) })
			writePGMessage(conn, 'Z', func(b []byte) []byte { return append(b, 'I') })
		case 'X':
			return
		default:
			return
		}
	}
}

func readPGUntypedFrame(conn net.Conn) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := readFull(conn, lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf[:])
	if length < 4 || length > 1<<20 {
		return nil, errFakePGProtocol
	}
	body := make([]byte, length-4)
	if _, err := readFull(conn, body); err != nil {
		return nil, err
	}
	return body, nil
}

func readPGTypedFrame(conn net.Conn) (byte, []byte, error) {
	var head [5]byte
	if _, err := readFull(conn, head[:]); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(head[1:5])
	if length < 4 || length > 1<<20 {
		return 0, nil, errFakePGProtocol
	}
	body := make([]byte, length-4)
	if _, err := readFull(conn, body); err != nil {
		return 0, nil, err
	}
	return head[0], body, nil
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if n > 0 {
			total += n
		}
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

type fakePGError struct{}

func (fakePGError) Error() string { return "fake pg protocol error" }

var errFakePGProtocol = fakePGError{}

func writePGMessage(conn net.Conn, msgType byte, build func([]byte) []byte) {
	body := build(nil)
	payload := make([]byte, 0, 5+len(body))
	payload = append(payload, msgType)
	payload = append(payload, pgUint32(uint32(4+len(body)))...)
	payload = append(payload, body...)
	_, _ = conn.Write(payload)
}

func writePGParameterStatus(conn net.Conn, key, value string) {
	writePGMessage(conn, 'S', func(b []byte) []byte {
		b = append(b, key...)
		b = append(b, 0)
		b = append(b, value...)
		return append(b, 0)
	})
}

func pgUint32(v uint32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// TestRunPGStoreSuccessBranch 覆盖 pg 分支的 NewPGStore 成功路径
// （「使用 PostgreSQL 存储: ...」打印）。
func TestRunPGStoreSuccessBranch(t *testing.T) {
	dsn := fakePGWireServer(t)
	var err error
	out := captureStdout(t, func() { err = run([]string{"approvals-storage", "pg", dsn}) })
	if !strings.Contains(out, "使用 PostgreSQL 存储: "+dsn) {
		t.Fatalf("pg success banner missing, got:\n%s", out)
	}
	if err == nil {
		t.Fatal("demonstrateStore against the wire stub is expected to fail")
	}
	if !strings.Contains(err.Error(), "创建失败") && !strings.Contains(err.Error(), "获取失败") {
		t.Fatalf("unexpected error after pg banner: %v", err)
	}
}

// ── main 进程入口覆盖：以子进程方式重入测试二进制执行 main() ──────────

const mainHelperEnv = "CROUPIER_APPROVALS_MAIN_HELPER"

// TestMainHelperProcess 仅在子进程中被 -test.run 选中执行：按环境变量切换
// os.Args 形态后直接调用 main()（main 内 os.Exit 不会返回）。
func TestMainHelperProcess(t *testing.T) {
	switch os.Getenv(mainHelperEnv) {
	case "usage":
		os.Args = os.Args[:1]
	case "unknown":
		os.Args = append(os.Args[:1], "redis")
	default:
		return
	}
	main()
}

func TestMainProcessForwardsExit(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		wantErr int
		wantOut string
		wantLog string
	}{
		{name: "usage", mode: "usage", wantErr: 1, wantOut: "用法:"},
		{name: "unknown type", mode: "unknown", wantErr: 1, wantLog: "不支持的存储类型: redis"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
			cmd.Env = append(os.Environ(), mainHelperEnv+"="+tc.mode)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("child err = %v, want exit status %d", err, tc.wantErr)
			}
			if code := exitErr.ExitCode(); code != tc.wantErr {
				t.Fatalf("exit code = %d, want %d (stderr=%q stdout=%q)", code, tc.wantErr, stderr.String(), stdout.String())
			}
			if tc.wantOut != "" && !strings.Contains(stdout.String(), tc.wantOut) {
				t.Fatalf("stdout missing %q, got %q", tc.wantOut, stdout.String())
			}
			if tc.wantLog != "" && !strings.Contains(stderr.String(), tc.wantLog) {
				t.Fatalf("stderr missing %q, got %q", tc.wantLog, stderr.String())
			}
		})
	}
}
