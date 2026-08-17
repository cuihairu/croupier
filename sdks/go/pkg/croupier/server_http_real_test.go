package croupier

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestHTTPInvoker_RealServerLifecycle starts the repository's isolated
// Server/Agent/SDK fixture as a separate process, then uses the public Go L3
// invoker to verify the complete Server HTTP lifecycle. Keeping the fixture
// out of this process is intentional: the Server and Go SDK each own generated
// protobuf packages, while production uses them in separate processes too.
func TestHTTPInvoker_RealServerLifecycle(t *testing.T) {
	if os.Getenv("CROUPIER_RUN_REAL_SERVER_TESTS") != "1" {
		t.Skip("real Server integration test disabled; set CROUPIER_RUN_REAL_SERVER_TESTS=1")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	fixture := startRealServerFixture(t, ctx)
	defer fixture.close(t)

	token := fixture.login(t, ctx)
	invoker := NewHTTPInvoker(&InvokerConfig{
		Address:          fixture.serverURL(),
		AuthToken:        token,
		GameID:           fixture.ready.GameID,
		Env:              fixture.ready.Env,
		DefaultTimeout:   10 * time.Second,
		Reconnect:        &ReconnectConfig{Enabled: false},
		Retry:            &RetryConfig{Enabled: false},
		TaskPollInterval: 10 * time.Millisecond,
	})
	t.Cleanup(func() { _ = invoker.Close() })
	requireNoError(t, invoker.Connect(ctx))

	// A request without the fixture's bearer token must be rejected by the
	// real Server; successful calls below therefore cannot bypass auth.
	unauthenticated := NewHTTPInvoker(&InvokerConfig{
		Address: fixture.serverURL(),
		GameID:  fixture.ready.GameID,
		Env:     fixture.ready.Env,
		Retry:   &RetryConfig{Enabled: false},
	})
	_, err := unauthenticated.Invoke(ctx, "mail.send", `{"player_id":"p-001","title":"denied"}`, InvokeOptions{})
	if err == nil {
		t.Fatal("unauthenticated Server invoke unexpectedly succeeded")
	}

	result, err := invoker.Invoke(ctx, "mail.send", `{"player_id":"p-001","title":"Hello","content":"body"}`, InvokeOptions{
		IdempotencyKey: "go-l3-real-invoke",
	})
	requireNoError(t, err)
	if !strings.Contains(result, `"mail_id":"mail-0001"`) || !strings.Contains(result, `"title":"Hello"`) {
		t.Fatalf("unexpected synchronous Server result: %s", result)
	}

	completedTaskID, err := invoker.StartTask(ctx, "mail.send", `{"player_id":"p-001","title":"Task","content":"body"}`, InvokeOptions{
		IdempotencyKey: "go-l3-real-task",
	})
	requireNoError(t, err)
	if strings.TrimSpace(completedTaskID) == "" {
		t.Fatal("Server returned an empty task ID")
	}
	completedEvents := collectTaskEvents(t, ctx, invoker, completedTaskID)
	requireTaskEvent(t, completedEvents, "started")
	requireTaskEvent(t, completedEvents, "completed")
	completed := waitForTaskStatus(t, ctx, invoker, completedTaskID, "succeeded")
	if completed.GameID != fixture.ready.GameID || completed.Env != fixture.ready.Env {
		t.Fatalf("task scope = (%q, %q), want (%q, %q)", completed.GameID, completed.Env, fixture.ready.GameID, fixture.ready.Env)
	}
	if !strings.Contains(completed.Result, `"mail_id":"mail-0001"`) {
		t.Fatalf("completed task result is not persisted by Server: %s", completed.Result)
	}

	cancelledTaskID, err := invoker.StartTask(ctx, "mail.wait", `{"wait_ms":30000}`, InvokeOptions{
		IdempotencyKey: "go-l3-real-cancel",
	})
	requireNoError(t, err)
	_ = waitForTaskStatus(t, ctx, invoker, cancelledTaskID, "running")
	requireNoError(t, invoker.CancelTask(ctx, cancelledTaskID))
	cancelledEvents := collectTaskEvents(t, ctx, invoker, cancelledTaskID)
	requireTaskEvent(t, cancelledEvents, "cancelled")
	_ = waitForTaskStatus(t, ctx, invoker, cancelledTaskID, "cancelled")
}

type realFixtureReady struct {
	GameID      string `json:"gameId"`
	Env         string `json:"env"`
	HTTPAddr    string `json:"httpAddr"`
	FixtureAddr string `json:"fixtureAddr"`
}

type realServerFixture struct {
	cmd   *exec.Cmd
	ready realFixtureReady
}

func (f *realServerFixture) serverURL() string { return "http://" + f.ready.HTTPAddr }

func (f *realServerFixture) close(t *testing.T) {
	t.Helper()
	if f.cmd == nil || f.cmd.Process == nil || f.cmd.ProcessState != nil {
		return
	}
	_ = f.cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- f.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Logf("fixture exited after cleanup signal: %v", err)
		}
	case <-time.After(15 * time.Second):
		_ = f.cmd.Process.Kill()
		<-done
		t.Fatal("fixture did not stop after cleanup signal")
	}
}

