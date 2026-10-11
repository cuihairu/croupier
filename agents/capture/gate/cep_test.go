package gate

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/source"
)

// amountRow 行像 [ key, amount ]：键下标 0、数量下标 1。
func amountRow(key string, amount int) json.RawMessage {
	return rowJSON(key, amount)
}

func amountEvent(op source.Op, key string, before, after int, tsMs int64) source.ChangeEvent {
	ev := source.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "wallet", Op: op,
		GTID: "g-" + key + "-" + string(op) + "-" + strconv.FormatInt(tsMs, 10), TsUnixMs: tsMs,
	}
	switch op {
	case source.OpInsert:
		ev.AfterJSON = amountRow(key, after)
	case source.OpDelete:
		ev.BeforeJSON = amountRow(key, before)
	default:
		ev.BeforeJSON = amountRow(key, before)
		ev.AfterJSON = amountRow(key, after)
	}
	return ev
}

// --- off_hours ---

func TestOffHoursInsideWindowNoHit(t *testing.T) {
	rule := OffHoursRule{
		Name: "wallet-night", Table: "game.wallet",
		Windows: []OffHoursWindow{{StartMin: 120, EndMin: 240}}, // 02:00-04:00 UTC
	}
	g, err := NewCEP([]OffHoursRule{rule}, nil, nil)
	if err != nil {
		t.Fatalf("NewCEP: %v", err)
	}
	// 03:00 UTC = 120+60=180min 落窗内
	ev := amountEvent(source.OpInsert, "a", 0, 100, tsOf(0, 3, 0))
	if hits := g.Evaluate(&ev); len(hits) != 0 {
		t.Fatalf("inside window should not hit: %+v", hits)
	}
}

func TestOffHoursOutsideWindowHits(t *testing.T) {
	rule := OffHoursRule{
		Name: "wallet-night", Table: "game.wallet",
		Windows: []OffHoursWindow{{StartMin: 120, EndMin: 240}},
	}
	g, _ := NewCEP([]OffHoursRule{rule}, nil, nil)
	// 12:00 UTC 落窗外
	ev := amountEvent(source.OpUpdate, "a", 100, 500, tsOf(0, 12, 0))
	hits := g.Evaluate(&ev)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	h := hits[0]
	if h.Rule != RuleOffHours || h.Severity != SeverityWarn || h.Table != "game.wallet" {
		t.Errorf("hit = %+v", h)
	}
	if !strings.Contains(h.Detail, "12:00:00") || !strings.Contains(h.Detail, "update") {
		t.Errorf("detail = %q", h.Detail)
	}
}

func TestOffHoursWrapWindowAndTz(t *testing.T) {
	// 22:00-04:00（跨零点），东八区（tzOffsetMin=480）
	rule := OffHoursRule{
		Name: "wallet-night", Table: "game.wallet", TzOffsetMin: 480,
		Windows: []OffHoursWindow{{StartMin: 22 * 60, EndMin: 4 * 60}},
	}
	g, _ := NewCEP([]OffHoursRule{rule}, nil, nil)
	// 18:00 UTC = 次日 02:00 北京（跨零点窗内）→ 不命中
	ev := amountEvent(source.OpInsert, "a", 0, 1, tsOf(0, 18, 0))
	if hits := g.Evaluate(&ev); len(hits) != 0 {
		t.Fatalf("02:00 CST should be inside wrap window: %+v", hits)
	}
	// 20:00 UTC = 04:00 北京（右端开）→ 命中
	ev2 := amountEvent(source.OpInsert, "a", 0, 1, tsOf(0, 20, 0))
	hits := g.Evaluate(&ev2)
	if len(hits) != 1 {
		t.Fatalf("04:00 CST (exclusive end) should hit: %+v", hits)
	}
	if !strings.Contains(hits[0].Detail, "04:00:00") || !strings.Contains(hits[0].Detail, "tz+480min") {
		t.Errorf("detail = %q", hits[0].Detail)
	}
}

func TestOffHoursOpsFilter(t *testing.T) {
	rule := OffHoursRule{
		Name: "insert-only", Table: "game.wallet", Ops: []string{"insert"},
		Windows: []OffHoursWindow{{StartMin: 600, EndMin: 610}},
	}
	g, _ := NewCEP([]OffHoursRule{rule}, nil, nil)
	// update 不在 ops 名单 → 直通
	ev := amountEvent(source.OpUpdate, "a", 1, 2, tsOf(0, 23, 0))
	if hits := g.Evaluate(&ev); len(hits) != 0 {
		t.Fatalf("filtered op should not hit: %+v", hits)
	}
	// insert 落窗外 → 命中
	ev2 := amountEvent(source.OpInsert, "a", 0, 2, tsOf(0, 23, 0))
	if hits := g.Evaluate(&ev2); len(hits) != 1 {
		t.Fatalf("insert outside window should hit: %+v", hits)
	}
}

