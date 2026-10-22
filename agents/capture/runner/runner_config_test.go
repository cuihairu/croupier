package runner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/core/configsync"
)

// gatesJSON 组装面板下发形态的 capture.gates 块（与配置文件语义一致）。
func gatesJSON(t *testing.T, raw string) []byte {
	t.Helper()
	return []byte(raw)
}

func reconcileRuleJSON(name, itemTable, ledgerTable string) string {
	return fmt.Sprintf(`{"reconcile":{"rules":[{"name":%q,"itemTable":%q,"ledgerTable":%q,"itemKey":"1","ledgerKey":"0","windowSec":60}]}}`,
		name, itemTable, ledgerTable)
}

func TestApplyGatesHotSwapsSnapshot(t *testing.T) {
	r := New(testConfig(t, true), nil)
	if gs := r.gates.Load(); gs.reconcile != nil {
		t.Fatal("initial snapshot must come from empty file rules")
	}

	if err := r.applyGates(context.Background(), gatesJSON(t, reconcileRuleJSON("item-ledger", "game.items", "game.ledger"))); err != nil {
		t.Fatalf("applyGates: %v", err)
	}
	if r.gates.Load().reconcile == nil {
		t.Fatal("reconcile gate should be wired after hot apply")
	}

	// 换新版本：旧快照整体被替换（指针切换，不叠加）。
	if err := r.applyGates(context.Background(), gatesJSON(t, `{}`)); err != nil {
		t.Fatalf("applyGates empty: %v", err)
	}
	if gs := r.gates.Load(); gs.reconcile != nil || gs.cep != nil || gs.lineage != nil {
		t.Fatalf("snapshot after empty apply = %+v, want all gates unwired", gs)
	}
}

func TestApplyGatesKeepsLastGoodOnInvalid(t *testing.T) {
	r := New(testConfig(t, true), nil)
	if err := r.applyGates(context.Background(), gatesJSON(t, reconcileRuleJSON("item-ledger", "game.items", "game.ledger"))); err != nil {
		t.Fatalf("applyGates: %v", err)
	}

	cases := map[string][]byte{
		"malformed json":                 []byte(`{"reconcile":`),
		"same item/ledger table":         gatesJSON(t, reconcileRuleJSON("bad", "game.items", "game.items")),
		"unknown fields ok but bad rule": []byte(`{"cep":{"burst":[{"name":"x"}]}}`),
	}
	for name, raw := range cases {
		if err := r.applyGates(context.Background(), raw); err == nil {
			t.Fatalf("%s: expected rejection", name)
		}
		if r.gates.Load().reconcile == nil {
			t.Fatalf("%s: last-good snapshot must survive failed apply", name)
		}
	}
}

// scriptedFetch 按序回放拉取结果（版本化载荷/空/错误）。
type scriptedFetch struct {
	results []fetchResult
	calls   int
}

type fetchResult struct {
	version string
	payload []byte
	nilResp bool
	err     error
}

func (s *scriptedFetch) fetch(context.Context) (*configsync.Versioned[[]byte], error) {
	if s.calls >= len(s.results) {
		return nil, errors.New("script exhausted")
	}
	res := s.results[s.calls]
	s.calls++
	if res.err != nil {
		return nil, res.err
	}
	if res.nilResp {
		return nil, nil
	}
	return &configsync.Versioned[[]byte]{Version: res.version, Payload: res.payload}, nil
}

func TestGatesPullerVersionSkipsAndLastGood(t *testing.T) {
	r := New(testConfig(t, true), nil)
	script := &scriptedFetch{results: []fetchResult{
		{version: "3", payload: gatesJSON(t, reconcileRuleJSON("v3", "game.items", "game.ledger"))},
		{version: "3", payload: gatesJSON(t, reconcileRuleJSON("v3-again", "game.items", "game.ledger"))},
		{nilResp: true}, // 服务端未发布（空版本）→ 保持现状
		{err: errors.New("dial timeout")},
		{version: "4", payload: []byte(`{"reconcile":`)}, // 非法规则 → 拒绝
		{version: "5", payload: gatesJSON(t, reconcileRuleJSON("v5", "game.items", "game.ledger"))},
	}}
	r.fetchConfig = script.fetch
	puller := r.newGatesPuller()
	ctx := context.Background()

	applied, err := puller.PullOnce(ctx)
	if err != nil || !applied {
		t.Fatalf("first pull = %v, %v; want applied", applied, err)
	}
	if r.gates.Load().reconcile == nil {
		t.Fatal("v3 should wire reconcile gate")
	}

	applied, err = puller.PullOnce(ctx)
	if err != nil || applied {
		t.Fatalf("same-version pull = %v, %v; want skipped", applied, err)
	}

	applied, err = puller.PullOnce(ctx)
	if err != nil || applied {
		t.Fatalf("empty-version pull = %v, %v; want no-op", applied, err)
	}
	if r.gates.Load().reconcile == nil {
		t.Fatal("empty version must keep current gates")
	}

	if _, err = puller.PullOnce(ctx); err == nil {
		t.Fatal("fetch error must propagate")
	}
	if r.gates.Load().reconcile == nil {
		t.Fatal("fetch error must keep current gates")
	}

	applied, err = puller.PullOnce(ctx)
	if err == nil || applied {
		t.Fatalf("invalid-rule pull = %v, %v; want apply failure", applied, err)
	}
	if r.gates.Load().reconcile == nil {
		t.Fatal("failed apply must keep last-good gates")
	}

	applied, err = puller.PullOnce(ctx)
	if err != nil || !applied {
		t.Fatalf("v5 pull = %v, %v; want applied", applied, err)
	}
	if gs := r.gates.Load(); gs.reconcile == nil {
		t.Fatal("v5 should wire reconcile gate")
	}
}