func (f *realServerFixture) login(t *testing.T, ctx context.Context) string {
	t.Helper()
	body := bytes.NewBufferString(`{"username":"admin","password":"admin123"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.serverURL()+"/api/v1/auth/login", body)
	requireNoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	requireNoError(t, err)
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	requireNoError(t, err)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fixture login status = %d, body = %s", resp.StatusCode, responseBody)
	}
	var session struct {
		Token string `json:"token"`
	}
	requireNoError(t, json.Unmarshal(responseBody, &session))
	if !strings.HasPrefix(session.Token, "eyJ") {
		t.Fatalf("fixture returned invalid JWT token: %q", session.Token)
	}
	return session.Token
}

func startRealServerFixture(t *testing.T, ctx context.Context) *realServerFixture {
	t.Helper()
	repoRoot := findRepositoryRoot(t)
	bin := filepath.Join(t.TempDir(), "croupier-server")
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/server")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real Server fixture: %v\n%s", err, output)
	}

	cmd := exec.CommandContext(ctx, bin, "dev-fixture", "--http-addr", "127.0.0.1:0", "--bootstrap-dir", filepath.Join(repoRoot, "configs"))
	cmd.Dir = repoRoot
	stdout, err := cmd.StdoutPipe()
	requireNoError(t, err)
	cmd.Stderr = os.Stderr
	requireNoError(t, cmd.Start())

	readyCh := make(chan realFixtureReady, 1)
	scanErrCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "FIXTURE_READY ") {
				continue
			}
			var ready realFixtureReady
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "FIXTURE_READY ")), &ready); err != nil {
				scanErrCh <- fmt.Errorf("decode fixture ready line: %w", err)
				return
			}
			readyCh <- ready
			return
		}
		scanErrCh <- scanner.Err()
	}()

	var ready realFixtureReady
	select {
	case ready = <-readyCh:
	case err := <-scanErrCh:
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("real Server fixture stopped before ready: %v", err)
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("real Server fixture did not become ready: %v", ctx.Err())
	}
	if ready.GameID == "" || ready.Env == "" || ready.HTTPAddr == "" || ready.FixtureAddr == "" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("invalid fixture readiness payload: %+v", ready)
	}

	fixture := &realServerFixture{cmd: cmd, ready: ready}
	waitForFixtureHealth(t, ctx, fixture)
	return fixture
}

func findRepositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	requireNoError(t, err)
	for {
		if info, err := os.Stat(filepath.Join(dir, "cmd", "server", "root.go")); err == nil && !info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root containing cmd/server/root.go")
		}
		dir = parent
	}
}

func waitForFixtureHealth(t *testing.T, ctx context.Context, fixture *realServerFixture) {
	t.Helper()
	deadline := time.NewTimer(60 * time.Second)
	defer deadline.Stop()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+fixture.ready.FixtureAddr+"/__fixture__/health", nil)
		requireNoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			var health struct {
				Status    string   `json:"status"`
				Agent     bool     `json:"agentConnected"`
				Functions []string `json:"functions"`
			}
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr == nil && resp.StatusCode == http.StatusOK && json.Unmarshal(body, &health) == nil && health.Status == "ok" && health.Agent && containsString(health.Functions, "mail.send") && containsString(health.Functions, "mail.wait") {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("fixture health check cancelled: %v", ctx.Err())
		case <-deadline.C:
			t.Fatal("fixture did not register both mail.send and mail.wait within 60 seconds")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func collectTaskEvents(t *testing.T, parent context.Context, invoker Invoker, taskID string) []TaskEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	stream, err := invoker.StreamTask(ctx, taskID)
	requireNoError(t, err)
	var events []TaskEvent
	for event := range stream {
		if event.EventType == "error" {
			t.Fatalf("task %s stream failed: %s", taskID, event.Error)
		}
		events = append(events, event)
	}
	return events
}

func waitForTaskStatus(t *testing.T, parent context.Context, invoker Invoker, taskID, expected string) *TaskStatus {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	for {
		status, err := invoker.GetTaskStatus(ctx, taskID)
		if err == nil && status.Status == expected {
			return status
		}
		select {
		case <-ctx.Done():
			if err != nil {
				t.Fatalf("wait for task %s status %q: %v", taskID, expected, err)
			}
			t.Fatalf("task %s status = %q, want %q", taskID, status.Status, expected)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func requireTaskEvent(t *testing.T, events []TaskEvent, expected string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == expected {
			return
		}
	}
	t.Fatalf("task events %v do not contain %q", taskEventTypes(events), expected)
}

func taskEventTypes(events []TaskEvent) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.EventType)
	}
	return types
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
