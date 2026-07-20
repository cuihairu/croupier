package telemetry

import (
	"os"
	"strings"
)

// MergeEnv overlays OTEL_* environment variables on top of file config.
func MergeEnv(config TelemetryConfig) TelemetryConfig {
	if v, ok := lookupEnv("OTEL_ENABLED"); ok {
		config.Enabled = parseBool(v)
	}
	if v, ok := lookupEnv("CROUPIER_TELEMETRY_ENABLED"); ok {
		config.Enabled = parseBool(v)
	}
	if v, ok := lookupEnv("OTEL_SERVICE_NAME"); ok {
		config.ServiceName = v
	}
	if v, ok := lookupEnv("OTEL_SERVICE_VERSION"); ok {
		config.ServiceVersion = v
	}
	if v, ok := lookupEnv("OTEL_ENVIRONMENT"); ok {
		config.Environment = v
	}
	if v, ok := lookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT"); ok {
		config.CollectorURL = v
	}
	if v, ok := lookupEnv("GAME_ID"); ok {
		config.GameID = v
	}
	if v, ok := lookupEnv("OTEL_ENABLE_TRACING"); ok {
		config.EnableTracing = parseBool(v)
		if config.EnableTracing {
			config.Enabled = true
		}
	}
	if v, ok := lookupEnv("OTEL_ENABLE_METRICS"); ok {
		config.EnableMetrics = parseBool(v)
		if config.EnableMetrics {
			config.Enabled = true
		}
	}
	if v, ok := lookupEnv("OTEL_SAMPLING_RATIO"); ok {
		config.SamplingRatio = parseFloatOrDefault(v)
	}
	if v, ok := lookupEnv("ANALYTICS_BRIDGE_ENABLED"); ok {
		config.Analytics.Enabled = parseBool(v)
		if config.Analytics.Enabled {
			config.Enabled = true
		}
	}
	if v, ok := lookupEnv("ANALYTICS_REDIS_ADDR"); ok {
		config.Analytics.RedisAddr = v
	}
	if v, ok := lookupEnv("ANALYTICS_REDIS_PASSWORD"); ok {
		config.Analytics.RedisPassword = v
	}
	if v, ok := lookupEnv("ANALYTICS_REDIS_DB"); ok {
		config.Analytics.RedisDB = parseIntOrDefault(v)
	}
	if v, ok := lookupEnv("ANALYTICS_TOPIC_PREFIX"); ok {
		config.Analytics.TopicPrefix = v
	}
	if v, ok := lookupEnv("ANALYTICS_RETENTION_HOURS"); ok {
		config.Analytics.RetentionHours = parseIntOrDefault(v)
	}
	if v, ok := lookupEnv("ANALYTICS_BATCH_SIZE"); ok {
		config.Analytics.BatchSize = parseIntOrDefault(v)
	}
	if v, ok := lookupEnv("ANALYTICS_FLUSH_INTERVAL"); ok {
		config.Analytics.FlushInterval = parseDurationOrDefault(v)
	}

	normalizeConfig(&config)
	return config
}

func lookupEnv(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	return strings.TrimSpace(value), ok
}

func parseBool(value string) bool {
	value = strings.TrimSpace(value)
	return strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes") || strings.EqualFold(value, "on")
}
