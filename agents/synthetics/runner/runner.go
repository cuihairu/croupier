package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/controlconn"
	"github.com/cuihairu/croupier/agents/synthetics/config"
	"github.com/cuihairu/croupier/agents/synthetics/pluginhost"
	"github.com/cuihairu/croupier/agents/synthetics/probe"
	"github.com/cuihairu/croupier/core/register"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

var Version = "dev"
var GitCommit, BuildTime = "unknown", "unknown"

type Runner struct {
	cfg  config.Config
	log  *slog.Logger
	conn *controlconn.Conn
	// plugins hosts external probe plugins (go-plugin local gRPC).
	// nil when no plugin is declared: no plugin process is ever started
	// and every target keeps the native HTTP probe.
	plugins *pluginhost.Controller
}

func New(cfg config.Config, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	r := &Runner{cfg: cfg, log: log}
	// Runtime master switch (todo.md:36 第二片): pluginsEnabled: false
	// runs pure native HTTP probing; no plugin controller is built and
	// no plugin process is ever started. In the default build the
	// controller exists whenever plugin support is compiled in and
	// declared; under the no_go_plugin build tag config validation
	// rejects declared plugins at startup instead.
	if cfg.Synthetics.PluginsOn() && len(cfg.Synthetics.Plugins) > 0 {
		r.plugins = pluginhost.NewController(cfg.Synthetics, log)
	}
	return r
}
func (r *Runner) Run(ctx context.Context) error {
	reg := register.New(register.Config{AgentID: r.cfg.Agent.ID, Dial: func(context.Context) (register.Conn, error) {
		c, e := controlconn.Dial(r.cfg.Agent.ServerAddr, tlsFromCfg(r.cfg))
		if e == nil {
			r.conn = c
		}
		return c, e
	}, Payload: r.buildPayload, Logger: r.log})
	reg.Start(ctx)
	defer reg.Stop()
	if r.plugins != nil {
		defer r.plugins.Stop()
	}
	for _, target := range r.cfg.Synthetics.Targets {
		r.execute(ctx, target)
	}
	t := time.NewTicker(r.cfg.Synthetics.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			for _, target := range r.cfg.Synthetics.Targets {
				if ctx.Err() != nil {
					return nil
				}
				r.execute(ctx, target)
			}
		}
	}
}
func (r *Runner) execute(ctx context.Context, t config.Target) {
	res, details := r.runProbe(ctx, t)
	ev := buildProbeEvent(r.cfg, t, res, details)
	if r.conn == nil || !r.conn.Connected() {
		r.log.Warn("probe result not reported: session unavailable", "target", t.Name)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := r.conn.SendProbeEvent(sendCtx, ev); err != nil {
		r.log.Warn("probe result report failed", "target", t.Name, "error", err)
	}
	r.log.Info("probe completed", "target", t.Name, "ok", res.OK, "status", res.Status, "latencyMs", res.Latency.Milliseconds(), "error", res.Error)
}

// runProbe routes one target through its declared external plugin, falling
// back to the native HTTP probe with an explicit plugin cause when the
// plugin side degrades or is disabled. Targets without plugin routing
// keep the native HTTP probe untouched. A nil controller with a
// plugin-routed target only happens with the runtime switch off (or a
// command line shim) and is reported as plugin_disabled instead of
// silently dropping the routing decision.
func (r *Runner) runProbe(ctx context.Context, t config.Target) (probe.Result, map[string]string) {
	if t.Plugin != "" {
		if r.plugins == nil {
			return r.fallbackHTTP(ctx, t, fmt.Sprintf("%s: plugin runtime switch is off", pluginhost.CauseDisabled))
		}
		out := r.plugins.Check(ctx, t)
		if out.Cause == "" {
			details := map[string]string{"probeType": "plugin", "status": statusString(out.Result.OK), "errorClass": errorClass(out.Result.Error), "pluginName": t.Plugin}
			if v := r.plugins.Version(t.Plugin); v != "" {
				details["pluginVersion"] = v
			}
			if out.Detail != "" {
				details["pluginDetail"] = out.Detail
			}
			return out.Result, details
		}
		return r.fallbackHTTP(ctx, t, out.Cause)
	}
	res := probe.Run(ctx, t)
	return res, map[string]string{"probeType": "http", "status": statusString(res.OK), "httpStatus": itoa(res.Status), "errorClass": errorClass(res.Error)}
}

// fallbackHTTP is the explicit degradation path (todo.md:36): the plugin
// subprocess could not answer (unavailable / timed out / call error /
// backpressure / backoff / circuit / disabled); the native HTTP probe
// keeps the target alive and the cause is recorded for the operator
// instead of masking the plugin fault.
func (r *Runner) fallbackHTTP(ctx context.Context, t config.Target, cause string) (probe.Result, map[string]string) {
	r.log.Warn("probe plugin degraded; falling back to native HTTP probe", "target", t.Name, "plugin", t.Plugin, "cause", cause)
	res := probe.Run(ctx, t)
	details := map[string]string{
		"probeType": "http", "status": statusString(res.OK), "httpStatus": itoa(res.Status),
		"errorClass": errorClass(res.Error), "pluginName": t.Plugin,
		"pluginFallback": "true", "pluginCause": cause,
		"probeError": "plugin degraded: " + cause,
	}
	return res, details
}

func statusString(ok bool) string {
	if ok {
		return "success"
	}
	return "failure"
}

// buildProbeEvent maps one probe outcome onto the existing AlertEvent /
// ProbeDetail contract; plugin results land in the same event shape as
// HTTP results, with routing metadata carried in Details.
func buildProbeEvent(c config.Config, t config.Target, res probe.Result, details map[string]string) *opsv1.AlertEvent {
	return &opsv1.AlertEvent{Kind: "probe.detail", AgentId: c.Agent.ID, GameId: c.Agent.GameID, Env: c.Agent.Env, Detail: &opsv1.ProbeDetail{Target: t.Name, Semantics: "liveness", Ok: res.OK, Err: res.Error, AtTsUnixMs: res.At.UnixMilli(), LatencyMs: res.Latency.Milliseconds(), Details: details}}
}
func (r *Runner) buildPayload(context.Context) (*agentv1.RegisterRequest, error) {
	host, _ := os.Hostname()
	return &agentv1.RegisterRequest{AgentId: r.cfg.Agent.ID, Version: Version, GameId: r.cfg.Agent.GameID, Env: r.cfg.Agent.Env, Labels: map[string]string{"role": "synthetics", "hostname": host, "os": runtime.GOOS, "arch": runtime.GOARCH, "gitCommit": GitCommit, "buildTime": BuildTime}}, nil
}
func tlsFromCfg(c config.Config) *controlconn.TLS {
	if !c.OutboundTLS.Enabled {
		return nil
	}
	return &controlconn.TLS{Enabled: true, CertFile: c.OutboundTLS.CertFile, KeyFile: c.OutboundTLS.KeyFile, CAFile: c.OutboundTLS.CAFile, ServerName: c.OutboundTLS.ServerName, InsecureSkipVerify: c.OutboundTLS.InsecureSkipVerify}
}
func itoa(i int) string { return strconv.Itoa(i) }
func errorClass(s string) string {
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, ':'); i > 0 {
		return s[:i]
	}
	return "probe_error"
}
