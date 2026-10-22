//go:build !no_go_plugin

package funcplugin

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}

// testHostWithFunctionsFile builds a host whose reference adapter declares
// one function per line of a file that the test can rewrite mid-run, so
// the host's declared set changes without the plugin process ever being
// restarted.
func testHostWithFunctionsFile(t *testing.T, target, list string) (*Host, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "functions.txt")
	if err := os.WriteFile(file, []byte(list), 0o600); err != nil {
		t.Fatal(err)
	}
	plugins := []PluginConfig{{
		Name:    "httpget",
		Command: httpgetBin,
		Args:    []string{"-target", target, "-functions-file", file},
		Timeout: 10 * time.Second,
	}}
	h, err := NewHost(AgentScope{GameID: "game-1", Env: "prod"}, plugins, discardLogger())
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	t.Cleanup(h.Stop)
	return h, file
}

func registeredIDs(h *Host) map[string]Registered {
	out := map[string]Registered{}
	for _, r := range h.Registered() {
		out[r.Declared.ID] = r
	}
	return out
}

// requireRegistered asserts the registered set is exactly the given ids.
func requireRegistered(t *testing.T, h *Host, ids ...string) {
	t.Helper()
	got := registeredIDs(h)
	if len(got) != len(ids) {
		t.Fatalf("registered ids %v, want %v", keys(got), ids)
	}
	for _, id := range ids {
		if _, ok := got[id]; !ok {
			t.Fatalf("registered ids %v, want %v", keys(got), ids)
		}
	}
}

func keys(m map[string]Registered) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func writeList(t *testing.T, file, list string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(list), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestHostReconcileAddsNewlyDeclaredFunction covers the half the
// Start-time collection misses: a plugin whose declared list grows keeps
// the new function out of the host's registration set forever unless the
// host re-enumerates it.
func TestHostReconcileAddsNewlyDeclaredFunction(t *testing.T) {
	h, file := testHostWithFunctionsFile(t, "http://127.0.0.1:1", "health\n")
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	requireRegistered(t, h, "plugin.httpget.health")

	writeList(t, file, "health\norders\n")
	if err := h.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	requireRegistered(t, h, "plugin.httpget.health", "plugin.httpget.orders")

	// The Describe stamp is re-read from the live process, not frozen at
	// Start.
	if r := registeredIDs(h)["plugin.httpget.orders"]; r.Version != "1.0.0" {
		t.Fatalf("version must come from the current process: %+v", r)
	}
}

// TestHostReconcileDropsWithdrawnFunction pins the safety half: a
// function the plugin stopped declaring must stop being a plugin
// function, so the dispatch seam refuses it instead of routing it to a
// process that no longer offers it.
func TestHostReconcileDropsWithdrawnFunction(t *testing.T) {
	h, file := testHostWithFunctionsFile(t, "http://127.0.0.1:1", "health\norders\n")
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	requireRegistered(t, h, "plugin.httpget.health", "plugin.httpget.orders")

	writeList(t, file, "health\n")
	if err := h.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	requireRegistered(t, h, "plugin.httpget.health")
	if h.IsPluginFunction("plugin.httpget.orders") {
		t.Fatal("withdrawn function must no longer be a plugin function")
	}
	// The withdrawal is visible to dispatch, not only to registration: the
	// sidecar can no longer route it at all.
	if _, cause := h.Invoke(context.Background(), "plugin.httpget.orders", `{}`, time.Second); !strings.HasPrefix(cause, "plugin_not_configured") {
		t.Fatalf("withdrawn function must not dispatch, cause=%s", cause)
	}
}

// TestHostReconcileKeepsLastKnownSetOnFailure pins the failure rule: a
// plugin's registered set only ever changes on a fully valid answer. An
// unreachable process and an invalid declaration both keep the last good
// set instead of unregistering a working function.
func TestHostReconcileKeepsLastKnownSetOnFailure(t *testing.T) {
	t.Run("plugin unreachable keeps the last set", func(t *testing.T) {
		h, file := testHostWithFunctionsFile(t, "http://127.0.0.1:1", "health\norders\n")
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		// An unreadable list makes ListFunctions fail; the call fails, the
		// engine kills the process and backs off, and the next answer
		// simply does not arrive.
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
		if err := h.Reconcile(context.Background()); err != nil {
			t.Fatalf("an unreachable plugin is not a reconcile error: %v", err)
		}
		requireRegistered(t, h, "plugin.httpget.health", "plugin.httpget.orders")
		if _, cause := h.Invoke(context.Background(), "plugin.httpget.orders", `{}`, time.Second); cause == "" {
			t.Fatal("an unreachable plugin must degrade visibly, not silently unregister")
		}
	})
	t.Run("invalid declaration keeps the last set", func(t *testing.T) {
		h, file := testHostWithFunctionsFile(t, "http://127.0.0.1:1", "health\norders\n")
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		writeList(t, file, "health\nHealth\n") // uppercase is outside the id contract
		if err := h.Reconcile(context.Background()); err == nil || !strings.Contains(err.Error(), "declaration rejected") {
			t.Fatalf("invalid declaration must fail: %v", err)
		}
		requireRegistered(t, h, "plugin.httpget.health", "plugin.httpget.orders")
	})
	t.Run("duplicate declaration keeps the last set", func(t *testing.T) {
		h, file := testHostWithFunctionsFile(t, "http://127.0.0.1:1", "health\norders\n")
		if err := h.Start(context.Background()); err != nil {
			t.Fatalf("start: %v", err)
		}
		writeList(t, file, "health\nhealth\n")
		if err := h.Reconcile(context.Background()); err == nil || !strings.Contains(err.Error(), "duplicate id") {
			t.Fatalf("duplicate declaration must fail: %v", err)
		}
		requireRegistered(t, h, "plugin.httpget.health", "plugin.httpget.orders")
	})
}

// TestHostReconcileAfterStopIsInert pins the shutdown boundary: a direct
// Reconcile after Stop degrades with the stopped controller instead of
// racing the drained engine or touching the registered set.
func TestHostReconcileAfterStopIsInert(t *testing.T) {
	h, _ := testHostWithFunctionsFile(t, "http://127.0.0.1:1", "health\n")
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	h.Stop()
	if err := h.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile after stop must stay inert: %v", err)
	}
	requireRegistered(t, h, "plugin.httpget.health")
}

