// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package croupier

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// quietShutdownConfig keeps race-test output readable while keeping the
// default reconnection settings used by the lifecycle under test.
func quietShutdownConfig() *ClientConfig {
	cfg := DefaultClientConfig()
	cfg.DisableLogging = true
	cfg.AgentAddr = "127.0.0.1:1" // unreachable: connect/reconnect fail fast
	cfg.Reconnect = &ReconnectConfig{
		Enabled:           true,
		MaxAttempts:       2,
		InitialDelayMs:    1,
		MaxDelayMs:        1,
		BackoffMultiplier: 1,
	}
	return cfg
}

// TestClientConcurrentStopClose checks the AUD-070 shutdown edge: many
// Stop/Close callers must produce exactly one channel close, not a
// select-check-then-close race or a send on a closed channel.
func TestClientConcurrentStopClose(t *testing.T) {
	c := NewClient(quietShutdownConfig()).(*client)

	const callers = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, callers*2)
	for i := 0; i < callers; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			errs <- c.Stop()
		}()
		go func() {
			defer wg.Done()
			<-start
			errs <- c.Close()
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Stop/Close returned error: %v", err)
		}
	}

	select {
	case <-c.stopCh:
	default:
		t.Fatal("stopCh must be closed after concurrent Stop/Close")
	}
	if c.connected.Load() || c.running.Load() {
		t.Fatal("client must be disconnected and not running after concurrent Stop/Close")
	}
}

// TestClientConcurrentStopRegisterReconnect exercises the lifecycle under
// simultaneous Stop, registration and reconnect attempts. It uses a fake
// agent so Connect/RegisterWithAgent and the backoff loop run their real
// code paths under the race detector.
func TestClientConcurrentStopRegisterReconnect(t *testing.T) {
	agent := startFakeAgent(t, "127.0.0.1:0", defaultAgentHandler("sess-shutdown-race"))
	cfg := quietShutdownConfig()
	cfg.AgentAddr = agent.addr()

	c := NewClient(cfg).(*client)
	handler := func(ctx context.Context, payload []byte) ([]byte, error) {
		return payload, nil
	}
	const numFunctions = 50
	for i := 0; i < numFunctions; i++ {
		if err := c.RegisterFunction(FunctionDescriptor{ID: fmt.Sprintf("race.func.%d", i)}, handler); err != nil {
			t.Fatalf("RegisterFunction: %v", err)
		}
	}
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Force a disconnect notification so Serve runs the reconnect loop.
	tcp, ok := c.manager.(*TCPManager)
	if !ok {
		t.Fatalf("manager is %T, want *TCPManager", c.manager)
	}
	tcp.handleDisconnect()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() { serveDone <- c.Serve(ctx) }()

	const goroutines = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	var stopCount atomic.Int32
	for i := 0; i < goroutines; i++ {
		wg.Add(3)
		go func(i int) {
			defer wg.Done()
			<-start
			if err := c.Stop(); err != nil {
				t.Errorf("Stop: %v", err)
			}
			stopCount.Add(1)
		}(i)
		go func(i int) {
			defer wg.Done()
			<-start
			desc := FunctionDescriptor{ID: fmt.Sprintf("race.register.%d", i)}
			_ = c.RegisterFunction(desc, handler) // rejected once running/stopped
		}(i)
		go func() {
			defer wg.Done()
			<-start
			// Concurrent reconnection attempt; Stop must interrupt it and
			// no caller may close an already closed channel.
			_ = c.reconnectWithBackoff(ctx)
		}()
	}
	close(start)
	wg.Wait()

	// Serve must observe the single shutdown signal and exit.
	select {
	case <-serveDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after concurrent Stop")
	}
	cancel()

	// A reconnect landing after the last Stop may legitimately leave
	// connected=true; settle with one more Stop before asserting state.
	if err := c.Stop(); err != nil {
		t.Fatalf("final Stop: %v", err)
	}

	if got := stopCount.Load(); got != goroutines {
		t.Fatalf("Stop calls = %d, want %d", got, goroutines)
	}
	select {
	case <-c.stopCh:
	default:
		t.Fatal("stopCh must be closed after concurrent Stop/Close")
	}
	if c.running.Load() || c.connected.Load() {
		t.Fatal("client must be disconnected and not running after Stop")
	}
}

// TestClientSequentialRepeatedStop preserves the existing contract: repeated
// Stop/Close calls in sequence stay no-ops and never panic.
func TestClientSequentialRepeatedStop(t *testing.T) {
	c := NewClient(quietShutdownConfig()).(*client)
	c.running.Store(true)
	c.connected.Store(true)

	for i := 0; i < 5; i++ {
		if err := c.Stop(); err != nil {
			t.Fatalf("Stop #%d: %v", i+1, err)
		}
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close after repeated Stop: %v", err)
	}
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop after Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	select {
	case <-c.stopCh:
	default:
		t.Fatal("stopCh must be closed after repeated Stop/Close")
	}
}
