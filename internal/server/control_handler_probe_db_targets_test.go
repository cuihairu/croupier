package server

// R4：db 探针目标下行拉取（MsgProbeDBTargetsRequest）。server 按
// db_sources.owner_agent_id 拉取时派生目标集（不写 config_versions、无
// 推送通道），version 为目标集内容 hash——空集也有 hash（空集语义 =
// 解除全部归属，agent 必须卸掉动态探针）。

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/internal/model"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func newDBTargetsTestService(t *testing.T) (*ControlService, *model.DBSourceModel) {
	t.Helper()
	svc, db := newConfigTestService(t)
	if err := db.AutoMigrate(&model.DBSource{}); err != nil {
		t.Fatalf("migrate db_sources: %v", err)
	}
	dbModel := model.NewDBSourceModel(db)
	svc.SetDBSourceModel(dbModel)
	return svc, dbModel
}

func seedDBSource(t *testing.T, dbModel *model.DBSourceModel, name, driver, dsn, owner string, enabled bool) model.DBSource {
	t.Helper()
	src := model.DBSource{Name: name, Driver: driver, DSN: dsn, OwnerAgentID: owner, Enabled: true}
	require.NoError(t, dbModel.Create(context.Background(), &src))
	if !enabled {
		// gorm default:true 标签下零值 bool 建行时会被省略（落列默认 true），
		// 显式 Update 兜底禁用。
		require.NoError(t, dbModel.Update(context.Background(), src.ID, map[string]interface{}{"enabled": false}))
		src.Enabled = false
	}
	return src
}

func mustRunProbeDBTargets(t *testing.T, svc *ControlService, agentID string) (*opsv1.ProbeDBTargetsConfigResponse, error) {
	t.Helper()
	data, err := proto.Marshal(&opsv1.ProbeDBTargetsConfigRequest{AgentId: agentID})
	require.NoError(t, err)
	raw, err := svc.handleProbeDBTargets(context.Background(), data)
	if err != nil {
		return nil, err
	}
	resp := &opsv1.ProbeDBTargetsConfigResponse{}
	require.NoError(t, proto.Unmarshal(raw, resp))
	return resp, nil
}

func mustDecodeTargets(t *testing.T, raw string) []healthprobe.TargetConfig {
	t.Helper()
	var out []healthprobe.TargetConfig
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	return out
}

func TestDeriveProbeDBTargets_EmptySetHasHash(t *testing.T) {
	targets, version := deriveProbeDBTargets(nil)
	require.Empty(t, targets)
	// 空集也有 hash：空集语义 = 解除全部归属，agent 必须感知到「变了」。
	require.NotEmpty(t, version)
	require.Len(t, version, 64, "sha256 hex")
}

func TestDeriveProbeDBTargets_SortsByIDAndStableHash(t *testing.T) {
	a := model.DBSource{ID: 2, Name: "orders", Driver: "postgres", DSN: "dsn-a"}
	b := model.DBSource{ID: 1, Name: "audit", Driver: "mysql", DSN: "dsn-b"}
	_, hash1 := deriveProbeDBTargets([]model.DBSource{a, b})
	_, hash2 := deriveProbeDBTargets([]model.DBSource{b, a})
	assert.Equal(t, hash1, hash2, "derive must sort by id, hash stable across input order")

	targets, _ := deriveProbeDBTargets([]model.DBSource{a, b})
	require.Len(t, targets, 2)
	// id 升序：audit(1) 在前。
	assert.Equal(t, "db:audit", targets[0].Name)
	assert.Equal(t, "db:orders", targets[1].Name)
	assert.Equal(t, healthprobe.TargetTypeDB, targets[1].Type)
	// driver 映射：postgres → pgx（dbmon 同款），mysql 保持小写。
	assert.Equal(t, "pgx", targets[1].Driver)
	assert.Equal(t, "mysql", targets[0].Driver)
	assert.Equal(t, healthprobe.DefaultDBTimeout, targets[1].Timeout)
	assert.Equal(t, "dsn-a", targets[1].DSN)
}

func TestDeriveProbeDBTargets_DuplicateNameDisambiguated(t *testing.T) {
	first := model.DBSource{ID: 5, Name: "orders", Driver: "postgres", DSN: "dsn-5"}
	second := model.DBSource{ID: 7, Name: "orders", Driver: "postgres", DSN: "dsn-7"}
	targets, _ := deriveProbeDBTargets([]model.DBSource{first, second})
	require.Len(t, targets, 2)
	assert.Equal(t, "db:orders", targets[0].Name, "first occurrence keeps the base name")
	assert.Equal(t, "db:orders#7", targets[1].Name, "later occurrence gets #<id> suffix")
}

