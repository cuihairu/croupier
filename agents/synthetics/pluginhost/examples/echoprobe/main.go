//go:build !no_go_plugin

// Command echoprobe is the slice-1 example external probe plugin
// (todo.md:36): a minimal resident probe program speaking the Croupier
// synthetics go-plugin contract and answering with fixed echo results.
//
// Build locally, then reference the binary from a synthetics-agent
// config via synthetics.plugins[].command:
//
//	go build ./agents/synthetics/pluginhost/examples/echoprobe
//
// It reports a successful check (status 200) for every Check request and
// echoes the target identity back in the detail field. Env knobs exist
// for harness and fault-injection use (slices 1-2):
//
//	ECHO_PROBE_DELAY_MS           sleep before answering (0 = instant)
//	ECHO_PROBE_FAIL=1             answer with an error instead of success
//	ECHO_PROBE_HOLD_MS            explicit occupancy window (backpressure tests)
//	ECHO_PROBE_CONC=1             append process-local concurrency (cur/max) to detail
//	ECHO_PROBE_EXIT_AFTER_CHECKS  os.Exit before answering the Nth check of
//	                              this process (crash injection)
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/pluginhost"
)

// echoProbe is the in-process plugin implementation. With
// ECHO_PROBE_CONC=1 it maintains process-local concurrency counters so
// hosts can assert their bounded queue keeps observed concurrency at or
// below the configured slot limit.
type echoProbe struct {
	mu   sync.Mutex
	cur  int
	max  int
	done int
}

func (p *echoProbe) enter() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cur++
	if p.cur > p.max {
		p.max = p.cur
	}
	return p.cur
}

func (p *echoProbe) exit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cur--
}

func (p *echoProbe) concurrencySnapshot() (cur int, max int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cur, p.max
}

func (p *echoProbe) Describe(context.Context) (string, string, error) { return "echo", "1.0.0", nil }

func (p *echoProbe) Check(ctx context.Context, in pluginhost.CheckInput) (pluginhost.CheckOutput, error) {
	start := time.Now()
	const crashEnv = "ECHO_PROBE_EXIT_AFTER_CHECKS"
	if n, e := strconv.Atoi(os.Getenv(crashEnv)); e == nil && n > 0 {
		p.mu.Lock()
		p.done++
		done := p.done
		p.mu.Unlock()
		if done >= n {
			// Crash before answering: the host observes the dead
			// process (client.Exited) and applies its own backoff and
			// circuit breaker instead of a tight restart loop.
			os.Exit(2)
		}
	}
	cur := p.enter()
	defer p.exit()
	if ms, e := strconv.Atoi(os.Getenv("ECHO_PROBE_DELAY_MS")); e == nil && ms > 0 {
		select {
		case <-time.After(time.Duration(ms) * time.Millisecond):
		case <-ctx.Done():
			return pluginhost.CheckOutput{}, ctx.Err()
		}
	}
	if ms, e := strconv.Atoi(os.Getenv("ECHO_PROBE_HOLD_MS")); e == nil && ms > 0 {
		select {
		case <-time.After(time.Duration(ms) * time.Millisecond):
		case <-ctx.Done():
			return pluginhost.CheckOutput{}, ctx.Err()
		}
	}
	if os.Getenv("ECHO_PROBE_FAIL") == "1" {
		return pluginhost.CheckOutput{OK: false, Error: "echo: forced failure", At: time.Now()}, nil
	}
	detail := "echo: " + in.Target
	if os.Getenv("ECHO_PROBE_CONC") == "1" {
		_, maxConc := p.concurrencySnapshot()
		detail = fmt.Sprintf("%s cur=%d max=%d", detail, cur, maxConc)
	}
	return pluginhost.CheckOutput{
		OK:      true,
		Status:  200,
		Latency: time.Since(start),
		Detail:  detail,
		At:      time.Now(),
	}, nil
}

func main() { pluginhost.Serve(&echoProbe{}) }
