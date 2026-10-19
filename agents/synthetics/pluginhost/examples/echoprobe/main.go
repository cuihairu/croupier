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
// echoes the target identity back in the detail field. Two env knobs
// exist for harness and fault-injection use:
//
//	ECHO_PROBE_DELAY_MS  sleep before answering (0 = instant)
//	ECHO_PROBE_FAIL=1    answer with an error instead of success
package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/pluginhost"
)

type echoProbe struct{}

func (echoProbe) Describe(context.Context) (string, string, error) { return "echo", "1.0.0", nil }

func (echoProbe) Check(ctx context.Context, in pluginhost.CheckInput) (pluginhost.CheckOutput, error) {
	start := time.Now()
	if ms, e := strconv.Atoi(os.Getenv("ECHO_PROBE_DELAY_MS")); e == nil && ms > 0 {
		select {
		case <-time.After(time.Duration(ms) * time.Millisecond):
		case <-ctx.Done():
			return pluginhost.CheckOutput{}, ctx.Err()
		}
	}
	if os.Getenv("ECHO_PROBE_FAIL") == "1" {
		return pluginhost.CheckOutput{OK: false, Error: "echo: forced failure", At: time.Now()}, nil
	}
	return pluginhost.CheckOutput{
		OK:      true,
		Status:  200,
		Latency: time.Since(start),
		Detail:  "echo: " + in.Target,
		At:      time.Now(),
	}, nil
}

func main() { pluginhost.Serve(echoProbe{}) }
