// Package config devops-agent 配置加载（与 capture/config 同构：极简 YAML，
// gopkg.in/yaml.v3 直读，不经 viper；agents/* 不 import internal/ 的配置栈）。
// 命名遵循仓内配置契约（lowerCamelCase）；`devops.enabled` 缺省关（会话面
// 缺省关，注册上线不受影响，设计 agent-core §7.4）。
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cuihairu/croupier/core/agentscope"

	"gopkg.in/yaml.v3"
)

// OutboundTLS 上行控制通道 TLS（与 capture OutboundTLS 同形——各 agent 配置
// 自包含，不跨 agent import）。
type OutboundTLS struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	CertFile           string `yaml:"certFile" json:"certFile"`
	KeyFile            string `yaml:"keyFile" json:"keyFile"`
	CAFile             string `yaml:"caFile" json:"caFile"`
	ServerName         string `yaml:"serverName" json:"serverName"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify" json:"insecureSkipVerify"`
}

// Agent 进程身份与上游地址。
type Agent struct {
	// ID 注册用 agent 唯一 id（必填）。
	ID string `yaml:"id" json:"id"`
	// ServerAddr 上游控制面地址 host:port（必填，tcp:// 前缀可省）。
	ServerAddr string `yaml:"serverAddr" json:"serverAddr"`
	// GameID 游戏域标签（必填，注册携带，面板按此过滤）。
	GameID string `yaml:"gameId" json:"gameId"`
	// Env 逻辑环境标签（必填）。
	Env string `yaml:"env" json:"env"`
	// HeartbeatFile supervisable 心跳打点文件路径（监管方采样 mtime）。
	HeartbeatFile string `yaml:"heartbeatFile" json:"heartbeatFile"`
}

// AllowHost 白名单条目：主机 + 显式端口/协议闭集（§7.6「仅限白名单主机/端口」）。
// 端口与协议都必须显式列出——空列表按配置错误拒起，不做隐式通配（堡垒机
// 白名单的默认开放面越小越好）。host/ports/protocols 的 JSON 形态与 YAML
// 一致（json tag 同名），是 configsync 下发（config_versions devops.allowlist）
// 与面板发布的统一载荷。
type AllowHost struct {
	// Host 精确主机名或 IP（大小写不敏感；不做 CIDR/通配，另批评估）。
	Host string `yaml:"host" json:"host"`
	// Ports 允许连接的端口（显式闭集）。
	Ports []int `yaml:"ports" json:"ports"`
	// Protocols 允许的协议（ssh/rdp/vnc 闭集）。
	Protocols []string `yaml:"protocols" json:"protocols"`
}

// SSH SSH 通道凭据与主机指纹策略（devops.enabled=true 必填完整——凭据缺失
// 或主机指纹策略未声明宁可不起会话面，不静默降级为无认证/无校验通道）。
type SSH struct {
	// User SSH 登录用户（必填；全部白名单目标共用一个身份，目标级差异
	// 由后续批次按 host 细化）。
	User string `yaml:"user" json:"user"`
	// PrivateKeyFile 私钥 PEM 文件路径（与 passwordFile 至少配一）。
	PrivateKeyFile string `yaml:"privateKeyFile" json:"privateKeyFile"`
	// PasswordFile 口令文件路径（首行为口令；与 privateKeyFile 至少配一；
	// 口令不进 YAML、不进日志，与 dsnEnv 同款文件化纪律）。
	PasswordFile string `yaml:"passwordFile" json:"passwordFile"`
	// KnownHostsFile OpenSSH known_hosts 文件（host key 固定信任锚；
	// 与 insecureHostKey 二选一必须显式声明其一，缺省两者皆空 = 拒起）。
	KnownHostsFile string `yaml:"knownHostsFile" json:"knownHostsFile"`
	// InsecureHostKey 跳过 host key 校验（TOFU 之外的测试逃生门；生产
	// 必须 false，开启时启动日志显式标注）。
	InsecureHostKey bool `yaml:"insecureHostKey" json:"insecureHostKey"`
	// ConnectTimeoutMs TCP+握手超时毫秒（0 = 缺省 10s）。
	ConnectTimeoutMs int `yaml:"connectTimeoutMs" json:"connectTimeoutMs"`
}

