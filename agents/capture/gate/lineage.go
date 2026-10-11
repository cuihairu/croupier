package gate

import (
	"fmt"

	"github.com/cuihairu/croupier/agents/capture/source"
)

// RuleLineageWhitelistViolation 血缘闸规则 ID（设计 §4.4）。
const RuleLineageWhitelistViolation = "lineage.whitelist_violation"

// LineageRule 闸三·血缘（设计 §4.3）：非白名单账号对受监控表的写入 = 异常
// （人工直改库未走审批流）。
//
// C1 降级态（诚实清单）：binlog/WAL 事件均不携带会话账号，`account` 恒空
// （§3.3）——空账号事件**跳过并计数 Degraded**（既不误报也不放过：告警无
// 归因主体，落出来只会是噪声）；一旦源侧补上账号（general log/audit 侧通道
// 评估中），本闸零改动生效。面板「降级态」标注随面板配置面批。
type LineageRule struct {
	Name          string   `yaml:"name" json:"name"`
	Table         string   `yaml:"table" json:"table"`
	Ops           []string `yaml:"ops" json:"ops"`
	AllowAccounts []string `yaml:"allowAccounts" json:"allowAccounts"`
	Severity      string   `yaml:"severity" json:"severity"`
}

// Validate 血缘规则静态校验。
func (r LineageRule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("lineage rule name is required")
	}
	if !isQualifiedTable(r.Table) {
		return fmt.Errorf("lineage rule %s: table must be \"db.table\", got %q", r.Name, r.Table)
	}
	if err := checkOps(r.Ops); err != nil {
		return fmt.Errorf("lineage rule %s: %w", r.Name, err)
	}
	if len(r.AllowAccounts) == 0 {
		return fmt.Errorf("lineage rule %s: allowAccounts must not be empty (app account at minimum)", r.Name)
	}
	return checkSeverity(r.Severity)
}

// ValidateLineageRules 血缘规则全量校验（config.Load 启动即拦）。
func ValidateLineageRules(rules []LineageRule) error {
	for i, r := range rules {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("lineage rules[%d]: %w", i, err)
		}
	}
	return nil
}

// LineageGate 血缘闸：账号白名单逐事件判定。
type LineageGate struct {
	rules    []*LineageRule
	allow    []map[string]bool
	degraded int64 // 空账号事件跳过计数（C1 恒空 → 即总评估量；面板降级态标注）
}

// NewLineage 构造血缘闸（规则先过 ValidateLineageRules；此处防御性复验）。
func NewLineage(rules []LineageRule) (*LineageGate, error) {
	if err := ValidateLineageRules(rules); err != nil {
		return nil, err
	}
	g := &LineageGate{rules: make([]*LineageRule, len(rules)), allow: make([]map[string]bool, len(rules))}
	for i := range rules {
		g.rules[i] = &rules[i]
		m := make(map[string]bool, len(rules[i].AllowAccounts))
		for _, a := range rules[i].AllowAccounts {
			m[a] = true
		}
		g.allow[i] = m
	}
	return g, nil
}

// Degraded 空账号跳过累计计数（C1 恒空：即血缘闸总评估量）。
func (g *LineageGate) Degraded() int64 { return g.degraded }

// Evaluate 消费一个变更事件：命中该表白名单规则的违规写入。
func (g *LineageGate) Evaluate(ev *source.ChangeEvent) []Hit {
	table := ev.Database + "." + ev.Table
	var hits []Hit
	for i, r := range g.rules {
		if r.Table != table || !opAllowed(r.Ops, ev.Op) {
			continue
		}
		if ev.Account == "" {
			g.degraded++ // 源未提供账号：跳过（降级态，不误报）
			continue
		}
		if g.allow[i][ev.Account] {
			continue
		}
		detail := fmt.Sprintf("account %q writes %s outside allow list", ev.Account, ev.Op)
		hits = append(hits, hitOf(RuleLineageWhitelistViolation, r.Severity, r.Table, ev.Account, detail, *ev))
	}
	sortHitOrder(hits)
	return hits
}
