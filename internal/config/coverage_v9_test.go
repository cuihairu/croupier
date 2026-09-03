package config

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// mustUnmarshalV9 asserts YAML decoding succeeds.
func mustUnmarshalV9(t *testing.T, input string, target interface{}) {
	t.Helper()
	if err := yaml.Unmarshal([]byte(input), target); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
}

// mustFailUnmarshalV9 asserts YAML decoding returns an error.
func mustFailUnmarshalV9(t *testing.T, input string, target interface{}) {
	t.Helper()
	if err := yaml.Unmarshal([]byte(input), target); err == nil {
		t.Fatalf("yaml.Unmarshal() expected error, input = %q", input)
	}
}

func TestTelemetryPrometheusPathV9(t *testing.T) {
	cases := []struct {
		name string
		cfg  TelemetryPrometheusConfig
		want string
	}{
		{"empty path defaults", TelemetryPrometheusConfig{}, "/metrics/prometheus"},
		{"whitespace path defaults", TelemetryPrometheusConfig{Path: "   "}, "/metrics/prometheus"},
		{"custom path kept", TelemetryPrometheusConfig{Path: "/custom-metrics"}, "/custom-metrics"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.PrometheusPath(); got != tc.want {
				t.Fatalf("PrometheusPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoginLockoutDefaultsV9(t *testing.T) {
	cases := []struct {
		name          string
		cfg           LoginLockoutConfig
		wantThreshold int
		wantLock      time.Duration
	}{
		{"zero values", LoginLockoutConfig{}, 5, 15 * time.Minute},
		{"negative values", LoginLockoutConfig{Threshold: -3, LockMinutes: -10}, 5, 15 * time.Minute},
		{"configured values", LoginLockoutConfig{Threshold: 10, LockMinutes: 30}, 10, 30 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			th, lock := tc.cfg.LoginLockoutDefaults()
			if th != tc.wantThreshold {
				t.Fatalf("threshold = %d, want %d", th, tc.wantThreshold)
			}
			if lock != tc.wantLock {
				t.Fatalf("lock = %v, want %v", lock, tc.wantLock)
			}
		})
	}
}

func TestSchemaDiffWarnEnabledV9(t *testing.T) {
	var zero, withTrue, withFalse DescriptorConfig
	tr := true
	fl := false
	withTrue.SchemaDiffWarn = &tr
	withFalse.SchemaDiffWarn = &fl
	if !zero.SchemaDiffWarnEnabled() {
		t.Fatal("nil SchemaDiffWarn should default to enabled")
	}
	if !withTrue.SchemaDiffWarnEnabled() {
		t.Fatal("explicit true should be enabled")
	}
	if withFalse.SchemaDiffWarnEnabled() {
		t.Fatal("explicit false should be disabled")
	}
}

func TestSSEConfigGetUpdateIntervalV9(t *testing.T) {
	cases := []struct {
		cfg  SSEConfig
		want int
	}{
		{SSEConfig{}, 60},
		{SSEConfig{UpdateInterval: 15}, 15},
		{SSEConfig{UpdateInterval: -5}, 60},
	}
	for i, tc := range cases {
		if got := tc.cfg.GetUpdateInterval(); got != tc.want {
			t.Fatalf("case %d: GetUpdateInterval() = %d, want %d", i, got, tc.want)
		}
	}
}

func TestConfigUnmarshalYAMLErrorsV9(t *testing.T) {
	t.Run("plain decode error", func(t *testing.T) {
		mustFailUnmarshalV9(t, "server: 123\n", &Config{})
	})
	t.Run("compat decode error", func(t *testing.T) {
		mustFailUnmarshalV9(t, "Log: 123\n", &Config{})
	})
}

func TestServerConfigUnmarshalYAMLCompatV9(t *testing.T) {
	t.Run("legacy full fallback", func(t *testing.T) {
		var cfg ServerConfig
		mustUnmarshalV9(t, "Mode: prod\nTimeout: 30000\nMaxConns: 100\n", &cfg)
		if cfg.Mode != "prod" || cfg.Timeout != 30000 || cfg.MaxConns != 100 {
			t.Fatalf("legacy fallback failed: %+v", cfg)
		}
	})
	t.Run("plain decode error", func(t *testing.T) {
		mustFailUnmarshalV9(t, "\"scalar\"\n", &ServerConfig{})
	})
	t.Run("compat decode error", func(t *testing.T) {
		mustFailUnmarshalV9(t, "Port: notanint\n", &ServerConfig{})
	})
}

func TestControlConfigUnmarshalYAMLCompatV9(t *testing.T) {
	t.Run("legacy full fallback", func(t *testing.T) {
		var cfg ControlConfig
		mustUnmarshalV9(t, "Transport: tcp\nIPCAddr: /tmp/ipc\nCert: c.pem\nKey: k.pem\nCA: ca.pem\n", &cfg)
		if cfg.Transport != "tcp" || cfg.IPCAddr != "/tmp/ipc" || cfg.Cert != "c.pem" || cfg.Key != "k.pem" || cfg.CA != "ca.pem" {
			t.Fatalf("legacy fallback failed: %+v", cfg)
		}
	})
	t.Run("plain decode error", func(t *testing.T) {
		mustFailUnmarshalV9(t, "\"scalar\"\n", &ControlConfig{})
	})
	t.Run("compat decode error", func(t *testing.T) {
		mustFailUnmarshalV9(t, "Cert: [nope]\n", &ControlConfig{})
	})
}

func TestDatabaseConfigUnmarshalYAMLErrorsV9(t *testing.T) {
	mustFailUnmarshalV9(t, "\"scalar\"\n", &DatabaseConfig{})
	mustFailUnmarshalV9(t, "Driver: [nope]\n", &DatabaseConfig{})
}

func TestRegistryConfigUnmarshalYAMLErrorsV9(t *testing.T) {
	mustFailUnmarshalV9(t, "\"scalar\"\n", &RegistryConfig{})
	mustFailUnmarshalV9(t, "RateLimitsPath: [nope]\n", &RegistryConfig{})
}

func TestAgentDispatchConfigUnmarshalYAMLCompatV9(t *testing.T) {
	t.Run("legacy routing ttl and tls", func(t *testing.T) {
		var cfg AgentDispatchConfig
		mustUnmarshalV9(t, "TaskRoutingTTL: 5m\nToAgentTLS:\n  CertFile: c.pem\n", &cfg)
		if cfg.TaskRoutingTTL != "5m" || cfg.ToAgentTLS.CertFile != "c.pem" {
			t.Fatalf("legacy fallback failed: %+v", cfg)
		}
	})
	t.Run("plain decode error", func(t *testing.T) {
		mustFailUnmarshalV9(t, "\"scalar\"\n", &AgentDispatchConfig{})
	})
	t.Run("compat decode error", func(t *testing.T) {
		mustFailUnmarshalV9(t, "TaskRoutingTTL: [nope]\n", &AgentDispatchConfig{})
	})
}

func TestTLSClientConfigUnmarshalYAMLCompatV9(t *testing.T) {
	t.Run("canonical full", func(t *testing.T) {
		var cfg TLSClientConfig
		mustUnmarshalV9(t, "enabled: true\ncertFile: c.pem\nkeyFile: k.pem\ncaFile: ca.pem\nserverName: svc\ninsecureSkipVerify: true\n", &cfg)
		if !cfg.Enabled || cfg.CertFile != "c.pem" || cfg.KeyFile != "k.pem" || cfg.CAFile != "ca.pem" || cfg.ServerName != "svc" || !cfg.InsecureSkipVerify {
			t.Fatalf("canonical decode failed: %+v", cfg)
		}
	})
	t.Run("plain decode error", func(t *testing.T) {
		mustFailUnmarshalV9(t, "\"scalar\"\n", &TLSClientConfig{})
	})
	t.Run("compat decode error", func(t *testing.T) {
		mustFailUnmarshalV9(t, "Enabled: notabool\n", &TLSClientConfig{})
	})
}

func TestHealthCheckConfigUnmarshalYAMLFullV9(t *testing.T) {
	t.Run("canonical all fields", func(t *testing.T) {
		var cfg HealthCheckConfig
		mustUnmarshalV9(t, "scoreDecayRate: 0.1\nscoreSuccessBonus: 1.5\nscoreFailurePenalty: 2.5\nminScore: 0.5\nmaxScore: 99.5\ndecayInterval: 45s\n", &cfg)
		if cfg.ScoreDecayRate != 0.1 || cfg.ScoreSuccessBonus != 1.5 || cfg.ScoreFailurePenalty != 2.5 || cfg.MinScore != 0.5 || cfg.MaxScore != 99.5 || cfg.DecayInterval != "45s" {
			t.Fatalf("canonical decode failed: %+v", cfg)
		}
	})
	t.Run("legacy all fields", func(t *testing.T) {
		var cfg HealthCheckConfig
		mustUnmarshalV9(t, "ScoreDecayRate: 0.2\nScoreSuccessBonus: 2.5\nScoreFailurePenalty: 3.5\nMinScore: 1.5\nMaxScore: 98.5\nDecayInterval: 60s\n", &cfg)
		if cfg.ScoreDecayRate != 0.2 || cfg.ScoreSuccessBonus != 2.5 || cfg.ScoreFailurePenalty != 3.5 || cfg.MinScore != 1.5 || cfg.MaxScore != 98.5 || cfg.DecayInterval != "60s" {
			t.Fatalf("legacy fallback failed: %+v", cfg)
		}
	})
	t.Run("decode errors", func(t *testing.T) {
		mustFailUnmarshalV9(t, "\"scalar\"\n", &HealthCheckConfig{})
		mustFailUnmarshalV9(t, "MinScore: notafloat\n", &HealthCheckConfig{})
	})
}

func TestCircuitBreakerConfigUnmarshalYAMLErrorsV9(t *testing.T) {
	mustFailUnmarshalV9(t, "\"scalar\"\n", &CircuitBreakerConfig{})
	mustFailUnmarshalV9(t, "FailureThreshold: notanint\n", &CircuitBreakerConfig{})
}

func TestReconnectionConfigUnmarshalYAMLFullV9(t *testing.T) {
	t.Run("legacy full fallback", func(t *testing.T) {
		var cfg ReconnectionConfig
		mustUnmarshalV9(t, "MaxRetries: 9\nInitialDelay: 2s\nMaxDelay: 40s\nMultiplier: 3.5\nJitter: 0.2\n", &cfg)
		if cfg.MaxRetries != 9 || cfg.InitialDelay != "2s" || cfg.MaxDelay != "40s" || cfg.Multiplier != 3.5 || cfg.Jitter != 0.2 {
			t.Fatalf("legacy fallback failed: %+v", cfg)
		}
	})
	t.Run("decode errors", func(t *testing.T) {
		mustFailUnmarshalV9(t, "\"scalar\"\n", &ReconnectionConfig{})
		mustFailUnmarshalV9(t, "Multiplier: notafloat\n", &ReconnectionConfig{})
	})
}

func TestAuthConfigUnmarshalYAMLErrorsV9(t *testing.T) {
	mustFailUnmarshalV9(t, "\"scalar\"\n", &AuthConfig{})
	mustFailUnmarshalV9(t, "JWTSecret: [nope]\n", &AuthConfig{})
}

func TestSmallSectionConfigsUnmarshalYAMLErrorsV9(t *testing.T) {
	t.Run("bootstrap", func(t *testing.T) {
		var b BootstrapDataConfig
		mustFailUnmarshalV9(t, "\"scalar\"\n", &b)
		mustFailUnmarshalV9(t, "BaseDir: [nope]\n", &b)
	})
	t.Run("descriptors", func(t *testing.T) {
		var d DescriptorConfig
		mustFailUnmarshalV9(t, "\"scalar\"\n", &d)
		mustFailUnmarshalV9(t, "Dir: [nope]\n", &d)
	})
	t.Run("schemas", func(t *testing.T) {
		var s SchemasConfig
		mustFailUnmarshalV9(t, "\"scalar\"\n", &s)
		mustFailUnmarshalV9(t, "Dir: [nope]\n", &s)
	})
}

func TestStorageConfigUnmarshalYAMLCompatV9(t *testing.T) {
	t.Run("legacy full fallback", func(t *testing.T) {
		var cfg StorageConfig
		mustUnmarshalV9(t, "Driver: s3\nBucket: bkt\nRegion: us-1\nEndpoint: http://e\nAccessKey: ak\nSecretKey: sk\nForcePathStyle: true\nSignedURLTTL: 5m\nBaseDir: /data\n", &cfg)
		if cfg.Driver != "s3" || cfg.Bucket != "bkt" || cfg.Region != "us-1" || cfg.Endpoint != "http://e" ||
			cfg.AccessKey != "ak" || cfg.SecretKey != "sk" || !cfg.ForcePathStyle || cfg.SignedURLTTL != "5m" || cfg.BaseDir != "/data" {
			t.Fatalf("legacy fallback failed: %+v", cfg)
		}
	})
	t.Run("decode errors", func(t *testing.T) {
		mustFailUnmarshalV9(t, "\"scalar\"\n", &StorageConfig{})
		mustFailUnmarshalV9(t, "Bucket: [nope]\n", &StorageConfig{})
	})
}

func TestMetricsConfigUnmarshalYAMLCompatV9(t *testing.T) {
	t.Run("legacy bools", func(t *testing.T) {
		var cfg MetricsConfig
		mustUnmarshalV9(t, "PerFunction: true\nPerGameDenies: true\n", &cfg)
		if !cfg.PerFunction || !cfg.PerGameDenies {
			t.Fatalf("legacy fallback failed: %+v", cfg)
		}
	})
	t.Run("decode errors", func(t *testing.T) {
		mustFailUnmarshalV9(t, "\"scalar\"\n", &MetricsConfig{})
		mustFailUnmarshalV9(t, "PerFunction: notabool\n", &MetricsConfig{})
	})
}

func TestCacheConfigUnmarshalYAMLFullV9(t *testing.T) {
	t.Run("canonical full", func(t *testing.T) {
		var cfg CacheConfig
		mustUnmarshalV9(t, "enabled: true\ntype: redis\naddr: r:6379\npassword: pw\ndb: 2\npoolSize: 8\nttl: 5m\nmaxItems: 100\nevictTTL: 30s\n", &cfg)
		if !cfg.Enabled || cfg.Type != "redis" || cfg.Addr != "r:6379" || cfg.Password != "pw" ||
			cfg.DB != 2 || cfg.PoolSize != 8 || cfg.TTL != "5m" || cfg.MaxItems != 100 || cfg.EvictTTL != "30s" {
			t.Fatalf("canonical decode failed: %+v", cfg)
		}
	})
	t.Run("legacy full fallback", func(t *testing.T) {
		var cfg CacheConfig
		mustUnmarshalV9(t, "Enabled: true\nType: local\nAddr: l:6379\nPassword: pw2\nDB: 3\nPoolSize: 16\nTTL: 1h\nMaxItems: 50\nEvictTTL: 1m\n", &cfg)
		if !cfg.Enabled || cfg.Type != "local" || cfg.Addr != "l:6379" || cfg.Password != "pw2" ||
			cfg.DB != 3 || cfg.PoolSize != 16 || cfg.TTL != "1h" || cfg.MaxItems != 50 || cfg.EvictTTL != "1m" {
			t.Fatalf("legacy fallback failed: %+v", cfg)
		}
	})
	t.Run("decode errors", func(t *testing.T) {
		mustFailUnmarshalV9(t, "\"scalar\"\n", &CacheConfig{})
		mustFailUnmarshalV9(t, "PoolSize: notanint\n", &CacheConfig{})
	})
}

func TestSSEConfigUnmarshalYAMLCompatV9(t *testing.T) {
	t.Run("legacy fallback", func(t *testing.T) {
		var cfg SSEConfig
		mustUnmarshalV9(t, "UpdateInterval: 10\nKeepAliveInterval: 5\n", &cfg)
		if cfg.UpdateInterval != 10 || cfg.KeepAliveInterval != 5 {
			t.Fatalf("legacy fallback failed: %+v", cfg)
		}
	})
	t.Run("canonical keepalive", func(t *testing.T) {
		var cfg SSEConfig
		mustUnmarshalV9(t, "updateInterval: 3\nkeepAliveInterval: 7\n", &cfg)
		if cfg.UpdateInterval != 3 || cfg.KeepAliveInterval != 7 {
			t.Fatalf("canonical decode failed: %+v", cfg)
		}
	})
	t.Run("decode errors", func(t *testing.T) {
		mustFailUnmarshalV9(t, "\"scalar\"\n", &SSEConfig{})
		mustFailUnmarshalV9(t, "UpdateInterval: notanint\n", &SSEConfig{})
	})
}

func TestConfigUnmarshalYAMLCanonicalLogPrecedenceV9(t *testing.T) {
	var cfg Config
	mustUnmarshalV9(t, "log:\n  level: warn\n  format: json\n", &cfg)
	if cfg.Logging.Level != "warn" || cfg.Logging.Format != "json" {
		t.Fatalf("canonical log decode failed: %+v", cfg.Logging)
	}
}
