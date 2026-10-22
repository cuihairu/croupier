package runner

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/filter"
)

// 业务规则迁出 YAML（CDC 平台化批阶段三）的优先链契约：文件规则只是
// fallback——启动生效，中心（config_versions capture.gates）已发布版本
// 拉取生效后整体覆盖；无中心版本时文件规则持续兜底。

func TestFileRulesFallbackOverriddenByCenterPublish(t *testing.T) {
	cfg := testConfig(t, true)
	cfg.Capture.Gates.Event.Ops = []string{"delete"} // 文件规则：只放行 delete
	r := New(cfg, nil)

	if gs := r.gates.Load(); gs == nil || gs.event == nil {
		t.Fatal("file event filter missing at startup")
	} else if got := gs.event.Evaluate("delete"); got != filter.ResultMatch {
		t.Fatalf("file rule Evaluate(delete) = %v, want match", got)
	}

	// 中心版本生效：操作白名单换成 insert——文件规则被整体覆盖。
	script := &scriptedFetch{results: []fetchResult{
		{version: "5", payload: []byte(`{"event":{"ops":["insert"]}}`)},
	}}
	r.fetchConfig = script.fetch
	if applied, err := r.newGatesPuller().PullOnce(context.Background()); err != nil || !applied {
		t.Fatalf("center pull = %v, %v; want applied", applied, err)
	}

	gs := r.gates.Load()
	if gs.event == nil {
		t.Fatal("center event filter missing after apply")
	}
	if got := gs.event.Evaluate("delete"); got != filter.ResultNoMatch {
		t.Fatalf("after center apply Evaluate(delete) = %v, want no-match (file rule overridden)", got)
	}
	if got := gs.event.Evaluate("insert"); got != filter.ResultMatch {
		t.Fatalf("after center apply Evaluate(insert) = %v, want match (center rule active)", got)
	}
	// appliedRevision 跟随中心版本：注册 label 反映中心发布态而非文件语义。
	if v := r.appliedGatesRevision.Load(); v == nil || *v != "5" {
		t.Fatalf("appliedGatesRevision = %v, want 5", r.appliedGatesRevision.Load())
	}
}

func TestFileRulesHoldWithoutCenterVersion(t *testing.T) {
	cfg := testConfig(t, true)
	cfg.Capture.Gates.Event.Ops = []string{"delete"}
	r := New(cfg, nil)

	// 服务端未发布（fetchConfig 语义：空版本 → nil 响应）→ 本轮无动作，
	// 文件规则继续兜底，appliedRevision 不记录。
	r.fetchConfig = (&scriptedFetch{results: []fetchResult{
		{nilResp: true},
	}}).fetch
	if applied, err := r.newGatesPuller().PullOnce(context.Background()); err != nil || applied {
		t.Fatalf("unpublished pull = %v, %v; want no-op", applied, err)
	}

	if gs := r.gates.Load(); gs == nil || gs.event == nil {
		t.Fatal("file event filter missing")
	} else if got := gs.event.Evaluate("delete"); got != filter.ResultMatch {
		t.Fatalf("file rule Evaluate(delete) = %v, want match (fallback holds)", got)
	}
	if r.appliedGatesRevision.Load() != nil {
		t.Fatalf("appliedGatesRevision = %v, want nil without center version", *r.appliedGatesRevision.Load())
	}
}
