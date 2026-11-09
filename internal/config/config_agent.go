package config

import (
	"gopkg.in/yaml.v3"
)

type AgentDispatchConfig struct {
	TaskRoutingDir string          `json:"taskRoutingDir,omitempty" yaml:"taskRoutingDir,omitempty"`
	TaskRoutingTTL string          `json:"taskRoutingTTL,omitempty" yaml:"taskRoutingTTL,omitempty"`
	ToAgentTLS     TLSClientConfig `json:"toAgentTLS,omitempty" yaml:"toAgentTLS,omitempty"` // Server → Agent TLS
	// HA configuration
	LoadBalanceStrategy string               `json:"loadBalanceStrategy,omitempty" yaml:"loadBalanceStrategy,omitempty"` // min_id, round_robin, least_conn, weighted
	HealthCheck         HealthCheckConfig    `json:"healthCheck,omitempty" yaml:"healthCheck,omitempty"`
	CircuitBreaker      CircuitBreakerConfig `json:"circuitBreaker,omitempty" yaml:"circuitBreaker,omitempty"`
	Reconnection        ReconnectionConfig   `json:"reconnection,omitempty" yaml:"reconnection,omitempty"`
	EnableHA            bool                 `json:"enableHA,omitempty" yaml:"enableHA,omitempty"` // Enable HA features
}

func (c *AgentDispatchConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain AgentDispatchConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		TaskRoutingDir      string          `yaml:"TaskRoutingDir,omitempty"`
		TaskRoutingTTL      string          `yaml:"TaskRoutingTTL,omitempty"`
		ToAgentTLS          TLSClientConfig `yaml:"ToAgentTLS,omitempty"`
		LoadBalanceStrategy string          `yaml:"LoadBalanceStrategy,omitempty"`
		EnableHA            *bool           `yaml:"EnableHA,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.TaskRoutingDir == "" {
		decoded.TaskRoutingDir = compat.TaskRoutingDir
	}
	if decoded.TaskRoutingTTL == "" {
		decoded.TaskRoutingTTL = compat.TaskRoutingTTL
	}
	if isZeroTLSClientConfig(decoded.ToAgentTLS) {
		decoded.ToAgentTLS = compat.ToAgentTLS
	}
	if decoded.LoadBalanceStrategy == "" {
		decoded.LoadBalanceStrategy = compat.LoadBalanceStrategy
	}
	if !decoded.EnableHA && compat.EnableHA != nil {
		decoded.EnableHA = *compat.EnableHA
	}
	*c = AgentDispatchConfig(decoded)
	return nil
}

type TLSClientConfig struct {
	// Enabled 是三态开关（nil=未配置）。nil 时「证书面齐备」默认启用
	// （ARCH-003 设计语义：cert/key/ca 全配 + 非 insecureSkip 即 mTLS），
	// 显式 false 才可降级明文；ToAgentTLS/InterconnectTLS 共用同一开关
	// 语义。
	Enabled            *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	CertFile           string `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile            string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
	CAFile             string `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	ServerName         string `json:"serverName,omitempty" yaml:"serverName,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
}

