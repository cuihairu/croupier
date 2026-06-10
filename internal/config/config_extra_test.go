package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRegistryConfig_UnmarshalYAML_Canonical(t *testing.T) {
	input := `
assignmentsPath: data/assignments.json
analyticsFiltersPath: data/filters.json
rateLimitsPath: data/limits.json
`
	var cfg RegistryConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.AssignmentsPath != "data/assignments.json" {
		t.Errorf("AssignmentsPath = %q", cfg.AssignmentsPath)
	}
	if cfg.AnalyticsFiltersPath != "data/filters.json" {
		t.Errorf("AnalyticsFiltersPath = %q", cfg.AnalyticsFiltersPath)
	}
	if cfg.RateLimitsPath != "data/limits.json" {
		t.Errorf("RateLimitsPath = %q", cfg.RateLimitsPath)
	}
}

func TestRegistryConfig_UnmarshalYAML_Legacy(t *testing.T) {
	input := `
AssignmentsPath: legacy/assignments.json
AnalyticsFiltersPath: legacy/filters.json
RateLimitsPath: legacy/limits.json
`
	var cfg RegistryConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.AssignmentsPath != "legacy/assignments.json" {
		t.Errorf("AssignmentsPath = %q", cfg.AssignmentsPath)
	}
}

func TestHealthCheckConfig_UnmarshalYAML_Canonical(t *testing.T) {
	input := `
scoreDecayRate: 0.1
scoreSuccessBonus: 5.0
scoreFailurePenalty: 10.0
minScore: 0.0
maxScore: 100.0
decayInterval: 30s
`
	var cfg HealthCheckConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.ScoreDecayRate != 0.1 {
		t.Errorf("ScoreDecayRate = %f", cfg.ScoreDecayRate)
	}
	if cfg.DecayInterval != "30s" {
		t.Errorf("DecayInterval = %q", cfg.DecayInterval)
	}
}

func TestHealthCheckConfig_UnmarshalYAML_Legacy(t *testing.T) {
	input := `
ScoreDecayRate: 0.2
DecayInterval: 60s
`
	var cfg HealthCheckConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.ScoreDecayRate != 0.2 {
		t.Errorf("ScoreDecayRate = %f", cfg.ScoreDecayRate)
	}
	if cfg.DecayInterval != "60s" {
		t.Errorf("DecayInterval = %q", cfg.DecayInterval)
	}
}

func TestCircuitBreakerConfig_UnmarshalYAML_Canonical(t *testing.T) {
	input := `
failureThreshold: 5
circuitOpenTimeout: 30s
halfOpenMaxRequests: 3
`
	var cfg CircuitBreakerConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %d", cfg.FailureThreshold)
	}
	if cfg.CircuitOpenTimeout != "30s" {
		t.Errorf("CircuitOpenTimeout = %q", cfg.CircuitOpenTimeout)
	}
	if cfg.HalfOpenMaxRequests != 3 {
		t.Errorf("HalfOpenMaxRequests = %d", cfg.HalfOpenMaxRequests)
	}
}

func TestCircuitBreakerConfig_UnmarshalYAML_Legacy(t *testing.T) {
	input := `
FailureThreshold: 10
CircuitOpenTimeout: 60s
HalfOpenMaxRequests: 5
`
	var cfg CircuitBreakerConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.FailureThreshold != 10 {
		t.Errorf("FailureThreshold = %d", cfg.FailureThreshold)
	}
}

func TestReconnectionConfig_UnmarshalYAML_Canonical(t *testing.T) {
	input := `
maxRetries: 3
initialDelay: 1s
maxDelay: 30s
multiplier: 2.0
jitter: 0.5
enableAutoReconnect: true
`
	var cfg ReconnectionConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d", cfg.MaxRetries)
	}
	if !cfg.EnableAutoReconnect {
		t.Error("EnableAutoReconnect should be true")
	}
}

