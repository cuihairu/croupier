// Package config capture-agent 配置加载（极简 YAML，gopkg.in/yaml.v3 直读，
// 不经 viper：agents/* 不 import internal/ 的配置栈）。命名遵循仓内配置契约
// （lowerCamelCase）；`capture.enabled` 缺省关（设计 §2，C1 口径）。
package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
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
// Gates 闸层规则集（文件面 = fallback：中心 config_versions 键
// capture.gates 已发布版本拉取生效后整体覆盖本块；无中心/未发布时兜底，
// 形状与面板下发 JSON 一致）。
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
	// Event 事件粗过滤（EventFilterPolicy v1，操作类型白名单；缺省空 =
	// 全放行，与既有部署行为一致）。库表范围归 SourceFilter（授权语义），
	// 不在此配置——见 docs/design/cdc-filter-policy-design.md §1.2。
	Event struct {
		Ops []string `yaml:"ops" json:"ops"`
	} `yaml:"event" json:"event"`
}

// Capture CDC 功能块。
type Capture struct {
	// Enabled 总开关，缺省 false（设计 §2：缺省关）。
	Enabled bool `yaml:"enabled" json:"enabled"`
	// BookmarkDir 位点文件目录（每源一文件：mysql.json）。
	BookmarkDir string `yaml:"bookmarkDir" json:"bookmarkDir"`
	// BookmarkFlushIntervalSec 位点周期落盘间隔（秒，默认 5）。
	BookmarkFlushIntervalSec int `yaml:"bookmarkFlushIntervalSec" json:"bookmarkFlushIntervalSec"`
	// SourceType 数据源类型（"mysql" | "postgres"；闭集随 Connector 实现
	// 扩展）。与 DSNEnv 同用：enabled=true 时两者齐备即走 Connector 契约面。
	SourceType string `yaml:"sourceType" json:"sourceType"`
	// DSNEnv 连接串环境变量名（凭据红线：明文 DSN 只从环境变量取值，不进
	// 配置文件、不落日志）。
	DSNEnv string `yaml:"dsnEnv" json:"dsnEnv"`
	// Options 非密连接参数（serverId/flavor/tables/slotName/publication…，
	// lowerCamelCase）。tables 必填（逗号分隔 "库.表" 白名单）。
	Options map[string]string `yaml:"options" json:"options"`
	// MySQL legacy 源连接配置（host/port/user/password）。仅在兼容解析代码
	// 容忍，不再写入任何示例/文档：凭据留源实现内部，不进契约。
	MySQL *source.MySQLConfig `yaml:"mysql" json:"mysql"`
	// Gates 闸层规则（空 = 只采不限，C1 口径）。
	Gates Gates `yaml:"gates" json:"gates"`
}

// SourceTypes Connector 数据源类型闭集（校验真值；新增=新 Connector 落地）。
var SourceTypes = map[string]bool{"mysql": true, "postgres": true, "mongodb": true}

// Tables 采集白名单 "库.表" 列表（canonical 取 options.tables 逗号分隔；
// legacy 取 capture.mysql.tables；均排序输出）。enabled=true 时必填非空。
func (c Capture) Tables() []string {
	if c.MySQL != nil {
		out := make([]string, 0, len(c.MySQL.Tables))
		for t := range c.MySQL.Tables {
			out = append(out, t)
		}
		sort.Strings(out)
		return out
	}
	var out []string
	for _, t := range strings.Split(c.Options["tables"], ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

// SourceConfig 契约面数据源配置（connector.Open 入参）。canonical 路径携带
// DSNEnv 与 Options；legacy 路径凭据不经契约（MySQLConfig 直连），仅带身份与
// 非密参数。
func (c Capture) SourceConfig(sourceID string) connector.SourceConfig {
	if c.MySQL != nil {
		return connector.SourceConfig{SourceID: sourceID, SourceType: "mysql"}
	}
	return connector.SourceConfig{
		SourceID:   sourceID,
		SourceType: c.SourceType,
		DSNEnv:     c.DSNEnv,
		Options:    c.Options,
	}
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
		switch {
		case c.Capture.MySQL != nil:
			// legacy 兼容路径（凭据留 YAML，仅代码容忍；见 SourceType 字段注释）。
			if len(c.Capture.MySQL.Tables) == 0 {
				return fmt.Errorf("capture.mysql.tables must not be empty (whitelist required)")
			}
		case c.Capture.SourceType == "":
			return fmt.Errorf("capture.sourceType is required when capture.enabled is true (or legacy capture.mysql)")
		case !SourceTypes[c.Capture.SourceType]:
			return fmt.Errorf("capture.sourceType %q unknown (want mysql|postgres|mongodb)", c.Capture.SourceType)
		case c.Capture.DSNEnv == "":
			return fmt.Errorf("capture.dsnEnv is required when capture.enabled is true (credential channel)")
		case len(c.Capture.Tables()) == 0:
			return fmt.Errorf("capture.options.tables must not be empty (whitelist required)")
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

// BookmarkPath 位点文件路径（<bookmarkDir>/<sourceType>.json）。按源类型分
// 文件：legacy capture.mysql 与 canonical sourceType=mysql 同为 mysql.json
// （配置面迁移不丢续读位点），postgres 独立文件。
func (c Config) BookmarkPath() string {
	dir := c.Capture.BookmarkDir
	if dir == "" {
		dir = "data/capture/bookmarks"
	}
	t := c.Capture.SourceType
	if t == "" {
		t = "mysql" // legacy capture.mysql 路径（含 canonical 空值兜底）
	}
	return fmt.Sprintf("%s/%s.json", dir, t)
}

// FlushInterval 位点落盘间隔（秒→Duration，非法值回落 5s）。
func (c Config) FlushInterval() time.Duration {
	if c.Capture.BookmarkFlushIntervalSec > 0 {
		return time.Duration(c.Capture.BookmarkFlushIntervalSec) * time.Second
	}
	return 5 * time.Second
}
