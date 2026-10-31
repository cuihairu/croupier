package assignment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGateSvcCtx(t *testing.T, dir string) *svc.ServiceContext {
	t.Helper()
	cfg := config.Config{}
	cfg.Registry.AssignmentsPath = filepath.Join(dir, "assignments.json")
	return &svc.ServiceContext{Config: cfg}
}

func writeGateAssignments(t *testing.T, svcCtx *svc.ServiceContext, data string) {
	t.Helper()
	path := AssignmentsPath(svcCtx)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))
}

func TestDomainEnsureFunctionAssigned_NoRecordOpenByDefault(t *testing.T) {
	svcCtx := newGateSvcCtx(t, t.TempDir())
	assert.NoError(t, EnsureFunctionAssigned(context.Background(), svcCtx, "demo.echo", "demo", "prod"))
}

func TestDomainEnsureFunctionAssigned_ListedFunctionPasses(t *testing.T) {
	svcCtx := newGateSvcCtx(t, t.TempDir())
	writeGateAssignments(t, svcCtx, `{"demo|prod":["demo.echo","demo.kick"]}`)
	assert.NoError(t, EnsureFunctionAssigned(context.Background(), svcCtx, "demo.echo", "demo", "prod"))
	assert.NoError(t, EnsureFunctionAssigned(context.Background(), svcCtx, "demo.kick", "demo", "prod"))
}

func TestDomainEnsureFunctionAssigned_UnlistedDenied(t *testing.T) {
	svcCtx := newGateSvcCtx(t, t.TempDir())
	writeGateAssignments(t, svcCtx, `{"demo|prod":["demo.echo"]}`)
	err := EnsureFunctionAssigned(context.Background(), svcCtx, "demo.other", "demo", "prod")
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 403, codeErr.Code)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
}

func TestDomainEnsureFunctionAssigned_EmptyListDeniesAll(t *testing.T) {
	svcCtx := newGateSvcCtx(t, t.TempDir())
	writeGateAssignments(t, svcCtx, `{"demo|prod":[]}`)
	err := EnsureFunctionAssigned(context.Background(), svcCtx, "demo.echo", "demo", "prod")
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
}

func TestDomainEnsureFunctionAssigned_NilSvcCtxOpenByDefault(t *testing.T) {
	assert.NoError(t, EnsureFunctionAssigned(context.Background(), nil, "demo.echo", "demo", "prod"))
}
