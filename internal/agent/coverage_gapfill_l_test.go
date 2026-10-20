package agent

// 覆盖率补洞（group L）：task_runner.go marshalTaskInvoke 的 Marshal 失败
// 分支。只新增用例，不改产品代码与既有测试。
// （provider_keepalive.go 的 Marshal 失败用例已随实现迁至 agents/sidecar。）

import (
	"testing"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestMarshalTaskInvoke_NilAndInvalidUTF8(t *testing.T) {
	assert.Nil(t, marshalTaskInvoke(nil))
	assert.Nil(t, marshalTaskInvoke(&sdkv1.InvokeRequest{
		FunctionId: "demo.ping",
		Metadata:   map[string]string{"k": "\xff\xfe"},
	}))

	data := marshalTaskInvoke(&sdkv1.InvokeRequest{FunctionId: "demo.ok"})
	require.NotNil(t, data)
	var back sdkv1.InvokeRequest
	require.NoError(t, proto.Unmarshal(data, &back))
	assert.Equal(t, "demo.ok", back.GetFunctionId())
}
