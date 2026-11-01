package ops

import (
	"context"
	"encoding/base64"
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