func TestOffHoursCriticalSeverity(t *testing.T) {
	rule := OffHoursRule{
		Name: "wallet-night", Table: "game.wallet", Severity: SeverityCritical,
		Windows: []OffHoursWindow{{StartMin: 600, EndMin: 610}},
	}
	g, _ := NewCEP([]OffHoursRule{rule}, nil, nil)
	ev := amountEvent(source.OpInsert, "a", 0, 1, tsOf(0, 23, 0))
	if hits := g.Evaluate(&ev); len(hits) != 1 || hits[0].Severity != SeverityCritical {
		t.Fatalf("hits = %+v", hits)
	}
}

// --- burst_delta ---

func TestBurstSingleOverThreshold(t *testing.T) {
	rule := BurstRule{
		Name: "wallet-drain", Table: "game.wallet", DeltaField: "1",
		Threshold: 1000, Mode: "single",
	}
	g, _ := NewCEP(nil, []BurstRule{rule}, nil)
	// delta 400+500=900 < 1000：不命中
	ev := amountEvent(source.OpUpdate, "a", 100, 500, tsOf(0, 1, 0))
	if hits := g.Evaluate(&ev); len(hits) != 0 {
		t.Fatalf("below threshold should not hit: %+v", hits)
	}
	// delta -5000：|delta| 超阈命中（双向突变）
	ev2 := amountEvent(source.OpUpdate, "a", 5000, 0, tsOf(0, 1, 1))
	hits := g.Evaluate(&ev2)
	if len(hits) != 1 || hits[0].Rule != RuleBurstDelta {
		t.Fatalf("hits = %+v", hits)
	}
	if !strings.Contains(hits[0].Detail, "single delta -5000") {
		t.Errorf("detail = %q", hits[0].Detail)
	}
}

func TestBurstInsertDeleteFullAmount(t *testing.T) {
	rule := BurstRule{
		Name: "wallet-drain", Table: "game.wallet", DeltaField: "1",
		Threshold: 1000, Mode: "single",
	}
	g, _ := NewCEP(nil, []BurstRule{rule}, nil)
	// insert：后像值即 delta
	ev := amountEvent(source.OpInsert, "a", 0, 2000, tsOf(0, 1, 0))
	if hits := g.Evaluate(&ev); len(hits) != 1 {
		t.Fatalf("insert delta = after image, want hit: %+v", hits)
	}
	// delete：前像值即 delta
	ev2 := amountEvent(source.OpDelete, "a", 3000, 0, tsOf(0, 1, 1))
	if hits := g.Evaluate(&ev2); len(hits) != 1 {
		t.Fatalf("delete delta = before image, want hit: %+v", hits)
	}
}

func TestBurstWindowCumulative(t *testing.T) {
	rule := BurstRule{
		Name: "wallet-churn", Table: "game.wallet", DeltaField: "1",
		Threshold: 1000, Mode: "window", WindowSec: 60,
	}
	g, _ := NewCEP(nil, []BurstRule{rule}, nil)
	base := tsOf(0, 1, 0)
	// 600+500 = 1100 ≥ 1000：第 2 笔命中
	ev1 := amountEvent(source.OpUpdate, "a", 0, 600, base)
	ev2 := amountEvent(source.OpUpdate, "b", 0, 500, base+1000)
	if hits := g.Evaluate(&ev1); len(hits) != 0 {
		t.Fatalf("first event should not hit: %+v", hits)
	}
	hits := g.Evaluate(&ev2)
	if len(hits) != 1 {
		t.Fatalf("cumulative over threshold should hit: %+v", hits)
	}
	if !strings.Contains(hits[0].Detail, "sum") || !strings.Contains(hits[0].Detail, "60s window") {
		t.Errorf("detail = %q", hits[0].Detail)
	}
	// 命中即清零再计时：第 3 笔 400 不命中
	ev3 := amountEvent(source.OpUpdate, "c", 0, 400, base+2000)
	if hits := g.Evaluate(&ev3); len(hits) != 0 {
		t.Fatalf("after reset should not hit: %+v", hits)
	}
	// 越窗桶项不再累计：60s 后再来 500 也不命中
	ev4 := amountEvent(source.OpUpdate, "d", 0, 500, base+61_000)
	if hits := g.Evaluate(&ev4); len(hits) != 0 {
		t.Fatalf("expired bucket should not accumulate: %+v", hits)
	}
}

