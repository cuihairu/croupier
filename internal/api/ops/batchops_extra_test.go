package ops

// batchops.go 补测：agentOpsExecutor 的 ExecuteCommand/PushFile 全分支
// （resolver 未装配早退、经 stub resolver+SessionCaller 的往返面）、
// BatchOpGet 的草稿读取路径与 orchestrator 装配缺失早退。

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/cuihairu/croupier/core/transport"
	agentops "github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/batchop"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// stubAgentResolver 把任意 agentID 解析到同一个 stub caller（批面执行器的
// 会话解析替身）。
type stubAgentResolver struct {
	caller transport.SessionCaller
}

func (r stubAgentResolver) ResolveAgentConn(string) (transport.SessionCaller, bool) {
	return r.caller, true
}

// stubBatchCaller canned 响应的会话替身（Call 按 msgID 回预置载荷）。
type stubBatchCaller struct {
	resp  map[uint32][]byte
	calls []uint32
}

func (c *stubBatchCaller) Call(_ context.Context, msgID uint32, _ []byte) (uint32, []byte, error) {
	c.calls = append(c.calls, msgID)
	return msgID + 1, c.resp[msgID], nil
}

func TestAgentOpsExecutorWithoutResolver(t *testing.T) {
	e := &agentOpsExecutor{}
	_, err := e.ExecuteCommand(context.Background(), "a1", &opsv1.ExecuteCommandRequest{Command: "echo"})
	require.ErrorContains(t, err, "session resolver not configured")

	_, err = e.PushFile(context.Background(), "a1", &opsv1.PushFileRequest{Path: "/tmp/x"})
	require.ErrorContains(t, err, "session resolver not configured")
}

func TestAgentOpsExecutorRoundTrip(t *testing.T) {
	execResp, err := proto.Marshal(&opsv1.ExecuteCommandResponse{ExitCode: 0, StdOut: "ok"})
	require.NoError(t, err)
	pushResp, err := proto.Marshal(&opsv1.PushFileResponse{Success: true})
	require.NoError(t, err)
	caller := &stubBatchCaller{resp: map[uint32][]byte{
		uint32(protocol.MsgExecuteCommandRequest): execResp,
		uint32(protocol.MsgPushFileRequest):       pushResp,
	}}

	client := agentops.GetAgentOpsClient()
	client.SetSessionResolver(stubAgentResolver{caller: caller})
	t.Cleanup(func() { client.SetSessionResolver(nil) })

	e := &agentOpsExecutor{}
	got, err := e.ExecuteCommand(context.Background(), "a1", &opsv1.ExecuteCommandRequest{Command: "echo"})
	require.NoError(t, err)
	require.Equal(t, "ok", got.StdOut)

	got2, err := e.PushFile(context.Background(), "a1", &opsv1.PushFileRequest{Path: "/tmp/x"})
	require.NoError(t, err)
	require.True(t, got2.Success)
}

func TestBatchOpGetDraft(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/bg.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.BatchOp{}))
	svcCtx := &svc.ServiceContext{DB: db, RegistryStore: registry.NewStore()}
	s := NewService(svcCtx)

	// 装配缺失（nil svcCtx）→ orchestrator 早退
	bare := NewService(&svc.ServiceContext{})
	_, err = bare.BatchOpGet(context.Background(), "g", "e", "b1")
	require.ErrorContains(t, err, "batch op store or registry unavailable")

	op, err := s.BatchOpCreate(context.Background(), &batchop.CreateRequest{
		GameID: "g", Env: "e", Kind: "config", AgentIDs: []string{"a"},
		Config: &batchop.ConfigPayload{Path: "/etc/x", ContentBase64: base64.StdEncoding.EncodeToString([]byte("v")), Mode: 0o644},
	})
	require.NoError(t, err)

	got, err := s.BatchOpGet(context.Background(), "g", "e", op.BatchID)
	require.NoError(t, err)
	require.Equal(t, op.BatchID, got.BatchID)
	require.Equal(t, model.BatchOpStatusDraft, got.Status)
}