func TestReconnectionConfig_UnmarshalYAML_Legacy(t *testing.T) {
	input := `
MaxRetries: 5
InitialDelay: 2s
EnableAutoReconnect: true
`
	var cfg ReconnectionConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d", cfg.MaxRetries)
	}
	if cfg.InitialDelay != "2s" {
		t.Errorf("InitialDelay = %q", cfg.InitialDelay)
	}
	if !cfg.EnableAutoReconnect {
		t.Error("EnableAutoReconnect should be true")
	}
}

func TestAuthConfig_UnmarshalYAML_Canonical(t *testing.T) {
	input := `
jwtSecret: my-secret
rbacConfig: configs/rbac.json
usersConfig: configs/users.json
gamesConfig: configs/games.json
`
	var cfg AuthConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.JWTSecret != "my-secret" {
		t.Errorf("JWTSecret = %q", cfg.JWTSecret)
	}
	if cfg.RBACConfig != "configs/rbac.json" {
		t.Errorf("RBACConfig = %q", cfg.RBACConfig)
	}
}

func TestAuthConfig_UnmarshalYAML_Legacy(t *testing.T) {
	input := `
JWTSecret: legacy-secret
RBACConfig: legacy/rbac.json
UsersConfig: legacy/users.json
GamesConfig: legacy/games.json
`
	var cfg AuthConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.JWTSecret != "legacy-secret" {
		t.Errorf("JWTSecret = %q", cfg.JWTSecret)
	}
	if cfg.UsersConfig != "legacy/users.json" {
		t.Errorf("UsersConfig = %q", cfg.UsersConfig)
	}
}

func TestAgentDispatchConfig_UnmarshalYAML_Legacy(t *testing.T) {
	input := `
TaskRoutingDir: legacy/data
TaskRoutingTTL: 5m
LoadBalanceStrategy: round_robin
EnableHA: true
`
	var cfg AgentDispatchConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.TaskRoutingDir != "legacy/data" {
		t.Errorf("TaskRoutingDir = %q", cfg.TaskRoutingDir)
	}
	if cfg.LoadBalanceStrategy != "round_robin" {
		t.Errorf("LoadBalanceStrategy = %q", cfg.LoadBalanceStrategy)
	}
	if !cfg.EnableHA {
		t.Error("EnableHA should be true")
	}
}

func TestTLSClientConfig_UnmarshalYAML_Legacy(t *testing.T) {
	input := `
Enabled: true
CertFile: /path/to/cert.pem
KeyFile: /path/to/key.pem
CAFile: /path/to/ca.pem
ServerName: example.com
InsecureSkipVerify: true
`
	var cfg TLSClientConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.CertFile != "/path/to/cert.pem" {
		t.Errorf("CertFile = %q", cfg.CertFile)
	}
	if !cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}

func TestFullConfig_UnmarshalYAML_WithAllLegacyKeys(t *testing.T) {
	input := `
Server:
  Host: 0.0.0.0
  Port: 9999
database:
  driver: mysql
  dataSource: "user:pass@tcp(localhost:3306)/db"
auth:
  jwtSecret: test-secret
AgentDispatch:
  TaskRoutingDir: data
  EnableHA: true
  HealthCheck:
    ScoreDecayRate: 0.1
  CircuitBreaker:
    FailureThreshold: 5
  Reconnection:
    MaxRetries: 3
    EnableAutoReconnect: true
registry:
  assignmentsPath: data/assign.json
Control:
  Addr: ":19090"
Log:
  Level: info
  Format: json
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("Server.Port = %d", cfg.Server.Port)
	}
	if cfg.Database.Driver != "mysql" {
		t.Errorf("Database.Driver = %q", cfg.Database.Driver)
	}
	if cfg.Auth.JWTSecret != "test-secret" {
		t.Errorf("Auth.JWTSecret = %q", cfg.Auth.JWTSecret)
	}
	if cfg.AgentDispatch.TaskRoutingDir != "data" {
		t.Errorf("AgentDispatch.TaskRoutingDir = %q", cfg.AgentDispatch.TaskRoutingDir)
	}
	if cfg.Registry.AssignmentsPath != "data/assign.json" {
		t.Errorf("Registry.AssignmentsPath = %q", cfg.Registry.AssignmentsPath)
	}
}
