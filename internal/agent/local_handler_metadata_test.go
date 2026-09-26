// handleProviderConnect 实例元数据回归：用户多 KV 并入注册 metadata，
// 保留键冲突丢弃并在 response warnings 告警，空键跳过。
package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/platform/agentlocal"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

func decodeProviderConnectResponse(t *testing.T, data []byte) *sdkv1.ProviderConnectResponse {
	t.Helper()
	resp := &sdkv1.ProviderConnectResponse{}
	require.NoError(t, proto.Unmarshal(data, resp))
	return resp
}

func TestLocalHandler_HandleProviderConnect_UserMetadataMerged(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)
	handler.SetExpectedGameEnv("default", "dev")

	req := &sdkv1.ProviderConnectRequest{
		ServiceId:   "svc-meta",
		Version:     "1.0.0",
		SdkLanguage: "go",
		SdkVersion:  "1.4.0",
		GameId:      "default",
		Env:         "dev",
		Functions: []*sdkv1.ProviderFunctionDescriptor{
			{Id: "fn.echo"},
		},
		Metadata: map[string]string{
			"serverId": "s1",
			"pod":      "game-7c4d",
		},
	}
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	raw, err := handler.handleProviderConnect(context.Background(), data)
	require.NoError(t, err)
	resp := decodeProviderConnectResponse(t, raw)
	assert.Empty(t, resp.Warnings, "正常用户元数据不应产生告警")

	instances := store.List()["fn.echo"]
	require.Len(t, instances, 1)
	md := instances[0].Metadata
	assert.Equal(t, "s1", md["serverId"])
	assert.Equal(t, "game-7c4d", md["pod"])
	// 平台固定键仍在（SDK 信息照常上报）
	assert.Equal(t, "go", md["sdkLanguage"])
	assert.Equal(t, "dev", md["env"])
}

func TestLocalHandler_HandleProviderConnect_ReservedKeyDroppedWithWarning(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)
	handler.SetExpectedGameEnv("default", "dev")

	req := &sdkv1.ProviderConnectRequest{
		ServiceId:   "svc-reserved",
		SdkLanguage: "go",
		GameId:      "default",
		Env:         "dev",
		Functions: []*sdkv1.ProviderFunctionDescriptor{
			{Id: "fn.echo"},
		},
		Metadata: map[string]string{
			"sdkLanguage": "rust", // 撞保留键：必须丢弃并告警，不得覆盖平台值
			"serverId":    "s2",
			"":            "blank-key", // 空键：静默跳过
		},
	}
	data, err := proto.Marshal(req)
	require.NoError(t, err)

	raw, err := handler.handleProviderConnect(context.Background(), data)
	require.NoError(t, err)
	resp := decodeProviderConnectResponse(t, raw)
	require.Len(t, resp.Warnings, 1, "仅保留键冲突告警（空键静默）")
	assert.Contains(t, resp.Warnings[0], "reserved")
	assert.Contains(t, resp.Warnings[0], "sdkLanguage")

	instances := store.List()["fn.echo"]
	require.Len(t, instances, 1)
	md := instances[0].Metadata
	assert.Equal(t, "go", md["sdkLanguage"], "保留键必须保持平台值不被用户元数据覆盖")
	assert.Equal(t, "s2", md["serverId"])
	assert.NotContains(t, md, "", "空键不得进入注册 metadata")
}
