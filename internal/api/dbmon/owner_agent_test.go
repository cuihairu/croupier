package dbmon

// R4：DB 源归属 agent（ownerAgentId）的 API 面——创建携带、更新换绑/解
// 归属、未提交不动现值。归属 agent 的周期探测下发消费面在 server
// control_handler（按 owner_agent_id 派生 db 目标）。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBMonSource_OwnerAgentRoundTrip(t *testing.T) {
	r, _, _, _ := newDBMonTestEnv(t)

	// 创建带归属。
	rec := doDBMonReq(r, http.MethodPost, "/dbmon/sources", fmt.Sprintf(`{
		"name":"归属主库","driver":"mysql","kind":"self","dsn":%q,
		"gameId":"demo","env":"prod","ownerAgentId":"agent-sidecar-1"
	}`, goodDSN))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var created SourceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "agent-sidecar-1", created.OwnerAgentId)

	// 换绑归属 agent。
	rec = doDBMonReq(r, http.MethodPut, fmt.Sprintf("/dbmon/sources/%d", created.Id),
		`{"ownerAgentId":"agent-devops-2"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var updated SourceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.Equal(t, "agent-devops-2", updated.OwnerAgentId)

	// 空串 = 解归属（回落 server 诊断端点兜底）。
	rec = doDBMonReq(r, http.MethodPut, fmt.Sprintf("/dbmon/sources/%d", created.Id),
		`{"ownerAgentId":""}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// 新变量：clear 响应因 omitempty 缺 ownerAgentId 键，复用 updated 会残留旧值。
	var cleared SourceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cleared))
	assert.Empty(t, cleared.OwnerAgentId)

	// 未提交归属字段：不改动现值（指针语义，与「空串=解归属」区分）。
	rec = doDBMonReq(r, http.MethodPut, fmt.Sprintf("/dbmon/sources/%d", created.Id),
		`{"ownerAgentId":"agent-sidecar-3","name":"改名不丢归属"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var rebound SourceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rebound))
	assert.Equal(t, "agent-sidecar-3", rebound.OwnerAgentId)
	assert.Equal(t, "改名不丢归属", rebound.Name)
}