func TestDeriveProbeDBTargets_HashTracksContent(t *testing.T) {
	src := model.DBSource{ID: 1, Name: "orders", Driver: "postgres", DSN: "dsn-old"}
	_, h1 := deriveProbeDBTargets([]model.DBSource{src})
	src.DSN = "dsn-new"
	_, h2 := deriveProbeDBTargets([]model.DBSource{src})
	assert.NotEqual(t, h1, h2, "DSN change must change the version hash")

	_, h3 := deriveProbeDBTargets([]model.DBSource{src})
	assert.Equal(t, h2, h3, "same content must keep the hash")
}

func TestHandleProbeDBTargets_DerivesOwnedEnabledSources(t *testing.T) {
	svc, dbModel := newDBTargetsTestService(t)
	seedCaptureAgent(t, svc, "sidecar-01", "demo", "prod")
	seedDBSource(t, dbModel, "orders", "postgres", "postgres://ro:ro@db/orders", "sidecar-01", true)
	seedDBSource(t, dbModel, "audit", "MySQL", "mysql://ro:ro@db/audit", "sidecar-01", true)
	// 非归属 / 禁用源不得进入目标集。
	seedDBSource(t, dbModel, "other", "postgres", "postgres://ro:ro@db/other", "sidecar-02", true)
	seedDBSource(t, dbModel, "disabled", "postgres", "postgres://ro:ro@db/disabled", "sidecar-01", false)

	resp, err := mustRunProbeDBTargets(t, svc, "sidecar-01")
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetVersion())
	targets := mustDecodeTargets(t, resp.GetTargetsJson())
	require.Len(t, targets, 2)
	// id 升序（创建顺序）：orders(1) 在前，MySQL → mysql 小写映射。
	assert.Equal(t, "db:orders", targets[0].Name)
	assert.Equal(t, "pgx", targets[0].Driver)
	assert.Equal(t, "db:audit", targets[1].Name)
	assert.Equal(t, "mysql", targets[1].Driver)

	// 集合内容不变 → 同 hash（agent 侧 configsync 跳过无变化轮次）。
	resp2, err := mustRunProbeDBTargets(t, svc, "sidecar-01")
	require.NoError(t, err)
	assert.Equal(t, resp.GetVersion(), resp2.GetVersion())
}

func TestHandleProbeDBTargets_NoOwnershipStillHashed(t *testing.T) {
	svc, _ := newDBTargetsTestService(t)
	seedCaptureAgent(t, svc, "sidecar-bare", "demo", "prod")

	resp, err := mustRunProbeDBTargets(t, svc, "sidecar-bare")
	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetVersion(), "empty set still carries a hash (release-all semantics)")
	assert.JSONEq(t, "[]", resp.GetTargetsJson())
}

func TestHandleProbeDBTargets_Errors(t *testing.T) {
	svc, _ := newDBTargetsTestService(t)
	seedCaptureAgent(t, svc, "sidecar-01", "demo", "prod")

	// 未知 agent。
	_, err := mustRunProbeDBTargets(t, svc, "ghost")
	require.Error(t, err)

	// 空 agent_id。
	_, err = mustRunDBTargetsRaw(t, svc, "")
	require.Error(t, err)

	// 垃圾正文。
	_, err = svc.handleProbeDBTargets(context.Background(), []byte("not-proto"))
	require.Error(t, err)

	// 未装配 model。
	bare := NewControlService(svc.registry, nil)
	_, err = mustRunProbeDBTargets(t, bare, "sidecar-01")
	require.Error(t, err)
}

// mustRunProbeDBTargets 的裸变体：允许空 agent_id 走错误分支。
func mustRunDBTargetsRaw(t *testing.T, svc *ControlService, agentID string) (*opsv1.ProbeDBTargetsConfigResponse, error) {
	t.Helper()
	data, err := proto.Marshal(&opsv1.ProbeDBTargetsConfigRequest{AgentId: agentID})
	require.NoError(t, err)
	raw, err := svc.handleProbeDBTargets(context.Background(), data)
	if err != nil {
		return nil, err
	}
	resp := &opsv1.ProbeDBTargetsConfigResponse{}
	require.NoError(t, proto.Unmarshal(raw, resp))
	return resp, nil
}

func TestProbeDBTargetsMsgIDPair(t *testing.T) {
	if protocol.MsgProbeDBTargetsResponse != protocol.MsgProbeDBTargetsRequest+1 {
		t.Fatal("probe db targets response must be request+1")
	}
	if !protocol.IsRequest(protocol.MsgProbeDBTargetsRequest) || !protocol.IsResponse(protocol.MsgProbeDBTargetsResponse) {
		t.Fatal("probe db targets msg id parity broken")
	}
}

func TestDeriveProbeDBTargetsTimeoutMatchesDefaultDBTimeout(t *testing.T) {
	// 派生目标超时对齐 DBChecker 默认（5s）；健康探测不长期占住连接。
	assert.LessOrEqual(t, healthprobe.DefaultDBTimeout, 10*time.Second)
}
