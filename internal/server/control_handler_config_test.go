package server

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gosqlite "github.com/glebarez/sqlite"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newConfigTestService：:memory: sqlite + config_versions 表 + 注入
// ControlService（单连接防 :memory: 多实例库漂移）。
func newConfigTestService(t *testing.T) (*ControlService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gosqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.ConfigVersion{}); err != nil {
		t.Fatalf("migrate config_versions: %v", err)
	}
	svc := NewControlService(reg.NewStore(), nil)
	svc.SetConfigVersionModel(model.NewConfigVersionModel(db))
	return svc, db
}

// seedCaptureAgent 直接灌注册会话（UpsertAgent 全量物化链对拉取测试冗余）。
func seedCaptureAgent(t *testing.T, svc *ControlService, agentID, gameID, env string) {
	t.Helper()
	svc.registry.Mu().Lock()
	svc.registry.AgentsUnsafe()[agentID] = &reg.AgentSession{AgentID: agentID, GameID: gameID, Env: env}
	svc.registry.Mu().Unlock()
}

func mustMarshalGatesReq(t *testing.T, req *opsv1.CaptureGatesConfigRequest) []byte {
	t.Helper()
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func mustUnmarshalGatesResp(t *testing.T, data []byte) *opsv1.CaptureGatesConfigResponse {
	t.Helper()
	resp := &opsv1.CaptureGatesConfigResponse{}
	if err := proto.Unmarshal(data, resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return resp
}

func TestHandleCaptureConfig_PassesThroughLatestVersion(t *testing.T) {
	svc, db := newConfigTestService(t)
	seedCaptureAgent(t, svc, "capture-01", "demo", "prod")
	gatesJSON := `{"reconcile":{"rules":[{"name":"item-ledger"}]}}`
	if err := db.Create(&model.ConfigVersion{
		Key:       "capture.gates",
		Version:   3,
		Value:     "stale",
		GameID:    "demo",
		Env:       "prod",
		Namespace: "ops",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ConfigVersion{
		Key:       "capture.gates",
		Version:   7,
		Value:     gatesJSON,
		GameID:    "demo",
		Env:       "prod",
		Namespace: "ops",
	}).Error; err != nil {
		t.Fatal(err)
	}

	resp := mustUnmarshalGatesResp(t, mustRunCaptureConfig(t, svc, "capture-01"))
	if resp.GetVersion() != "7" || resp.GetGatesJson() != gatesJSON {
		t.Fatalf("resp = %+v, want version 7 with gates json", resp)
	}
}

func TestHandleCaptureConfig_EmptyWhenNotPublished(t *testing.T) {
	svc, _ := newConfigTestService(t)
	seedCaptureAgent(t, svc, "capture-01", "demo", "prod")

	resp := mustUnmarshalGatesResp(t, mustRunCaptureConfig(t, svc, "capture-01"))
	if resp.GetVersion() != "" || resp.GetGatesJson() != "" {
		t.Fatalf("resp = %+v, want empty (keep file rules)", resp)
	}
}

func TestHandleCaptureConfig_ScopeIsolatedBySession(t *testing.T) {
	svc, db := newConfigTestService(t)
	// agent 会话属 demo/prod；发布的是 demo/dev → 不串。
	seedCaptureAgent(t, svc, "capture-01", "demo", "prod")
	if err := db.Create(&model.ConfigVersion{
		Key: "capture.gates", Version: 9, Value: "{}",
		GameID: "demo", Env: "dev", Namespace: "ops",
	}).Error; err != nil {
		t.Fatal(err)
	}

	resp := mustUnmarshalGatesResp(t, mustRunCaptureConfig(t, svc, "capture-01"))
	if resp.GetVersion() != "" {
		t.Fatalf("resp = %+v, want empty for other env", resp)
	}
}

func TestHandleCaptureConfig_UnknownAgentAndNilModel(t *testing.T) {
	svc, _ := newConfigTestService(t)
	if _, err := svc.handleCaptureConfig(context.Background(), mustMarshalGatesReq(t, &opsv1.CaptureGatesConfigRequest{AgentId: "ghost"})); err == nil {
		t.Fatal("unknown agent must error")
	}
	svc.SetConfigVersionModel(nil)
	seedCaptureAgent(t, svc, "capture-01", "demo", "prod")
	if _, err := svc.handleCaptureConfig(context.Background(), mustMarshalGatesReq(t, &opsv1.CaptureGatesConfigRequest{AgentId: "capture-01"})); err == nil {
		t.Fatal("nil config model must error")
	}
}

// fakeGameDBResolver GameDBResolver 测试桩：返回带独立 game 库覆盖的 ctx。
type fakeGameDBResolver struct {
	gameDB *gorm.DB
	err    error
}

func (f *fakeGameDBResolver) Resolve(ctx context.Context, gameID, env string) (context.Context, *gorm.DB, error) {
	if f.err != nil {
		return nil, nil, fmt.Errorf("resolve: %w", f.err)
	}
	return dbctx.WithDB(ctx, f.gameDB), f.gameDB, nil
}

func TestHandleCaptureConfig_GameDBResolverOverride(t *testing.T) {
	svc, _ := newConfigTestService(t)
	seedCaptureAgent(t, svc, "capture-01", "demo", "prod")

	// 独立 game 库：规则只发布在这里；meta 库（svc 里的）为空。
	gameDB, err := gorm.Open(gosqlite.Open("file:"+t.Name()+"-game?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := gameDB.AutoMigrate(&model.ConfigVersion{}); err != nil {
		t.Fatal(err)
	}
	if err := gameDB.Create(&model.ConfigVersion{
		Key: "capture.gates", Version: 7, Value: `{"lineage":{"rules":[]}}`,
		GameID: "demo", Env: "prod", Namespace: "ops",
	}).Error; err != nil {
		t.Fatal(err)
	}
	svc.SetGameDBResolver(&fakeGameDBResolver{gameDB: gameDB})

	resp := mustUnmarshalGatesResp(t, mustRunCaptureConfig(t, svc, "capture-01"))
	if resp.GetVersion() != "7" {
		t.Fatalf("resp = %+v, want game-db override to resolve version 7", resp)
	}
}

func TestHandleCaptureConfig_ResolverFailureReturnsEmpty(t *testing.T) {
	svc, _ := newConfigTestService(t)
	seedCaptureAgent(t, svc, "capture-01", "demo", "prod")
	svc.SetGameDBResolver(&fakeGameDBResolver{err: errors.New("db down")})

	resp := mustUnmarshalGatesResp(t, mustRunCaptureConfig(t, svc, "capture-01"))
	if resp.GetVersion() != "" || resp.GetGatesJson() != "" {
		t.Fatalf("resp = %+v, want empty on resolver failure", resp)
	}
}

func mustRunCaptureConfig(t *testing.T, svc *ControlService, agentID string) []byte {
	t.Helper()
	data, err := svc.handleCaptureConfig(context.Background(), mustMarshalGatesReq(t, &opsv1.CaptureGatesConfigRequest{AgentId: agentID}))
	if err != nil {
		t.Fatalf("handleCaptureConfig: %v", err)
	}
	return data
}

func TestCaptureConfigMsgIDPair(t *testing.T) {
	if protocol.MsgCaptureConfigResponse != protocol.MsgCaptureConfigRequest+1 {
		t.Fatal("capture config response must be request+1")
	}
	if !protocol.IsRequest(protocol.MsgCaptureConfigRequest) || !protocol.IsResponse(protocol.MsgCaptureConfigResponse) {
		t.Fatal("capture config msg id parity broken")
	}
}
