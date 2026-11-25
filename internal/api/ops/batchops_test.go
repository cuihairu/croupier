package ops

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/batchop"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBatchOpCreate_Validation(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/b.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.BatchOp{}))
	svcCtx := &svc.ServiceContext{DB: db, RegistryStore: registry.NewStore()}
	s := NewService(svcCtx)

	_, err = s.BatchOpCreate(context.Background(), &batchop.CreateRequest{
		GameID: "g", Env: "e", Kind: "exec", AgentIDs: []string{"a"},
	})
	require.Error(t, err)

	op, err := s.BatchOpCreate(context.Background(), &batchop.CreateRequest{
		GameID: "g", Env: "e", Kind: "config", AgentIDs: []string{"a"},
		Config: &batchop.ConfigPayload{Path: "/etc/x", ContentBase64: base64.StdEncoding.EncodeToString([]byte("v")), Mode: 0o644},
	})
	require.NoError(t, err)
	assert.Equal(t, model.BatchOpStatusDraft, op.Status)
	assert.Equal(t, "g", op.GameID)
}

func TestBatchOpCreate_UnknownKind(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/b.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.BatchOp{}))
	svcCtx := &svc.ServiceContext{DB: db, RegistryStore: registry.NewStore()}
	s := NewService(svcCtx)
	_, err = s.BatchOpCreate(context.Background(), &batchop.CreateRequest{
		GameID: "g", Env: "e", Kind: "bogus", AgentIDs: []string{"a"},
		Exec: &batchop.ExecPayload{Command: "echo"},
	})
	require.Error(t, err)
}

// Audit finding: when the body supplies gameId/env, BatchOpCreateHandler
// overrides the authorized scope resolved by middleware, allowing batch
// creation/dispatch into unauthorized game environments. Regression: a scope
// mismatch must yield 403.
func TestBatchOpCreate_CrossScopeRejected_403(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/b.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.BatchOp{}))
	svcCtx := &svc.ServiceContext{DB: db, RegistryStore: registry.NewStore()}
	h := NewHandler(NewService(svcCtx))

	// Authenticated scope demo/prod, body supplies other/prod -> 403.
	body := `{"gameId":"other","env":"prod","kind":"config","agentIds":["a"],"config":{"path":"/etc/x","contentBase64":"djE=","mode":644}}`
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/batch", body)
	ctx.Request = ctx.Request.WithContext(svc.WithGameScope(ctx.Request.Context(),
		svc.GameScope{GameID: "demo", Env: "prod"}))
	h.BatchOpCreateHandler(ctx)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "scope_mismatch")
}
