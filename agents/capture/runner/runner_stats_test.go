package runner

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
)

// 过滤统计装配链（过滤策略简档 §8 口径）：evaluated=进闸评估才计；
// policy_apply_errors=应用失败 keep last-good 计数；rule_evaluation_errors=
// 检测失败保留事件，sweep 周期聚合门上计数（SkippedNoDelta 口径推广）。

func statsEvent(op connector.Operation) connector.ChangeEvent {
	return connector.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "items", Operation: op,
		After: contractRow("junk", "9001"), Position: connector.Position{Kind: "gtid", Value: "g1"},
		TsUnixMs: time.Now().UnixMilli(),
	}
}

// cepSkipRules 单事件突变规则，deltaField 指向越界列（键规格配错形态）：
// 事件可进闸但 delta 不可数值化 = 检测失败保留事件只计数。
func cepSkipRules() string {
	return `{"cep":{"burst":[{"name":"amount-spike","table":"game.items","ops":["update"],"deltaField":"9","threshold":100,"mode":"single","severity":"warn"}]}}`
}

func TestConsumeCountsEvaluated(t *testing.T) {
	r := New(testConfig(t, true), nil)

	// 启动=文件规则空：无任何闸实例，事件直通不计 evaluated
	r.consume(statsEvent("insert"))
	if st := r.Stats(); st.Evaluated != 0 {
		t.Fatalf("no gates: Evaluated = %d, want 0", st.Evaluated)
	}

	if err := r.applyGates(context.Background(), []byte(reconcileRulesFor())); err != nil {
		t.Fatalf("applyGates: %v", err)
	}
	r.consume(statsEvent("insert"))   // 白名单内 → 进闸评估
	r.consume(statsEvent("update"))   // 未列入白名单 → 粗过滤丢弃，不计
	r.consume(statsEvent("truncate")) // 非法操作形态 → 计 FilteredError，不计

	st := r.Stats()
	if st.Evaluated != 1 || st.FilteredNoMatch != 1 || st.FilteredError != 1 {
		t.Fatalf("stats = %+v, want Evaluated=1 FilteredNoMatch=1 FilteredError=1", st)
	}
}

func TestApplyGatesFailureCountsPolicyApplyErrors(t *testing.T) {
	r := New(testConfig(t, true), nil)
	if err := r.applyGates(context.Background(), []byte(reconcileRulesFor())); err != nil {
		t.Fatalf("applyGates: %v", err)
	}

	if err := r.applyGates(context.Background(), []byte(`not-json`)); err == nil {
		t.Fatal("malformed json should fail to apply")
	}
	if err := r.applyGates(context.Background(), []byte(`{"event":{"ops":["bogus"]}}`)); err == nil {
		t.Fatal("invalid rules should fail to apply")
	}
	if st := r.Stats(); st.PolicyApplyErrors != 2 {
		t.Fatalf("PolicyApplyErrors = %d, want 2", st.PolicyApplyErrors)
	}

	// 应用失败=没生效（keep last-good）：旧闸仍在位，计数是可观测信号
	gs := r.gates.Load()
	if gs.event == nil || gs.reconcile == nil {
		t.Fatalf("last-good gateSet must survive apply failures: %+v", gs)
	}
}

func TestSweepAggregatesRuleEvalErrors(t *testing.T) {
	r := New(testConfig(t, true), nil)
	if err := r.applyGates(context.Background(), []byte(cepSkipRules())); err != nil {
		t.Fatalf("applyGates: %v", err)
	}

	// 事件保留（不丢不静默），门上计数，sweep 聚合进统一可观测面
	e := statsEvent("update")
	r.consume(e)
	if st := r.Stats(); st.RuleEvalErrors != 0 {
		t.Fatalf("before sweep RuleEvalErrors = %d, want 0", st.RuleEvalErrors)
	}
	r.sweepGates(time.Now().UnixMilli())
	if st := r.Stats(); st.RuleEvalErrors != 1 {
		t.Fatalf("RuleEvalErrors = %d, want 1", st.RuleEvalErrors)
	}

	r.consume(e)
	r.sweepGates(time.Now().UnixMilli())
	if st := r.Stats(); st.RuleEvalErrors != 2 {
		t.Fatalf("RuleEvalErrors = %d, want 2", st.RuleEvalErrors)
	}

	// 热换新门实例：门上计数归零，基线重置不产生虚增
	if err := r.applyGates(context.Background(), []byte(cepSkipRules())); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	r.sweepGates(time.Now().UnixMilli())
	if st := r.Stats(); st.RuleEvalErrors != 2 {
		t.Fatalf("RuleEvalErrors after hot-swap = %d, want 2 (no phantom delta)", st.RuleEvalErrors)
	}
	r.consume(e)
	r.sweepGates(time.Now().UnixMilli())
	if st := r.Stats(); st.RuleEvalErrors != 3 {
		t.Fatalf("RuleEvalErrors = %d, want 3", st.RuleEvalErrors)
	}
}
