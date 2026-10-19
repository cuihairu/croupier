//go:build !no_go_plugin

package pluginhost

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

var echoBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "core-pluginhost-test")
	if err != nil {
		panic(err)
	}
	echoBin = filepath.Join(dir, "echo")
	cmd := exec.Command("go", "build", "-o", echoBin, "./examples/echo")
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build echo plugin: " + string(output))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func newEchoController(t *testing.T, knobs Knobs) *Controller[testImpl] {
	t.Helper()
	knobs.Log = testLogger(t)
	return NewController([]Config{{
		Name:    "echo",
		Command: echoBin,
		Timeout: 5 * time.Second,
		Enabled: true,
	}}, echoContract(), knobs)
}

func echoContract() Contract[testImpl] {
	return Contract[testImpl]{
		Key:       "echo",
		Handshake: plugin.HandshakeConfig{ProtocolVersion: 1, MagicCookieKey: "CROUPIER_ECHO", MagicCookieValue: "echo"},
		GRPCServer: func(impl testImpl) plugin.Plugin {
			return &echoGRPCPlugin{impl: impl}
		},
		Client: func(any) (testImpl, error) {
			return testImpl{}, nil
		},
		Describe: func(ctx context.Context, impl testImpl) (string, string, error) {
			return impl.Describe(ctx)
		},
	}
}

type testImpl struct{}

func (testImpl) Describe(context.Context) (string, string, error) {
	return "echo", "1.0.0", nil
}

func (testImpl) Call(context.Context, string) (string, error) {
	return "echo", nil
}

type echoGRPCPlugin struct {
	plugin.Plugin
	impl testImpl
}

func (plugin *echoGRPCPlugin) GRPCServer(*plugin.GRPCBroker, *grpc.Server) error { return nil }

func (plugin *echoGRPCPlugin) GRPCClient(context.Context, *plugin.GRPCBroker, *grpc.ClientConn) (any, error) {
	return plugin.impl, nil
}

func TestGenericControllerStartsCallsAndStopsPlugin(t *testing.T) {
	controller := newEchoController(t, Knobs{})
	t.Cleanup(controller.Stop)

	var response string
	cause := controller.Call(context.Background(), "echo", time.Second, func(_ context.Context, impl testImpl) error {
		var err error
		response, err = impl.Call(context.Background(), "hello")
		return err
	})
	if cause != "" {
		t.Fatalf("successful call degraded: %s", cause)
	}
	if response != "echo" {
		t.Fatalf("plugin result = %q, want %q", response, "echo")
	}
	if version := controller.Version("echo"); version != "1.0.0" {
		t.Fatalf("describe version = %q, want %q", version, "1.0.0")
	}

}

func TestGenericControllerStopRejectsAndRemovesProcess(t *testing.T) {
	controller := newEchoController(t, Knobs{})
	controller.Call(context.Background(), "echo", time.Second, func(context.Context, testImpl) error { return nil })
	controller.Stop()
	cause := controller.Call(context.Background(), "echo", time.Second, func(context.Context, testImpl) error { return nil })
	if !strings.HasPrefix(cause, CauseDisabled) {
		t.Fatalf("post-stop cause = %q, want prefix %q", cause, CauseDisabled)
	}
	if version := controller.Version("echo"); version != "" {
		t.Fatalf("stopped controller still exposes version %q", version)
	}
}

func TestGenericControllerFailureDegradesAndOpensCircuit(t *testing.T) {
	controller := newEchoController(t, Knobs{Backoff: time.Millisecond, BreakAfter: 2, OpenWindow: time.Second})
	t.Cleanup(controller.Stop)

	call := func() string {
		return controller.Call(context.Background(), "echo", time.Second, func(context.Context, testImpl) error {
			return errors.New("contract failure")
		})
	}
	if cause := call(); !strings.HasPrefix(cause, CauseError) {
		t.Fatalf("first cause = %q, want prefix %q", cause, CauseError)
	}
	if version := controller.Version("echo"); version != "" {
		t.Fatalf("failed controller still exposes version %q", version)
	}
	time.Sleep(2 * time.Millisecond)
	if cause := call(); !strings.HasPrefix(cause, CauseCircuitOpen) {
		t.Fatalf("second cause = %q, want circuit %q", cause, CauseCircuitOpen)
	}
}

func TestGenericControllerBackpressureBoundedQueue(t *testing.T) {
	controller := newEchoController(t, Knobs{MaxConcurrent: 1, QueueWindow: 50 * time.Millisecond})
	t.Cleanup(controller.Stop)

	release := make(chan struct{})
	firstDone := make(chan string, 1)
	go func() {
		firstDone <- controller.Call(context.Background(), "echo", time.Second, func(ctx context.Context, _ testImpl) error {
			<-release
			return nil
		})
	}()

	deadline := time.Now().Add(time.Second)
	for controller.InFlight() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if controller.InFlight() != 1 {
		t.Fatal("first call did not acquire the only concurrency slot")
	}
	start := time.Now()
	cause := controller.Call(context.Background(), "echo", time.Second, func(context.Context, testImpl) error { return nil })
	if !strings.HasPrefix(cause, CauseBackpressure) {
		t.Fatalf("overflow cause = %q, want prefix %q", cause, CauseBackpressure)
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Fatalf("overflow call waited %s, want prompt rejection", elapsed)
	}
	close(release)
	if cause := <-firstDone; cause != "" {
		t.Fatalf("first call degraded: %s", cause)
	}
	for controller.InFlight() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if controller.InFlight() != 0 {
		t.Fatal("concurrency slot did not drain")
	}
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
