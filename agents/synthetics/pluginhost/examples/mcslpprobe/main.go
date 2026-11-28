//go:build !no_go_plugin

// Command mcslpprobe is the Minecraft Java Server List Ping (SLP) probe
// plugin: the first game-protocol reference probe on the synthetics
// external-plugin path (todo PROBE-001). It wraps the in-process SLP
// checker (core/healthprobe/games/minecraft) and delivers it over the
// go-plugin contract, so a real game server is verified through the
// external plugin hop exactly as a production synthetics agent would
// run it — Describe/Check, host-side timeout, credential boundary all
// apply unchanged.
//
// Build locally, then reference the binary from a synthetics-agent
// config via synthetics.plugins[].command:
//
//	go build ./agents/synthetics/pluginhost/examples/mcslpprobe
//
// Host payload: {"host": "127.0.0.1", "port": 25565} — port optional,
// defaulting to Minecraft Java's 25565.
//
// Binding clause (mc-online-login-survey-2026-11-28): SLP is a read-only
// status query. It never touches the login/encryption chain, so a pass
// here is server-status evidence only and must never be reported as
// player-login evidence.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/synthetics/pluginhost"
	mc "github.com/cuihairu/croupier/core/healthprobe/games/minecraft"
)

type mcslpProbe struct{}

func (p *mcslpProbe) Describe(context.Context) (string, string, error) {
	return "probe-mc-slp", "1.0.0", nil
}

func (p *mcslpProbe) Check(ctx context.Context, in pluginhost.CheckInput) (pluginhost.CheckOutput, error) {
	start := time.Now()
	now := func() time.Time { return time.Now() }
	fail := func(detail, format string, args ...any) pluginhost.CheckOutput {
		return pluginhost.CheckOutput{
			OK:      false,
			Latency: time.Since(start),
			Detail:  detail,
			Error:   fmt.Sprintf(format, args...),
			At:      now(),
		}
	}

	var payload struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	if err := json.Unmarshal([]byte(in.PayloadJSON), &payload); err != nil {
		return fail("probe-mc-slp payload parse",
			"configuration: payload must be JSON with host (port optional, default %d)", mc.DefaultPort), nil
	}
	if strings.TrimSpace(payload.Host) == "" {
		return fail("probe-mc-slp payload parse",
			"configuration: payload must contain host (port optional, default %d)", mc.DefaultPort), nil
	}
	if payload.Port < 0 || payload.Port > 65535 {
		return fail("probe-mc-slp payload parse",
			"configuration: port must be an integer in [1, 65535]"), nil
	}
	port := payload.Port
	if port == 0 {
		port = mc.DefaultPort
	}
	addr := net.JoinHostPort(payload.Host, strconv.Itoa(port))

	// Internal read budget is 80% of the host deadline so a silent
	// non-SLP target surfaces as a plugin business failure before the
	// host-side degradation deadline races it (hostdriver probe-mc-slp
	// asserts exactly this ordering).
	budget := in.Timeout * 4 / 5
	if budget < 50*time.Millisecond {
		budget = in.Timeout
	}
	chk := mc.NewSLPChecker(addr, budget)
	if err := chk.Check(ctx); err != nil {
		phase := ""
		for k, v := range chk.Details() {
			if k == "phase" {
				phase = v
			}
		}
		return fail("probe-mc-slp phase="+phase, "minecraft slp: %v", err), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "probe-mc-slp checked %s", addr)
	if v := chk.Details()["version"]; v != "" {
		fmt.Fprintf(&b, " version=%s", v)
	}
	if v := chk.Details()["serverProtocol"]; v != "" {
		fmt.Fprintf(&b, " protocol=%s", v)
	}
	if v := chk.Details()["players"]; v != "" {
		fmt.Fprintf(&b, " players=%s", v)
	}
	return pluginhost.CheckOutput{
		OK:      true,
		Status:  200,
		Latency: time.Since(start),
		Detail:  b.String(),
		At:      now(),
	}, nil
}

func main() { pluginhost.Serve(&mcslpProbe{}) }
