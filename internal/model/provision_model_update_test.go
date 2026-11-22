package model

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// AUD-019：ProvisionRunModel.Update 按业务键 run_id 更新既有记录。
//
// 传入行由 provision.Run 重新映射而来（见 svc.runToModel）——该运行体无数值
// 主键字段，映射后 ID 恒为零；此前 Update 直接 Save(ID=0)，退化为再 INSERT
// 并触发 run_id 唯一键冲突。此用例复刻生产形态：Create 后再用 ID=0 行写回。
func TestProvisionRunModel_UpdateByBusinessKey(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ProvisionRun{}))
	m := NewProvisionRunModel(db)
	ctx := context.Background()

	created := &ProvisionRun{
		RunID: "prov-upd", Provider: "hetzner", Region: "eu-central", GameID: "demo", Env: "prod",
		Policy: "ticket", Count: 2, State: "pending", RequestedBy: "ops-lead",
	}
	require.NoError(t, m.Create(ctx, created))
	require.NotZero(t, created.ID)

	// ticket 审批关联：重映射行（ID=0）写回 approval id。
	link := &ProvisionRun{
		RunID: "prov-upd", Provider: "hetzner", Region: "eu-central", GameID: "demo", Env: "prod",
		Policy: "ticket", Count: 2, State: "pending", ApprovalID: "ap-1", RequestedBy: "ops-lead",
	}
	require.NoError(t, m.Update(ctx, link))

	got, err := m.FindByRunID(ctx, "prov-upd")
	require.NoError(t, err)
	require.Equal(t, "ap-1", got.ApprovalID)
	require.Equal(t, created.ID, got.ID, "业务键更新不得换行/换主键")

	var count int64
	require.NoError(t, db.Model(&ProvisionRun{}).Where("run_id = ?", "prov-upd").Count(&count).Error)
	require.EqualValues(t, 1, count, "Create→Update→Get 必须只保留一行")

	// apply 后各状态持久化：状态、outputs、agent_ids、错误信息全字段回写。
	got.State = "adopted"
	got.OutputsJSON = []byte(`{"ip":"10.0.0.1"}`)
	got.AgentIDsJSON = []byte(`["a-1"]`)
	require.NoError(t, m.Update(ctx, got))

	again, err := m.FindByRunID(ctx, "prov-upd")
	require.NoError(t, err)
	assert.Equal(t, "adopted", again.State)
	assert.JSONEq(t, `{"ip":"10.0.0.1"}`, string(again.OutputsJSON))
	assert.JSONEq(t, `["a-1"]`, string(again.AgentIDsJSON))

	require.NoError(t, db.Model(&ProvisionRun{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	// 不存在的 run：报 ErrRecordNotFound，且不得隐式插入新行。
	err = m.Update(ctx, &ProvisionRun{RunID: "prov-none", State: "pending"})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, db.Model(&ProvisionRun{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
