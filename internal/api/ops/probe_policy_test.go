package ops

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

func validPolicyContent() string {
	return `{"interval":"30s","targets":[{"target":"db-primary","interval":"10s"}],"windows":[{"start":"19:00","end":"23:00","interval":"8s"}]}`
}

// newProbePolicyService 复用 capture gates 的 harness（同构：:memory: sqlite +
// config_versions 表 + demo/prod scope ctx）。
func newProbePolicyService(t *testing.T) (*Service, context.Context) {
	return newCaptureGatesService(t)
}

func TestProbePolicyPutCreatesIncrementingVersions(t *testing.T) {
	s, ctx := newProbePolicyService(t)

	resp, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: validPolicyContent(), Message: "first publish"})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if resp.Version != 1 {
		t.Fatalf("first version = %d, want 1", resp.Version)
	}
	if resp2, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: `{"interval":"60s"}`}); err != nil {
		t.Fatalf("second put: %v", err)
	} else if resp2.Version != 2 {
		t.Fatalf("second version = %d, want 2", resp2.Version)
	}

	got, err := s.ProbePolicyGet(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Version != 2 || got.Content != `{"interval":"60s"}` {
		t.Fatalf("get = %+v, want latest version 2", got)
	}
	if got.Key != "probe.policy" || got.Namespace != "ops" {
		t.Fatalf("get = %+v, want probe.policy/ops", got)
	}
	if got.CreatedBy != "system" {
		t.Fatalf("createdBy = %q, want system (no ctx username)", got.CreatedBy)
	}
}

func TestProbePolicyGetEmptyWhenUnpublished(t *testing.T) {
	s, ctx := newProbePolicyService(t)
	got, err := s.ProbePolicyGet(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Version != 0 || got.Content != "" || got.Key != "probe.policy" {
		t.Fatalf("get = %+v, want unpublished empty shape", got)
	}
}

func TestProbePolicyPutValidation(t *testing.T) {
	s, ctx := newProbePolicyService(t)

	if _, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: "  "}); err == nil {
		t.Fatal("empty content must be rejected")
	}
	noScope := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo"})
	if _, err := s.ProbePolicyPut(noScope, &ProbePolicyPutRequest{Content: `{}`}); err == nil {
		t.Fatal("missing env scope must be rejected")
	}
	for name, content := range map[string]string{
		"not json":       `{interval:`,
		"top not object": `[]`,
		"interval 非法":    `{"interval":"abc"}`,
		"interval 非正":    `{"interval":"0s"}`,
		"target 重名":      `{"targets":[{"target":"a","interval":"1s"},{"target":"a","interval":"2s"}]}`,
		"窗口钟点非法":         `{"windows":[{"start":"19:00","end":"xx:00","interval":"8s"}]}`,
		"窗口跨午夜":          `{"windows":[{"start":"23:00","end":"01:00","interval":"8s"}]}`,
		"event 缺偏移":      `{"events":[{"at":"2026-10-20T09:00:00","duration":"30m","interval":"5s"}]}`,
		"event 时长非正":     `{"events":[{"at":"2026-10-20T09:00:00+08:00","duration":"0","interval":"5s"}]}`,
	} {
		if _, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: content}); err == nil {
			t.Fatalf("%s must be rejected", name)
		} else if code := httpCode(err); code != 422 {
			t.Fatalf("%s: error code = %d, want 422", name, code)
		}
	}
	// 校验失败不得落版本。
	if got, err := s.ProbePolicyGet(ctx); err != nil || got.Version != 0 {
		t.Fatalf("post-validation-failure get = %+v, %v; want unpublished", got, err)
	}
	// 未知键放行（前向扩展不需要改面板）。
	if _, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: `{"interval":"30s","future":{"x":1}}`}); err != nil {
		t.Fatalf("unknown key must pass: %v", err)
	}
}

