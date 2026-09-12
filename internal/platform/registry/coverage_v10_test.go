// 覆盖目标（组 J）：
//   - agent_session_db.go LoadActiveSessionsByAgentIDs：空 ID 列表、命中/过期/脏行过滤、查询失败
//   - agent_session_db.go toDBSession：Providers 非空时的 JSON 序列化分支
//   - store.go UpsertAgent：db 已配置 + scope ctx 已注入但无 contract service 的直写分支及其失败路径
//   - store.go UpsertRegistrationWarning：已有条目缺失 AgentID/FunctionID 时的补全合并
//   - store.go UpsertOpenAPI/cloneOpenAPIOperation：克隆序列化失败与扩展字段反序列化失败
package registry

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentSessionModel_LoadActiveSessionsByAgentIDsV10(t *testing.T) {
	db := setupTestDB(t)
	m := NewAgentSessionModel(db)
	ctx := context.Background()
	now := time.Now()

	// 空 ID 列表：不触发查询，直接返回 nil
	sessions, err := m.LoadActiveSessionsByAgentIDs(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, sessions)
	sessions, err = m.LoadActiveSessionsByAgentIDs(ctx, []string{})
	require.NoError(t, err)
	assert.Nil(t, sessions)

	// 命中行携带 Providers；过期行与脏 JSON 行都必须被过滤
	require.NoError(t, m.Upsert(ctx, &AgentSession{
		AgentID:   "byid-hit",
		GameID:    "g",
		Env:       "e",
		ExpireAt:  now.Add(time.Hour),
		LastSeen:  now,
		Providers: []ProviderSession{{ProviderID: "p-hit", SDKLanguage: "go"}},
	}))
	require.NoError(t, m.Upsert(ctx, &AgentSession{
		AgentID:  "byid-stale",
		GameID:   "g",
		Env:      "e",
		ExpireAt: now.Add(-time.Hour),
		LastSeen: now,
	}))
	require.NoError(t, db.Exec(`INSERT INTO agent_sessions
		(agent_id, game_id, env, labels, functions, providers, expire_at, last_seen, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"byid-corrupt", "g", "e", "not-json", "{}", "[]",
		now.Add(time.Hour), now, now, now).Error)

	sessions, err = m.LoadActiveSessionsByAgentIDs(ctx,
		[]string{"byid-hit", "byid-stale", "byid-corrupt", "byid-missing"})
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "byid-hit", sessions[0].AgentID)
	require.Len(t, sessions[0].Providers, 1)
	assert.Equal(t, "p-hit", sessions[0].Providers[0].ProviderID)

	// 查询失败分支
	require.NoError(t, db.Migrator().DropTable(&AgentSessionDB{}))
	_, err = m.LoadActiveSessionsByAgentIDs(ctx, []string{"byid-hit"})
	require.Error(t, err)
}

func TestUpsertAgentScopeDBWithoutContractServiceV10(t *testing.T) {
	db := setupTestDB(t)
	s := NewStoreWithDB(db)
	s.SetScopeContextResolver(func(gameID, env string) context.Context {
		return dbctx.WithDB(context.Background(), db)
	})
	// 无 contract service：scope ctx 已注入时既不走单库事务，也不走跨库补偿，
	// 直接物化（no-op）后把会话写进 meta 库。
	now := time.Now()
	require.NoError(t, s.UpsertAgent(&AgentSession{
		AgentID:   "a-scope-db",
		GameID:    "g",
		Env:       "e",
		Functions: map[string]FunctionMeta{"fn": {Enabled: true}},
		ExpireAt:  now.Add(time.Hour),
		LastSeen:  now,
	}))

	var count int64
	require.NoError(t, db.Model(&AgentSessionDB{}).Where("agent_id = ?", "a-scope-db").Count(&count).Error)
	assert.Equal(t, int64(1), count)
	s.mu.RLock()
	require.NotNil(t, s.agents["a-scope-db"])
	s.mu.RUnlock()

	// 会话表缺失 → writeToDB 失败，注册整体报错且内存不可见
	require.NoError(t, db.Migrator().DropTable(&AgentSessionDB{}))
	err := s.UpsertAgent(&AgentSession{
		AgentID:   "a-scope-db-2",
		GameID:    "g",
		Env:       "e",
		Functions: map[string]FunctionMeta{"fn": {Enabled: true}},
		ExpireAt:  now.Add(time.Hour),
		LastSeen:  now,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write agent session to database")
	s.mu.RLock()
	assert.Nil(t, s.agents["a-scope-db-2"])
	s.mu.RUnlock()
}

func TestUpsertRegistrationWarningMergesMissingFieldsV10(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	// 首次入库：AgentID/FunctionID/Version 均缺失
	require.NoError(t, s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
		Key:     "merge-key",
		GameID:  "g",
		Env:     "e",
		Code:    "schema_mismatch",
		Message: "boom",
	}))
	// 同 key 再来一条补齐这些字段 → 合并进已有条目并累加计数
	require.NoError(t, s.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
		Key:        "merge-key",
		GameID:     "g",
		Env:        "e",
		Code:       "schema_mismatch",
		Message:    "boom",
		AgentID:    "agent-x",
		FunctionID: "fn-x",
		Version:    "1.2.3",
	}))

	items := s.ListRegistrationWarnings(RegistrationWarningFilter{GameID: "g"})
	require.Len(t, items, 1)
	assert.Equal(t, "agent-x", items[0].AgentID)
	assert.Equal(t, "fn-x", items[0].FunctionID)
	assert.Equal(t, "1.2.3", items[0].Version)
	assert.Equal(t, 2, items[0].Count)
	assert.False(t, items[0].FirstSeen.IsZero())
	assert.False(t, items[0].LastSeen.IsZero())
}

func TestUpsertOpenAPICloneMarshalFailureV10(t *testing.T) {
	s := NewStore()

	// 扩展字段携带不可 JSON 序列化的值 → Operation.MarshalJSON 失败
	op := openapi3.NewOperation()
	op.Extensions = map[string]interface{}{"x-bad": make(chan int)}
	err := s.UpsertOpenAPI("fn-chan", op)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clone operation failed")

	// 同一错误在 cloneOpenAPIOperation 内部的返回分支
	_, err = cloneOpenAPIOperation(&openapi3.Operation{
		Extensions: map[string]interface{}{"x-bad": make(chan int)},
	})
	require.Error(t, err)
}

func TestCloneOpenAPIOperationUnmarshalFailureV10(t *testing.T) {
	// 扩展字段 key 与内建字段重名且类型不符（tags=数字）：序列化成功，
	// 但反序列化回 Operation 时 []string 字段收不下 number → 失败。
	_, err := cloneOpenAPIOperation(&openapi3.Operation{
		Extensions: map[string]interface{}{"tags": 42},
	})
	require.Error(t, err)
}
