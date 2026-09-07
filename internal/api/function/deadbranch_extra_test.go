package function

import (
	"context"
	"encoding/json"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 以下测试文档化本包中不可达的防御分支（对齐 internal/analytics/mq/deadbranch_doc_test.go 惯例）：
//
//  1. handler.go 八个 handler 的 service 错误分支（:54/:99/:186/:203/:218/:264/:301/:316）：
//     FunctionsPending/FunctionCopy/FunctionPublish/FunctionInstances/
//     FunctionInstancesAll/FunctionWarnings/BatchCopyFunctions/
//     BatchDeleteFunctions 对应的 helpers.go 实现均为 stub，唯一返回语句
//     `return &XxxResponse{...}, nil`，error 恒为 nil，handler 的
//     `if err != nil` 分支不可触发。
//  2. helpers.go rawJSONFromBytes（:1239）：json.Marshal(string(value)) 对
//     string 序列化恒成功（无效 UTF-8 也只会被替换为 U+FFFD），错误分支
//     不可触发。
//  3. helpers.go functionHistory（:227）：FunctionHistory 恒以非 nil items
//     初始化（function_history_logic.go:40 `items := []FunctionHistoryItem{...}`），
//     items==nil 分支不可触发。

func TestStubServiceFunctionsNeverReturnErrorV11(t *testing.T) {
	svcCtx := &svc.ServiceContext{RegistryStore: reg.NewStore()}
	ctx := context.Background()

	resp, err := functionsPending(ctx, svcCtx, &FunctionsPendingRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	copyResp, err := functionCopy(ctx, svcCtx, &FunctionCopyRequest{})
	require.NoError(t, err)
	require.NotNil(t, copyResp)

	pubResp, err := functionPublish(ctx, svcCtx, &FunctionPublishRequest{})
	require.NoError(t, err)
	require.NotNil(t, pubResp)

	instResp, err := functionInstances(ctx, svcCtx, &FunctionInstancesRequest{})
	require.NoError(t, err)
	require.NotNil(t, instResp)

	allResp, err := functionInstancesAll(ctx, svcCtx, &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	require.NotNil(t, allResp)

	warnResp, err := functionWarnings(ctx, svcCtx, &FunctionWarningsRequest{})
	require.NoError(t, err)
	require.NotNil(t, warnResp)

	batchCopyResp, err := batchCopyFunctions(ctx, svcCtx, &BatchCopyFunctionsRequest{})
	require.NoError(t, err)
	require.NotNil(t, batchCopyResp)

	batchDelResp, err := batchDeleteFunctions(ctx, svcCtx, &BatchDeleteFunctionsRequest{})
	require.NoError(t, err)
	require.NotNil(t, batchDelResp)
}

func TestRawJSONFromBytesMarshalStringNeverFailsV11(t *testing.T) {
	// 非 JSON 字节回退为 JSON 字符串编码。
	out := rawJSONFromBytes([]byte("not-json\x00\xff"))
	require.NotNil(t, out)
	var decoded string
	require.NoError(t, json.Unmarshal(out, &decoded))
	assert.Equal(t, "not-json\x00\ufffd", decoded)

	// 合法 JSON 原样保留；空输入返回 nil。
	assert.Equal(t, json.RawMessage(`{"a":1}`), rawJSONFromBytes([]byte(`{"a":1}`)))
	assert.Nil(t, rawJSONFromBytes(nil))
}

func TestFunctionHistoryLogicNeverReturnsNilItemsV11(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	createTestFunction(t, svcCtx.DB, "demo.histv11", "HistV11")

	resp, err := functionHistory(context.Background(), svcCtx, &FunctionHistoryRequest{ID: "demo.histv11", Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.NotNil(t, resp.Items)
	assert.NotEmpty(t, resp.Items, "FunctionHistory always seeds a function_created item; the items==nil guard is dead code")
}
