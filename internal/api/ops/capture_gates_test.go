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

// newCaptureGatesService: :memory: sqlite + config_versions table (single
// connection guards against :memory: multi-instance database drift).
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
	// A lint failure must not land a version.
	if got, err := s.CaptureGatesGet(ctx); err != nil || got.Version != 0 {
		t.Fatalf("post-lint-failure get = %+v, %v; want unpublished", got, err)
	}
	// Unknown keys are allowed (forward extension on the agent side should not
	// require panel changes).
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: `{"future":{"x":1}}`}); err != nil {
		t.Fatalf("unknown section must pass lint: %v", err)
	}
	// A well-shaped event section is allowed (operation allowlist,
	// EventFilterPolicy in phase two).
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

	// Rollback to v1: new version number 3 with v1 content; the version log is
	// immutable (v1/v2 remain).
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

	// Rolling back the current version is legal (republishing the same content =
	// explicitly re-activating it); the version keeps incrementing.
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

	// Invalid version number -> 422.
	if _, err := s.CaptureGatesRollback(ctx, &CaptureGatesRollbackRequest{Version: 0}); err == nil {
		t.Fatal("version 0 must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("version 0: code = %d, want 422", code)
	}
	// Non-existent version -> 404.
	if _, err := s.CaptureGatesRollback(ctx, &CaptureGatesRollbackRequest{Version: 9}); err == nil {
		t.Fatal("unknown version must be rejected")
	} else if code := httpCode(err); code != 404 {
		t.Fatalf("unknown version: code = %d, want 404", code)
	}
	// Scope isolation: other envs cannot fetch demo/prod versions (404 instead
	// of leaking content).
	other := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "dev"})
	if _, err := s.CaptureGatesRollback(other, &CaptureGatesRollbackRequest{Version: 1}); err == nil {
		t.Fatal("cross-scope rollback must be rejected")
	} else if code := httpCode(err); code != 404 {
		t.Fatalf("cross-scope rollback: code = %d, want 404", code)
	}
	// Missing env scope -> 422.
	noScope := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo"})
	if _, err := s.CaptureGatesRollback(noScope, &CaptureGatesRollbackRequest{Version: 1}); err == nil {
		t.Fatal("missing env scope must be rejected")
	}
}

func TestCaptureGatesRollbackRejectsContentFailingCurrentLint(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	// Simulates a pre-existing bad row left behind after lint rules evolved:
	// insert directly through the model to bypass service-layer lint.
	if _, err := s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "capture.gates", Content: `{"reconcile":`, Format: "json",
		GameID: "demo", Env: "prod", Namespace: model.ConfigNamespaceOps,
	}, "legacy"); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("put v2: %v", err)
	}

	// Rollback to bad content -> 422, and no new version lands (latest stays at
	// v2).
	if _, err := s.CaptureGatesRollback(ctx, &CaptureGatesRollbackRequest{Version: 1}); err == nil {
		t.Fatal("rollback to content failing current lint must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("stale lint content: code = %d, want 422", code)
	}
	if got, err := s.CaptureGatesGet(ctx); err != nil || got.Version != 2 {
		t.Fatalf("get after failed rollback = %+v, %v; want latest still v2", got, err)
	}
}

func TestCaptureGatesPutOptimisticLock(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("put v1: %v", err)
	}
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: `{}`, BaseVersion: 1}); err != nil {
		t.Fatalf("put v2 with base 1: %v", err)
	}

	// Stale base (editor still on v1) -> 409.
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent(), BaseVersion: 1}); err == nil {
		t.Fatal("stale base version must conflict")
	} else if code := httpCode(err); code != 409 {
		t.Fatalf("stale base: code = %d, want 409", code)
	}
	// Correct base -> increment succeeds.
	resp, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: `{"event":{"ops":["insert"]}}`, BaseVersion: 2})
	if err != nil {
		t.Fatalf("put v3 with base 2: %v", err)
	}
	if resp.Version != 3 {
		t.Fatalf("version = %d, want 3", resp.Version)
	}
	// Default base (0) = lock disabled, backward compatible with existing
	// callers.
	if resp, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: `{}`}); err != nil {
		t.Fatalf("put without base: %v", err)
	} else if resp.Version != 4 {
		t.Fatalf("version = %d, want 4", resp.Version)
	}
	// Base specified against a non-existent version chain (first publish with
	// baseVersion) -> 409.
	fresh := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "dev"})
	if _, err := s.CaptureGatesPut(fresh, &CaptureGatesPutRequest{Content: `{}`, BaseVersion: 1}); err == nil {
		t.Fatal("base on empty chain must conflict")
	} else if code := httpCode(err); code != 409 {
		t.Fatalf("base on empty chain: code = %d, want 409", code)
	}

	// Conflicts land no version: latest remains v4.
	if got, err := s.CaptureGatesGet(ctx); err != nil || got.Version != 4 {
		t.Fatalf("get after conflicts = %+v, %v; want v4", got, err)
	}
}

