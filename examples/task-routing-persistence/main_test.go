package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/registry"
)

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

// faultRoutingStore 在真实内存 store 之上按「方法名 -> 第 N 次调用」注入
// 一次错误，用于覆盖 demonstrate 的错误分支与 Get miss 分支。
type faultRoutingStore struct {
	dispatch.TaskRoutingStore
	failAt map[string]int
	calls  map[string]int
}

func newFaultRoutingStore(failAt map[string]int) *faultRoutingStore {
	return &faultRoutingStore{
		TaskRoutingStore: dispatch.NewMemoryTaskRoutingStore(),
		failAt:           failAt,
		calls:            map[string]int{},
	}
}

func (f *faultRoutingStore) step(name string) error {
	f.calls[name]++
	if n, ok := f.failAt[name]; ok && f.calls[name] == n {
		return errors.New("injected " + name + " failure")
	}
	return nil
}

func (f *faultRoutingStore) Get(taskID string) (*dispatch.TaskRouting, error) {
	if err := f.step("Get"); err != nil {
		return nil, err
	}
	return f.TaskRoutingStore.Get(taskID)
}

func (f *faultRoutingStore) List() ([]*dispatch.TaskRouting, error) {
	if err := f.step("List"); err != nil {
		return nil, err
	}
	return f.TaskRoutingStore.List()
}

func (f *faultRoutingStore) Cleanup(ttl time.Duration) error {
	if err := f.step("Cleanup"); err != nil {
		return err
	}
	return f.TaskRoutingStore.Cleanup(ttl)
}

func TestRunDemonstratesRoutingPersistence(t *testing.T) {
	dir := t.TempDir()
	var err error
	out := captureStdout(t, func() { err = run(dir) })
	if err != nil {
		t.Fatalf("run(%s) = %v", dir, err)
	}
	for _, want := range []string{
		"=== Task Routing Persistence Demo ===",
		"1. Registering task routes...",
		"   Registered task task-001 -> agent-1",
		"   Registered task task-003 -> agent-3",
		"2. Listing all task routes...",
		"   Task: task-002, Agent: agent-2, Created: ",
		"3. Testing task route lookup...",
		"   Found task task-001 -> agent-1",
		"4. Simulating task completion...",
		"   Unregistered task task-003",
		"5. Checking remaining tasks...",
		"   Remaining tasks: 1",
		"6. Testing cleanup of old tasks...",
		"   Tasks after cleanup: 0",
		"=== Demo Complete ===",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q, got:\n%s", want, out)
		}
	}
	if _, statErr := os.Stat(filepath.Join(dir, "task_routing.json")); statErr != nil {
		t.Fatalf("routing file not persisted: %v", statErr)
	}
}

func TestRunRoutingStoreCreateError(t *testing.T) {
	dir := t.TempDir()
	// data 是普通文件 → MkdirAll 失败
	if err := os.WriteFile(filepath.Join(dir, "data"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := run(filepath.Join(dir, "data"))
	if err == nil {
		t.Fatal("expected store creation error, got nil")
	}
	if !strings.Contains(err.Error(), "Failed to create task routing store") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRoutingStoreCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "task_routing.json"), []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	err := run(dir)
	if err == nil {
		t.Fatal("expected load error, got nil")
	}
	if !strings.Contains(err.Error(), "Failed to create task routing store") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDemonstrateFaults(t *testing.T) {
	// 构造 Dispatcher 时 loadTaskRouting() 已消费 List 第 1 次调用，
	// demonstrate 内的两次 List 分别是第 2、3 次。
	t.Run("list first", func(t *testing.T) {
		err := demonstrate(registry.NewStore(), newFaultRoutingStore(map[string]int{"List": 2}))
		if err == nil || !strings.Contains(err.Error(), "Failed to list task routes") {
			t.Fatalf("err = %v, want list failure", err)
		}
	})
	t.Run("list second", func(t *testing.T) {
		err := demonstrate(registry.NewStore(), newFaultRoutingStore(map[string]int{"List": 3}))
		if err == nil || !strings.Contains(err.Error(), "Failed to list task routes") {
			t.Fatalf("err = %v, want list failure", err)
		}
	})
	t.Run("cleanup", func(t *testing.T) {
		err := demonstrate(registry.NewStore(), newFaultRoutingStore(map[string]int{"Cleanup": 1}))
		if err == nil || !strings.Contains(err.Error(), "Failed to cleanup old tasks") {
			t.Fatalf("err = %v, want cleanup failure", err)
		}
	})
	t.Run("get miss", func(t *testing.T) {
		var err error
		out := captureStdout(t, func() {
			err = demonstrate(registry.NewStore(), newFaultRoutingStore(map[string]int{"Get": 2}))
		})
		if err != nil {
			t.Fatalf("demonstrate = %v, want nil", err)
		}
		if !strings.Contains(out, "   Task task-002 not found\n") {
			t.Fatalf("missing not-found branch output, got:\n%s", out)
		}
		if !strings.Contains(out, "=== Demo Complete ===") {
			t.Fatalf("demo did not finish, got:\n%s", out)
		}
	})
}

func TestDemonstrateMemoryStore(t *testing.T) {
	var err error
	out := captureStdout(t, func() {
		err = demonstrate(registry.NewStore(), dispatch.NewMemoryTaskRoutingStore())
	})
	if err != nil {
		t.Fatalf("demonstrate = %v", err)
	}
	if !strings.Contains(out, "=== Demo Complete ===") {
		t.Fatalf("demo did not finish, got:\n%s", out)
	}
}

// ── main 进程入口覆盖：子进程重入测试二进制执行 main() ─────────────────

const mainHelperEnv = "CROUPIER_TASK_ROUTING_MAIN_HELPER"

func TestMainHelperProcess(t *testing.T) {
	mode := os.Getenv(mainHelperEnv)
	if mode == "" {
		return
	}
	dir, err := os.MkdirTemp("", "task-routing-main-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if mode == "blocked" {
		// data 是普通文件 → NewFileTaskRoutingStore 的 MkdirAll 失败
		if err := os.WriteFile(filepath.Join(dir, "data"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	main()
}

func TestMainProcessForwardsExit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stdout, stderr, code := runMainHelper(t, "ok")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr=%q stdout=%q)", code, stderr, stdout)
		}
		if !strings.Contains(stdout, "=== Demo Complete ===") {
			t.Fatalf("stdout missing demo completion, got:\n%s", stdout)
		}
	})
	t.Run("failure", func(t *testing.T) {
		stdout, stderr, code := runMainHelper(t, "blocked")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1 (stderr=%q stdout=%q)", code, stderr, stdout)
		}
		if !strings.Contains(stderr, "Failed to create task routing store") {
			t.Fatalf("stderr missing fatal message, got %q", stderr)
		}
	})
}

func runMainHelper(t *testing.T, mode string) (string, string, int) {
	t.Helper()
	bin, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	cmd := exec.Command(bin, "-test.run=^TestMainHelperProcess$")
	cmd.Env = append(os.Environ(), mainHelperEnv+"="+mode)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("child run: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}
