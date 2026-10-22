package gate

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
)

// CEP 规则 ID 闭集（设计 §4.4）。
const (
	RuleOffHours   = "cep.off_hours"
	RuleBurstDelta = "cep.burst_delta"
	RuleProbing    = "cep.probing"
)

const (
	burstModeSingle = "single" // 单事件 delta 超阈值
	burstModeWindow = "window" // 滑动窗口累计 |delta| 超阈值

	defaultBurstMaxEvents  = 10000 // 累计桶上限（环形窗口容量）
	defaultProbeMaxTracked = 10000 // 追踪键上限
)

// OffHoursWindow 每日允许时段（分钟偏移 0..1440；startMin > endMin 表示跨零点）。
type OffHoursWindow struct {
	StartMin int `yaml:"startMin" json:"startMin"`
	EndMin   int `yaml:"endMin" json:"endMin"`
}

// OffHoursRule 闸二·非白名单时段（设计 §4.2）：表/操作 × 时段白名单，
// 名单外时段的变更即告警。
type OffHoursRule struct {
	Name        string           `yaml:"name" json:"name"`
	Table       string           `yaml:"table" json:"table"`
	Ops         []string         `yaml:"ops" json:"ops"` // 空 = 全部变更
	Windows     []OffHoursWindow `yaml:"windows" json:"windows"`
	TzOffsetMin int              `yaml:"tzOffsetMin" json:"tzOffsetMin"` // 0 = UTC；东八区写 480
	Severity    string           `yaml:"severity" json:"severity"`
}

// BurstRule 闸二·数量突变（设计 §4.2）：单事件 delta 或滑动窗口累计 |delta|
// 超阈值。deltaField 键规格同 ReconcileRule.ItemKey 形态（整数串=位置下标）。
//
// 百分比基线（设计 §4.2 提及）v1 不做：基线需历史分布来源（analytics/面板侧），
// 事件流本身给不出基线值——随 C2 差异记录。
type BurstRule struct {
	Name       string   `yaml:"name" json:"name"`
	Table      string   `yaml:"table" json:"table"`
	Ops        []string `yaml:"ops" json:"ops"`
	DeltaField string   `yaml:"deltaField" json:"deltaField"`
	Threshold  float64  `yaml:"threshold" json:"threshold"`
	Mode       string   `yaml:"mode" json:"mode"`           // single | window
	WindowSec  int      `yaml:"windowSec" json:"windowSec"` // mode=window 必填 > 0
	MaxEvents  int      `yaml:"maxEvents" json:"maxEvents"` // 0 → defaultBurstMaxEvents
	Severity   string   `yaml:"severity" json:"severity"`
}

// ProbeRule 闸二·连续试探（设计 §4.2）：同 (键, 表) 滑动窗口内 N 次小变更
// ——低频慢刷形态。
//
// 键降级：设计原文按 (账号, 表)，C1 `account` 恒空（binlog/WAL 不带会话账号，
// §3.3），故键降级为 (行键, 表)：同一行被反复小额改动即可疑，不同行不累计。
type ProbeRule struct {
	Name       string   `yaml:"name" json:"name"`
	Table      string   `yaml:"table" json:"table"`
	Ops        []string `yaml:"ops" json:"ops"`
	KeyField   string   `yaml:"keyField" json:"keyField"`
	DeltaField string   `yaml:"deltaField" json:"deltaField"`
	SmallBelow float64  `yaml:"smallBelow" json:"smallBelow"` // 单次 |delta| < 此值算小变更
	Count      int      `yaml:"count" json:"count"`
	WindowSec  int      `yaml:"windowSec" json:"windowSec"`
	MaxTracked int      `yaml:"maxTracked" json:"maxTracked"` // 0 → defaultProbeMaxTracked
	Severity   string   `yaml:"severity" json:"severity"`
}

