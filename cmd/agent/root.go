package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
	"github.com/cuihairu/croupier/internal/cli/common"
	"github.com/cuihairu/croupier/internal/devcert"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = ""

	cfgFile string
	mode    string
	debug   bool
)

var rootCmd = &cobra.Command{
	Use:   "croupier-agent",
	Short: "Croupier Agent 服务守护进程",
	Long: `Croupier Agent 用于与游戏实例交互、分发作业以及上报指标。

支持通过配置文件或命令行参数覆盖运行时行为。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgent()
	},
}

// Execute runs the CLI entrypoint.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// main is the entry point for the croupier-agent CLI
func main() {
	Execute()
}

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("{{printf \"%s\\n\" .Version}}")

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "f", "etc/agent.yaml", "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&mode, "mode", "m", "dev", "运行模式 (dev|prod|test)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "启用调试模式")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "agent",
		Short: "Run agent (alias)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgent()
		},
	})
	rootCmd.AddCommand(versionCmd)
}

// AgentConfig represents the agent-specific configuration
type AgentConfig struct {
	Name        string              `json:"name" yaml:"name"`
	Host        string              `json:"host" yaml:"host"`
	Port        int                 `json:"port" yaml:"port"`
	Server      AgentServerConfig   `json:"server" yaml:"server"`
	Agent       AgentInfoConfig     `json:"agent" yaml:"agent"`
	Upstream    AgentUpstreamConfig `json:"upstream" yaml:"upstream"`
	Logging     common.LogConfig    `json:"log" yaml:"log"`
	TLS         AgentTLSConfig      `json:"tls" yaml:"tls"`
	OutboundTLS AgentTLSConfig      `json:"outboundTLS" yaml:"outboundTLS"`
	Ops         *OpsConfig          `json:"ops,omitempty" yaml:"ops,omitempty"`
}

// OpsConfig represents the ops module configuration
type OpsConfig struct {
	Enabled         bool   `json:"enabled" yaml:"enabled"`
	MetricsInterval string `json:"metrics_interval" yaml:"metrics_interval"`
	MetricsEnabled  bool   `json:"metrics_enabled" yaml:"metrics_enabled"`
}

type AgentTLSConfig struct {
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	CertFile           string `json:"certFile" yaml:"certFile"`
	KeyFile            string `json:"keyFile" yaml:"keyFile"`
	CAFile             string `json:"caFile" yaml:"caFile"`
	ServerName         string `json:"serverName,omitempty" yaml:"serverName,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify" yaml:"insecureSkipVerify"`
}

func (c *AgentTLSConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain AgentTLSConfig
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
	if !decoded.Enabled && compat.Enabled != nil {
		decoded.Enabled = *compat.Enabled
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
	*c = AgentTLSConfig(decoded)
	return nil
}

func (c *AgentConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain AgentConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	var compat struct {
		Name        string               `yaml:"Name"`
		Host        string               `yaml:"Host"`
		Port        int                  `yaml:"Port"`
		Server      AgentServerConfig    `yaml:"Server"`
		Agent       AgentInfoConfig      `yaml:"Agent"`
		Upstream    AgentUpstreamConfig  `yaml:"Upstream"`
		Logging     legacyAgentLogConfig `yaml:"Logging"`
		TLS         AgentTLSConfig       `yaml:"TLS"`
		OutboundTLS AgentTLSConfig       `yaml:"OutboundTLS"`
	}
	var canonical struct {
		Logging     canonicalAgentLogConfig `yaml:"log"`
		TLS         AgentTLSConfig          `yaml:"tls"`
		OutboundTLS AgentTLSConfig          `yaml:"outboundTLS"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if err := value.Decode(&canonical); err != nil {
		return err
	}

	if decoded.Name == "" {
		decoded.Name = compat.Name
	}
	if decoded.Host == "" {
		decoded.Host = compat.Host
	}
	if decoded.Port == 0 {
		decoded.Port = compat.Port
	}
	if isZeroAgentServerConfig(decoded.Server) {
		decoded.Server = compat.Server
	}
	if isZeroAgentInfoConfig(decoded.Agent) {
		decoded.Agent = compat.Agent
	}
	if isZeroAgentUpstreamConfig(decoded.Upstream) {
		decoded.Upstream = compat.Upstream
	}
	if isZeroCommonLogConfig(decoded.Logging) {
		switch {
		case !canonical.Logging.isZero():
			decoded.Logging = canonical.Logging.toCommon()
		case !compat.Logging.isZero():
			decoded.Logging = compat.Logging.toCommon()
		}
	}
	if isZeroInlineTLSConfig(decoded.TLS) {
		switch {
		case !isZeroAgentTLSConfig(canonical.TLS):
			decoded.TLS = canonical.TLS
		case !isZeroAgentTLSConfig(compat.TLS):
			decoded.TLS = compat.TLS
		}
	}
	if isZeroInlineTLSConfig(decoded.OutboundTLS) {
		switch {
		case !isZeroAgentTLSConfig(canonical.OutboundTLS):
			decoded.OutboundTLS = canonical.OutboundTLS
		case !isZeroAgentTLSConfig(compat.OutboundTLS):
			decoded.OutboundTLS = compat.OutboundTLS
		}
	}

	cfg := AgentConfig(decoded)
	applyAgentConfigDefaults(&cfg)
	*c = cfg
	return nil
}

