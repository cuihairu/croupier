package server

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	registry "github.com/cuihairu/croupier/internal/platform/registry"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newLoadFixture：带 sqlite 的 registry store（LoadFromDBFiltered 要求
// store 挂 DB）+ 注入 stub loader。
func newLoadFixture(t *testing.T, sessions []*registry.AgentSession) (*ControlService, *registry.Store) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	store := registry.NewStoreWithDB(db)
	return NewControlService(store, &mockAgentSessionLoader{sessions: sessions}), store
}

// 端到端断连清理：注册经 ControlService 写 registry → 断开 → serveConn
// 的 RemoveAgentIfStale 同步清掉 registry 条目（此前僵尸到 ExpireAt 24h）。
// TCPListener 与 ControlService 必须共享同一 registry store（装配期由
// svcCtx.RegistryStore 保证）。
func TestServeConn_DisconnectPrunesRegistry(t *testing.T) {
	store := registry.NewStore()
	config := &TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}
	listener, err := NewTCPListener(config, NewAgentSessionStore(), store, nil)
	require.NoError(t, err)

	svc := NewControlService(store, nil)
	listener.SetHandler(svc)

	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()
	go func() { // 排水：mux 响应帧同步写管道
		buf := make([]byte, 4096)
		for {
			if _, rerr := clientEnd.Read(buf); rerr != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go listener.serveConn(ctx, serverEnd)

	writeFrame := func(msgID uint32, reqID uint32, body []byte) {
		frame := protocol.NewMessageBody(msgID, reqID, body)
		wrapped := make([]byte, 4+len(frame))
		wrapped[0] = byte(len(frame) >> 24)
		wrapped[1] = byte(len(frame) >> 16)
		wrapped[2] = byte(len(frame) >> 8)
		wrapped[3] = byte(len(frame))
		copy(wrapped[4:], frame)
		_, werr := clientEnd.Write(wrapped)
		require.NoError(t, werr)
	}

	regReq, _ := proto.Marshal(&agentv1.RegisterRequest{AgentId: "agent-prune", GameId: "game-1", Env: "dev"})
	writeFrame(protocol.MsgRegisterRequest, 1, regReq)

	deadline := time.Now().Add(2 * time.Second)
	var inRegistry bool
	for time.Now().Before(deadline) {
		store.Mu().RLock()
		_, inRegistry = store.AgentsUnsafe()["agent-prune"]
		store.Mu().RUnlock()
		if inRegistry {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, inRegistry, "register must populate registry")

	clientEnd.Close()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		store.Mu().RLock()
		_, inRegistry = store.AgentsUnsafe()["agent-prune"]
		store.Mu().RUnlock()
		if !inRegistry {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.False(t, inRegistry, "disconnect must prune registry entry immediately")
}

// directory stub：可控活跃集合/错误。
type stubActiveDirectory struct {
	ids []string
	err error
}

func (d stubActiveDirectory) ActiveAgentIDs(context.Context) ([]string, error) {
	return d.ids, d.err
}

// LoadAgentSessions 注入 directory 时按归属表活跃集合过滤快照行。
func TestLoadAgentSessions_FiltersByActiveDirectory(t *testing.T) {
	svc, store := newLoadFixture(t, []*registry.AgentSession{
		{AgentID: "agent-alive", GameID: "g", Env: "dev", ExpireAt: time.Now().Add(time.Hour), LastSeen: time.Now()},
		{AgentID: "agent-dead", GameID: "g", Env: "dev", ExpireAt: time.Now().Add(time.Hour), LastSeen: time.Now()},
	})
	svc.SetActiveAgentDirectory(stubActiveDirectory{ids: []string{"agent-alive"}})

	require.NoError(t, svc.LoadAgentSessions())

	store.Mu().RLock()
	agents := store.AgentsUnsafe()
	store.Mu().RUnlock()
	assert.NotNil(t, agents["agent-alive"])
	assert.Nil(t, agents["agent-dead"], "归属表无行的快照行不得恢复")
}

// directory 查询失败降级全量恢复（启动路径不放大归属表故障）。
func TestLoadAgentSessions_DirectoryFailureFallsBackToAll(t *testing.T) {
	svc, store := newLoadFixture(t, []*registry.AgentSession{
		{AgentID: "agent-a", GameID: "g", Env: "dev", ExpireAt: time.Now().Add(time.Hour), LastSeen: time.Now()},
		{AgentID: "agent-b", GameID: "g", Env: "dev", ExpireAt: time.Now().Add(time.Hour), LastSeen: time.Now()},
	})
	svc.SetActiveAgentDirectory(stubActiveDirectory{err: errors.New("owner table unavailable")})

	require.NoError(t, svc.LoadAgentSessions())

	store.Mu().RLock()
	count := len(store.AgentsUnsafe())
	store.Mu().RUnlock()
	assert.Equal(t, 2, count, "directory 故障时降级全量恢复")
}
