package function

// #26 函数历史版本索引的排序/去重边界与存储错误路径（既有用例已覆盖
// scope 校验、semver 降序、DB 未初始化、30s TTL 缓存）：
//  ① dedupeVersions 的重复分支——GROUP BY(function_id, version) 拦不住
//     空白差异的同值（'1.0.0' 与 ' 1.0.0' 是两行），TrimSpace 后必须收敛；
//  ② 排序的「双不可解析」分支——可解析排前之后，剩余版本按字典序；
//  ③ ListDistinctVersions 的存储错误（表缺失）→ 统一错误对象而非静默空集。

import (
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 空白差异的同值版本收敛为一个选项。
func TestHandler_VersionHistoryIndex_DedupesWhitespaceVariants(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	const gameID = "vh-dedupe"
	svcCtx := setupTestServiceContext(t)
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.0.0")
	// 同一版本号的空白变体：SQL 层是 distinct 的两行，handler 须去重
	seedContractVersion(t, svcCtx, gameID, "a.fn", " 1.0.0")

	h := NewHandler(NewService(svcCtx))
	ctx, rec := newVersionIndexRequest(gameID)
	h.VersionHistoryIndex(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	items := decodeVersionIndex(t, rec.Body.Bytes())
	require.Len(t, items, 1)
	assert.Equal(t, []string{"1.0.0"}, items[0].Versions)
}

// 不可解析版本在可解析版本之后，按字典序排列（两个都不可解析才走此分支）。
func TestHandler_VersionHistoryIndex_UnparseableSortedLexicographically(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	const gameID = "vh-unparseable"
	svcCtx := setupTestServiceContext(t)
	seedContractVersion(t, svcCtx, gameID, "a.fn", "nightly-x")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "alpha")
	seedContractVersion(t, svcCtx, gameID, "a.fn", "1.0.0")

	h := NewHandler(NewService(svcCtx))
	ctx, rec := newVersionIndexRequest(gameID)
	h.VersionHistoryIndex(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	items := decodeVersionIndex(t, rec.Body.Bytes())
	require.Len(t, items, 1)
	assert.Equal(t, []string{"1.0.0", "alpha", "nightly-x"}, items[0].Versions)
}

// 版本表缺失（迁移未跑）→ 500，不得静默返回空下拉（空下拉会被误读为「无历史版本」）。
func TestHandler_VersionHistoryIndex_StoreError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable(&model.FunctionContractVersion{}))

	h := NewHandler(NewService(svcCtx))
	ctx, rec := newVersionIndexRequest("vh-store-err")
	h.VersionHistoryIndex(ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"error"`)
}
