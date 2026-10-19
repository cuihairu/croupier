package runner

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/controlconn"
	"github.com/cuihairu/croupier/agents/synthetics/config"
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
}

func New(cfg config.Config, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	return &Runner{cfg: cfg, log: log}
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
	res := probe.Run(ctx, t)
	status := "failure"
	if res.OK {
		status = "success"
	}
	details := map[string]string{"probeType": "http", "status": status, "httpStatus": itoa(res.Status), "errorClass": errorClass(res.Error)}
	ev := &opsv1.AlertEvent{Kind: "probe.detail", AgentId: r.cfg.Agent.ID, GameId: r.cfg.Agent.GameID, Env: r.cfg.Agent.Env, Detail: &opsv1.ProbeDetail{Target: t.Name, Semantics: "liveness", Ok: res.OK, Err: res.Error, AtTsUnixMs: res.At.UnixMilli(), LatencyMs: res.Latency.Milliseconds(), Details: details}}
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