// Devops 会话面配置。
type Devops struct {
	// Enabled 会话面总开关，缺省 false（注册上线不受影响，面板可见）。
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Allowlist 可接入主机白名单（enabled=true 必填非空；空名单 = 全拒绝，
	// 属误配，宁可不起进程）。configsync 下发（devops.allowlist）生效后
	// 整体覆盖本块，无中心/未发布时文件配置兜底。
	Allowlist []AllowHost `yaml:"allowlist" json:"allowlist"`
	// SessionDir 会话存档目录（会话起止元数据 JSONL + asciicast 内容回放）。
	SessionDir string `yaml:"sessionDir" json:"sessionDir"`
	// ExeclogDir 审计文件目录（execlog 本地真值，audit-first 前提）。
	ExeclogDir string `yaml:"execlogDir" json:"execlogDir"`
	// SSH SSH 通道凭据与主机指纹策略（enabled=true 必填完整）。
	SSH SSH `yaml:"ssh" json:"ssh"`
	// GuacdAddr guacd（Guacamole 协议网关）地址——预留字段，RDP/VNC 通道
	// 另批（agent-core §7.5）。
	GuacdAddr string `yaml:"guacdAddr" json:"guacdAddr"`
}

// Config devops-agent 全量配置。
type Config struct {
	Agent       Agent       `yaml:"agent" json:"agent"`
	OutboundTLS OutboundTLS `yaml:"outboundTLS" json:"outboundTLS"`
	Devops      Devops      `yaml:"devops" json:"devops"`
}

// Load 读入并校验 YAML 配置。
func Load(path string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Validate 必填项与取值校验（缺 id/serverAddr/gameId/env 直接报错，宁可不起进程）。
func (c Config) Validate() error {
	// 注册身份四字段走公共校验规则（core/agentscope：去空白即拒、声明顺序报错）。
	if err := agentscope.Identity(c.Agent.ID, c.Agent.ServerAddr, c.Agent.GameID, c.Agent.Env); err != nil {
		return err
	}
	if c.Devops.Enabled {
		if err := ValidateAllowlist(c.Devops.Allowlist); err != nil {
			return err
		}
		if err := c.Devops.SSH.validate(); err != nil {
			return err
		}
	}
	return nil
}

// ValidateAllowlist 白名单条目全量校验（文件配置与 configsync 下发 apply、
// 面板发布预检共用同一校验真值——任一条非法即整体拒绝，不半换）。
func ValidateAllowlist(entries []AllowHost) error {
	if len(entries) == 0 {
		return fmt.Errorf("devops.allowlist must not be empty")
	}
	for i, h := range entries {
		if h.Host == "" {
			return fmt.Errorf("devops.allowlist[%d].host is required", i)
		}
		if len(h.Ports) == 0 {
			return fmt.Errorf("devops.allowlist[%d].ports must not be empty", i)
		}
		for _, p := range h.Ports {
			if p <= 0 || p > 65535 {
				return fmt.Errorf("devops.allowlist[%d].ports: invalid port %d", i, p)
			}
		}
		if len(h.Protocols) == 0 {
			return fmt.Errorf("devops.allowlist[%d].protocols must not be empty", i)
		}
		for _, proto := range h.Protocols {
			if proto != "ssh" && proto != "rdp" && proto != "vnc" {
				return fmt.Errorf("devops.allowlist[%d].protocols: unknown protocol %q", i, proto)
			}
		}
	}
	return nil
}

// ParseAllowlistJSON 解析 configsync 下发的白名单载荷（面板发布与 agent
// apply 同一解析真值；未知字段拒绝，防手滑键名静默失效）。
func ParseAllowlistJSON(data []byte) ([]AllowHost, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var entries []AllowHost
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("parse allowlist json: %w", err)
	}
	return entries, nil
}

// validate SSH 通道凭据与主机指纹策略校验（audit-first 纪律的凭据面：
// 缺用户/缺凭据/缺指纹策略都拒起，不静默降级）。
func (s SSH) validate() error {
	if strings.TrimSpace(s.User) == "" {
		return fmt.Errorf("devops.ssh.user is required when devops.enabled is true")
	}
	if s.PrivateKeyFile == "" && s.PasswordFile == "" {
		return fmt.Errorf("devops.ssh requires privateKeyFile or passwordFile")
	}
	if !s.InsecureHostKey && s.KnownHostsFile == "" {
		return fmt.Errorf("devops.ssh requires knownHostsFile or explicit insecureHostKey (fail-closed: no host key policy = refuse)")
	}
	return nil
}

// SessionDir 会话存档目录（缺省 data/devops/sessions）。
func (c Config) SessionDir() string {
	if c.Devops.SessionDir != "" {
		return c.Devops.SessionDir
	}
	return "data/devops/sessions"
}

// ExeclogDir 审计文件目录（缺省 data/devops/execlog）。
func (c Config) ExeclogDir() string {
	if c.Devops.ExeclogDir != "" {
		return c.Devops.ExeclogDir
	}
	return "data/devops/execlog"
}
