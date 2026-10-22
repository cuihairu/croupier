package runner

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/core/register"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// fakeUpstreamConn is an in-memory control connection satisfying both the
// register.Conn face (registration/heartbeat/alive) and the runner's
// probeSender face (probe upload). It records what the runner uploads.
type fakeUpstreamConn struct {
	mu     sync.Mutex
	closed bool
	events []*opsv1.AlertEvent
}

func (c *fakeUpstreamConn) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed
}
func (c *fakeUpstreamConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}
func (c *fakeUpstreamConn) Register(context.Context, *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	return &agentv1.RegisterResponse{}, nil
}
func (c *fakeUpstreamConn) Heartbeat(context.Context, *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	return &agentv1.HeartbeatResponse{}, nil
}
func (c *fakeUpstreamConn) SendProbeEvent(_ context.Context, ev *opsv1.AlertEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, ev)
	return nil
}
func (c *fakeUpstreamConn) eventCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

func newConnTestRunner(t *testing.T) (*Runner, config.Target) {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(s.Close)
	r := New(config.Config{
		Agent:      config.Agent{ID: "syn-race", GameID: "demo", Env: "prod"},
		Synthetics: config.Synthetics{Interval: time.Minute},
	}, nil)
	target := config.Target{Name: "web", URL: s.URL, Method: "GET", Timeout: 5 * time.Second, ExpectedStatus: 200, BodyContains: "ok"}
	return r, target
}

func waitForUpstream(t *testing.T, reg *register.Client) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if reg.Connected() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("register client never reached a connected session")
}

// TestExecuteReportsAfterFirstConnect pins the first-connect state: once
// the background registration completes, the probe loop reads the
// session through the register client's synchronized snapshot and the
// event reaches the connection it dialed.
func TestExecuteReportsAfterFirstConnect(t *testing.T) {
	r, target := newConnTestRunner(t)
	conn := &fakeUpstreamConn{}
	reg := register.New(register.Config{AgentID: r.cfg.Agent.ID, Dial: func(context.Context) (register.Conn, error) {
		return conn, nil
	}, Payload: r.buildPayload})
	r.reg = reg
	reg.Start(context.Background())
	defer reg.Stop()
	waitForUpstream(t, reg)

	r.execute(context.Background(), target)
	if conn.eventCount() != 1 {
		t.Fatalf("first connect must deliver exactly one probe event, got %d", conn.eventCount())
	}
	if ev := conn.events[0]; ev.GetDetail().GetTarget() != target.Name {
		t.Fatalf("delivered event must carry the probed target, got %q", ev.GetDetail().GetTarget())
	}
}

// TestExecuteConcurrentWithRegistrationCycles is the race regression
// (todo L8): probe execution runs concurrently with the register
// client's background first dial, a failed dial, and repeated
// reconnections. The old runner cached its own conn pointer written
// inside the dialer (background goroutine) and read it in the probe
// loop; under -race this interleaving now stays clean because probes
// only take the client's mutex-guarded snapshot.
func TestExecuteConcurrentWithRegistrationCycles(t *testing.T) {
	r, target := newConnTestRunner(t)
	conns := []*fakeUpstreamConn{{}, {}, {}}
	var dialCalls int32
	reg := register.New(register.Config{AgentID: r.cfg.Agent.ID,
		Dial: func(context.Context) (register.Conn, error) {
			n := atomic.AddInt32(&dialCalls, 1)
			time.Sleep(2 * time.Millisecond) // widen the write window for the race detector
			switch {
			case n == 2:
				return nil, errors.New("connection refused") // registration failure mid-cycle
			case n == 1:
				return conns[0], nil
			default:
				return conns[1+int((n-3)%2)], nil // alternate fresh conns on reconnects
			}
		}, Payload: r.buildPayload})
	r.reg = reg
	reg.Start(context.Background())
	defer reg.Stop()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			r.execute(context.Background(), target)
		}
	}()
	// Registration cycles racing the probe loop: reconnects and a failed
	// dial swap/clear the session under the client's lock while probes run.
	for i := 0; i < 6; i++ {
		_ = reg.DialAndRegister(context.Background())
		if reg.Connected() {
			time.Sleep(3 * time.Millisecond) // hold a live window so probes deliver
		}
	}
	close(stop)
	wg.Wait()

	total := 0
	for _, c := range conns {
		total += c.eventCount()
	}
	if total == 0 {
		t.Fatal("probe loop must deliver events across registration cycles")
	}
	if atomic.LoadInt32(&dialCalls) < 3 {
		t.Fatalf("expected first connect, a failed dial and a reconnect, got %d dials", dialCalls)
	}
}

// TestExecuteWithoutSessionAndAfterStop pins the degraded states: no
// session at all, a failed registration, and a stopped client must all
// keep the probe loop alive (warn + drop) instead of panicking.
func TestExecuteWithoutSessionAndAfterStop(t *testing.T) {
	r, target := newConnTestRunner(t)

	r.execute(context.Background(), target) // never started: nil upstream

	reg := register.New(register.Config{AgentID: r.cfg.Agent.ID, Dial: func(context.Context) (register.Conn, error) {
		return nil, errors.New("connection refused")
	}, Payload: r.buildPayload})
	r.reg = reg
	ctx, cancel := context.WithCancel(context.Background())
	reg.Start(ctx)
	r.execute(context.Background(), target) // registration failing: still no session

	cancel()
	reg.Stop()
	r.execute(context.Background(), target) // after stop: connection cleared
	if r.upstream() != nil {
		t.Fatal("stopped client must expose no upstream session")
	}
}
