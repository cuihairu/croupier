package svc

import (
	"context"
	"strings"
	"time"

	"log/slog"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/vault"
	"github.com/cuihairu/croupier/internal/telemetry"
)

func NewTelemetryService(c config.Config, secrets vault.Resolver, serviceName string, logger *slog.Logger) (*telemetry.GameTelemetryService, error) {
	// analytics redis 密码支持 vault:// 引用（todo 4251）；解析失败按
	// 「未配置」降级（fail-closed，不把引用串当真凭据发往 redis）。secrets
	// 允许 nil（直通，兼容测试面）。
	analyticsPassword := c.Telemetry.Analytics.RedisPassword
	if secrets != nil {
		if pw, rerr := secrets(context.Background(), analyticsPassword); rerr != nil {
			slog.Warn("telemetry analytics redis password resolve failed; treat as unset", "error", rerr)
			analyticsPassword = ""
		} else {
			analyticsPassword = pw
		}
	}
	cfg := telemetry.MergeEnv(telemetry.TelemetryConfig{
		Enabled:        c.Telemetry.Enabled,
		ServiceName:    firstNonEmpty(c.Telemetry.ServiceName, serviceName),
		ServiceVersion: c.Telemetry.ServiceVersion,
		Environment:    firstNonEmpty(c.Telemetry.Environment, c.Server.Mode),
		CollectorURL:   c.Telemetry.CollectorURL,
		GameID:         c.Telemetry.GameID,
		EnableTracing:  c.Telemetry.EnableTracing,
		EnableMetrics:  c.Telemetry.EnableMetrics,
		SamplingRatio:  c.Telemetry.SamplingRatio,
		UseTLS:         c.Telemetry.UseTLS,
		Headers:        c.Telemetry.Headers,
		Analytics: telemetry.AnalyticsBridgeConfig{
			Enabled:        c.Telemetry.Analytics.Enabled,
			RedisAddr:      c.Telemetry.Analytics.RedisAddr,
			RedisPassword:  analyticsPassword,
			RedisDB:        c.Telemetry.Analytics.RedisDB,
			TopicPrefix:    c.Telemetry.Analytics.TopicPrefix,
			RetentionHours: c.Telemetry.Analytics.RetentionHours,
			BatchSize:      c.Telemetry.Analytics.BatchSize,
			FlushInterval:  parseTelemetryDuration(c.Telemetry.Analytics.FlushInterval),
		},
	})
	if !cfg.Enabled {
		return nil, nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return telemetry.NewGameTelemetryService(cfg, logger)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseTelemetryDuration(value string) time.Duration {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return duration
}
