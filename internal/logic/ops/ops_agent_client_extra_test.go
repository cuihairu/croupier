package ops

// agent_ops_client.go 补测：UpdateProcess / SetSchedulePause / PushFile /
// GetSupervisorLog 四方法的 marshal / call / unmarshal 错误臂与成功路径
// （先前整方法零覆盖）。
//
// 诚实跳过：GetSupervisorLog 的 proto.Marshal 错误臂——请求体仅 int32 字段，
// Marshal 恒成功（nil 请求按空消息处理亦不报错，encode.go「nil 视为空消息」）。

import (
	"context"
	"testing"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// respCaller 返回预置合法 protobuf 响应（成功路径用）。
type respCaller struct{ resp []byte }

func (c *respCaller) Call(_ context.Context, msgID uint32, _ []byte) (uint32, []byte, error) {
	return msgID, c.resp, nil
}

func mustResp(t *testing.T, m proto.Message) []byte {
	t.Helper()
	data, err := proto.Marshal(m)
	require.NoError(t, err)
	return data
}

func TestOpsClientWrapper_UpdateProcess_Arms(t *testing.T) {
	ctx := context.Background()

	// marshal 臂（非法 UTF-8 string）
	w := &OpsClientWrapper{caller: &mockSessionCaller{}}
	_, err := w.UpdateProcess(ctx, &opsv1.UpdateProcessRequest{ProcessName: invalidUTF8})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal update request failed")

	// call 臂
	w = &OpsClientWrapper{caller: &errorCaller{errMsg: "conn lost"}}
	_, err = w.UpdateProcess(ctx, &opsv1.UpdateProcessRequest{ProcessName: "p"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conn lost")

	// unmarshal 臂
	w = &OpsClientWrapper{caller: &badDataCaller{}}
	_, err = w.UpdateProcess(ctx, &opsv1.UpdateProcessRequest{ProcessName: "p"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal update response failed")

	// 成功路径
	w = &OpsClientWrapper{caller: &respCaller{resp: mustResp(t, &opsv1.UpdateProcessResponse{})}}
	resp, err := w.UpdateProcess(ctx, &opsv1.UpdateProcessRequest{ProcessName: "p"})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestOpsClientWrapper_SetSchedulePause_Arms(t *testing.T) {
	ctx := context.Background()

	w := &OpsClientWrapper{caller: &mockSessionCaller{}}
	_, err := w.SetSchedulePause(ctx, &opsv1.SchedulePauseRequest{ProcessName: invalidUTF8})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal schedule pause request failed")

	w = &OpsClientWrapper{caller: &errorCaller{errMsg: "conn lost"}}
	_, err = w.SetSchedulePause(ctx, &opsv1.SchedulePauseRequest{ProcessName: "p"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conn lost")

	w = &OpsClientWrapper{caller: &badDataCaller{}}
	_, err = w.SetSchedulePause(ctx, &opsv1.SchedulePauseRequest{ProcessName: "p"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal schedule pause response failed")

	w = &OpsClientWrapper{caller: &respCaller{resp: mustResp(t, &opsv1.SchedulePauseResponse{})}}
	resp, err := w.SetSchedulePause(ctx, &opsv1.SchedulePauseRequest{ProcessName: "p", Suspend: true})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestOpsClientWrapper_PushFile_Arms(t *testing.T) {
	ctx := context.Background()

	w := &OpsClientWrapper{caller: &mockSessionCaller{}}
	_, err := w.PushFile(ctx, &opsv1.PushFileRequest{Path: invalidUTF8})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal push file request failed")

	w = &OpsClientWrapper{caller: &errorCaller{errMsg: "conn lost"}}
	_, err = w.PushFile(ctx, &opsv1.PushFileRequest{Path: "/etc/app.yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conn lost")

	w = &OpsClientWrapper{caller: &badDataCaller{}}
	_, err = w.PushFile(ctx, &opsv1.PushFileRequest{Path: "/etc/app.yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal push file response failed")

	w = &OpsClientWrapper{caller: &respCaller{resp: mustResp(t, &opsv1.PushFileResponse{})}}
	resp, err := w.PushFile(ctx, &opsv1.PushFileRequest{Path: "/etc/app.yaml", Content: []byte("k: v")})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestOpsClientWrapper_GetSupervisorLog_Arms(t *testing.T) {
	ctx := context.Background()

	// call 臂
	w := &OpsClientWrapper{caller: &errorCaller{errMsg: "conn lost"}}
	_, err := w.GetSupervisorLog(ctx, &opsv1.GetSupervisorLogRequest{MaxBytes: 1024})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conn lost")

	// unmarshal 臂
	w = &OpsClientWrapper{caller: &badDataCaller{}}
	_, err = w.GetSupervisorLog(ctx, &opsv1.GetSupervisorLogRequest{MaxBytes: 1024})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal supervisor log response failed")

	// 成功路径
	w = &OpsClientWrapper{caller: &respCaller{resp: mustResp(t, &opsv1.GetSupervisorLogResponse{})}}
	resp, err := w.GetSupervisorLog(ctx, &opsv1.GetSupervisorLogRequest{MaxBytes: 1024})
	require.NoError(t, err)
	require.NotNil(t, resp)
}