func (c *TLSClientConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain TLSClientConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	var compat struct {
		Enabled            *bool  `yaml:"Enabled,omitempty"`
		CertFile           string `yaml:"CertFile,omitempty"`
		KeyFile            string `yaml:"KeyFile,omitempty"`
		CAFile             string `yaml:"CAFile,omitempty"`
		ServerName         string `yaml:"ServerName,omitempty"`
		InsecureSkipVerify *bool  `yaml:"InsecureSkipVerify,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}

	if decoded.Enabled == nil && compat.Enabled != nil {
		v := *compat.Enabled
		decoded.Enabled = &v
	}
	if decoded.CertFile == "" {
		decoded.CertFile = compat.CertFile
	}
	if decoded.KeyFile == "" {
		decoded.KeyFile = compat.KeyFile
	}
	if decoded.CAFile == "" {
		decoded.CAFile = compat.CAFile
	}
	if decoded.ServerName == "" {
		decoded.ServerName = compat.ServerName
	}
	if !decoded.InsecureSkipVerify && compat.InsecureSkipVerify != nil {
		decoded.InsecureSkipVerify = *compat.InsecureSkipVerify
	}

	*c = TLSClientConfig(decoded)
	return nil
}

// EffectiveEnabled returns whether TLS should be activated. Unset (nil)
// defaults to true when any certificate path is configured — the ARCH-003
// contract is "certs present → mTLS unless explicitly disabled". This
// restores backwards compatibility for deployments that predate the explicit
// `enabled` knob while keeping an explicit opt-out available.
func (c *TLSClientConfig) EffectiveEnabled() bool {
	if c.Enabled != nil {
		return *c.Enabled
	}
	return c.CertFile != "" || c.KeyFile != "" || c.CAFile != ""
}

// HealthCheckConfig configures health check behavior for agent dispatch
type HealthCheckConfig struct {
	ScoreDecayRate      float64 `json:"scoreDecayRate,omitempty" yaml:"scoreDecayRate,omitempty"`
	ScoreSuccessBonus   float64 `json:"scoreSuccessBonus,omitempty" yaml:"scoreSuccessBonus,omitempty"`
	ScoreFailurePenalty float64 `json:"scoreFailurePenalty,omitempty" yaml:"scoreFailurePenalty,omitempty"`
	MinScore            float64 `json:"minScore,omitempty" yaml:"minScore,omitempty"`
	MaxScore            float64 `json:"maxScore,omitempty" yaml:"maxScore,omitempty"`
	DecayInterval       string  `json:"decayInterval,omitempty" yaml:"decayInterval,omitempty"` // duration string, e.g., "30s"
}

func (c *HealthCheckConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain HealthCheckConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		ScoreDecayRate      float64 `yaml:"ScoreDecayRate,omitempty"`
		ScoreSuccessBonus   float64 `yaml:"ScoreSuccessBonus,omitempty"`
		ScoreFailurePenalty float64 `yaml:"ScoreFailurePenalty,omitempty"`
		MinScore            float64 `yaml:"MinScore,omitempty"`
		MaxScore            float64 `yaml:"MaxScore,omitempty"`
		DecayInterval       string  `yaml:"DecayInterval,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.ScoreDecayRate == 0 {
		decoded.ScoreDecayRate = compat.ScoreDecayRate
	}
	if decoded.ScoreSuccessBonus == 0 {
		decoded.ScoreSuccessBonus = compat.ScoreSuccessBonus
	}
	if decoded.ScoreFailurePenalty == 0 {
		decoded.ScoreFailurePenalty = compat.ScoreFailurePenalty
	}
	if decoded.MinScore == 0 {
		decoded.MinScore = compat.MinScore
	}
	if decoded.MaxScore == 0 {
		decoded.MaxScore = compat.MaxScore
	}
	if decoded.DecayInterval == "" {
		decoded.DecayInterval = compat.DecayInterval
	}
	*c = HealthCheckConfig(decoded)
	return nil
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold    int32  `json:"failureThreshold,omitempty" yaml:"failureThreshold,omitempty"`
	CircuitOpenTimeout  string `json:"circuitOpenTimeout,omitempty" yaml:"circuitOpenTimeout,omitempty"` // duration string
	HalfOpenMaxRequests int32  `json:"halfOpenMaxRequests,omitempty" yaml:"halfOpenMaxRequests,omitempty"`
}

func (c *CircuitBreakerConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain CircuitBreakerConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		FailureThreshold    int32  `yaml:"FailureThreshold,omitempty"`
		CircuitOpenTimeout  string `yaml:"CircuitOpenTimeout,omitempty"`
		HalfOpenMaxRequests int32  `yaml:"HalfOpenMaxRequests,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.FailureThreshold == 0 {
		decoded.FailureThreshold = compat.FailureThreshold
	}
	if decoded.CircuitOpenTimeout == "" {
		decoded.CircuitOpenTimeout = compat.CircuitOpenTimeout
	}
	if decoded.HalfOpenMaxRequests == 0 {
		decoded.HalfOpenMaxRequests = compat.HalfOpenMaxRequests
	}
	*c = CircuitBreakerConfig(decoded)
	return nil
}

// ReconnectionConfig configures reconnection behavior
type ReconnectionConfig struct {
	MaxRetries          int     `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty"`
	InitialDelay        string  `json:"initialDelay,omitempty" yaml:"initialDelay,omitempty"` // duration string
	MaxDelay            string  `json:"maxDelay,omitempty" yaml:"maxDelay,omitempty"`         // duration string
	Multiplier          float64 `json:"multiplier,omitempty" yaml:"multiplier,omitempty"`
	Jitter              float64 `json:"jitter,omitempty" yaml:"jitter,omitempty"`
	EnableAutoReconnect bool    `json:"enableAutoReconnect,omitempty" yaml:"enableAutoReconnect,omitempty"`
}

func (c *ReconnectionConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain ReconnectionConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		MaxRetries          int     `yaml:"MaxRetries,omitempty"`
		InitialDelay        string  `yaml:"InitialDelay,omitempty"`
		MaxDelay            string  `yaml:"MaxDelay,omitempty"`
		Multiplier          float64 `yaml:"Multiplier,omitempty"`
		Jitter              float64 `yaml:"Jitter,omitempty"`
		EnableAutoReconnect *bool   `yaml:"EnableAutoReconnect,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.MaxRetries == 0 {
		decoded.MaxRetries = compat.MaxRetries
	}
	if decoded.InitialDelay == "" {
		decoded.InitialDelay = compat.InitialDelay
	}
	if decoded.MaxDelay == "" {
		decoded.MaxDelay = compat.MaxDelay
	}
	if decoded.Multiplier == 0 {
		decoded.Multiplier = compat.Multiplier
	}
	if decoded.Jitter == 0 {
		decoded.Jitter = compat.Jitter
	}
	if !decoded.EnableAutoReconnect && compat.EnableAutoReconnect != nil {
		decoded.EnableAutoReconnect = *compat.EnableAutoReconnect
	}
	*c = ReconnectionConfig(decoded)
	return nil
}
