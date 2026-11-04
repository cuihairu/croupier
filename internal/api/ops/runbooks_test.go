package ops

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/batchop"
	"github.com/cuihairu/croupier/internal/ops/runbook"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newRunbookTestService(t *testing.T) (*Service, *svc.ServiceContext) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/rb.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Runbook{}, &model.RunbookRun{}, &model.BatchOp{},
		&model.DeployPackage{}, &model.Deployment{}))
	svcCtx := &svc.ServiceContext{DB: db, RegistryStore: registry.NewStore()}
	return NewService(svcCtx), svcCtx
}

func execStepPayload(command string) *batchop.ExecPayload {
	return &batchop.ExecPayload{Command: command}
}

// TestRunbookService_CreateAndList 定义→落库→按 scope 列表。
func TestRunbookService_CreateAndList(t *testing.T) {
	s, _ := newRunbookTestService(t)
	ctx := context.Background()

	rb, err := s.RunbookCreate(ctx, &runbook.DefineRequest{
		Name: "restart fleet", GameID: "demo", Env: "prod",
		Steps: []runbook.StepSpec{{
			Name: "restart", Kind: model.RunbookStepBatchExec,
			Exec:   execStepPayload("systemctl restart game"),
			Labels: map[string]string{"role": "game"},
		}},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, rb.RunbookID)
	assert.Equal(t, "demo", rb.GameID)

	items, err := s.RunbookList(ctx, "demo", "prod", 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, rb.RunbookID, items[0].RunbookID)

	// scope 隔离
	items, err = s.RunbookList(ctx, "other", "prod", 10)
	require.NoError(t, err)
	assert.Empty(t, items)

	// 校验拒绝
	_, err = s.RunbookCreate(ctx, &runbook.DefineRequest{Name: "x", GameID: "demo", Env: "prod", Steps: nil})
	require.Error(t, err)
}

// TestRunbookService_NotInitialized 无 DB 时如实报错。
func TestRunbookService_NotInitialized(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	_, err := s.RunbookCreate(context.Background(), &runbook.DefineRequest{
		Name: "x", GameID: "g", Env: "e",
		Steps: []runbook.StepSpec{{Name: "s", Kind: model.RunbookStepDeployStart, DeploymentID: "d1"}},
	})
	require.Error(t, err)

	err = s.RunbookDelete(context.Background(), "g", "e", "rb")
	require.Error(t, err)
}

// TestRunbookService_Run_WithRealBatch 端到端：batch_exec 步骤经真实 OPS-006
// batchop 编排（注册表空 → 0 目标 → completed）→ run succeeded，批次已创建。
func TestRunbookService_Run_WithRealBatch(t *testing.T) {
	s, _ := newRunbookTestService(t)
	ctx := context.Background()

	rb, err := s.RunbookCreate(ctx, &runbook.DefineRequest{
		Name: "rollout", GameID: "demo", Env: "prod",
		Steps: []runbook.StepSpec{{
			Name: "restart", Kind: model.RunbookStepBatchExec,
			Exec:   execStepPayload("echo"),
			Labels: map[string]string{"role": "game"},
		}},
	})
	require.NoError(t, err)

	res, err := s.RunbookRun(ctx, "demo", "prod", rb.RunbookID, nil, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusSucceeded, res.Status)
	require.Len(t, res.Steps, 1)
	assert.Equal(t, "success", res.Steps[0].Status)

	// run 落库可回查
	run, err := s.RunbookRunGet(ctx, "demo", "prod", res.RunID)
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusSucceeded, run.Status)

	// OPS-006 能力被引用：批次记录已创建
	batches, err := s.BatchOpList(ctx, "demo", "prod", 10)
	require.NoError(t, err)
	require.Len(t, batches, 1)
}

// TestRunbookService_Run_ParamSubstitution 端到端参数替换：batch_exec 命令引用
// {{cmd}}，运行后创建的批次载荷里命令已是替换后的真实值。
func TestRunbookService_Run_ParamSubstitution(t *testing.T) {
	s, _ := newRunbookTestService(t)
	ctx := context.Background()

	rb, err := s.RunbookCreate(ctx, &runbook.DefineRequest{
		Name: "restart", GameID: "demo", Env: "prod",
		Steps: []runbook.StepSpec{{
			Name: "restart", Kind: model.RunbookStepBatchExec,
			Exec:   execStepPayload("{{cmd}}"),
			Labels: map[string]string{"role": "game"},
		}},
	})
	require.NoError(t, err)

	res, err := s.RunbookRun(ctx, "demo", "prod", rb.RunbookID,
		map[string]string{"cmd": "systemctl restart game"}, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.RunbookRunStatusSucceeded, res.Status)

	// 批次载荷已是替换后的命令（OPS-006 能力收到参数化值）
	batches, err := s.BatchOpList(ctx, "demo", "prod", 10)
	require.NoError(t, err)
	require.Len(t, batches, 1)
	assert.Contains(t, batches[0].PayloadJSON, "systemctl restart game")
	assert.NotContains(t, batches[0].PayloadJSON, "{{cmd}}", "placeholder must be substituted before dispatch")
}

// TestRunbookService_RunList runs 列表按 runbook 过滤。
func TestRunbookService_RunList(t *testing.T) {
	s, _ := newRunbookTestService(t)
	ctx := context.Background()

	rb1, err := s.RunbookCreate(ctx, &runbook.DefineRequest{
		Name: "a", GameID: "demo", Env: "prod",
		Steps: []runbook.StepSpec{{
			Name: "start", Kind: model.RunbookStepDeployStart, DeploymentID: "{{deployment_id}}",
		}},
	})
	require.NoError(t, err)
	rb2, err := s.RunbookCreate(ctx, &runbook.DefineRequest{
		Name: "b", GameID: "demo", Env: "prod",
		Steps: []runbook.StepSpec{{
			Name: "start", Kind: model.RunbookStepDeployStart, DeploymentID: "{{deployment_id}}",
		}},
	})
	require.NoError(t, err)
	_, _ = s.RunbookRun(ctx, "demo", "prod", rb1.RunbookID, map[string]string{"deployment_id": "d1"}, "admin")
	_, _ = s.RunbookRun(ctx, "demo", "prod", rb1.RunbookID, map[string]string{"deployment_id": "d1"}, "admin")
	_, _ = s.RunbookRun(ctx, "demo", "prod", rb2.RunbookID, map[string]string{"deployment_id": "d2"}, "admin")

	runs, err := s.RunbookRunList(ctx, "demo", "prod", rb1.RunbookID, 10)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	for _, run := range runs {
		assert.Equal(t, rb1.RunbookID, run.RunbookID)
		// 参数快照落库
		var params map[string]string
		require.NoError(t, json.Unmarshal([]byte(run.ParamsJSON), &params))
		assert.Equal(t, "d1", params["deployment_id"])
	}
}

// TestRunbookCreateHandler_CrossScopeRejected 认证 scope demo/prod，body 提供
// other/prod → 403（fail-closed，防越权定义 runbook）。
func TestRunbookCreateHandler_CrossScopeRejected(t *testing.T) {
	_, svcCtx := newRunbookTestService(t)
	h := NewHandler(NewService(svcCtx))

	body := `{"name":"x","gameId":"other","env":"prod","steps":[{"name":"s","kind":"deploy_start","deploymentId":"d1"}]}`
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/runbooks", body)
	ctx.Request = ctx.Request.WithContext(svc.WithGameScope(ctx.Request.Context(),
		svc.GameScope{GameID: "demo", Env: "prod"}))
	h.RunbookCreateHandler(ctx)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "scope_mismatch")
}
