package svc

import (
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
)

func TestDemoApprovalScopes_PrefersDefaultGame(t *testing.T) {
	bindings := []model.GameEnvBinding{
		{GameID: "default", Env: "prod"},
		{GameID: "default", Env: "dev"},
		{GameID: "other", Env: "prod"},
		{GameID: "other", Env: "stage"},
	}
	scopes := demoApprovalScopes(bindings)
	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes (default game only), got %d: %+v", len(scopes), scopes)
	}
	seen := map[string]bool{}
	for _, s := range scopes {
		if s.GameID != "default" {
			t.Fatalf("unexpected game %q in scopes", s.GameID)
		}
		seen[s.Env] = true
	}
	if !seen["prod"] || !seen["dev"] {
		t.Fatalf("expected default game's prod/dev envs, got %+v", scopes)
	}
}

func TestDemoApprovalScopes_FallsBackToFirstGame(t *testing.T) {
	// 无 default 游戏时取 (game_id, env) 排序第一的游戏，与
	// resolveFirstAuthorizedGame 的首个授权 scope 对齐。
	bindings := []model.GameEnvBinding{
		{GameID: "beta", Env: "prod"},
		{GameID: "alpha", Env: "stage"},
		{GameID: "alpha", Env: "dev"},
	}
	scopes := demoApprovalScopes(bindings)
	if len(scopes) != 2 {
		t.Fatalf("expected alpha's 2 envs, got %d: %+v", len(scopes), scopes)
	}
	for _, s := range scopes {
		if s.GameID != "alpha" {
			t.Fatalf("expected first game alpha, got %q", s.GameID)
		}
	}
}

func TestDemoApprovalScopes_EmptyBindingsFallback(t *testing.T) {
	scopes := demoApprovalScopes(nil)
	if len(scopes) != len(fallbackDefaultEnvs) {
		t.Fatalf("expected %d fallback scopes, got %d", len(fallbackDefaultEnvs), len(scopes))
	}
	for _, s := range scopes {
		if s.GameID != "default" {
			t.Fatalf("fallback scope game should be default, got %q", s.GameID)
		}
	}
}

func TestSeedDemoApprovalsInto_StatesAndTwoPerson(t *testing.T) {
	store := approvals.NewMemStore()
	scopes := []GameScope{{GameID: "default", Env: "dev"}}
	created := seedDemoApprovalsInto(store, scopes, time.Now())
	if created != len(demoApprovalFixtures) {
		t.Fatalf("expected %d records, created %d", len(demoApprovalFixtures), created)
	}

	list, total, err := store.List(approvals.Filter{GameID: "default", Env: "dev"}, approvals.Page{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != len(demoApprovalFixtures) {
		t.Fatalf("expected total %d, got %d", len(demoApprovalFixtures), total)
	}

	countByState := map[string]int{}
	for _, a := range list {
		countByState[a.State]++
		switch a.State {
		case "pending":
			// 两人复核演示：申请人不得是演示账号 admin
			if a.Actor == "admin" {
				t.Errorf("pending %s actor should not be admin", a.ID)
			}
			if a.Approver != "" || a.ReviewedAt != nil {
				t.Errorf("pending %s should have no approver/reviewedAt", a.ID)
			}
			if a.Reason != "" {
				t.Errorf("pending %s should have no reason", a.ID)
			}
		case "approved", "rejected":
			if a.Approver == "" {
				t.Errorf("reviewed %s missing approver", a.ID)
			}
			if a.ReviewedAt == nil || a.ReviewedAt.Before(a.CreatedAt) {
				t.Errorf("reviewed %s reviewedAt must be set and after createdAt", a.ID)
			}
			if a.State == "rejected" && a.Reason == "" {
				t.Errorf("rejected %s missing reason", a.ID)
			}
		default:
			t.Errorf("unexpected state %q", a.State)
		}
		if a.GameID != "default" || a.Env != "dev" {
			t.Errorf("record %s has wrong scope %s/%s", a.ID, a.GameID, a.Env)
		}
		if len(a.Payload) == 0 || a.FunctionID == "" || a.Route == "" {
			t.Errorf("record %s missing function payload fields", a.ID)
		}
	}
	if countByState["pending"] != 2 || countByState["approved"] != 2 || countByState["rejected"] != 1 {
		t.Fatalf("expected 2 pending / 2 approved / 1 rejected, got %+v", countByState)
	}

	// 跨 scope ID 不碰撞：第二个 scope 全量写入成功
	created2 := seedDemoApprovalsInto(store, []GameScope{{GameID: "default", Env: "prod"}}, time.Now())
	if created2 != len(demoApprovalFixtures) {
		t.Fatalf("second scope should seed %d records, got %d", len(demoApprovalFixtures), created2)
	}
	prodList, prodTotal, err := store.List(approvals.Filter{GameID: "default", Env: "prod"}, approvals.Page{})
	if err != nil {
		t.Fatalf("list prod: %v", err)
	}
	if prodTotal != len(demoApprovalFixtures) || len(prodList) != len(demoApprovalFixtures) {
		t.Fatalf("expected prod scope isolated, got total=%d len=%d", prodTotal, len(prodList))
	}
}

func TestSeedDemoApprovals_GateGuards(t *testing.T) {
	// 非开发配置：直接跳过，不触碰 store
	prodCtx := &ServiceContext{Config: config.Config{}}
	prodCtx.Config.Server.Mode = "prod"
	// isDevelopmentConfig 仅在 CROUPIER_MODE 显式 prod 时判否
	t.Setenv("CROUPIER_MODE", "prod")
	seedDemoApprovals(prodCtx) // store/gameModel 为 nil，不得 panic

	store := approvals.NewMemStore()
	devCtx := &ServiceContext{Config: config.Config{}, ApprovalsStore: store}
	seedDemoApprovals(devCtx) // GameModel 为 nil，跳过
	list, total, err := store.List(approvals.Filter{}, approvals.Page{})
	if err != nil || total != 0 || len(list) != 0 {
		t.Fatalf("nil GameModel should skip seeding, got total=%d err=%v", total, err)
	}
}