type AgentServerConfig struct {
	Addr               string `json:"addr" yaml:"addr"`
	Transport          string `json:"transport,omitempty" yaml:"transport,omitempty"`
	Insecure           bool   `json:"insecure" yaml:"insecure"`
	ServerName         string `json:"serverName" yaml:"serverName"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify" yaml:"insecureSkipVerify"`
	TLSCertFile        string `json:"tlsCertFile" yaml:"tlsCertFile"`
	TLSKeyFile         string `json:"tlsKeyFile" yaml:"tlsKeyFile"`
	CAFile             string `json:"caFile" yaml:"caFile"`
}

func (c *AgentServerConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain AgentServerConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		Addr               string `yaml:"Addr,omitempty"`
		Transport          string `yaml:"Transport,omitempty"`
		Insecure           *bool  `yaml:"Insecure,omitempty"`
		ServerName         string `yaml:"ServerName,omitempty"`
		InsecureSkipVerify *bool  `yaml:"InsecureSkipVerify,omitempty"`
		TLSCertFile        string `yaml:"TLSCertFile,omitempty"`
		TLSKeyFile         string `yaml:"TLSKeyFile,omitempty"`
		CAFile             string `yaml:"CAFile,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.Addr == "" {
		decoded.Addr = compat.Addr
	}
	if decoded.Transport == "" {
		decoded.Transport = compat.Transport
	}
	if !decoded.Insecure && compat.Insecure != nil {
		decoded.Insecure = *compat.Insecure
	}
	if decoded.ServerName == "" {
		decoded.ServerName = compat.ServerName
	}
	if !decoded.InsecureSkipVerify && compat.InsecureSkipVerify != nil {
		decoded.InsecureSkipVerify = *compat.InsecureSkipVerify
	}
	if decoded.TLSCertFile == "" {
		decoded.TLSCertFile = compat.TLSCertFile
	}
	if decoded.TLSKeyFile == "" {
		decoded.TLSKeyFile = compat.TLSKeyFile
	}
	if decoded.CAFile == "" {
		decoded.CAFile = compat.CAFile
	}
	*c = AgentServerConfig(decoded)
	return nil
}

type AgentInfoConfig struct {
	ID        string            `json:"id" yaml:"id"`
	GameID    string            `json:"gameId" yaml:"gameId"`
	Env       string            `json:"env" yaml:"env"`
	Transport string            `json:"transport,omitempty" yaml:"transport,omitempty"`
	LocalAddr string            `json:"localAddr" yaml:"localAddr"`
	HTTPAddr  string            `json:"httpAddr" yaml:"httpAddr"`
	Labels    map[string]string `json:"labels" yaml:"labels"`
}

func (c *AgentInfoConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain AgentInfoConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		ID        string            `yaml:"ID,omitempty"`
		GameID    string            `yaml:"GameID,omitempty"`
		Env       string            `yaml:"Env,omitempty"`
		Transport string            `yaml:"Transport,omitempty"`
		LocalAddr string            `yaml:"LocalAddr,omitempty"`
		HTTPAddr  string            `yaml:"HTTPAddr,omitempty"`
		Labels    map[string]string `yaml:"Labels,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.ID == "" {
		decoded.ID = compat.ID
	}
	if decoded.GameID == "" {
		decoded.GameID = compat.GameID
	}
	if decoded.Env == "" {
		decoded.Env = compat.Env
	}
	if decoded.Transport == "" {
		decoded.Transport = compat.Transport
	}
	if decoded.LocalAddr == "" {
		decoded.LocalAddr = compat.LocalAddr
	}
	if decoded.HTTPAddr == "" {
		decoded.HTTPAddr = compat.HTTPAddr
	}
	if decoded.Labels == nil {
		decoded.Labels = compat.Labels
	}
	*c = AgentInfoConfig(decoded)
	return nil
}

