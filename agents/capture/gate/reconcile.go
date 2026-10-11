// Package gate capture-agent 闸层（C2）：统一变更事件流上的规则评估层，
// 与源解耦——所有闸只认 source.ChangeEvent（设计 §4）。规则模板是代码，
// 规则本体是纯数据（设计 §5 择最简：无表达式语言、无 DSL、无脚本注入面）。
//
// 并发契约：单 goroutine 消费（runner 事件循环内 Evaluate、位点 tick 内
// Sweep 同循环串行），闸内状态不加锁。
//
// 命中（Hit）落 agent 日志+计数；告警上报链（outlet kind capture.rule_hit
// 闭集已备）随闸层收口批接线。
package gate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/capture/source"
)

// 规则 ID 闭集（设计 §4.4）。
const (
	// RuleReconcileMissingLedger 对账闸：道具变更超窗未对到业务流水。
	RuleReconcileMissingLedger = "reconcile.missing_ledger"
)

// 严重度闭集（设计 §4.4）。
const (
	SeverityWarn     = "warn"
	SeverityCritical = "critical"
)

const (
	defaultWindowSec   = 300           // 设计 §4.1 默认 ±5 分钟
	defaultMaxPending  = 10000         // 单规则在窗条目上限（环形窗口容量）
	missingKeySentinel = "\x00missing" // 道具侧取不到键的 never-match 哨兵
)

// ReconcileRule 对账规则（设计 §4.1，纯数据）：道具表变更必须能在时间窗内
// 对上业务流水（订单/邮件附件/GM 补偿单），无流水 = 异常。
//
// 键取值形态（C1 行像为位置数组 JSON、无列名映射）：整数串（如 "3"）= 按
// 位置索引取值；字段名形态前向兼容列名映射/对象行像，在数组像上取不到值
// ——道具侧取不到键按 never-match 哨兵入窗（超窗告警，宁误报不漏报），
// 流水侧取不到键跳过并计数（SkippedNoKey）。
type ReconcileRule struct {
	Name        string   `yaml:"name" json:"name"`
	ItemTable   string   `yaml:"itemTable" json:"itemTable"`     // "库.表"
	LedgerTable string   `yaml:"ledgerTable" json:"ledgerTable"` // "库.表"
	ItemKey     string   `yaml:"itemKey" json:"itemKey"`         // 道具行像键取值规格
	LedgerKey   string   `yaml:"ledgerKey" json:"ledgerKey"`     // 流水行像键取值规格
	WindowSec   int      `yaml:"windowSec" json:"windowSec"`     // 0 → defaultWindowSec
	Severity    string   `yaml:"severity" json:"severity"`       // "" → warn
	ExemptKeys  []string `yaml:"exemptKeys" json:"exemptKeys"`   // 关联键值精确豁免（批处理流水等）
	MaxPending  int      `yaml:"maxPending" json:"maxPending"`   // 单规则在窗条目上限，0 → defaultMaxPending
}

// Validate 规则静态校验（config.Load 启动即拦，宁可不起进程）。
func (r ReconcileRule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("reconcile rule name is required")
	}
	if !isQualifiedTable(r.ItemTable) {
		return fmt.Errorf("reconcile rule %s: itemTable must be \"db.table\", got %q", r.Name, r.ItemTable)
	}
	if !isQualifiedTable(r.LedgerTable) {
		return fmt.Errorf("reconcile rule %s: ledgerTable must be \"db.table\", got %q", r.Name, r.LedgerTable)
	}
	if r.ItemTable == r.LedgerTable {
		return fmt.Errorf("reconcile rule %s: itemTable and ledgerTable must differ", r.Name)
	}
	if r.ItemKey == "" || r.LedgerKey == "" {
		return fmt.Errorf("reconcile rule %s: itemKey/ledgerKey are required", r.Name)
	}
	if r.WindowSec < 0 {
		return fmt.Errorf("reconcile rule %s: windowSec must be >= 0", r.Name)
	}
	switch r.Severity {
	case "", SeverityWarn, SeverityCritical:
	default:
		return fmt.Errorf("reconcile rule %s: severity must be warn|critical, got %q", r.Name, r.Severity)
	}
	if r.MaxPending < 0 {
		return fmt.Errorf("reconcile rule %s: maxPending must be >= 0", r.Name)
	}
	return nil
}

// Window 规则时间窗（秒→时长；非法值回落默认）。
func (r ReconcileRule) Window() time.Duration {
	if r.WindowSec > 0 {
		return time.Duration(r.WindowSec) * time.Second
	}
	return defaultWindowSec * time.Second
}

// cap 在窗条目上限（非法值回落默认）。
func (r ReconcileRule) cap() int {
	if r.MaxPending > 0 {
		return r.MaxPending
	}
	return defaultMaxPending
}