func TestBurstNoDeltaSkipped(t *testing.T) {
	rule := BurstRule{
		Name: "wallet-drain", Table: "game.wallet", DeltaField: "9", // 越界列
		Threshold: 1, Mode: "single",
	}
	g, _ := NewCEP(nil, []BurstRule{rule}, nil)
	ev := amountEvent(source.OpUpdate, "a", 100, 500, tsOf(0, 1, 0))
	if hits := g.Evaluate(&ev); len(hits) != 0 {
		t.Fatalf("no-delta should not hit: %+v", hits)
	}
	if g.SkippedNoDelta() != 1 {
		t.Fatalf("SkippedNoDelta = %d, want 1", g.SkippedNoDelta())
	}
}

// --- probing ---

func TestProbingCountAndRearm(t *testing.T) {
	rule := ProbeRule{
		Name: "slow-scrape", Table: "game.wallet", KeyField: "0", DeltaField: "1",
		SmallBelow: 10, Count: 3, WindowSec: 60,
	}
	g, _ := NewCEP(nil, nil, []ProbeRule{rule})
	base := tsOf(0, 1, 0)
	// 2 次小变更：不命中
	for i := int64(0); i < 2; i++ {
		ev := amountEvent(source.OpUpdate, "p1", 0, 5, base+i*1000)
		if hits := g.Evaluate(&ev); len(hits) != 0 {
			t.Fatalf("before count reached should not hit: %+v", hits)
		}
	}
	// 第 3 次：命中，Key = 行键
	ev3 := amountEvent(source.OpUpdate, "p1", 0, 5, base+2000)
	hits := g.Evaluate(&ev3)
	if len(hits) != 1 || hits[0].Rule != RuleProbing || hits[0].Key != "p1" {
		t.Fatalf("hits = %+v", hits)
	}
	if !strings.Contains(hits[0].Detail, "3 small changes") {
		t.Errorf("detail = %q", hits[0].Detail)
	}
	// 命中即重置：同键继续小额变更需再积 3 次
	for i := int64(0); i < 2; i++ {
		ev := amountEvent(source.OpUpdate, "p1", 0, 5, base+3000+i*1000)
		if hits := g.Evaluate(&ev); len(hits) != 0 {
			t.Fatalf("re-arming should not hit: %+v", hits)
		}
	}
	evR := amountEvent(source.OpUpdate, "p1", 0, 5, base+5000)
	if hits := g.Evaluate(&evR); len(hits) != 1 {
		t.Fatalf("re-armed count should hit: %+v", hits)
	}
}

func TestProbingKeyIsolationAndBigBreak(t *testing.T) {
	rule := ProbeRule{
		Name: "slow-scrape", Table: "game.wallet", KeyField: "0", DeltaField: "1",
		SmallBelow: 10, Count: 3, WindowSec: 60,
	}
	g, _ := NewCEP(nil, nil, []ProbeRule{rule})
	base := tsOf(0, 1, 0)
	// 不同键不累计：p1×2 + p2×2 都不命中
	for i := int64(0); i < 2; i++ {
		e1 := amountEvent(source.OpUpdate, "p1", 0, 5, base+i*1000)
		e2 := amountEvent(source.OpUpdate, "p2", 0, 5, base+i*1000)
		if hits := g.Evaluate(&e1); len(hits) != 0 {
			t.Fatalf("p1 should not hit: %+v", hits)
		}
		if hits := g.Evaluate(&e2); len(hits) != 0 {
			t.Fatalf("p2 should not hit: %+v", hits)
		}
	}
	// 大额变更打断序列：p1 归零
	big := amountEvent(source.OpUpdate, "p1", 0, 999, base+3000)
	if hits := g.Evaluate(&big); len(hits) != 0 {
		t.Fatalf("big change should not hit probing: %+v", hits)
	}
	small := amountEvent(source.OpUpdate, "p1", 0, 5, base+4000)
	if hits := g.Evaluate(&small); len(hits) != 0 {
		t.Fatalf("sequence broken, one small change should not hit: %+v", hits)
	}
	// p2 未被打断：第 3 次命中
	p2 := amountEvent(source.OpUpdate, "p2", 0, 5, base+4000)
	if hits := g.Evaluate(&p2); len(hits) != 1 {
		t.Fatalf("p2 third small change should hit: %+v", hits)
	}
}

