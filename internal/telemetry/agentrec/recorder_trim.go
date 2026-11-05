//go:build no_otel

package agentrec

import (
	"context"
	"errors"
	"log/slog"

	"github.com/cuihairu/croupier/core/otelrec"
)

// Recorder no_otel 裁剪构建下的占位（otelrec.Recorder 满足面，无 SDK）。
// Enabled=false 时零开销；Enabled=true 时 Build fail-closed 报错（不静默
// 无导出，与 server 的 provider_sdk_trim.go 同语义）。
type Recorder struct{}

// Build 在裁剪构建下：Enabled=true → fail-closed；否则返回 noop recorder。
func Build(ctx context.Context, cfg otelrec.TelemetryConfig, logger *slog.Logger) (*Recorder, error) {
	_ = ctx
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Enabled {
		return nil, errors.New("agent telemetry enabled but binary built with no_otel (trimmed); rebuild without no_otel to export")
	}
	return &Recorder{}, nil
}

// Shutdown 裁剪构建下无导出，直通。
func (r *Recorder) Shutdown(context.Context) error { return nil }

func (r *Recorder) Start(context.Context, map[string]string)                    {}
func (r *Recorder) Stop(context.Context, error)                                 {}
func (r *Recorder) Connected(context.Context)                                   {}
func (r *Recorder) Disconnected(context.Context, error)                         {}
func (r *Recorder) Count(context.Context, string, int64, map[string]string)     {}
func (r *Recorder) Observe(context.Context, string, float64, map[string]string) {}