func TestCaptureGatesVersionsList(t *testing.T) {
	s, ctx := newCaptureGatesService(t)

	// Unpublished -> empty array (normal state, not 404).
	items, err := s.CaptureGatesVersionsGet(ctx)
	if err != nil {
		t.Fatalf("versions before publish: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("versions = %+v, want empty", items)
	}

	// Three versions (v1 published / v2 published / v3 = rollback of v1) ->
	// descending order + metadata.
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent(), Message: "first"}); err != nil {
		t.Fatalf("put v1: %v", err)
	}
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: `{}`, Message: "second"}); err != nil {
		t.Fatalf("put v2: %v", err)
	}
	if _, err := s.CaptureGatesRollback(ctx, &CaptureGatesRollbackRequest{Version: 1}); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	items, err = s.CaptureGatesVersionsGet(ctx)
	if err != nil {
		t.Fatalf("versions: %v", err)
	}
	if len(items) != 3 || items[0].Version != 3 || items[2].Version != 1 {
		t.Fatalf("versions = %+v, want [3 2 1]", items)
	}
	if items[0].Message != "rollback from version 1" {
		t.Fatalf("v3 message = %q, want rollback origin", items[0].Message)
	}
	if items[2].Message != "first" || items[2].CreatedBy != "system" || items[2].UpdatedAt == "" {
		t.Fatalf("v1 item = %+v, want message/author/time", items[2])
	}

	// Scope isolation: the other env has no history.
	other := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "dev"})
	if items, err := s.CaptureGatesVersionsGet(other); err != nil || len(items) != 0 {
		t.Fatalf("other env versions = %+v, %v; want empty", items, err)
	}
	// Missing env scope -> 422.
	noScope := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo"})
	if _, err := s.CaptureGatesVersionsGet(noScope); err == nil {
		t.Fatal("missing env scope must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("missing scope: code = %d, want 422", code)
	}
}

// httpCode extracts the HTTP semantic code of an errorx error (first element
// of CodeError.Data).
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

func TestCaptureGatesValidate(t *testing.T) {
	s, _ := newCaptureGatesService(t)

	// Empty content -> 422.
	if _, err := s.CaptureGatesValidate(nil); err == nil {
		t.Fatal("nil request must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("nil request: code = %d, want 422", code)
	}

	// Valid content -> valid + per-gate rule count preview.
	resp, err := s.CaptureGatesValidate(&CaptureGatesValidateRequest{Content: validGatesContent()})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !resp.Valid || resp.Reconcile != 1 || resp.CepOffHours != 0 || resp.Lineage != 0 {
		t.Fatalf("unexpected summary: %+v", resp)
	}

	// Shallow lint failure (non-object) -> 422.
	if _, err := s.CaptureGatesValidate(&CaptureGatesValidateRequest{Content: `[1,2]`}); err == nil {
		t.Fatal("non-object content must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("lint failure: code = %d, want 422", code)
	}

	// Deep validation failure (lint passes, agent apply must reject) -> 422.
	badTable := `{"reconcile":{"rules":[{"name":"x","itemTable":"bad","ledgerTable":"game.ledger","itemKey":"1","ledgerKey":"0"}]}}`
	if _, err := s.CaptureGatesValidate(&CaptureGatesValidateRequest{Content: badTable}); err == nil {
		t.Fatal("bad itemTable must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("deep failure: code = %d, want 422", code)
	}

	// Construction-time check failure (static validation passes, table routing
	// uniqueness blocks) -> 422.
	dup := `{"reconcile":{"rules":[
		{"name":"a","itemTable":"game.item","ledgerTable":"game.ledger","itemKey":"1","ledgerKey":"0"},
		{"name":"b","itemTable":"game.item","ledgerTable":"game.ledger2","itemKey":"1","ledgerKey":"0"}]}}`
	if _, err := s.CaptureGatesValidate(&CaptureGatesValidateRequest{Content: dup}); err == nil {
		t.Fatal("duplicate table routing must be rejected")
	} else if code := httpCode(err); code != 422 {
		t.Fatalf("duplicate routing: code = %d, want 422", code)
	}

	// Validation passes but nothing lands in the database (version count
	// unchanged — dry-run has no side effects).
	if _, err := s.CaptureGatesValidate(&CaptureGatesValidateRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("re-validate: %v", err)
	}
	got, err := s.CaptureGatesGet(context.Background())
	if err == nil && got.Version != 0 {
		t.Fatalf("validate must not persist, version = %d, want 0", got.Version)
	}
}