// TestHostReconcileLoopReconcilesOnTick covers the loop itself: Start
// owns a ticker that keeps the registered set in step with the plugins,
// so the registration source handed to the upstream sync chain always
// sees the current declarations without a restart.
func TestHostReconcileLoopReconcilesOnTick(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	h, file := testHostWithFunctionsFile(t, srv.URL, "health\n")
	h.reconcileInterval = 25 * time.Millisecond
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	requireRegistered(t, h, "plugin.httpget.health")

	// Invokes run concurrently with the reconcile ticks: the registered
	// set is swapped under the host mutex, so an in-flight dispatch never
	// sees a torn snapshot.
	var wg sync.WaitGroup
	stopInvokes := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopInvokes:
					return
				default:
				}
				out, cause := h.Invoke(context.Background(), "plugin.httpget.health", `{"path":"/health"}`, 2*time.Second)
				if cause != "" {
					t.Errorf("concurrent invoke degraded: %s", cause)
					return
				}
				if !out.OK {
					t.Errorf("concurrent invoke failed: %+v", out)
					return
				}
				select {
				case <-stopInvokes:
					return
				case <-time.After(time.Millisecond):
				}
			}
		}()
	}

	deadline := time.Now().Add(5 * time.Second)
	for !h.IsPluginFunction("plugin.httpget.orders") {
		if time.Now().After(deadline) {
			close(stopInvokes)
			wg.Wait()
			t.Fatalf("loop never reconciled the new declaration: %v", keys(registeredIDs(h)))
		}
		writeList(t, file, "health\norders\n")
		time.Sleep(20 * time.Millisecond)
	}

	writeList(t, file, "health\n")
	deadline = time.Now().Add(5 * time.Second)
	for h.IsPluginFunction("plugin.httpget.orders") {
		if time.Now().After(deadline) {
			close(stopInvokes)
			wg.Wait()
			t.Fatalf("loop never reconciled the withdrawal: %v", keys(registeredIDs(h)))
		}
		time.Sleep(20 * time.Millisecond)
	}
	close(stopInvokes)
	wg.Wait()
	requireRegistered(t, h, "plugin.httpget.health")
}
