package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/tasks"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func newRealStore(t *testing.T) *tasks.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TaskRun{}, &model.TaskEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return tasks.NewStore(model.NewTaskRunModel(db), model.NewTaskEventModel(db))
}

// faultStore 在真实 tasks.Store 之上按「方法名 -> 第 N 次调用」注入一次错误，
// 用于逐条覆盖 demonstrate 的错误分支。
type faultStore struct {
	taskStore
	failAt map[string]int
	calls  map[string]int
}

func newFaultStore(t *testing.T, failAt map[string]int) *faultStore {
	t.Helper()
	return &faultStore{
		taskStore: newRealStore(t),
		failAt:    failAt,
		calls:     map[string]int{},
	}
}

func (f *faultStore) step(name string) error {
	f.calls[name]++
	if n, ok := f.failAt[name]; ok && f.calls[name] == n {
		return errors.New("injected " + name + " failure")
	}
	return nil
}

func (f *faultStore) CreateRun(ctx context.Context, run *model.TaskRun) error {
	if err := f.step("CreateRun"); err != nil {
		return err
	}
	return f.taskStore.CreateRun(ctx, run)
}

func (f *faultStore) GetRun(ctx context.Context, taskID string) (*model.TaskRun, error) {
	if err := f.step("GetRun"); err != nil {
		return nil, err
	}
	return f.taskStore.GetRun(ctx, taskID)
}

func (f *faultStore) UpdateRun(ctx context.Context, taskID string, updates map[string]interface{}) error {
	if err := f.step("UpdateRun"); err != nil {
		return err
	}
	return f.taskStore.UpdateRun(ctx, taskID, updates)
}

func (f *faultStore) AppendEvent(ctx context.Context, taskID string, eventType tasks.EventType, progress int32, message string, payload []byte) error {
	if err := f.step("AppendEvent"); err != nil {
		return err
	}
	return f.taskStore.AppendEvent(ctx, taskID, eventType, progress, message, payload)
}

func (f *faultStore) ListEvents(ctx context.Context, taskID string, afterSeq int64) ([]model.TaskEvent, error) {
	if err := f.step("ListEvents"); err != nil {
		return nil, err
	}
	return f.taskStore.ListEvents(ctx, taskID, afterSeq)
}

func TestRunHappyPath(t *testing.T) {
	var err error
	out := captureStdout(t, func() { err = run(":memory:") })
	if err != nil {
		t.Fatalf("run(:memory:) = %v", err)
	}
	for _, want := range []string{
		"=== Task Result Tracking Demo ===",
		"1. Simulating task execution...",
		"   Started task task-001 (status: queued)",
		"   Started task task-003 (status: queued)",
		"2. Simulating task progress...",
		"   Updated task task-003 (status: failed)",
		"3. Querying task runs...",
		"   Task task-001: running (25%)",
		"   Task task-003: failed (0%)",
		"     Error: connection timeout",
		"4. Completing remaining tasks...",
		"   Completed task task-001",
		"5. Event history for task-001:",
		"#1 queued 0% - task queued",
		"#3 completed 100% - task completed",
		"=== Demo Complete ===",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("happy path output missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunOpenDatabaseError(t *testing.T) {
	err := run(t.TempDir()) // 目录不能当 sqlite 文件
	if err == nil {
		t.Fatal("expected open error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to open database") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunMigrateError(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "poison.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// task_runs 先被占成视图：AutoMigrate 的 CREATE TABLE 会失败。
	if err := db.Exec("CREATE VIEW task_runs AS SELECT 1 AS id").Error; err != nil {
		t.Fatalf("create view: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	_ = sqlDB.Close()

	err = run(dsn)
	if err == nil {
		t.Fatal("expected migrate error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to migrate task tables") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDemonstrateFaults(t *testing.T) {
	cases := []struct {
		name    string
		failAt  map[string]int
		wantErr string
		wantOut string
	}{
		{name: "create run", failAt: map[string]int{"CreateRun": 1}, wantErr: "failed to create task run"},
		{name: "append queued event", failAt: map[string]int{"AppendEvent": 1}, wantErr: "failed to append queued event"},
		{name: "update progress", failAt: map[string]int{"UpdateRun": 1}, wantErr: "failed to update task run"},
		{name: "append progress event", failAt: map[string]int{"AppendEvent": 4}, wantErr: "failed to append progress event"},
		{name: "get run missing", failAt: map[string]int{"GetRun": 1}, wantOut: "   Task task-001: not found\n"},
		{name: "complete run", failAt: map[string]int{"UpdateRun": 4}, wantErr: "failed to complete task run"},
		{name: "append completion event", failAt: map[string]int{"AppendEvent": 7}, wantErr: "failed to append completion event"},
		{name: "list events", failAt: map[string]int{"ListEvents": 1}, wantErr: "failed to list task events"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFaultStore(t, tc.failAt)
			var err error
			out := captureStdout(t, func() { err = demonstrate(store) })
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("demonstrate = %v, want nil", err)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Fatalf("output missing %q, got:\n%s", tc.wantOut, out)
			}
		})
	}
}

func TestDemonstrateRealStore(t *testing.T) {
	var err error
	out := captureStdout(t, func() { err = demonstrate(newRealStore(t)) })
	if err != nil {
		t.Fatalf("demonstrate = %v", err)
	}
	if !strings.Contains(out, "=== Demo Complete ===") {
		t.Fatalf("demo did not finish, got:\n%s", out)
	}
}

// ── main 进程入口覆盖：子进程重入测试二进制执行 main() ─────────────────

const mainHelperEnv = "CROUPIER_JOB_RESULT_MAIN_HELPER"

func TestMainHelperProcess(t *testing.T) {
	switch os.Getenv(mainHelperEnv) {
	case "ok":
		demoDSN = ":memory:"
	case "bad":
		demoDSN = t.TempDir() // 目录当 DSN → 打开失败
	default:
		return
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
		stdout, stderr, code := runMainHelper(t, "bad")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1 (stderr=%q stdout=%q)", code, stderr, stdout)
		}
		if !strings.Contains(stderr, "failed to open database") {
			t.Fatalf("stderr missing fatal message, got %q", stderr)
		}
	})
}

func runMainHelper(t *testing.T, mode string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
	cmd.Env = append(os.Environ(), mainHelperEnv+"="+mode)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
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