func TestProbingExpiredEntriesAndSweep(t *testing.T) {
	rule := ProbeRule{
		Name: "slow-scrape", Table: "game.wallet", KeyField: "0", DeltaField: "1",
		SmallBelow: 10, Count: 3, WindowSec: 60,
	}
	g, _ := NewCEP(nil, nil, []ProbeRule{rule})
	base := tsOf(0, 1, 0)
	// t0、t1 各一次，t+70s 时窗口已空 → 之后 3 连发才命中
	for i := int64(0); i < 2; i++ {
		ev := amountEvent(source.OpUpdate, "p1", 0, 5, base+i*1000)
		g.Evaluate(&ev)
	}
	g.Sweep(base + 70_000)
	for i := int64(0); i < 2; i++ {
		ev := amountEvent(source.OpUpdate, "p1", 0, 5, base+70_000+i*1000)
		if hits := g.Evaluate(&ev); len(hits) != 0 {
			t.Fatalf("fresh window should not hit: %+v", hits)
		}
	}
	ev := amountEvent(source.OpUpdate, "p1", 0, 5, base+72_000)
	if hits := g.Evaluate(&ev); len(hits) != 1 {
		t.Fatalf("third in fresh window should hit: %+v", hits)
	}
}

func TestProbingNoKeySkipped(t *testing.T) {
	rule := ProbeRule{
		Name: "slow-scrape", Table: "game.wallet", KeyField: "5", DeltaField: "1", // 键越界
		SmallBelow: 10, Count: 2, WindowSec: 60,
	}
	g, _ := NewCEP(nil, nil, []ProbeRule{rule})
	ev := amountEvent(source.OpUpdate, "p1", 0, 5, tsOf(0, 1, 0))
	if hits := g.Evaluate(&ev); len(hits) != 0 {
		t.Fatalf("no-key should not hit: %+v", hits)
	}
	if g.SkippedProbeKey() != 1 {
		t.Fatalf("SkippedProbeKey = %d, want 1", g.SkippedProbeKey())
	}
}

func TestProbingMaxTrackedFIFOEviction(t *testing.T) {
	rule := ProbeRule{
		Name: "slow-scrape", Table: "game.wallet", KeyField: "0", DeltaField: "1",
		SmallBelow: 10, Count: 2, WindowSec: 600, MaxTracked: 2,
	}
	g, _ := NewCEP(nil, nil, []ProbeRule{rule})
	base := tsOf(0, 1, 0)
	for _, k := range []string{"a", "b"} {
		ev := amountEvent(source.OpUpdate, k, 0, 5, base)
		g.Evaluate(&ev)
	}
	// 第 3 键 c 挤掉最老的 a → 在册 [b, c]
	evC := amountEvent(source.OpUpdate, "c", 0, 5, base+1000)
	g.Evaluate(&evC)
	// a 回册：挤掉 b → 在册 [c, a]；a 从零计起不命中
	evA := amountEvent(source.OpUpdate, "a", 0, 5, base+2000)
	if hits := g.Evaluate(&evA); len(hits) != 0 {
		t.Fatalf("evicted key must restart from zero: %+v", hits)
	}
	// b 已被挤掉：从零计起不命中（同时挤掉 c → 在册 [a, b]）
	evB := amountEvent(source.OpUpdate, "b", 0, 5, base+3000)
	if hits := g.Evaluate(&evB); len(hits) != 0 {
		t.Fatalf("re-evicted key must restart from zero: %+v", hits)
	}
	// a 在册已有 1 次 + 本次 = 2 → 命中
	evA2 := amountEvent(source.OpUpdate, "a", 0, 5, base+4000)
	if hits := g.Evaluate(&evA2); len(hits) != 1 {
		t.Fatalf("tracked key should hit on second: %+v", hits)
	}
}

// --- 校验 ---