// ValidateReconcileRules 全量规则校验：逐条静态校验 + 表路由唯一（同表不得
// 出现在两条规则——角色路由必须无歧义）。
func ValidateReconcileRules(rules []ReconcileRule) error {
	seen := map[string]string{} // "库.表" → 规则名
	for i, r := range rules {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("reconcile rules[%d]: %w", i, err)
		}
		for _, t := range []string{r.ItemTable, r.LedgerTable} {
			if prev, dup := seen[t]; dup {
				return fmt.Errorf("reconcile rule %s: table %s already used by rule %s", r.Name, t, prev)
			}
			seen[t] = r.Name
		}
	}
	return nil
}

// Hit 规则命中（设计 §4.4 命中详情的进程内形态；告警上报时映射
// capture.rule_hit 载荷）。
type Hit struct {
	Rule     string             // 规则 ID（reconcile.missing_ledger）
	Severity string             // warn | critical
	Table    string             // "库.表"
	Key      string             // 关联键值（"" = 道具侧键缺失哨兵）
	Detail   string             // 人读细节（窗口/驱逐原因）
	Event    source.ChangeEvent // 触发事件快照
}

// entryKind 在窗条目角色。
type entryKind uint8

const (
	kindDebit  entryKind = iota // 道具侧待销（超窗未销 = 告警）
	kindCredit                  // 流水侧挂窗（超窗静默清：流水无道具变更非异常）
)

type entry struct {
	ruleIdx int
	key     string
	tsMs    int64 // 入窗时刻 = max(事件 ts, now)：binlog 回放补扫不即刻超窗
	kind    entryKind
	ev      source.ChangeEvent
}

// pairQueue 同 (规则, 键) 的双向待配对队列（设计 §4.1：窗口内双向匹配销账，
// 道具先动/流水先动两种时序都成立）。
type pairQueue struct {
	debits  []entry
	credits []entry
}

type pairKey struct {
	ruleIdx int
	key     string
}

// ReconcileGate 对账闸：道具变更 ↔ 业务流水时间窗双向销账；超窗未销道具
// 变更 = reconcile.missing_ledger 告警。环形窗口 + map，不引对账引擎。
type ReconcileGate struct {
	rules        []ReconcileRule
	itemIdx      map[string]int
	ledgerIdx    map[string]int
	exempt       map[int]map[string]bool
	queues       map[pairKey]*pairQueue
	order        [][]pairKey  // 逐规则入窗 FIFO（容量驱逐用；空队列惰性清理）
	pending      []int        // 逐规则在窗条目数
	now          func() int64 // 毫秒（测试注入）
	skippedNoKey int64        // 流水侧取不到键计数（单 goroutine 契约）
}

// NewReconcile 构造对账闸（规则先过 ValidateReconcileRules；此处防御性
// 复验，出错由调用方决定降级形态）。
func NewReconcile(rules []ReconcileRule) (*ReconcileGate, error) {
	if err := ValidateReconcileRules(rules); err != nil {
		return nil, err
	}
	g := &ReconcileGate{
		rules:     rules,
		itemIdx:   map[string]int{},
		ledgerIdx: map[string]int{},
		exempt:    map[int]map[string]bool{},
		queues:    map[pairKey]*pairQueue{},
		order:     make([][]pairKey, len(rules)),
		pending:   make([]int, len(rules)),
		now:       func() int64 { return time.Now().UnixMilli() },
	}
	for i, r := range rules {
		g.itemIdx[r.ItemTable] = i
		g.ledgerIdx[r.LedgerTable] = i
		if len(r.ExemptKeys) > 0 {
			m := make(map[string]bool, len(r.ExemptKeys))
			for _, k := range r.ExemptKeys {
				m[k] = true
			}
			g.exempt[i] = m
		}
	}
	return g, nil
}

// SetNow 注入时钟（测试用）。
func (g *ReconcileGate) SetNow(now func() int64) { g.now = now }

// SkippedNoKey 流水侧取不到键的累计计数（键形态与行像不匹配的配置错误信号）。
func (g *ReconcileGate) SkippedNoKey() int64 { return g.skippedNoKey }

// PendingCount 当前在窗条目数（全部规则 debits + credits）。
func (g *ReconcileGate) PendingCount() int {
	n := 0
	for _, p := range g.pending {
		n += p
	}
	return n
}

// Evaluate 消费一个变更事件：先销到期，再按角色入窗/销账。非订阅表零开销
// 直通。
func (g *ReconcileGate) Evaluate(ev *source.ChangeEvent) []Hit {
	table := ev.Database + "." + ev.Table
	nowMs := g.now()
	if idx, ok := g.itemIdx[table]; ok {
		hits := g.sweep(nowMs)
		hits = append(hits, g.onItem(idx, ev, nowMs)...)
		return hits
	}
	if idx, ok := g.ledgerIdx[table]; ok {
		hits := g.sweep(nowMs)
		g.onLedger(idx, ev, nowMs)
		return hits
	}
	return nil
}

