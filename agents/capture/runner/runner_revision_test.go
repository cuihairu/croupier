package runner

import (
	"context"
	"errors"
	"testing"
)

// appliedRevision 上报链（CDC 平台化批阶段三）：拉取器 onApply 记录生效
// 版本 → buildPayload 以 captureGatesRevision label 捎带（经重注册刷新
// server session labels）。reg 未就绪（测试/启动窗口）容忍跳过。
func TestAppliedRevisionLabelFollowsPull(t *testing.T) {
	r := New(testConfig(t, true), nil)

	// 未生效任何面板版本：不带键（面板侧语义 = 文件规则）
	if req, err := r.buildPayload(context.Background()); err != nil {
		t.Fatalf("buildPayload: %v", err)
	} else if _, ok := req.Labels["captureGatesRevision"]; ok {
		t.Fatalf("labels = %v, want no revision label before apply", req.Labels)
	}

	script := &scriptedFetch{results: []fetchResult{
		{version: "7", payload: []byte(reconcileRuleJSON("item-ledger", "game.items", "game.ledger"))},
		{version: "9", payload: []byte(reconcileRuleJSON("item-ledger", "game.items", "game.ledger"))},
	}}
	r.fetchConfig = script.fetch
	puller := r.newGatesPuller()

	if applied, err := puller.PullOnce(context.Background()); err != nil || !applied {
		t.Fatalf("v7 pull = %v, %v; want applied", applied, err)
	}
	req, err := r.buildPayload(context.Background())
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if got := req.Labels["captureGatesRevision"]; got != "7" {
		t.Fatalf("captureGatesRevision = %q, want 7", got)
	}

	// 版本推进：label 跟随（reg 未就绪不 panic——重注册是尽力而为）
	if applied, err := puller.PullOnce(context.Background()); err != nil || !applied {
		t.Fatalf("v9 pull = %v, %v; want applied", applied, err)
	}
	req, err = r.buildPayload(context.Background())
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if got := req.Labels["captureGatesRevision"]; got != "9" {
		t.Fatalf("captureGatesRevision = %q, want 9", got)
	}

	// 生效失败（非法规则）不推进版本游标，label 保持上一生效值
	r.fetchConfig = (&scriptedFetch{results: []fetchResult{
		{version: "10", payload: []byte(`{"reconcile":`)},
	}}).fetch
	puller = r.newGatesPuller()
	if _, err := puller.PullOnce(context.Background()); err == nil {
		t.Fatal("invalid payload must fail apply")
	}
	req, err = r.buildPayload(context.Background())
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if got := req.Labels["captureGatesRevision"]; got != "9" {
		t.Fatalf("captureGatesRevision after failed apply = %q, want 9 (keep last applied)", got)
	}
}

func TestAppliedRevisionSyncFailureTolerated(t *testing.T) {
	r := New(testConfig(t, true), nil)
	// reg 保持 nil（Run 未装配）：onApply 回调不得因重注册句柄缺失 panic
	script := &scriptedFetch{results: []fetchResult{
		{version: "3", payload: []byte(reconcileRuleJSON("item-ledger", "game.items", "game.ledger"))},
	}}
	r.fetchConfig = script.fetch
	if _, err := r.newGatesPuller().PullOnce(context.Background()); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if r.appliedGatesRevision.Load() == nil || *r.appliedGatesRevision.Load() != "3" {
		t.Fatalf("appliedGatesRevision = %v, want 3", r.appliedGatesRevision.Load())
	}
}

func TestFetchErrorDoesNotAdvanceRevision(t *testing.T) {
	r := New(testConfig(t, true), nil)
	r.fetchConfig = (&scriptedFetch{results: []fetchResult{
		{err: errors.New("dial timeout")},
	}}).fetch
	if _, err := r.newGatesPuller().PullOnce(context.Background()); err == nil {
		t.Fatal("fetch error must propagate")
	}
	if r.appliedGatesRevision.Load() != nil {
		t.Fatalf("failed fetch must not record revision, got %v", *r.appliedGatesRevision.Load())
	}
}
