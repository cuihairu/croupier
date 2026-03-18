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
	Name     string              `json:"Name" yaml:"Name"`
	Host     string              `json:"Host" yaml:"Host"`
	Port     int                 `json:"Port" yaml:"Port"`
	Server   AgentServerConfig   `json:"Server" yaml:"Server"`
	Agent    AgentInfoConfig     `json:"Agent" yaml:"Agent"`
	Upstream AgentUpstreamConfig `json:"Upstream" yaml:"Upstream"`
	Logging  common.LogConfig    `json:"Logging" yaml:"Logging"`
	TLS      struct {
		Enabled            bool   `json:"Enabled" yaml:"Enabled"`
		CertFile           string `json:"CertFile" yaml:"CertFile"`
		KeyFile            string `json:"KeyFile" yaml:"KeyFile"`
		CAFile             string `json:"CAFile" yaml:"CAFile"`
		InsecureSkipVerify bool   `json:"InsecureSkipVerify" yaml:"InsecureSkipVerify"`
	} `json:"TLS" yaml:"TLS"`
	OutboundTLS struct {
		Enabled            bool   `json:"Enabled" yaml:"Enabled"`
		CertFile           string `json:"CertFile" yaml:"CertFile"`
		KeyFile            string `json:"KeyFile" yaml:"KeyFile"`
		CAFile             string `json:"CAFile" yaml:"CAFile"`
		ServerName         string `json:"ServerName" yaml:"ServerName"`
		InsecureSkipVerify bool   `json:"InsecureSkipVerify" yaml:"InsecureSkipVerify"`
	} `json:"OutboundTLS" yaml:"OutboundTLS"`
}

type AgentServerConfig struct {
	Addr               string `json:"Addr" yaml:"Addr"`
	Insecure           bool   `json:"Insecure" yaml:"Insecure"`
	ServerName         string `json:"ServerName" yaml:"ServerName"`
	InsecureSkipVerify bool   `json:"InsecureSkipVerify" yaml:"InsecureSkipVerify"`
	TLSCertFile        string `json:"TLSCertFile" yaml:"TLSCertFile"`
	TLSKeyFile         string `json:"TLSKeyFile" yaml:"TLSKeyFile"`
	CAFile             string `json:"CAFile" yaml:"CAFile"`
}

type AgentInfoConfig struct {
	ID        string            `json:"ID" yaml:"ID"`
	GameID    string            `json:"GameID" yaml:"GameID"`
	Env       string            `json:"Env" yaml:"Env"`
	LocalAddr string            `json:"LocalAddr" yaml:"LocalAddr"`
	HTTPAddr  string            `json:"HTTPAddr" yaml:"HTTPAddr"`
	Labels    map[string]string `json:"Labels" yaml:"Labels"`
}

type AgentUpstreamConfig struct {
	HeartbeatInterval int `json:"HeartbeatInterval" yaml:"HeartbeatInterval"`
	RetryInterval     int `json:"RetryInterval" yaml:"RetryInterval"`
	MaxRetries        int `json:"MaxRetries" yaml:"MaxRetries"`
	Timeout           int `json:"Timeout" yaml:"Timeout"`
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

	core, nngAddr, err := startAgentCore(runCtx, &c, configDir)
	if err != nil {
		return err
	}

	defer func() {
		if core != nil {
			core.Stop()
		}
	}()

	fmt.Printf("Starting Croupier Agent (mode: %s, debug: %v)...\n", mode, debug)
	slog.Info("agent nng core started", "listen", nngAddr)

	<-runCtx.Done()
	return nil
}

func startAgentCore(ctx context.Context, c *AgentConfig, configDir string) (*agentcore.App, string, error) {
	if c == nil {
		return nil, "", fmt.Errorf("missing config")
	}

	slog.Info("loading agent config", "config_file", cfgFile, "config_dir", configDir)

	// NNG local service address (for SDK→Agent communication)
	// Default address if not specified
	nngAddrStr := ":19091"
	if nngAddrStr == "" {
		nngAddrStr = ":19091" // Default NNG Agent port
	}
	// Remove leading colon if present for display
	nngDisplayAddr := nngAddrStr
	if strings.HasPrefix(nngAddrStr, ":") {
		nngDisplayAddr = "0.0.0.0" + nngAddrStr
	}
	nngAddr := nngDisplayAddr

	agentID := resolveAgentID(strings.TrimSpace(c.Agent.ID))

	rpcAddr := nngAddr

	// 收集系统标签
	labels := collectSystemLabels()
	// Merge with config labels
	for k, v := range c.Agent.Labels {
		labels[k] = v
	}

	// 使用 NewWithConfigDir 以确保 providers.yaml 能从正确的目录加载
	core := agentcore.NewWithConfigDir(strings.TrimSpace(c.Server.Addr), agentID, configDir)
	core.SetNNGAddr(nngAddr)
	core.WithUpstreamMetadata(agentcore.UpstreamMetadata{
		GameID:            strings.TrimSpace(c.Agent.GameID),
		Env:               strings.TrimSpace(c.Agent.Env),
		Version:           Version,
		RPCAddr:           rpcAddr,
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

	// Start the agent (which now starts NNG server internally)
	go func() {
		if err := core.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("agent run failed", "error", err)
		}
	}()

	return core, nngAddr, nil
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
