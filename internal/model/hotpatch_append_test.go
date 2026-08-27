package model

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestHotpatchModel_AppendResult_MultiAgent 回归：AppendResult 语义是
// per-agent last-wins 的跨 agent 累积——第二个 agent 上报不得静默丢弃
// 第一个 agent 的存量结果（曾因漏反序列化只留最后一条）。
func TestHotpatchModel_AppendResult_MultiAgent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/hotpatch.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Hotpatch{}))
	m := NewHotpatchModel(db)

	hp := &Hotpatch{GameID: "g", Env: "prod", Framework: "skynet", Status: "rolling"}
	require.NoError(t, db.Create(hp).Error)

	require.NoError(t, m.AppendResult(context.Background(), hp.ID,
		HotpatchResult{AgentID: "agent-1", Status: "ok", At: "t1"}))
	require.NoError(t, m.AppendResult(context.Background(), hp.ID,
		HotpatchResult{AgentID: "agent-2", Status: "failed", At: "t2"}))
	require.NoError(t, m.AppendResult(context.Background(), hp.ID,
		HotpatchResult{AgentID: "agent-1", Status: "rolled_back", At: "t3"}))

	var got Hotpatch
	require.NoError(t, db.First(&got, hp.ID).Error)
	var results []HotpatchResult
	require.NoError(t, json.Unmarshal(got.Results, &results))

	assert.Len(t, results, 2, "两个 agent 的结果都应保留")
	byAgent := map[string]HotpatchResult{}
	for _, r := range results {
		byAgent[r.AgentID] = r
	}
	assert.Equal(t, "rolled_back", byAgent["agent-1"].Status, "同 agent 重复上报 last-wins")
	assert.Equal(t, "failed", byAgent["agent-2"].Status, "其他 agent 的存量结果不得被覆盖")
}
