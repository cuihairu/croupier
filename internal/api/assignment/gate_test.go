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

// 分配闸门（OPEN-ISSUES #36 / BUG-032）：assignments 数据此前只有页面 CRUD
// 消费，执行链路从不读取——「未分配也能调用」。闸门语义：
//   - scope（gameID|env）无分配记录 → 默认开放（向后兼容）
//   - 有记录 → 白名单：函数必须列表内，否则 403 function_not_assigned
func writeAssignments(t *testing.T, svcCtx *svc.ServiceContext, data string) {
	t.Helper()
	path := assignmentsPath(svcCtx)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))
}

func newGateSvcCtx(t *testing.T, dir string) *svc.ServiceContext {
	t.Helper()
	cfg := config.Config{}
	cfg.Registry.AssignmentsPath = filepath.Join(dir, "assignments.json")
	return &svc.ServiceContext{Config: cfg}
}

func TestEnsureFunctionAssigned_NoRecordOpenByDefault(t *testing.T) {
	svcCtx := newGateSvcCtx(t, t.TempDir()) // 不写文件 = 无任何分配记录
	err := EnsureFunctionAssigned(context.Background(), svcCtx, "demo.echo", "demo", "prod")
	assert.NoError(t, err, "scope 无分配记录应默认开放")
}

func TestEnsureFunctionAssigned_ListedFunctionPasses(t *testing.T) {
	svcCtx := newGateSvcCtx(t, t.TempDir())
	writeAssignments(t, svcCtx, `{"demo|prod":["demo.echo","demo.kick"]}`)
	assert.NoError(t, EnsureFunctionAssigned(context.Background(), svcCtx, "demo.echo", "demo", "prod"))
	assert.NoError(t, EnsureFunctionAssigned(context.Background(), svcCtx, "demo.kick", "demo", "prod"))
}

func TestEnsureFunctionAssigned_UnlistedFunctionDenied(t *testing.T) {
	svcCtx := newGateSvcCtx(t, t.TempDir())
	writeAssignments(t, svcCtx, `{"demo|prod":["demo.echo"]}`)
	err := EnsureFunctionAssigned(context.Background(), svcCtx, "demo.other", "demo", "prod")
	require.Error(t, err, "白名单 scope 内未分配函数必须拒绝，不能静默放行")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 403, codeErr.Code)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
}

func TestEnsureFunctionAssigned_EmptyListDeniesAll(t *testing.T) {
	// 空列表 = 白名单空集（Update 已约定清空保存即删除记录回默认开放，
	// 这里守住「存在空 key 不静默放行」的真值表另一半）
	svcCtx := newGateSvcCtx(t, t.TempDir())
	writeAssignments(t, svcCtx, `{"demo|prod":[]}`)
	err := EnsureFunctionAssigned(context.Background(), svcCtx, "demo.echo", "demo", "prod")
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
}

func TestEnsureFunctionAssigned_ScopeKeyCaseInsensitive(t *testing.T) {
	svcCtx := newGateSvcCtx(t, t.TempDir())
	writeAssignments(t, svcCtx, `{"Demo|PROD":["demo.echo"]}`)
	assert.NoError(t, EnsureFunctionAssigned(context.Background(), svcCtx, "demo.echo", "demo", "prod"))
	// 其他 scope 不受该记录影响（默认开放）
	assert.NoError(t, EnsureFunctionAssigned(context.Background(), svcCtx, "demo.echo", "demo", "dev"))
}
