package server

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gosqlite "github.com/glebarez/sqlite"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// configVersionRow 指定 scope 的 config_versions 行构造。
func configVersionRow(key, gameID, env string, version int, value string) model.ConfigVersion {
	return model.ConfigVersion{Key: key, Version: version, Value: value, GameID: gameID, Env: env, Namespace: "ops"}
}

var errDBDown = errors.New("db down")

// newGameDBWithPolicy 独立 :memory: game 库，预置 probe.policy v7。
func newGameDBWithPolicy(t *testing.T, value string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gosqlite.Open("file:"+t.Name()+"-game?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open game sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ConfigVersion{}); err != nil {
		t.Fatalf("migrate config_versions: %v", err)
	}
	row := configVersionRow("probe.policy", "demo", "prod", 7, value)
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func mustMarshalPolicyReq(t *testing.T, req *opsv1.ProbePolicyConfigRequest) []byte {
	t.Helper()
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func mustUnmarshalPolicyResp(t *testing.T, data []byte) *opsv1.ProbePolicyConfigResponse {
	t.Helper()
	resp := &opsv1.ProbePolicyConfigResponse{}
	if err := proto.Unmarshal(data, resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return resp
}

func mustRunProbePolicy(t *testing.T, svc *ControlService, agentID string) []byte {
	t.Helper()
	data, err := svc.handleProbePolicyConfig(context.Background(), mustMarshalPolicyReq(t, &opsv1.ProbePolicyConfigRequest{AgentId: agentID}))
	if err != nil {
		t.Fatalf("handleProbePolicyConfig: %v", err)
	}
	return data
}

func TestHandleProbePolicyConfig_PassesThroughLatestVersion(t *testing.T) {
	svc, db := newConfigTestService(t)
	seedCaptureAgent(t, svc, "sidecar-01", "demo", "prod")
	policyJSON := `{"interval": "30s", "targets": [{"target": "db-primary", "interval": "10s"}]}`
	row3 := configVersionRow("probe.policy", "demo", "prod", 3, "stale")
	if err := db.Create(&row3).Error; err != nil {
		t.Fatal(err)
	}
	row7 := configVersionRow("probe.policy", "demo", "prod", 7, policyJSON)
	if err := db.Create(&row7).Error; err != nil {
		t.Fatal(err)
	}

	resp := mustUnmarshalPolicyResp(t, mustRunProbePolicy(t, svc, "sidecar-01"))
	if resp.GetVersion() != "7" || resp.GetPolicyJson() != policyJSON {
		t.Fatalf("resp = %+v, want version 7 with policy json", resp)
	}
}

func TestHandleProbePolicyConfig_EmptyWhenNotPublished(t *testing.T) {
	svc, _ := newConfigTestService(t)
	seedCaptureAgent(t, svc, "sidecar-01", "demo", "prod")

	resp := mustUnmarshalPolicyResp(t, mustRunProbePolicy(t, svc, "sidecar-01"))
	if resp.GetVersion() != "" || resp.GetPolicyJson() != "" {
		t.Fatalf("resp = %+v, want empty (keep local intervals)", resp)
	}
}

func TestHandleProbePolicyConfig_ScopeIsolatedBySession(t *testing.T) {
	svc, db := newConfigTestService(t)
	// agent 会话属 demo/prod；发布的是 demo/dev → 不串。
	seedCaptureAgent(t, svc, "sidecar-01", "demo", "prod")
	row9 := configVersionRow("probe.policy", "demo", "dev", 9, "{}")
	if err := db.Create(&row9).Error; err != nil {
		t.Fatal(err)
	}

	resp := mustUnmarshalPolicyResp(t, mustRunProbePolicy(t, svc, "sidecar-01"))
	if resp.GetVersion() != "" {
		t.Fatalf("resp = %+v, want empty for other env", resp)
	}
}

func TestHandleProbePolicyConfig_UnknownAgentAndNilModel(t *testing.T) {
	svc, _ := newConfigTestService(t)
	if _, err := svc.handleProbePolicyConfig(context.Background(), mustMarshalPolicyReq(t, &opsv1.ProbePolicyConfigRequest{AgentId: "ghost"})); err == nil {
		t.Fatal("unknown agent must error")
	}
	if _, err := svc.handleProbePolicyConfig(context.Background(), mustMarshalPolicyReq(t, &opsv1.ProbePolicyConfigRequest{AgentId: ""})); err == nil {
		t.Fatal("empty agent id must error")
	}
	svc.SetConfigVersionModel(nil)
	seedCaptureAgent(t, svc, "sidecar-01", "demo", "prod")
	if _, err := svc.handleProbePolicyConfig(context.Background(), mustMarshalPolicyReq(t, &opsv1.ProbePolicyConfigRequest{AgentId: "sidecar-01"})); err == nil {
		t.Fatal("nil config model must error")
	}
}

func TestHandleProbePolicyConfig_GameDBResolverOverride(t *testing.T) {
	svc, _ := newConfigTestService(t)
	seedCaptureAgent(t, svc, "sidecar-01", "demo", "prod")

	// 独立 game 库：策略只发布在这里；meta 库（svc 里的）为空。
	gameDB := newGameDBWithPolicy(t, `{"interval": "15s"}`)
	svc.SetGameDBResolver(&fakeGameDBResolver{gameDB: gameDB})

	resp := mustUnmarshalPolicyResp(t, mustRunProbePolicy(t, svc, "sidecar-01"))
	if resp.GetVersion() != "7" {
		t.Fatalf("resp = %+v, want game-db override to resolve version 7", resp)
	}
}

func TestHandleProbePolicyConfig_ResolverFailureReturnsEmpty(t *testing.T) {
	svc, _ := newConfigTestService(t)
	seedCaptureAgent(t, svc, "sidecar-01", "demo", "prod")
	svc.SetGameDBResolver(&fakeGameDBResolver{err: errDBDown})

	resp := mustUnmarshalPolicyResp(t, mustRunProbePolicy(t, svc, "sidecar-01"))
	if resp.GetVersion() != "" || resp.GetPolicyJson() != "" {
		t.Fatalf("resp = %+v, want empty on resolver failure", resp)
	}
}

func TestProbePolicyMsgIDPair(t *testing.T) {
	if protocol.MsgProbePolicyResponse != protocol.MsgProbePolicyRequest+1 {
		t.Fatal("probe policy response must be request+1")
	}
	if !protocol.IsRequest(protocol.MsgProbePolicyRequest) || !protocol.IsResponse(protocol.MsgProbePolicyResponse) {
		t.Fatal("probe policy msg id parity broken")
	}
}