// Validate 时段规则静态校验。
func (r OffHoursRule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("offHours rule name is required")
	}
	if !isQualifiedTable(r.Table) {
		return fmt.Errorf("offHours rule %s: table must be \"db.table\", got %q", r.Name, r.Table)
	}
	if err := checkOps(r.Ops); err != nil {
		return fmt.Errorf("offHours rule %s: %w", r.Name, err)
	}
	if len(r.Windows) == 0 {
		return fmt.Errorf("offHours rule %s: windows must not be empty", r.Name)
	}
	for i, w := range r.Windows {
		if w.StartMin < 0 || w.StartMin >= 1440 || w.EndMin <= 0 || w.EndMin > 1440 {
			return fmt.Errorf("offHours rule %s: windows[%d] needs startMin in [0,1439] and endMin in [1,1440]", r.Name, i)
		}
	}
	if r.TzOffsetMin < -720 || r.TzOffsetMin > 840 {
		return fmt.Errorf("offHours rule %s: tzOffsetMin must be in [-720,840], got %d", r.Name, r.TzOffsetMin)
	}
	return checkSeverity(r.Severity)
}

// Validate 数量突变规则静态校验。
func (r BurstRule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("burst rule name is required")
	}
	if !isQualifiedTable(r.Table) {
		return fmt.Errorf("burst rule %s: table must be \"db.table\", got %q", r.Name, r.Table)
	}
	if err := checkOps(r.Ops); err != nil {
		return fmt.Errorf("burst rule %s: %w", r.Name, err)
	}
	if r.DeltaField == "" {
		return fmt.Errorf("burst rule %s: deltaField is required", r.Name)
	}
	if r.Threshold <= 0 {
		return fmt.Errorf("burst rule %s: threshold must be > 0", r.Name)
	}
	switch r.Mode {
	case burstModeSingle:
	case burstModeWindow:
		if r.WindowSec <= 0 {
			return fmt.Errorf("burst rule %s: windowSec is required for mode=window", r.Name)
		}
	default:
		return fmt.Errorf("burst rule %s: mode must be single|window, got %q", r.Name, r.Mode)
	}
	if r.MaxEvents < 0 {
		return fmt.Errorf("burst rule %s: maxEvents must be >= 0", r.Name)
	}
	return checkSeverity(r.Severity)
}

// Validate 连续试探规则静态校验。
func (r ProbeRule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("probing rule name is required")
	}
	if !isQualifiedTable(r.Table) {
		return fmt.Errorf("probing rule %s: table must be \"db.table\", got %q", r.Name, r.Table)
	}
	if err := checkOps(r.Ops); err != nil {
		return fmt.Errorf("probing rule %s: %w", r.Name, err)
	}
	if r.KeyField == "" || r.DeltaField == "" {
		return fmt.Errorf("probing rule %s: keyField/deltaField are required", r.Name)
	}
	if r.SmallBelow <= 0 {
		return fmt.Errorf("probing rule %s: smallBelow must be > 0", r.Name)
	}
	if r.Count < 2 {
		return fmt.Errorf("probing rule %s: count must be >= 2", r.Name)
	}
	if r.WindowSec <= 0 {
		return fmt.Errorf("probing rule %s: windowSec must be > 0", r.Name)
	}
	if r.MaxTracked < 0 {
		return fmt.Errorf("probing rule %s: maxTracked must be >= 0", r.Name)
	}
	return checkSeverity(r.Severity)
}

// ValidateCEPRules 三模板全量校验（config.Load 启动即拦）。
func ValidateCEPRules(offHours []OffHoursRule, burst []BurstRule, probing []ProbeRule) error {
	for i, r := range offHours {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("cep.offHours[%d]: %w", i, err)
		}
	}
	for i, r := range burst {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("cep.burst[%d]: %w", i, err)
		}
	}
	for i, r := range probing {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("cep.probing[%d]: %w", i, err)
		}
	}
	return nil
}

// burstState 数量突变累计桶（滑动窗口，事件时间驱动）。
type burstState struct {
	rule   BurstRule
	tss    []int64
	deltas []float64
	sum    float64
}