func TestProbePolicyValidate(t *testing.T) {
	s, _ := newProbePolicyService(t)

	if _, err := s.ProbePolicyValidate(nil); err == nil {
		t.Fatal("nil request must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("nil request: code = %d, want 422", code)
	}

	resp, err := s.ProbePolicyValidate(&ProbePolicyValidateRequest{Content: validPolicyContent()})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !resp.Valid || resp.BaseInterval != "30s" || resp.Targets != 1 || resp.Windows != 1 || resp.Events != 0 || !resp.AutoBoostOn {
		t.Fatalf("unexpected summary: %+v", resp)
	}

	if _, err := s.ProbePolicyValidate(&ProbePolicyValidateRequest{Content: `{"interval":"abc"}`}); err == nil {
		t.Fatal("invalid content must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("validate failure: code = %d, want 422", code)
	}
	// autoBoost=false 预览。
	resp, err = s.ProbePolicyValidate(&ProbePolicyValidateRequest{Content: `{"autoBoost":false}`})
	if err != nil || resp.AutoBoostOn {
		t.Fatalf("autoBoost=false summary = %+v, %v", resp, err)
	}
	// dry-run 无副作用。
	got, err := s.ProbePolicyGet(context.Background())
	if err == nil && got.Version != 0 {
		t.Fatalf("validate must not persist, version = %d", got.Version)
	}
}

func TestProbePolicyRollbackAndVersions(t *testing.T) {
	s, ctx := newProbePolicyService(t)
	if _, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: validPolicyContent(), Message: "first"}); err != nil {
		t.Fatalf("put v1: %v", err)
	}
	if _, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: `{"interval":"60s"}`, Message: "second"}); err != nil {
		t.Fatalf("put v2: %v", err)
	}

	// 回滚到 v1：新版本号 3，内容=v1，版本日志不可变。
	resp, err := s.ProbePolicyRollback(ctx, &ProbePolicyRollbackRequest{Version: 1, Message: "undo"})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if resp.Version != 3 {
		t.Fatalf("rollback version = %d, want 3", resp.Version)
	}
	rec, err := s.svcCtx.ConfigVersionModel.Find(ctx, "probe.policy", 3)
	if err != nil {
		t.Fatalf("find v3: %v", err)
	}
	if rec.Message != "rollback from version 1: undo" {
		t.Fatalf("message = %q", rec.Message)
	}

	// 不存在的版本 → 404；版本号非法 → 422。
	if _, err := s.ProbePolicyRollback(ctx, &ProbePolicyRollbackRequest{Version: 9}); err == nil {
		t.Fatal("unknown version must be rejected")
	} else if code := httpCode(err); code != 404 {
		t.Fatalf("unknown version: code = %d, want 404", code)
	}
	if _, err := s.ProbePolicyRollback(ctx, &ProbePolicyRollbackRequest{Version: 0}); err == nil {
		t.Fatal("version 0 must be rejected")
	}

	// 版本历史倒序。
	items, err := s.ProbePolicyVersionsGet(ctx)
	if err != nil {
		t.Fatalf("versions: %v", err)
	}
	if len(items) != 3 || items[0].Version != 3 || items[2].Version != 1 {
		t.Fatalf("versions = %+v, want [3 2 1]", items)
	}
	// scope 隔离：其他 env 无历史。
	other := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "dev"})
	if items, err := s.ProbePolicyVersionsGet(other); err != nil || len(items) != 0 {
		t.Fatalf("other env versions = %+v, %v; want empty", items, err)
	}
}

func TestProbePolicyRollbackRejectsContentFailingCurrentValidation(t *testing.T) {
	s, ctx := newProbePolicyService(t)
	// 模拟校验规则演进后的存量坏行：直插 model 绕过服务层校验。
	if _, err := s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "probe.policy", Content: `{"interval":`, Format: "json",
		GameID: "demo", Env: "prod", Namespace: model.ConfigNamespaceOps,
	}, "legacy"); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if _, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: validPolicyContent()}); err != nil {
		t.Fatalf("put v2: %v", err)
	}

	if _, err := s.ProbePolicyRollback(ctx, &ProbePolicyRollbackRequest{Version: 1}); err == nil {
		t.Fatal("rollback to content failing current validation must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("stale content: code = %d, want 422", code)
	}
	if got, err := s.ProbePolicyGet(ctx); err != nil || got.Version != 2 {
		t.Fatalf("get after failed rollback = %+v, %v; want latest still v2", got, err)
	}
}

func TestProbePolicyPutOptimisticLock(t *testing.T) {
	s, ctx := newProbePolicyService(t)
	if _, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: validPolicyContent()}); err != nil {
		t.Fatalf("put v1: %v", err)
	}
	if _, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: `{"interval":"60s"}`, BaseVersion: 1}); err != nil {
		t.Fatalf("put v2 with base 1: %v", err)
	}
	// 基线过期 → 409。
	if _, err := s.ProbePolicyPut(ctx, &ProbePolicyPutRequest{Content: validPolicyContent(), BaseVersion: 1}); err == nil {
		t.Fatal("stale base version must conflict")
	} else if code := httpCode(err); code != 409 {
		t.Fatalf("stale base: code = %d, want 409", code)
	}
	// 冲突不落版本：latest 仍是 v2。
	if got, err := s.ProbePolicyGet(ctx); err != nil || got.Version != 2 {
		t.Fatalf("get after conflict = %+v, %v; want v2", got, err)
	}
}
