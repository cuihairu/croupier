package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件按 deadbranch 惯例（对齐 internal/analytics/mq/deadbranch_doc_test.go）
// 固化本包剩余不可达分支的结论，合计 18 条语句：
//
//  1. handler.go 八处 service 错误分支（:54 FunctionsPending、:99 FunctionCopy、
//     :186 FunctionPublish、:203 FunctionInstances、:218 FunctionInstancesAll、
//     :264 FunctionWarnings、:301 BatchCopyFunctions、:316 BatchDeleteFunctions，
//     各 2 条语句）：对应 helper 均为恒返回 nil error 的桩实现
//     （functionsPending/functionCopy/functionPublish/functionInstances/
//     functionInstancesAll/functionWarnings/batchCopyFunctions/batchDeleteFunctions），
//     Service 层为纯委托，不存在可构造的错误输入。
//  2. helpers.go:227 functionHistory 的 items==nil 分支：logic 层
//     FunctionHistory 在 err==nil 时恒返回非 nil 切片（至少含 function-created
//     条目；offset 越界时返回非 nil 空切片）。
//  3. helpers.go:1239 rawJSONFromBytes 的 json.Marshal(string) 错误分支：
//     对 string 类型 marshal 不存在失败输入。
//
// 上述分支构成该包语句覆盖率的理论上限（804/822 = 97.8%）。

// TestStubServicesNeverReturnError 逐一驱动八个桩 service，证明对任意
// svcCtx / 请求形态 err 恒为 nil，handler 错误分支不可触达。
func TestStubServicesNeverReturnError(t *testing.T) {
	ctx := context.Background()
	empty := &svc.ServiceContext{}

	t.Run("FunctionsPending", func(t *testing.T) {
		_, err := NewService(empty).FunctionsPending(ctx, &FunctionsPendingRequest{})
		assert.NoError(t, err)
	})

	t.Run("FunctionCopy", func(t *testing.T) {
		_, err := NewService(empty).FunctionCopy(ctx, &FunctionCopyRequest{ID: "stub.fn"})
		assert.NoError(t, err)
	})

	t.Run("FunctionPublish", func(t *testing.T) {
		_, err := NewService(empty).FunctionPublish(ctx, &FunctionPublishRequest{ID: "stub.fn"})
		assert.NoError(t, err)
	})

	t.Run("FunctionInstances nil store", func(t *testing.T) {
		resp, err := NewService(empty).FunctionInstances(ctx, &FunctionInstancesRequest{ID: "stub.fn"})
		require.NoError(t, err)
		assert.Empty(t, resp.Items)
	})

	t.Run("FunctionInstancesAll nil store", func(t *testing.T) {
		resp, err := NewService(empty).FunctionInstancesAll(ctx, &FunctionInstancesAllRequest{})
		require.NoError(t, err)
		assert.Empty(t, resp.Instances)
	})

	t.Run("FunctionWarnings nil store", func(t *testing.T) {
		resp, err := NewService(empty).FunctionWarnings(ctx, &FunctionWarningsRequest{})
		require.NoError(t, err)
		assert.Empty(t, resp.Items)
	})

	t.Run("BatchCopyFunctions", func(t *testing.T) {
		resp, err := NewService(empty).BatchCopyFunctions(ctx, &BatchCopyFunctionsRequest{
			Functions: []FunctionCopyRequest{{ID: "a.fn"}, {ID: "b.fn"}},
		})
		require.NoError(t, err)
		require.Len(t, resp.Results, 2)
	})

	t.Run("BatchDeleteFunctions per-item failure still nil error", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.db.Migrator().DropTable("functions"))
		resp, err := NewService(f.svcCtx).BatchDeleteFunctions(ctx, &BatchDeleteFunctionsRequest{
			FunctionIds: []string{"ghost.fn"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"ghost.fn"}, resp.Failed)
	})
}

// TestFunctionHistoryItemsNeverNil 证明 err==nil 时 logic 层恒返回非 nil 切片，
// helpers.go:227 的 items==nil 分支不可触达。
func TestFunctionHistoryItemsNeverNil(t *testing.T) {
	f := newInvokeFixture(t)
	resp, err := NewService(f.svcCtx).FunctionHistory(context.Background(), &FunctionHistoryRequest{ID: "doc.history.fn"})
	require.NoError(t, err)
	require.NotNil(t, resp.Items)
	require.GreaterOrEqual(t, len(resp.Items), 1)
	assert.Equal(t, "function_created", resp.Items[0].Action)
}

// TestRawJSONFromBytesInvalidBytes 证明非法 JSON 字节走 string marshal 路径
// 恒成功（json.Marshal 对 string 无失败输入），:1239 错误分支不可触达。
func TestRawJSONFromBytesInvalidBytes(t *testing.T) {
	out := rawJSONFromBytes([]byte("plain text"))
	require.NotNil(t, out)
	assert.Equal(t, []byte(`"plain text"`), []byte(out))
}
