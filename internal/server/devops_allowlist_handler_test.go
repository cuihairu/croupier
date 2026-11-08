package server

// control_handler.go handleDevopsAllowlistConfig 补测：坏载荷、装配缺失、
// 会话守卫（缺 id / 未知 agent / 无 game 作用域）、game DB 解析失败降级、
// config_versions 未发布与已发布的取数面，及 handleRequest 分发臂。

import (
	"context"
	"errors"
	"path/filepath"

	"log/slog"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gosqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type denyAllResolver struct{}

func (denyAllResolver) Resolve(context.Context, string, string) (context.Context, *gorm.DB, error) {
	return nil, nil, errors.New("game db down")
}

func newAllowlistTestService(t *testing.T) (*ControlService, *registry.Store, *gorm.DB) {
	t.Helper()
	store := registry.NewStore()
	svc := NewControlService(store, nil)
	svc.SetLogger(slog.New(slog.NewTextHandler(&testingWriter{t}, nil)))
	db, err := gorm.Open(gosqlite.Open(filepath.Join(t.TempDir(), "allow.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ConfigVersion{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	svc.SetConfigVersionModel(model.NewConfigVersionModel(db))
	return svc, store, db
}

// testingWriter 把 slog 输出转 t.Log，避免测试日志互相穿插。
type testingWriter struct{ t *testing.T }

func (w *testingWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestHandleDevopsAllowlistConfig_Guards(t *testing.T) {
	svc, store, _ := newAllowlistTestService(t)
	ctx := context.Background()

	// 坏载荷
	_, err := svc.handleDevopsAllowlistConfig(ctx, []byte("not protobuf"))
	require.ErrorContains(t, err, "unmarshal DevopsAllowlistConfigRequest")

	// agent_id 缺失
	data, _ := proto.Marshal(&opsv1.DevopsAllowlistConfigRequest{})
	_, err = svc.handleDevopsAllowlistConfig(ctx, data)
	require.ErrorContains(t, err, "agent_id is required")

	// 未知 agent 会话
	data, _ = proto.Marshal(&opsv1.DevopsAllowlistConfigRequest{AgentId: "ghost"})
	_, err = svc.handleDevopsAllowlistConfig(ctx, data)
	require.ErrorContains(t, err, "agent session not found")

	// 会话缺 game 作用域
	store.UpsertAgent(&registry.AgentSession{AgentID: "scopeless"})
	data, _ = proto.Marshal(&opsv1.DevopsAllowlistConfigRequest{AgentId: "scopeless"})
	_, err = svc.handleDevopsAllowlistConfig(ctx, data)
	require.ErrorContains(t, err, "missing game scope")

	// config model 未装配：另起裸 service
	bare := &ControlService{logger: slog.Default()}
	data, _ = proto.Marshal(&opsv1.DevopsAllowlistConfigRequest{AgentId: "x"})
	_, err = bare.handleDevopsAllowlistConfig(ctx, data)
	require.ErrorContains(t, err, "config model not configured")
}

func TestHandleDevopsAllowlistConfig_PublishAndDegrade(t *testing.T) {
	svc, store, db := newAllowlistTestService(t)
	ctx := context.Background()
	store.UpsertAgent(&registry.AgentSession{AgentID: "a-1", GameID: "demo", Env: "prod"})
	req := &opsv1.DevopsAllowlistConfigRequest{AgentId: "a-1"}
	data, _ := proto.Marshal(req)

	// 未发布任何版本 → 空响应（ ErrRecordNotFound 分支不报错）
	respData, err := svc.handleDevopsAllowlistConfig(ctx, data)
	require.NoError(t, err)
	var resp opsv1.DevopsAllowlistConfigResponse
	require.NoError(t, proto.Unmarshal(respData, &resp))
	require.Empty(t, resp.GetVersion())

	// 发布两个版本 → 取最新
	require.NoError(t, db.Create(&model.ConfigVersion{
		Key: devopsAllowlistConfigKey, Version: 3, Value: `{"hosts":["h1"]}`,
		GameID: "demo", Env: "prod",
	}).Error)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Key: devopsAllowlistConfigKey, Version: 7, Value: `{"hosts":["h1","h2"]}`,
		GameID: "demo", Env: "prod",
	}).Error)
	respData, err = svc.handleDevopsAllowlistConfig(ctx, data)
	require.NoError(t, err)
	require.NoError(t, proto.Unmarshal(respData, &resp))
	require.Equal(t, "7", resp.GetVersion())
	require.Contains(t, resp.GetAllowlistJson(), "h2")

	// 其他作用域不见这份发布
	store.UpsertAgent(&registry.AgentSession{AgentID: "a-2", GameID: "other", Env: "prod"})
	data2, _ := proto.Marshal(&opsv1.DevopsAllowlistConfigRequest{AgentId: "a-2"})
	respData, err = svc.handleDevopsAllowlistConfig(ctx, data2)
	require.NoError(t, err)
	require.NoError(t, proto.Unmarshal(respData, &resp))
	require.Empty(t, resp.GetVersion())

	// game DB 解析失败 → 降级空响应不报错
	svc.SetGameDBResolver(denyAllResolver{})
	respData, err = svc.handleDevopsAllowlistConfig(ctx, data)
	require.NoError(t, err)
	require.NoError(t, proto.Unmarshal(respData, &resp))
	require.Empty(t, resp.GetVersion())

	// 表被摘除（非 notfound 错误）→ 同样降级空响应
	require.NoError(t, db.Migrator().DropTable("config_versions"))
	respData, err = svc.handleDevopsAllowlistConfig(ctx, data)
	require.NoError(t, err)
	require.NoError(t, proto.Unmarshal(respData, &resp))
	require.Empty(t, resp.GetVersion())
}

func TestHandleRequestRoutesDevopsAllowlistConfig(t *testing.T) {
	svc, store, _ := newAllowlistTestService(t)
	store.UpsertAgent(&registry.AgentSession{AgentID: "a-1", GameID: "demo", Env: "prod"})
	data, _ := proto.Marshal(&opsv1.DevopsAllowlistConfigRequest{AgentId: "a-1"})
	resp, err := svc.handleRequest(context.Background(), protocol.MsgDevopsAllowlistConfigRequest, data)
	require.NoError(t, err)
	require.NotNil(t, resp)
}