func TestCEPValidation(t *testing.T) {
	if err := (OffHoursRule{Name: "x", Table: "db.t", Windows: []OffHoursWindow{{StartMin: 1, EndMin: 2}}}).Validate(); err != nil {
		t.Errorf("valid offHours: %v", err)
	}
	if err := (OffHoursRule{Name: "x", Table: "bad", Windows: []OffHoursWindow{{StartMin: 1, EndMin: 2}}}).Validate(); err == nil {
		t.Error("bad table must fail")
	}
	if err := (OffHoursRule{Name: "x", Table: "db.t"}).Validate(); err == nil || !strings.Contains(err.Error(), "windows must not be empty") {
		t.Errorf("empty windows error = %v", err)
	}
	if err := (OffHoursRule{Name: "x", Table: "db.t", Windows: []OffHoursWindow{{StartMin: 1, EndMin: 1441}}}).Validate(); err == nil {
		t.Error("endMin 1441 must fail")
	}
	if err := (OffHoursRule{Name: "x", Table: "db.t", TzOffsetMin: 900, Windows: []OffHoursWindow{{StartMin: 1, EndMin: 2}}}).Validate(); err == nil {
		t.Error("tz 900 must fail")
	}
	if err := (OffHoursRule{Name: "x", Table: "db.t", Ops: []string{"drop"}, Windows: []OffHoursWindow{{StartMin: 1, EndMin: 2}}}).Validate(); err == nil || !strings.Contains(err.Error(), "insert|update|delete") {
		t.Errorf("bad op error = %v", err)
	}

	if err := (BurstRule{Name: "x", Table: "db.t", DeltaField: "1", Threshold: 1, Mode: "single"}).Validate(); err != nil {
		t.Errorf("valid burst: %v", err)
	}
	if err := (BurstRule{Name: "x", Table: "db.t", DeltaField: "1", Threshold: 0, Mode: "single"}).Validate(); err == nil {
		t.Error("threshold 0 must fail")
	}
	if err := (BurstRule{Name: "x", Table: "db.t", DeltaField: "1", Threshold: 1, Mode: "avg"}).Validate(); err == nil || !strings.Contains(err.Error(), "single|window") {
		t.Errorf("bad mode error = %v", err)
	}
	if err := (BurstRule{Name: "x", Table: "db.t", DeltaField: "1", Threshold: 1, Mode: "window"}).Validate(); err == nil || !strings.Contains(err.Error(), "windowSec") {
		t.Errorf("window without windowSec error = %v", err)
	}

	if err := (ProbeRule{Name: "x", Table: "db.t", KeyField: "0", DeltaField: "1", SmallBelow: 1, Count: 2, WindowSec: 1}).Validate(); err != nil {
		t.Errorf("valid probing: %v", err)
	}
	if err := (ProbeRule{Name: "x", Table: "db.t", KeyField: "0", DeltaField: "1", SmallBelow: 1, Count: 1, WindowSec: 1}).Validate(); err == nil {
		t.Error("count 1 must fail")
	}
	if err := (ProbeRule{Name: "x", Table: "db.t", KeyField: "", DeltaField: "1", SmallBelow: 1, Count: 2, WindowSec: 1}).Validate(); err == nil {
		t.Error("empty keyField must fail")
	}
	if err := ValidateCEPRules(nil, nil, nil); err != nil {
		t.Errorf("empty CEP config: %v", err)
	}
}

func TestCEPMultiRuleSameTable(t *testing.T) {
	// 同表多实例合法（时段 + 突变并存），与对账闸"同表唯一"不同
	off := OffHoursRule{Name: "oh", Table: "game.wallet", Windows: []OffHoursWindow{{StartMin: 600, EndMin: 610}}}
	burst := BurstRule{Name: "bd", Table: "game.wallet", DeltaField: "1", Threshold: 100, Mode: "single"}
	g, err := NewCEP([]OffHoursRule{off}, []BurstRule{burst}, nil)
	if err != nil {
		t.Fatalf("NewCEP: %v", err)
	}
	// 大额 + 落窗外：两实例各出一条命中（评估序 = offHours → burst，稳定排序保持）
	ev := amountEvent(source.OpInsert, "a", 0, 5000, tsOf(0, 23, 0))
	hits := g.Evaluate(&ev)
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2: %+v", len(hits), hits)
	}
	if hits[0].Rule != RuleOffHours || hits[1].Rule != RuleBurstDelta {
		t.Errorf("order = %s, %s", hits[0].Rule, hits[1].Rule)
	}
}

func TestCEPNonSubscribedTableDirect(t *testing.T) {
	g, _ := NewCEP([]OffHoursRule{{Name: "oh", Table: "game.wallet", Windows: []OffHoursWindow{{StartMin: 1, EndMin: 2}}}}, nil, nil)
	ev := amountEvent(source.OpInsert, "a", 0, 1, tsOf(0, 23, 0))
	ev.Table = "other"
	if hits := g.Evaluate(&ev); len(hits) != 0 {
		t.Fatalf("non-subscribed table should not hit: %+v", hits)
	}
	empty, err := NewCEP(nil, nil, nil)
	if err != nil {
		t.Fatalf("NewCEP empty: %v", err)
	}
	if !empty.Empty() {
		t.Fatal("no-rule gate must be Empty")
	}
	if g.Empty() {
		t.Fatal("gate with offHours rule must not be Empty")
	}
}

// tsOf 构造 UTC 当日时分的事件 ts。
func tsOf(_ int, hour, minute int) int64 {
	// 固定锚日 2026-10-11T00:00:00Z = 1791676800s
	return int64(1791676800+hour*3600+minute*60) * 1000
}
