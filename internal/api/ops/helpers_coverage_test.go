package ops

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 覆盖 helpers.go 低覆盖函数的防御分支 + 正常路径（空 ServiceContext）。

func TestOpsAgentsList_NilSvc(t *testing.T) {
	// nil svcCtx → 空列表
	resp, err := opsAgentsList(context.Background(), nil, &OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Agents)

	// 空 svcCtx（RegistryStore nil）→ 空列表
	resp2, err := opsAgentsList(context.Background(), &svc.ServiceContext{}, &OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp2.Agents)
}

func TestOpsNodeCommands_Validation(t *testing.T) {
	svcCtx := &svc.ServiceContext{}

	// nodeId 为空 → BadRequest
	_, err := opsNodeDrain(context.Background(), svcCtx, &OpsNodeCommandsRequest{NodeId: ""})
	assert.Error(t, err)

	_, err = opsNodeRestart(context.Background(), svcCtx, &OpsNodeCommandsRequest{NodeId: ""})
	assert.Error(t, err)

	_, err = opsNodeUndrain(context.Background(), svcCtx, &OpsNodeCommandsRequest{NodeId: ""})
	assert.Error(t, err)

	// nodeId 非空但 AgentSessionResolver nil → 内部错误
	_, err = opsNodeDrain(context.Background(), svcCtx, &OpsNodeCommandsRequest{NodeId: "node-1"})
	assert.Error(t, err)
}

func TestOpsHealthRun_Validation(t *testing.T) {
	// nil req → BadRequest
	_, err := opsHealthRun(context.Background(), &svc.ServiceContext{}, nil)
	assert.Error(t, err)

	// 空 ID → BadRequest
	_, err = opsHealthRun(context.Background(), &svc.ServiceContext{}, &OpsHealthRunRequest{ID: ""})
	assert.Error(t, err)

	// OpsStateStore nil → 内部错误
	_, err = opsHealthRun(context.Background(), &svc.ServiceContext{}, &OpsHealthRunRequest{ID: "hc-1"})
	assert.Error(t, err)
}

func TestOpsBackupDelete_Validation(t *testing.T) {
	// BackupModel nil → no-op 成功
	resp, err := opsBackupDelete(context.Background(), &svc.ServiceContext{}, &OpsBackupDeleteRequest{ID: "bk-1"})
	require.NoError(t, err)
	assert.True(t, resp.Deleted)
}

func TestOpsBackupDownload_Validation(t *testing.T) {
	resp, err := opsBackupDownload(context.Background(), &svc.ServiceContext{}, &OpsBackupDownloadRequest{ID: "bk-1"})
	require.NoError(t, err)
	assert.Contains(t, resp.Url, "bk-1")
}

func TestOpsConfig_NilSettings(t *testing.T) {
	// 空 svcCtx + settings 未初始化 → 走 env 回落分支
	resp, err := opsConfig(context.Background(), &svc.ServiceContext{}, &OpsConfigRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}