type AgentUpstreamConfig struct {
	HeartbeatInterval int `json:"heartbeatInterval" yaml:"heartbeatInterval"`
	RetryInterval     int `json:"retryInterval" yaml:"retryInterval"`
	MaxRetries        int `json:"maxRetries" yaml:"maxRetries"`
	Timeout           int `json:"timeout" yaml:"timeout"`
}

func (c *AgentUpstreamConfig) UnmarshalYAML(value *yaml.Node) error {
	type plain AgentUpstreamConfig
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	var compat struct {
		HeartbeatInterval *int `yaml:"HeartbeatInterval,omitempty"`
		RetryInterval     *int `yaml:"RetryInterval,omitempty"`
		MaxRetries        *int `yaml:"MaxRetries,omitempty"`
		Timeout           *int `yaml:"Timeout,omitempty"`
	}
	if err := value.Decode(&compat); err != nil {
		return err
	}
	if decoded.HeartbeatInterval == 0 && compat.HeartbeatInterval != nil {
		decoded.HeartbeatInterval = *compat.HeartbeatInterval
	}
	if decoded.RetryInterval == 0 && compat.RetryInterval != nil {
		decoded.RetryInterval = *compat.RetryInterval
	}
	if decoded.MaxRetries == 0 && compat.MaxRetries != nil {
		decoded.MaxRetries = *compat.MaxRetries
	}
	if decoded.Timeout == 0 && compat.Timeout != nil {
		decoded.Timeout = *compat.Timeout
	}
	*c = AgentUpstreamConfig(decoded)
	return nil
}

type canonicalAgentLogConfig struct {
	Level      string `yaml:"level,omitempty"`
	Format     string `yaml:"format,omitempty"`
	Output     string `yaml:"output,omitempty"`
	File       string `yaml:"file,omitempty"`
	MaxSize    int    `yaml:"maxSize,omitempty"`
	MaxBackups int    `yaml:"maxBackups,omitempty"`
	MaxAge     int    `yaml:"maxAge,omitempty"`
	Compress   bool   `yaml:"compress,omitempty"`
}

func (c canonicalAgentLogConfig) isZero() bool { return c == (canonicalAgentLogConfig{}) }
func (c canonicalAgentLogConfig) toCommon() common.LogConfig {
	return common.LogConfig{
		Level: c.Level, Format: c.Format, Output: c.Output, File: c.File,
		MaxSize: c.MaxSize, MaxBackups: c.MaxBackups, MaxAge: c.MaxAge, Compress: c.Compress,
	}
}

type legacyAgentLogConfig struct {
	Level      string `yaml:"Level,omitempty"`
	Format     string `yaml:"Format,omitempty"`
	Output     string `yaml:"Output,omitempty"`
	File       string `yaml:"File,omitempty"`
	MaxSize    int    `yaml:"MaxSize,omitempty"`
	MaxBackups int    `yaml:"MaxBackups,omitempty"`
	MaxAge     int    `yaml:"MaxAge,omitempty"`
	Compress   bool   `yaml:"Compress,omitempty"`
}

func (c legacyAgentLogConfig) isZero() bool { return c == (legacyAgentLogConfig{}) }
func (c legacyAgentLogConfig) toCommon() common.LogConfig {
	return common.LogConfig{
		Level: c.Level, Format: c.Format, Output: c.Output, File: c.File,
		MaxSize: c.MaxSize, MaxBackups: c.MaxBackups, MaxAge: c.MaxAge, Compress: c.Compress,
	}
}