// probeState 连续试探按键计数（滑动窗口，事件时间驱动）。
type probeState struct {
	rule  ProbeRule
	seq   map[string][]int64 // 行键 → 小变更 ts 序列
	order []string           // 键入册 FIFO（容量驱逐确定性）
}

// CEPGate 闸二 CEP：三个固定模板的参数化实例集。环形桶计数，不引 CEP 引擎。
type CEPGate struct {
	offHours        []*OffHoursRule
	burst           map[string][]*burstState
	probing         map[string][]*probeState
	skippedNoDelta  int64 // delta 字段不可数值化计数（键规格配错信号）
	skippedProbeKey int64 // 试探行键不可提取计数
}

// NewCEP 构造 CEP 闸（规则先过 ValidateCEPRules；此处防御性复验）。
func NewCEP(offHours []OffHoursRule, burst []BurstRule, probing []ProbeRule) (*CEPGate, error) {
	if err := ValidateCEPRules(offHours, burst, probing); err != nil {
		return nil, err
	}
	g := &CEPGate{
		offHours: make([]*OffHoursRule, len(offHours)),
		burst:    map[string][]*burstState{},
		probing:  map[string][]*probeState{},
	}
	for i := range offHours {
		g.offHours[i] = &offHours[i]
	}
	for _, r := range burst {
		g.burst[r.Table] = append(g.burst[r.Table], &burstState{rule: r})
	}
	for _, r := range probing {
		g.probing[r.Table] = append(g.probing[r.Table], &probeState{rule: r, seq: map[string][]int64{}})
	}
	return g, nil
}

// Empty 无规则配置（runner 不必接闸）。
func (g *CEPGate) Empty() bool {
	return len(g.offHours) == 0 && len(g.burst) == 0 && len(g.probing) == 0
}

// SkippedNoDelta delta 字段不可数值化的累计计数。
func (g *CEPGate) SkippedNoDelta() int64 { return g.skippedNoDelta }

// SkippedProbeKey 试探行键不可提取的累计计数。
func (g *CEPGate) SkippedProbeKey() int64 { return g.skippedProbeKey }

// Evaluate 消费一个变更事件：评估该表命中的三模板实例。非订阅表零开销直通。
func (g *CEPGate) Evaluate(ev *connector.ChangeEvent) []Hit {
	table := ev.Database + "." + ev.Table
	var hits []Hit
	for _, r := range g.offHours {
		if r.Table == table {
			hits = append(hits, evalOffHours(r, ev)...)
		}
	}
	for _, s := range g.burst[table] {
		hits = append(hits, g.evalBurst(s, ev)...)
	}
	for _, s := range g.probing[table] {
		hits = append(hits, g.evalProbe(s, ev)...)
	}
	sortHitOrder(hits)
	return hits
}

// Sweep 周期清桶（runner 位点 tick 同频）：丢弃越窗桶项与空键序列。
// CEP 命中在事件到达时即时产出，Sweep 只做内存卫生。
func (g *CEPGate) Sweep(nowMs int64) {
	for _, states := range g.burst {
		for _, s := range states {
			if s.rule.Mode == burstModeWindow {
				trimBurst(s, nowMs-int64(s.rule.WindowSec)*1000)
			}
		}
	}
	for _, states := range g.probing {
		for _, s := range states {
			cutoff := nowMs - int64(s.rule.WindowSec)*1000
			for k, dq := range s.seq {
				dq = trimDeque(dq, cutoff)
				if len(dq) == 0 {
					delete(s.seq, k)
					continue
				}
				s.seq[k] = dq
			}
		}
	}
}

