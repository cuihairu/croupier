package ops

// GET /ops/agents/:agentId/system-info 路由面用例：缓存优先 → 隧道 live RPC
// 回填（§10.5 指纹随 SystemInfo 上行到 API DTO；离线 agent 暖缓存=最后已知
// 信息锚点）。此前该路由只回 registry 标签 stub，指纹到不了 API 面。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func sysInfoResp(hostname, fp string) *opsv1.SystemInfo {
	return &opsv1.SystemInfo{
		Hostname:          hostname,
		Os:                "linux",
		Arch:              "amd64",
		CpuCores:          8,
		AgentVersion:      "v2.0.0",
		AgentBinarySha256: fp,
	}
}

func TestOpsAgentSystemInfoLiveAndCached(t *testing.T) {
	svcCtx := &svc.ServiceContext{SystemInfoCache: registry.NewSystemInfoCache()}
	ctx := context.Background()

	// 冷缓存：隧道 live RPC，指纹随响应到 DTO。
	setExecCaller(t, "agent-1", &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgGetSystemInfoRequest: sysInfoResp("host-1", "1111111111111111111111111111111111111111111111111111111111111111"),
	}})
	resp, err := opsAgentSystemInfo(ctx, svcCtx, &OpsAgentSystemInfoRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, "host-1", resp.SystemInfo.Hostname)
	assert.Equal(t, "v2.0.0", resp.SystemInfo.AgentVersion)
	assert.Equal(t, "1111111111111111111111111111111111111111111111111111111111111111", resp.SystemInfo.AgentBinarySha256)

	// 暖缓存：换一个必然失败的 caller 仍命中缓存（不发起隧道调用）。
	svcCtx.SystemInfoCache.Set("agent-2", sysInfoResp("host-2", "22222222222222222222222222222222222222222222222222222222222222222"))
	setExecCaller(t, "agent-2", &fakeCaller{errOn: map[uint32]error{
		protocol.MsgGetSystemInfoRequest: assert.AnError,
	}})
	resp2, err := opsAgentSystemInfo(ctx, svcCtx, &OpsAgentSystemInfoRequest{AgentID: "agent-2"})
	require.NoError(t, err)
	assert.Equal(t, "host-2", resp2.SystemInfo.Hostname)
	assert.Equal(t, "22222222222222222222222222222222222222222222222222222222222222222", resp2.SystemInfo.AgentBinarySha256)
}

func TestOpsAgentSystemInfoTunnelError(t *testing.T) {
	svcCtx := &svc.ServiceContext{SystemInfoCache: registry.NewSystemInfoCache()}

	setExecCaller(t, "agent-err", &fakeCaller{errOn: map[uint32]error{
		protocol.MsgGetSystemInfoRequest: assert.AnError,
	}})
	_, err := opsAgentSystemInfo(context.Background(), svcCtx, &OpsAgentSystemInfoRequest{AgentID: "agent-err"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "system info")
}

func TestOpsAgentSystemInfoBootTimeFormat(t *testing.T) {
	// bootTime RFC3339 序列化（DTO 契约）。
	svcCtx := &svc.ServiceContext{SystemInfoCache: registry.NewSystemInfoCache()}
	info := sysInfoResp("h", "")
	info.BootTime = timestamppb.New(timestamppb.Now().AsTime().Add(-3600))
	svcCtx.SystemInfoCache.Set("agent-3", info)

	resp, err := opsAgentSystemInfo(context.Background(), svcCtx, &OpsAgentSystemInfoRequest{AgentID: "agent-3"})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.SystemInfo.BootTime)
}