func isZeroAgentServerConfig(cfg AgentServerConfig) bool { return cfg == (AgentServerConfig{}) }
func isZeroAgentInfoConfig(cfg AgentInfoConfig) bool {
	return cfg.ID == "" &&
		cfg.GameID == "" &&
		cfg.Env == "" &&
		cfg.LocalAddr == "" &&
		cfg.HTTPAddr == "" &&
		len(cfg.Labels) == 0
}
func isZeroAgentUpstreamConfig(cfg AgentUpstreamConfig) bool {
	return cfg == (AgentUpstreamConfig{})
}
func isZeroCommonLogConfig(cfg common.LogConfig) bool { return cfg == (common.LogConfig{}) }
func isZeroAgentTLSConfig(cfg AgentTLSConfig) bool    { return cfg == (AgentTLSConfig{}) }
func isZeroInlineTLSConfig(cfg AgentTLSConfig) bool   { return cfg == (AgentTLSConfig{}) }

func applyAgentConfigDefaults(cfg *AgentConfig) {
	if cfg == nil {
		return
	}

	// Agent <-> Server defaults to TLS. The legacy "insecure" switch only
	// changes this when callers explicitly opt out.
	if strings.TrimSpace(cfg.Server.Addr) != "" && !cfg.Server.Insecure {
		cfg.Server.Insecure = false
	}

	// SDK <-> Agent local gateway remains plain TCP by default unless TLS is
	// explicitly configured.
	if isZeroAgentTLSConfig(cfg.TLS) {
		cfg.TLS.Enabled = false
	}
}

func runAgent() error {
	if cfgFile == "" {
		return fmt.Errorf("配置文件是必需的")
	}

	// 获取配置文件所在目录（用于 providers.yaml 等辅助配置）
	configDir := filepath.Dir(cfgFile)

	// Read YAML config file
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var c AgentConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	slog.Info("parsed agent upstream config",
		"server_addr", strings.TrimSpace(c.Server.Addr),
		"local_addr", strings.TrimSpace(c.Agent.LocalAddr),
		"http_addr", strings.TrimSpace(c.Agent.HTTPAddr),
	)

	// 初始化日志系统
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "console"
	}
	common.SetupLoggerWithFile(
		c.Logging.Level,
		c.Logging.Format,
		c.Logging.Output,
		c.Logging.MaxSize,
		c.Logging.MaxBackups,
		c.Logging.MaxAge,
		c.Logging.Compress,
	)

	runCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	core, localAddr, err := startAgentCore(runCtx, &c, configDir)
	if err != nil {
		return err
	}

	defer func() {
		if core != nil {
			core.Stop()
		}
	}()

	fmt.Printf("Starting Croupier Agent (mode: %s, debug: %v)...\n", mode, debug)
	slog.Info("agent local server started", "listen", localAddr)

	<-runCtx.Done()
	return nil
}