// Sweep 周期销账（runner 位点 tick 同频调用）：超窗未销道具变更产出命中；
// 超窗 credit 静默清。
func (g *ReconcileGate) Sweep(nowMs int64) []Hit { return g.sweep(nowMs) }

// sweep 全窗到期销账 + 队列/序表惰性回收。map 遍历序不定，命中出函数前
// 统一排序保证确定性。
func (g *ReconcileGate) sweep(nowMs int64) []Hit {
	var hits []Hit
	for pk, q := range g.queues {
		windowMs := g.rules[pk.ruleIdx].Window().Milliseconds()
		for len(q.debits) > 0 && nowMs-q.debits[0].tsMs >= windowMs {
			hits = append(hits, g.hitFor(q.debits[0], noLedgerDetail(g.rules[pk.ruleIdx])))
			q.debits = q.debits[1:]
			g.pending[pk.ruleIdx]--
		}
		for len(q.credits) > 0 && nowMs-q.credits[0].tsMs >= windowMs {
			q.credits = q.credits[1:]
			g.pending[pk.ruleIdx]--
		}
		if len(q.debits) == 0 && len(q.credits) == 0 {
			delete(g.queues, pk)
		}
	}
	for ri := range g.order {
		kept := g.order[ri][:0]
		for _, pk := range g.order[ri] {
			if q, ok := g.queues[pk]; ok {
				kept = append(kept, pk)
				_ = q
			}
		}
		g.order[ri] = kept
	}
	sortHitOrder(hits)
	return hits
}

// onItem 道具侧：豁免直通；同键已挂窗 credit 先销（流水先到的双向时序，
// 否则 debit 会在 credit 之上超窗误报）；取不到键按哨兵入窗（永不匹配，
// 超窗即告警——NULL trace id 本身就是可疑形态）；容量驱逐命中一并返回。
func (g *ReconcileGate) onItem(idx int, ev *source.ChangeEvent, nowMs int64) []Hit {
	rule := g.rules[idx]
	key, keyOK := extractKey(rowImage(ev), rule.ItemKey)
	if keyOK && g.exempt[idx][key] {
		return nil
	}
	if !keyOK {
		key = missingKeySentinel
	}
	pk := pairKey{ruleIdx: idx, key: key}
	if q := g.queues[pk]; q != nil && len(q.credits) > 0 {
		q.credits = q.credits[1:]
		g.pending[idx]--
		if len(q.debits) == 0 && len(q.credits) == 0 {
			delete(g.queues, pk)
		}
		return nil
	}
	return g.enqueue(entry{ruleIdx: idx, key: key, tsMs: maxInt64(ev.TsUnixMs, nowMs), kind: kindDebit, ev: *ev})
}

// onLedger 流水侧：优先销最早同键 debit（双向窗口），无待销则挂窗 credit
// （流水先于道具变更到达的时序）。取不到键跳过并计数（无法定位销账对象，
// 通常是 itemKey/ledgerKey 规格配错）。
func (g *ReconcileGate) onLedger(idx int, ev *source.ChangeEvent, nowMs int64) {
	key, ok := extractKey(rowImage(ev), g.rules[idx].LedgerKey)
	if !ok {
		g.skippedNoKey++
		return
	}
	pk := pairKey{ruleIdx: idx, key: key}
	if q := g.queues[pk]; q != nil && len(q.debits) > 0 {
		q.debits = q.debits[1:]
		g.pending[idx]--
		if len(q.debits) == 0 && len(q.credits) == 0 {
			delete(g.queues, pk)
		}
		return
	}
	g.enqueue(entry{ruleIdx: idx, key: key, tsMs: maxInt64(ev.TsUnixMs, nowMs), kind: kindCredit, ev: *ev})
}

