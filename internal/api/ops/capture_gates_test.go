package ops

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gosqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newCaptureGatesService：:memory: sqlite + config_versions 表（单连接防
// :memory: 多实例库漂移）。
func newCaptureGatesService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	db, err := gorm.Open(gosqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.ConfigVersion{}); err != nil {
		t.Fatalf("migrate config_versions: %v", err)
	}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	return NewService(&svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}), ctx
}

func validGatesContent() string {
	return `{"reconcile":{"rules":[{"name":"item-ledger","itemTable":"game.items","ledgerTable":"game.ledger","itemKey":"1","ledgerKey":"0","windowSec":60}]}}`
}

func TestCaptureGatesPutCreatesIncrementingVersions(t *testing.T) {
	s, ctx := newCaptureGatesService(t)

	resp, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent(), Message: "first publish"})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if resp.Version != 1 {
		t.Fatalf("first version = %d, want 1", resp.Version)
	}
	if resp2, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: `{}`}); err != nil {
		t.Fatalf("second put: %v", err)
	} else if resp2.Version != 2 {
		t.Fatalf("second version = %d, want 2", resp2.Version)
	}

	got, err := s.CaptureGatesGet(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Version != 2 || got.Content != `{}` {
		t.Fatalf("get = %+v, want latest version 2", got)
	}
	if got.Key != "capture.gates" || got.Namespace != "ops" {
		t.Fatalf("get = %+v, want capture.gates/ops", got)
	}
	if got.CreatedBy != "system" {
		t.Fatalf("createdBy = %q, want system (no ctx username)", got.CreatedBy)
	}
}

func TestCaptureGatesGetEmptyWhenUnpublished(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	got, err := s.CaptureGatesGet(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Version != 0 || got.Content != "" || got.Key != "capture.gates" {
		t.Fatalf("get = %+v, want unpublished empty shape", got)
	}
}

func TestCaptureGatesGetIsScopeIsolated(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("put: %v", err)
	}
	other := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "dev"})
	got, err := s.CaptureGatesGet(other)
	if err != nil {
		t.Fatalf("get other env: %v", err)
	}
	if got.Version != 0 {
		t.Fatalf("other env get = %+v, want empty", got)
	}
}

func TestCaptureGatesPutValidation(t *testing.T) {
	s, ctx := newCaptureGatesService(t)

	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: "  "}); err == nil {
		t.Fatal("empty content must be rejected")
	}
	noScope := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo"})
	if _, err := s.CaptureGatesPut(noScope, &CaptureGatesPutRequest{Content: `{}`}); err == nil {
		t.Fatal("missing env scope must be rejected")
	}
	for name, content := range map[string]string{
		"not json":        `{reconcile:`,
		"top not object":  `[]`,
		"section not obj": `{"reconcile":[]}`,
		"rules not array": `{"reconcile":{"rules":{}}}`,
		"cep not array":   `{"cep":{"burst":"x"}}`,
		"event not obj":   `{"event":"x"}`,
		"ops not array":   `{"event":{"ops":"insert"}}`,
	} {
		if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: content}); err == nil {
			t.Fatalf("%s must be rejected", name)
		} else if code := httpCode(err); code != 422 {
			t.Fatalf("%s: error code = %d, want 422", name, code)
		}
	}
	// lint 失败不得落版本。
	if got, err := s.CaptureGatesGet(ctx); err != nil || got.Version != 0 {
		t.Fatalf("post-lint-failure get = %+v, %v; want unpublished", got, err)
	}
	// 未知键放行（agent 侧前向扩展不需要改面板）。
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: `{"future":{"x":1}}`}); err != nil {
		t.Fatalf("unknown section must pass lint: %v", err)
	}
	// event 段合法形状放行（操作白名单，阶段二 EventFilterPolicy）。
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: `{"event":{"ops":["insert","delete"]}}`}); err != nil {
		t.Fatalf("event section must pass lint: %v", err)
	}
}

