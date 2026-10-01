package function

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 存量函数惰性回填（B2 版本历史晚于注册面引入）：有契约、无历史的函数
// 首次打开变更历史时补一条 created 初始快照；幂等——再次拉取不再新增。

func seedContractWithoutHistory(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()
	m := model.NewFunctionContractModel(svcCtx.DB)
	err := m.UpsertContract(context.Background(), &model.FunctionContract{
		GameID:     "g",
		Env:        "e",
		FunctionID: "inventory.consume",
		Version:    "2.4.0",
		Source:     "sdk",
		InputSchema: model.JSON(`{
			"type": "object",
			"properties": {"count": {"type": "integer"}},
			"required": ["count"]
		}`),
		ExecutionState: "bound",
	})
	if err != nil {
		t.Fatalf("seed contract: %v", err)
	}
}

func listVersionsViaHandler(t *testing.T, h *Handler, functionID string) (int64, []map[string]interface{}) {
	t.Helper()
	ctx, rec := newFunctionTestContext(http.MethodGet,
		"/api/v1/functions/"+functionID+"/contract-versions?page=1&pageSize=10", "")
	ctx.Params = gin.Params{{Key: "id", Value: functionID}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))
	h.ContractVersions(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Items []map[string]interface{} `json:"items"`
		Total int64                    `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp), rec.Body.String())
	return resp.Total, resp.Items
}

func TestHandler_ContractVersions_BackfillsInitialVersionForLegacyContract(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	seedContractWithoutHistory(t, svcCtx)
	h := NewHandler(NewService(svcCtx))

	total, items := listVersionsViaHandler(t, h, "inventory.consume")
	require.EqualValues(t, 1, total, "legacy contract should gain exactly one initial version")
	require.Len(t, items, 1)
	require.Equal(t, "created", items[0]["changeType"])
	require.Equal(t, "backfill", items[0]["actor"])
	require.Equal(t, "2.4.0", items[0]["version"])

	// 幂等：第二次拉取不新增。
	total2, _ := listVersionsViaHandler(t, h, "inventory.consume")
	require.EqualValues(t, 1, total2)
}

func TestHandler_ContractVersions_NoBackfillWithoutContract(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// 无契约的函数（如历史遗留脏 id）不补快照，保持空列表不报错。
	svcCtx := setupTestServiceContext(t)
	h := NewHandler(NewService(svcCtx))

	total, items := listVersionsViaHandler(t, h, "ghost.function")
	require.EqualValues(t, 0, total)
	require.Empty(t, items)
}

// 直测 service 层：DB 无契约表（nil 合约模型不可用场景由构造保证）时
// BackfillInitialContractVersion 不 panic；契约存在时返回 created=true。
func TestBackfillInitialContractVersion_Direct(t *testing.T) {
	t.Parallel()

	svcCtx := setupTestServiceContext(t)
	cs := service.NewContractService(svcCtx.DB)
	seedContractWithoutHistory(t, svcCtx)

	created, err := cs.BackfillInitialContractVersion(context.Background(), "g", "e", "inventory.consume")
	require.NoError(t, err)
	require.True(t, created)

	created, err = cs.BackfillInitialContractVersion(context.Background(), "g", "e", "inventory.consume")
	require.NoError(t, err)
	require.False(t, created, "second backfill must be a no-op")

	// 未知函数：不补写、不报错。
	created, err = cs.BackfillInitialContractVersion(context.Background(), "g", "e", "ghost.function")
	require.NoError(t, err)
	require.False(t, created)
}
