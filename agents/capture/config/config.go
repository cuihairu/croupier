// Package config capture-agent 配置加载（极简 YAML，gopkg.in/yaml.v3 直读，
// 不经 viper：agents/* 不 import internal/ 的配置栈）。命名遵循仓内配置契约
// （lowerCamelCase）；`capture.enabled` 缺省关（设计 §2，C1 口径）。
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/cuihairu/croupier/agents/capture/gate"
	"github.com/cuihairu/croupier/agents/capture/source"
	"gopkg.in/yaml.v3"
)

// OutboundTLS 上行控制通道 TLS（对齐 configs/agent.yaml outboundTLS 块命名）。
// enabled=false 时裸连（insecure）；certFile+keyFile 提供客户端证书（mTLS）。
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
	// Env 逻辑环境标签（prod/stage/dev，可空）。
	Env string `yaml:"env" json:"env"`
	// HeartbeatFile supervisable 心跳打点文件路径（监管方采样 mtime）。
	HeartbeatFile string `yaml:"heartbeatFile" json:"heartbeatFile"`
}

// Gates C2 闸层规则集（纯数据；模板是代码，见设计 §4/§5）。
type Gates struct {
	// Reconcile 对账闸规则（道具变更 ↔ 业务流水时间窗销账）。
	Reconcile struct {
		Rules []gate.ReconcileRule `yaml:"rules" json:"rules"`
	} `yaml:"reconcile" json:"reconcile"`
	// CEP 闸规则三模板（时段白名单 / 数量突变 / 连续试探）。
	CEP struct {
		OffHours []gate.OffHoursRule `yaml:"offHours" json:"offHours"`
		Burst    []gate.BurstRule    `yaml:"burst" json:"burst"`
		Probing  []gate.ProbeRule    `yaml:"probing" json:"probing"`
	} `yaml:"cep" json:"cep"`
	// Lineage 血缘闸规则（账号白名单；C1 account 恒空 → 降级态）。
	Lineage struct {
		Rules []gate.LineageRule `yaml:"rules" json:"rules"`
	} `yaml:"lineage" json:"lineage"`
}

// Capture CDC 功能块。
type Capture struct {
	// Enabled 总开关，缺省 false（设计 §2：缺省关）。
	Enabled bool `yaml:"enabled" json:"enabled"`
	// BookmarkDir 位点文件目录（每源一文件：mysql.json）。
	BookmarkDir string `yaml:"bookmarkDir" json:"bookmarkDir"`
	// BookmarkFlushIntervalSec 位点周期落盘间隔（秒，默认 5）。
	BookmarkFlushIntervalSec int `yaml:"bookmarkFlushIntervalSec" json:"bookmarkFlushIntervalSec"`
	// MySQL 源连接配置；nil = 未配置（enabled=true 而 mysql 缺失报错）。
	MySQL *source.MySQLConfig `yaml:"mysql" json:"mysql"`
	// Gates 闸层规则（空 = 只采不限，C1 口径）。
	Gates Gates `yaml:"gates" json:"gates"`
}

// Config capture-agent 全量配置。
type Config struct {
	Agent       Agent       `yaml:"agent" json:"agent"`
	OutboundTLS OutboundTLS `yaml:"outboundTLS" json:"outboundTLS"`
	Capture     Capture     `yaml:"capture" json:"capture"`
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

// Validate 必填项与取值校验（缺 id/serverAddr/gameId 直接报错，宁可不起进程）。
func (c Config) Validate() error {
	if c.Agent.ID == "" {
		return fmt.Errorf("agent.id is required")
	}
	if c.Agent.ServerAddr == "" {
		return fmt.Errorf("agent.serverAddr is required")
	}
	if c.Agent.GameID == "" {
		return fmt.Errorf("agent.gameId is required")
	}
	if c.Capture.Enabled {
		if c.Capture.MySQL == nil {
			return fmt.Errorf("capture.mysql is required when capture.enabled is true")
		}
		if len(c.Capture.MySQL.Tables) == 0 {
			return fmt.Errorf("capture.mysql.tables must not be empty (whitelist required)")
		}
	}
	// 闸规则启动即全量校验（宁可不起进程）——含对账闸表路由唯一性。
	if err := gate.ValidateReconcileRules(c.Capture.Gates.Reconcile.Rules); err != nil {
		return fmt.Errorf("capture.gates: %w", err)
	}
	if err := gate.ValidateCEPRules(c.Capture.Gates.CEP.OffHours, c.Capture.Gates.CEP.Burst, c.Capture.Gates.CEP.Probing); err != nil {
		return fmt.Errorf("capture.gates: %w", err)
	}
	if err := gate.ValidateLineageRules(c.Capture.Gates.Lineage.Rules); err != nil {
		return fmt.Errorf("capture.gates: %w", err)
	}
	return nil
}

// BookmarkPath 位点文件路径（<bookmarkDir>/mysql.json，C1 单 MySQL 源）。
func (c Config) BookmarkPath() string {
	dir := c.Capture.BookmarkDir
	if dir == "" {
		dir = "data/capture/bookmarks"
	}
	return dir + "/mysql.json"
}

// FlushInterval 位点落盘间隔（秒→Duration，非法值回落 5s）。
func (c Config) FlushInterval() time.Duration {
	if c.Capture.BookmarkFlushIntervalSec > 0 {
		return time.Duration(c.Capture.BookmarkFlushIntervalSec) * time.Second
	}
	return 5 * time.Second
}