func TestCaptureGatesRollbackRepublishesHistoricalContent(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent(), Message: "first"}); err != nil {
		t.Fatalf("put v1: %v", err)
	}
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: `{}`, Message: "second"}); err != nil {
		t.Fatalf("put v2: %v", err)
	}

	// 回滚到 v1：新版本号 3，内容=v1，版本日志不可变（v1/v2 仍在）。
	resp, err := s.CaptureGatesRollback(ctx, &CaptureGatesRollbackRequest{Version: 1, Message: "undo burst change"})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if resp.Version != 3 {
		t.Fatalf("rollback version = %d, want 3", resp.Version)
	}
	got, err := s.CaptureGatesGet(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Version != 3 || got.Content != validGatesContent() {
		t.Fatalf("get = %+v, want v3 with v1 content", got)
	}
	rec, err := s.svcCtx.ConfigVersionModel.Find(ctx, "capture.gates", 3)
	if err != nil {
		t.Fatalf("find v3: %v", err)
	}
	if rec.Message != "rollback from version 1: undo burst change" {
		t.Fatalf("message = %q, want rollback origin + reason", rec.Message)
	}
	hist, err := s.svcCtx.ConfigVersionModel.Find(ctx, "capture.gates", 2)
	if err != nil || hist.Value != `{}` {
		t.Fatalf("history v2 = %+v, %v; want intact", hist, err)
	}

	// 回滚当前版本合法（重发同内容=显式重新生效），版本继续递增。
	if resp, err := s.CaptureGatesRollback(ctx, &CaptureGatesRollbackRequest{Version: 3}); err != nil {
		t.Fatalf("rollback current: %v", err)
	} else if resp.Version != 4 {
		t.Fatalf("rollback current version = %d, want 4", resp.Version)
	}
}

func TestCaptureGatesRollbackValidation(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("put: %v", err)
	}

	// 版本号非法 → 422。
	if _, err := s.CaptureGatesRollback(ctx, &CaptureGatesRollbackRequest{Version: 0}); err == nil {
		t.Fatal("version 0 must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("version 0: code = %d, want 422", code)
	}
	// 不存在的版本 → 404。
	if _, err := s.CaptureGatesRollback(ctx, &CaptureGatesRollbackRequest{Version: 9}); err == nil {
		t.Fatal("unknown version must be rejected")
	} else if code := httpCode(err); code != 404 {
		t.Fatalf("unknown version: code = %d, want 404", code)
	}
	// scope 隔离：其他 env 拿不到 demo/prod 的版本（404 而非泄漏内容）。
	other := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "dev"})
	if _, err := s.CaptureGatesRollback(other, &CaptureGatesRollbackRequest{Version: 1}); err == nil {
		t.Fatal("cross-scope rollback must be rejected")
	} else if code := httpCode(err); code != 404 {
		t.Fatalf("cross-scope rollback: code = %d, want 404", code)
	}
	// 缺 env scope → 422。
	noScope := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo"})
	if _, err := s.CaptureGatesRollback(noScope, &CaptureGatesRollbackRequest{Version: 1}); err == nil {
		t.Fatal("missing env scope must be rejected")
	}
}

func TestCaptureGatesRollbackRejectsContentFailingCurrentLint(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	// 模拟 lint 规则演进后的存量坏行：直插 model 绕过服务层 lint。
	if _, err := s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "capture.gates", Content: `{"reconcile":`, Format: "json",
		GameID: "demo", Env: "prod", Namespace: model.ConfigNamespaceOps,
	}, "legacy"); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("put v2: %v", err)
	}

	// 回滚到坏内容 → 422，且不落新版本（latest 停在 v2）。
	if _, err := s.CaptureGatesRollback(ctx, &CaptureGatesRollbackRequest{Version: 1}); err == nil {
		t.Fatal("rollback to content failing current lint must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("stale lint content: code = %d, want 422", code)
	}
	if got, err := s.CaptureGatesGet(ctx); err != nil || got.Version != 2 {
		t.Fatalf("get after failed rollback = %+v, %v; want latest still v2", got, err)
	}
}

// httpCode 取 errorx 错误的 HTTP 语义码（CodeError.Data 首元）。
func httpCode(err error) int {
	var codeErr *errorx.CodeError
	if errors.As(err, &codeErr) {
		if code, _ := codeErr.Data(); code != 0 {
			return code
		}
	}
	var valErr *errorx.ValidationError
	if errors.As(err, &valErr) {
		if code, _ := valErr.Data(); code != 0 {
			return code
		}
	}
	return 0
}
