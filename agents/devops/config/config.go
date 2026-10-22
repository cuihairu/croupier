// Package config devops-agent 配置加载（与 capture/config 同构：极简 YAML，
// gopkg.in/yaml.v3 直读，不经 viper；agents/* 不 import internal/ 的配置栈）。
// 命名遵循仓内配置契约（lowerCamelCase）；`devops.enabled` 缺省关（会话面
// 缺省关，注册上线不受影响，设计 agent-core §7.4）。
package config

import (
	"fmt"
	"os"

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
// 白名单的默认开放面越小越好）。
type AllowHost struct {
	// Host 精确主机名或 IP（大小写不敏感；不做 CIDR/通配，另批评估）。
	Host string `yaml:"host" json:"host"`
	// Ports 允许连接的端口（显式闭集）。
	Ports []int `yaml:"ports" json:"ports"`
	// Protocols 允许的协议（ssh/rdp/vnc 闭集）。
	Protocols []string `yaml:"protocols" json:"protocols"`
}

// Devops 会话面配置。
type Devops struct {
	// Enabled 会话面总开关，缺省 false（注册上线不受影响，面板可见）。
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Allowlist 可接入主机白名单（enabled=true 必填非空；空名单 = 全拒绝，
	// 属误配，宁可不起进程）。
	Allowlist []AllowHost `yaml:"allowlist" json:"allowlist"`
	// SessionDir 会话存档目录（会话起止元数据 JSONL；内容流存档另批）。
	SessionDir string `yaml:"sessionDir" json:"sessionDir"`
	// ExeclogDir 审计文件目录（execlog 本地真值，audit-first 前提）。
	ExeclogDir string `yaml:"execlogDir" json:"execlogDir"`
	// GuacdAddr guacd（Guacamole 协议网关）地址——预留字段，骨架批次不接
	//（协议通道另批；agent-core §7.5）。
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
		if len(c.Devops.Allowlist) == 0 {
			return fmt.Errorf("devops.allowlist must not be empty when devops.enabled is true")
		}
		for i, h := range c.Devops.Allowlist {
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
		}
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