// evalOffHours 时段白名单判定：取事件时间（源真值；回放补扫不改变历史判定时点，
// 重复告警由位点幂等兜底——重扫本身就是异常路径）。
func evalOffHours(r *OffHoursRule, ev *connector.ChangeEvent) []Hit {
	if !opAllowed(r.Ops, ev.Operation) {
		return nil
	}
	ts := ev.TsUnixMs
	if ts == 0 {
		ts = time.Now().UnixMilli() // 占位源未带事件时间时按落点时间判
	}
	// 秒级日模（跨时区偏移可能跨日）→ 分钟偏移
	minOfDay := int(((ts/1000+int64(r.TzOffsetMin)*60)%86400 + 86400) % 86400 / 60)
	if inWindows(minOfDay, r.Windows) {
		return nil
	}
	wall := time.UnixMilli(ts).UTC().Add(time.Duration(r.TzOffsetMin) * time.Minute).Format("15:04:05")
	detail := fmt.Sprintf("%s at %s (tz%+dmin) outside allowed windows", ev.Operation, wall, r.TzOffsetMin)
	return []Hit{hitOf(RuleOffHours, r.Severity, r.Table, "", detail, *ev)}
}

func (g *CEPGate) evalBurst(s *burstState, ev *connector.ChangeEvent) []Hit {
	r := s.rule
	if !opAllowed(r.Ops, ev.Operation) {
		return nil
	}
	d, ok := deltaOf(ev, r.DeltaField)
	if !ok {
		g.skippedNoDelta++
		return nil
	}
	if r.Mode == burstModeSingle {
		if math.Abs(d) >= r.Threshold {
			detail := fmt.Sprintf("single delta %.6g exceeds threshold %.6g", d, r.Threshold)
			return []Hit{hitOf(RuleBurstDelta, r.Severity, r.Table, "", detail, *ev)}
		}
		return nil
	}
	abs := math.Abs(d)
	s.tss = append(s.tss, ev.TsUnixMs)
	s.deltas = append(s.deltas, abs)
	s.sum += abs
	trimBurst(s, ev.TsUnixMs-int64(r.WindowSec)*1000)
	if s.sum >= r.Threshold {
		detail := fmt.Sprintf("|delta| sum %.6g over %ds window exceeds threshold %.6g", s.sum, r.WindowSec, r.Threshold)
		hits := []Hit{hitOf(RuleBurstDelta, r.Severity, r.Table, "", detail, *ev)}
		s.tss, s.deltas, s.sum = s.tss[:0], s.deltas[:0], 0 // 命中即清零再计时，避免逐事件刷屏
		return hits
	}
	return nil
}

func (g *CEPGate) evalProbe(s *probeState, ev *connector.ChangeEvent) []Hit {
	r := s.rule
	if !opAllowed(r.Ops, ev.Operation) {
		return nil
	}
	d, ok := deltaOf(ev, r.DeltaField)
	if !ok {
		g.skippedNoDelta++
		return nil
	}
	key, keyOK := extractKey(rowImage(ev), r.KeyField)
	if !keyOK {
		g.skippedProbeKey++
		return nil
	}
	if math.Abs(d) >= r.SmallBelow {
		// 大额变更打断试探序列（大额本身由数量突变规则判定）
		if _, tracked := s.seq[key]; tracked {
			delete(s.seq, key)
		}
		return nil
	}
	if _, tracked := s.seq[key]; !tracked {
		if len(s.seq) >= capOf(r.MaxTracked, defaultProbeMaxTracked) {
			// 追踪键达上限：FIFO 丢最老键（确定性驱逐）
			oldest := s.order[0]
			s.order = s.order[1:]
			delete(s.seq, oldest)
		}
		s.order = append(s.order, key)
	}
	dq := trimDeque(s.seq[key], ev.TsUnixMs-int64(r.WindowSec)*1000)
	dq = append(dq, ev.TsUnixMs)
	s.seq[key] = dq
	if len(dq) >= r.Count {
		delete(s.seq, key) // 命中即重置：每键每窗口只告警一次
		detail := fmt.Sprintf("%d small changes (< %.6g) within %ds", r.Count, r.SmallBelow, r.WindowSec)
		return []Hit{hitOf(RuleProbing, r.Severity, r.Table, key, detail, *ev)}
	}
	return nil
}

