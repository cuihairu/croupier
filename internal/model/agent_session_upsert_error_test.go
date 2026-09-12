// 覆盖目标：AgentSessionModel.Upsert 与 toDBSession 的序列化错误分支。
// registry.ProviderSession.OpenAPIDoc 是 json.RawMessage，内容非法时
// json.Marshal 会报错（appendCompact 校验失败），这是 toDBSession 唯一
// 可达的失败路径。
package model

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentSessionModelUpsertMarshalError(t *testing.T) {
	db := newMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&AgentSessionDB{}))

	sess := &registry.AgentSession{
		AgentID:  "agent-bad-openapi",
		GameID:   "demo",
		Env:      "prod",
		ExpireAt: time.Now().Add(time.Hour),
		LastSeen: time.Now(),
		Providers: []registry.ProviderSession{
			{ProviderID: "p1", OpenAPIDoc: json.RawMessage("{not-json")},
		},
	}

	_, err := toDBSession(sess)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MarshalJSON")

	m := NewAgentSessionModel(db)
	err = m.Upsert(context.Background(), sess)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MarshalJSON")

	var count int64
	require.NoError(t, db.Model(&AgentSessionDB{}).Count(&count).Error)
	assert.EqualValues(t, 0, count, "failed upsert must not write any row")
}