// enqueue 入窗 + 逐规则容量驱逐：超 cap 时按入窗 FIFO 驱逐该规则最老条目
// ——debit 驱逐产出命中（宁误报不漏报），credit 静默清。
func (g *ReconcileGate) enqueue(e entry) []Hit {
	pk := pairKey{ruleIdx: e.ruleIdx, key: e.key}
	q := g.queues[pk]
	if q == nil {
		q = &pairQueue{}
		g.queues[pk] = q
		g.order[e.ruleIdx] = append(g.order[e.ruleIdx], pk)
	}
	if e.kind == kindDebit {
		q.debits = append(q.debits, e)
	} else {
		q.credits = append(q.credits, e)
	}
	g.pending[e.ruleIdx]++

	var hits []Hit
	cap := g.rules[e.ruleIdx].cap()
	for g.pending[e.ruleIdx] > cap {
		front, fq := g.liveFront(e.ruleIdx)
		if fq == nil {
			break // 防御：计数与队列失配时不空转
		}
		// 驱逐该队列里更老的一侧（ts 相同优先 debit：先告警后静默）
		if len(fq.debits) > 0 && (len(fq.credits) == 0 || fq.debits[0].tsMs <= fq.credits[0].tsMs) {
			hits = append(hits, g.hitFor(fq.debits[0], evictDetail(g.rules[e.ruleIdx])))
			fq.debits = fq.debits[1:]
		} else {
			fq.credits = fq.credits[1:]
		}
		g.pending[e.ruleIdx]--
		if len(fq.debits) == 0 && len(fq.credits) == 0 {
			delete(g.queues, front)
		}
	}
	return hits
}

// liveFront 规则序表队首的存活队列（惰性跳过已清空键）。
func (g *ReconcileGate) liveFront(ruleIdx int) (pairKey, *pairQueue) {
	ord := g.order[ruleIdx]
	for len(ord) > 0 {
		pk := ord[0]
		if q, ok := g.queues[pk]; ok && len(q.debits)+len(q.credits) > 0 {
			g.order[ruleIdx] = ord
			return pk, q
		}
		ord = ord[1:]
	}
	g.order[ruleIdx] = ord[:0]
	return pairKey{}, nil
}

func (g *ReconcileGate) hitFor(e entry, detail string) Hit {
	rule := g.rules[e.ruleIdx]
	sev := rule.Severity
	if sev == "" {
		sev = SeverityWarn
	}
	key := e.key
	if key == missingKeySentinel {
		key = "" // 键缺失告警：空键即信号本体
	}
	return Hit{
		Rule:     RuleReconcileMissingLedger,
		Severity: sev,
		Table:    rule.ItemTable,
		Key:      key,
		Detail:   detail,
		Event:    e.ev,
	}
}

func noLedgerDetail(rule ReconcileRule) string {
	return fmt.Sprintf("no ledger match within %ds window", int64(rule.Window()/time.Second))
}

func evictDetail(rule ReconcileRule) string {
	return fmt.Sprintf("pending cap %d exceeded, evicted oldest unmatched item", rule.cap())
}

// sortHitOrder 命中确定性排序：事件 ts → 表 → 键 → GTID。
func sortHitOrder(hits []Hit) {
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.Event.TsUnixMs != b.Event.TsUnixMs {
			return a.Event.TsUnixMs < b.Event.TsUnixMs
		}
		if a.Table != b.Table {
			return a.Table < b.Table
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		return a.Event.GTID < b.Event.GTID
	})
}

// rowImage 键提取行像：insert/update 取后像，delete 取前像（C1 UPDATE 为
// 前像+后像成对的双行像）。
func rowImage(ev *source.ChangeEvent) json.RawMessage {
	if ev.Op == source.OpDelete {
		return ev.BeforeJSON
	}
	return ev.AfterJSON
}

// extractKey 行像键取值：整数串规格 = 位置索引取值（C1 数组行像）；其余 =
// 对象属性名（前向兼容列名映射）。取不到返回 ("", false)。
func extractKey(img json.RawMessage, spec string) (string, bool) {
	if len(img) == 0 || spec == "" {
		return "", false
	}
	if idx, err := strconv.Atoi(spec); err == nil {
		var row []json.RawMessage
		if json.Unmarshal(img, &row) != nil || idx < 0 || idx >= len(row) {
			return "", false
		}
		return keyValue(row[idx])
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(img, &obj) != nil {
		return "", false
	}
	raw, ok := obj[spec]
	if !ok {
		return "", false
	}
	return keyValue(raw)
}

// keyValue 键值字符串化（两侧规则各自提取后必须可比）：字符串原样；数值走
// json.Number 原文（避免浮点重排）；null 不可作键；对象/数组取紧凑 JSON。
func keyValue(raw json.RawMessage) (string, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return "", false
	}
	switch t := v.(type) {
	case nil:
		return "", false
	case string:
		return t, true
	case json.Number:
		return t.String(), true
	case bool:
		if t {
			return "true", true
		}
		return "false", true
	default:
		var buf bytes.Buffer
		if json.Compact(&buf, raw) != nil {
			return "", false
		}
		return buf.String(), true
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// isQualifiedTable "库.表" 形态校验（与 ChangeEvent.Database+"."+Table
// 路由键同构：两段非空）。
func isQualifiedTable(t string) bool {
	db, table, ok := strings.Cut(t, ".")
	return ok && db != "" && table != "" && !strings.Contains(table, ".")
}