// trimBurst 丢弃越窗桶项 + 桶上限（FIFO 丢最老），底层数组过量时回收。
func trimBurst(s *burstState, cutoff int64) {
	head := 0
	for head < len(s.tss) && s.tss[head] < cutoff {
		head++
	}
	drop := head
	if max := capOf(s.rule.MaxEvents, defaultBurstMaxEvents); len(s.tss)-drop > max {
		drop = len(s.tss) - max
	}
	if drop > 0 {
		s.sum -= sumOf(s.deltas[:drop])
		s.tss, s.deltas = s.tss[drop:], s.deltas[drop:]
	}
	if drop >= len(s.tss) && drop > 0 {
		s.tss, s.deltas = s.tss[:0], s.deltas[:0]
	} else if drop >= 1024 && drop >= len(s.tss)/2 {
		s.tss, s.deltas = compactTs(s.tss), compactOf(s.deltas)
	}
}

// trimDeque 丢弃早于 cutoff 的队首项。
func trimDeque(dq []int64, cutoff int64) []int64 {
	for len(dq) > 0 && dq[0] < cutoff {
		dq = dq[1:]
	}
	return dq
}

func capOf(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

func sumOf(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s
}

func compactOf(v []float64) []float64 {
	out := make([]float64, len(v))
	copy(out, v)
	return out
}

func compactTs(v []int64) []int64 {
	out := make([]int64, len(v))
	copy(out, v)
	return out
}

// hitOf 组装命中（severity 缺省 warn）。
func hitOf(ruleID, severity, table, key, detail string, ev connector.ChangeEvent) Hit {
	sev := severity
	if sev == "" {
		sev = SeverityWarn
	}
	return Hit{Rule: ruleID, Severity: sev, Table: table, Key: key, Detail: detail, Event: ev}
}

// opAllowed 操作过滤：空名单 = 全部变更。
func opAllowed(ops []string, op connector.Operation) bool {
	if len(ops) == 0 {
		return true
	}
	for _, o := range ops {
		if o == string(op) {
			return true
		}
	}
	return false
}

// checkOps 操作闭集校验。
func checkOps(ops []string) error {
	for _, o := range ops {
		switch connector.Operation(o) {
		case connector.OpInsert, connector.OpUpdate, connector.OpDelete:
		default:
			return fmt.Errorf("ops must be insert|update|delete, got %q", o)
		}
	}
	return nil
}

// checkSeverity 严重度闭集校验（空 = warn）。
func checkSeverity(s string) error {
	switch s {
	case "", SeverityWarn, SeverityCritical:
		return nil
	default:
		return fmt.Errorf("severity must be warn|critical, got %q", s)
	}
}

// inWindows 分钟偏移是否落在任一允许时段（start > end = 跨零点；区间左闭右开）。
func inWindows(minOfDay int, windows []OffHoursWindow) bool {
	for _, w := range windows {
		if w.StartMin <= w.EndMin {
			if minOfDay >= w.StartMin && minOfDay < w.EndMin {
				return true
			}
			continue
		}
		if minOfDay >= w.StartMin || minOfDay < w.EndMin {
			return true
		}
	}
	return false
}

// deltaOf 变更量：insert = 后像值（新增全额），delete = 前像值（清空全额），
// update = 后像 - 前像。字段不可数值化 → 不 ok（调用方计数跳过）。
func deltaOf(ev *connector.ChangeEvent, spec string) (float64, bool) {
	if ev.Operation == connector.OpInsert {
		return numericValue(ev.After, spec)
	}
	if ev.Operation == connector.OpDelete {
		return numericValue(ev.Before, spec)
	}
	after, aOK := numericValue(ev.After, spec)
	before, bOK := numericValue(ev.Before, spec)
	if !aOK || !bOK {
		return 0, false
	}
	return after - before, true
}

// numericValue 数量字段取值（键规格形态同 extractKey，值须可数值化）。
// float64 精度边界：整数金额 < 2^53 安全（§8 差异记录）。
func numericValue(img map[string]json.RawMessage, spec string) (float64, bool) {
	key, ok := extractKey(img, spec)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(key, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
