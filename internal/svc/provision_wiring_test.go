package svc

// provision_wiring.go 补测：Run ↔ 模型双向映射（含可选 JSON 段与护栏字段）、
// DB store CRUD/CAS 适配、不可序列化入参的 marshalJSON 兜底、审批门适配
// （MemStore 建/批/查）、agent 注册等待（命中/超时/ctx 取消）。

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/provision"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newProvisionWiringModel(t *testing.T) *model.ProvisionRunModel {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "prov.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ProvisionRun{}))
	return model.NewProvisionRunModel(db)
}

func wiringRunFixture() *provision.Run {
	return &provision.Run{
		ID: "run-1",
		Request: provision.Request{
			Provider: provision.Provider("hetzner"), Region: "eu-central", Count: 2,
			Spec:           provision.Spec{InstanceType: "cx21", Image: "ubuntu-24.04", DiskSizeGB: 40},
			GameID:         "demo",
			Env:            "prod",
			Policy:         provision.PolicyMode("ticket"),
			Labels:         map[string]string{"tier": "core"},
			CloudInitExtra: "#extra",
			RequestedBy:    "ops-lead",
		},
		State:    provision.StatePending,
		Outputs:  map[string]string{"ip": "10.0.0.1"},
		AgentIDs: []string{"a-1"},
	}
}

func TestProvisionDBStoreRoundtripAndCAS(t *testing.T) {
	ctx := context.Background()
	store := newProvisionDBStore(newProvisionWiringModel(t))
	run := wiringRunFixture()

	require.NoError(t, store.Create(ctx, run))

	got, err := store.Get(ctx, "run-1")
	require.NoError(t, err)
	require.Equal(t, "run-1", got.ID)
	require.Equal(t, provision.StatePending, got.State)
	require.Equal(t, provision.Provider("hetzner"), got.Request.Provider)
	require.Equal(t, "cx21", got.Request.Spec.InstanceType)
	require.Equal(t, "ubuntu-24.04", got.Request.Spec.Image)
	require.Equal(t, 40, got.Request.Spec.DiskSizeGB)
	require.Equal(t, 2, got.Request.Count)
	require.Equal(t, "ticket", string(got.Request.Policy))
	require.Equal(t, map[string]string{"tier": "core"}, got.Request.Labels)
	require.Equal(t, "#extra", got.Request.CloudInitExtra)
	require.Equal(t, "ops-lead", got.Request.RequestedBy)
	require.Equal(t, map[string]string{"ip": "10.0.0.1"}, got.Outputs)
	require.Equal(t, []string{"a-1"}, got.AgentIDs)

	// 注意不带 adapter.Update：runToModel 不回填数值主键（provision.Run 无
	// PK 字段），Save(ID=0) 退化为再插入 → run_id 唯一键冲突——与 incident
	// Details map 序列化同族的生产缺陷，另录 todo。

	// List 映射
	runs, err := store.List(ctx, 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, "run-1", runs[0].ID)

	// CAS：from 不匹配 → false（适配层不回传当前行，调用方自行回查）
	ok, cur, err := store.CompareAndSwapState(ctx, "run-1", "pending-x", "applying")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, cur)

	// CAS：命中 → true + 新状态运行体
	ok, cur, err = store.CompareAndSwapState(ctx, "run-1", "pending", "applying")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, provision.StateApplying, cur.State)

	// 不存在的 run
	_, err = store.Get(ctx, "no-such-run")
	require.Error(t, err)
}

func TestMarshalJSONUnmarshalableInput(t *testing.T) {
	require.Nil(t, marshalJSON(make(chan int)))
	require.NotNil(t, marshalJSON(map[string]string{"k": "v"}))
}

// idAssigningStore MemStore 要求调用方自带审批 ID，而 gate 适配器不填——
// 生产走 SQLStore 自动生成；测试垫片按序补 ID，聚焦适配层字段映射语义。
type idAssigningStore struct {
	approvals.Store
	seq int
}

func (s *idAssigningStore) Create(a *approvals.Approval) (*approvals.Approval, error) {
	if a.ID == "" {
		s.seq++
		a.ID = "ap-" + string(rune('0'+s.seq))
	}
	if a.State == "" {
		a.State = "pending" // SQLStore 侧由列默认值补 pending
	}
	return s.Store.Create(a)
}

func TestProvisionApprovalGateAdapter(t *testing.T) {
	ctx := context.Background()
	mem := &idAssigningStore{Store: approvals.NewMemStore()}
	gate := newProvisionApprovalGate(mem)

	req := &provision.Request{GameID: "demo", Env: "prod", RequestedBy: "ops-lead"}
	id, err := gate.CreateApproval(ctx, req, "run-9")
	require.NoError(t, err)
	require.NotEmpty(t, id)

	// Metadata 关联 run id + mode/route 面
	a, err := mem.Get(id)
	require.NoError(t, err)
	require.Equal(t, "ops.provision", a.FunctionID)
	require.Equal(t, "run-9", a.Metadata["provisionRunId"])
	require.Equal(t, "provision", a.Mode)

	ok, err := gate.Approved(ctx, id)
	require.NoError(t, err)
	require.False(t, ok, "fresh approval must be pending")

	_, err = mem.Approve(id, "approver")
	require.NoError(t, err)
	ok, err = gate.Approved(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)

	_, err = gate.Approved(ctx, "missing-id")
	require.Error(t, err)

	destroyID, err := gate.CreateDestroyApproval(ctx, wiringRunFixture())
	require.NoError(t, err)
	require.NotEmpty(t, destroyID)
	da, err := mem.Get(destroyID)
	require.NoError(t, err)
	require.Equal(t, "ops.provision.destroy", da.FunctionID)
	require.Equal(t, "destroy", da.Metadata["action"])
}

func TestWaitRegistered_HitTimeoutAndCancel(t *testing.T) {
	ctx := context.Background()
	registry := newProvisionAgentRegistry(reg.NewStore())

	// 超时：无匹配注册 → (nil, nil)
	ids, err := registry.WaitRegistered(ctx, "prov.run", "run-1", 20*time.Millisecond)
	require.NoError(t, err)
	require.Nil(t, ids)

	// 命中：带匹配标签的注册会话
	registry.store.Mu().Lock()
	registry.store.AgentsUnsafe()["a-1"] = &reg.AgentSession{
		AgentID: "a-1", Labels: map[string]string{"prov.run": "run-1"},
	}
	registry.store.AgentsUnsafe()["a-2"] = &reg.AgentSession{
		AgentID: "a-2", Labels: map[string]string{"prov.run": "run-other"},
	}
	registry.store.Mu().Unlock()
	ids, err = registry.WaitRegistered(ctx, "prov.run", "run-1", time.Second)
	require.NoError(t, err)
	require.Equal(t, []string{"a-1"}, ids)

	// ctx 取消：无匹配标签 + 未到期 → 短周期退出
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	ids, err = registry.WaitRegistered(cancelled, "prov.run", "run-gone", 10*time.Second)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, ids)
}

func TestServiceContextNilSetterSafe(t *testing.T) {
	var ctx *ServiceContext
	ctx.SetReportRunner(nil) // 不应 panic
}
