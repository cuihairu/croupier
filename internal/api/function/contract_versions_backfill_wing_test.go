package function

import (
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// R53：ContractVersions 惰性回填面（9c50e3d 落地）两处降级翼收口。
//
// ① handler.go:35-36 回填写失败 → slog.Warn 降级、响应不受影响
//   （源注释契约：「历史是衍生审计数据，回填失败降级为日志、不影响列表响应」）；
// ② handler.go:39-41 回填成功（created=true）后重查失败 → 错误透传。
//
// 注入口径（仓内既有技法）：本包 setupTestDB 为每用例独立 `:memory:` 库
// （非共享单例），sqlite 触发器注入天然按用例隔离，无跨用例毒化面——
// 与共享 cache=shared 库需唯一 scope 隔离的包不同。
// ① 写阻断：BEFORE INSERT TRIGGER + RAISE(ABORT)（svc C 批同款）；
// ② 读炸裂：AFTER INSERT TRIGGER 追加 seq 为非数字 TEXT 的毒行——
//   回填 INSERT 本身成功（created=true），紧随的 List Find 把 TEXT 扫进
//   int64 列即报 scan error，构造「写成功→紧接着读失败」这一单请求内
//   读故障形态（RAISE 族技法补不了读侧，毒行是读侧等价物）。

func TestHandler_ContractVersions_BackfillWriteFailureDegrades(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	seedContractWithoutHistory(t, svcCtx)
	require.NoError(t, svcCtx.DB.Exec(
		`CREATE TRIGGER r53_block_version_insert BEFORE INSERT ON function_contract_versions `+
			`BEGIN SELECT RAISE(ABORT, 'r53 injected write failure'); END;`).Error)

	// 回填写入被拦 → 只降级为 warn，响应仍是首查的空列表（200，非 500）。
	total, items := listVersionsViaHandler(t, NewHandler(NewService(svcCtx)), "inventory.consume")
	require.EqualValues(t, 0, total)
	require.Empty(t, items)
}

func TestHandler_ContractVersions_RequeryFailureAfterBackfill(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	seedContractWithoutHistory(t, svcCtx)
	// 回填 INSERT 成功后由触发器追加 seq='x' 毒行：Count 不受影响，
	// Find 把 TEXT 'x' 扫进 int64 Seq 即 scan error → 重查错误透传 500。
	require.NoError(t, svcCtx.DB.Exec(
		`CREATE TRIGGER r53_poison_after_insert AFTER INSERT ON function_contract_versions `+
			`BEGIN INSERT INTO function_contract_versions `+
			`(game_id, env, function_id, seq, version, change_type, actor, snapshot, created_at, updated_at) `+
			`VALUES ('g', 'e', 'inventory.consume', 'x', '0.0.0', 'created', 'r53-poison', '{}', `+
			`datetime('now'), datetime('now')); END;`).Error)

	h := NewHandler(NewService(svcCtx))
	ctx, rec := newFunctionTestContext(http.MethodGet,
		"/api/v1/functions/inventory.consume/contract-versions?page=1&pageSize=10", "")
	ctx.Params = gin.Params{{Key: "id", Value: "inventory.consume"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))
	h.ContractVersions(ctx)

	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}