func startAgentCore(ctx context.Context, c *AgentConfig, configDir string) (*agentcore.App, string, error) {
	if c == nil {
		return nil, "", fmt.Errorf("missing config")
	}

	slog.Info("loading agent config", "config_file", cfgFile, "config_dir", configDir)

	// SDK 连接的是 Agent 本地 TCP 服务，而不是 Server 控制端口。
	localListenAddr := strings.TrimSpace(c.Agent.LocalAddr)
	if localListenAddr == "" {
		host := strings.TrimSpace(c.Host)
		switch host {
		case "", "0.0.0.0":
			localListenAddr = net.JoinHostPort("0.0.0.0", strconv.Itoa(c.Port))
		default:
			localListenAddr = net.JoinHostPort(host, strconv.Itoa(c.Port))
		}
	}
	localListenAddr = strings.TrimPrefix(localListenAddr, "tcp://")
	localDisplayAddr := localListenAddr
	if strings.HasPrefix(localDisplayAddr, ":") {
		localDisplayAddr = "0.0.0.0" + localDisplayAddr
	}
	localAddr := localListenAddr

	agentID := resolveAgentID(strings.TrimSpace(c.Agent.ID))

	// 收集系统标签
	labels := collectSystemLabels()
	// Merge with config labels
	for k, v := range c.Agent.Labels {
		labels[k] = v
	}

	// 使用 NewWithConfigDir 以确保 providers.yaml 能从正确的目录加载
	core := agentcore.NewWithConfigDir(strings.TrimSpace(c.Server.Addr), agentID, configDir)
	core.SetLocalAddr(localAddr)
	core.SetUpstreamTransportKind(strings.TrimSpace(c.Server.Transport))
	core.WithUpstreamMetadata(agentcore.UpstreamMetadata{
		GameID:            strings.TrimSpace(c.Agent.GameID),
		Env:               strings.TrimSpace(c.Agent.Env),
		Version:           Version,
		Region:            "",
		Zone:              "",
		Labels:            labels,
		DialTimeout:       time.Duration(c.Upstream.Timeout) * time.Millisecond,
		RequestTimeout:    time.Duration(c.Upstream.Timeout) * time.Millisecond,
		HeartbeatInterval: time.Duration(c.Upstream.HeartbeatInterval) * time.Second,
	})

	// Ensure CA cert exists (if needed for Server certificate verification)
	if !c.Server.Insecure && strings.TrimSpace(c.Server.CAFile) != "" {
		caFile := strings.TrimSpace(c.Server.CAFile)
		if _, err := os.Stat(caFile); os.IsNotExist(err) {
			certDir := filepath.Join(filepath.Dir(cfgFile), "certs")
			fmt.Printf("CA cert not found, auto-generating dev certs in %s...\n", certDir)
			if _, _, err := devcert.EnsureDevCA(certDir); err != nil {
				return nil, "", fmt.Errorf("failed to generate CA cert: %w", err)
			}
			fmt.Printf("  CA: %s\n", filepath.Join(certDir, "ca.crt"))
		}
	}

	if !c.Server.Insecure {
		core.WithUpstreamTLSConfig(&tlsutil.ClientTLSConfig{
			CertFile:           strings.TrimSpace(c.Server.TLSCertFile),
			KeyFile:            strings.TrimSpace(c.Server.TLSKeyFile),
			CAFile:             strings.TrimSpace(c.Server.CAFile),
			ServerName:         strings.TrimSpace(c.Server.ServerName),
			InsecureSkipVerify: c.Server.InsecureSkipVerify,
		})
	} else {
		core.WithUpstreamTLSConfig(nil)
	}

	if c.OutboundTLS.Enabled {
		core.WithOutboundTLSConfig(&tlsutil.ClientTLSConfig{
			CertFile:           strings.TrimSpace(c.OutboundTLS.CertFile),
			KeyFile:            strings.TrimSpace(c.OutboundTLS.KeyFile),
			CAFile:             strings.TrimSpace(c.OutboundTLS.CAFile),
			ServerName:         strings.TrimSpace(c.OutboundTLS.ServerName),
			InsecureSkipVerify: c.OutboundTLS.InsecureSkipVerify,
		})
	} else {
		core.WithOutboundTLSConfig(nil)
	}

	// Configure ops module if enabled
	if c.Ops != nil && c.Ops.Enabled {
		opsCfg := agentcore.DefaultOpsConfig()
		opsCfg.Enabled = true
		opsCfg.MetricsEnabled = c.Ops.MetricsEnabled
		if c.Ops.MetricsInterval != "" {
			if d, err := time.ParseDuration(c.Ops.MetricsInterval); err == nil {
				opsCfg.MetricsInterval = d
			}
		}
		core.WithOpsConfig(opsCfg)
		slog.Info("ops module enabled", "metrics_interval", opsCfg.MetricsInterval)
	}

	// Start the agent (which now starts TCP local server internally)
	go func() {
		if err := core.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("agent run failed", "error", err)
		}
	}()

	return core, localAddr, nil
}

func resolveAgentID(configured string) string {
	if strings.TrimSpace(configured) != "" {
		return strings.TrimSpace(configured)
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "agent"
	}
	return fmt.Sprintf("%s-%d", host, time.Now().Unix())
}

// collectSystemLabels 收集系统信息作为标签
func collectSystemLabels() map[string]string {
	labels := make(map[string]string)

	// 操作系统
	labels["os"] = runtime.GOOS
	// CPU 架构
	labels["arch"] = runtime.GOARCH

	// 主机名
	if hostname, err := os.Hostname(); err == nil {
		labels["hostname"] = hostname
	}

	// 获取第一个非回环 IP 地址
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					labels["ip"] = ipnet.IP.String()
					break
				}
			}
		}
	}

	// CPU 核心数
	labels["cpu_count"] = fmt.Sprintf("%d", runtime.NumCPU())

	// Go 版本
	labels["go_version"] = runtime.Version()

	return labels
}
