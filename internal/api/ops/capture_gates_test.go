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
