package dbmon

// R4: the API surface for the agent a DB source is owned by (ownerAgentId)
// -- carried on create, rebind / unbind on update, and left untouched when
// the field is not submitted. The periodic probe dispatch consumed by the
// owning agent lives in the server control_handler (it derives db targets by
// owner_agent_id).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBMonSource_RejectsOwnerOutsideSourceScope(t *testing.T) {
	dbmonSvc, srcModel, _ := newDBMonFixture(t)
	require.NoError(t, registry.MigrateAgentSessions(dbmonSvc.svcCtx.DB))
	ctx := context.Background()
	expire := time.Now().Add(time.Hour)
	require.NoError(t, registry.NewAgentSessionModel(dbmonSvc.svcCtx.DB).Upsert(ctx, &registry.AgentSession{
		AgentID: "agent-other", GameID: "other", Env: "dev",
		ExpireAt: expire, LastSeen: time.Now(),
	}))

	_, err := dbmonSvc.CreateSource(ctx, &SourceUpsertRequest{Name: "scoped", Driver: "mysql", DSN: goodDSN,
		GameID: "demo", Env: "prod", OwnerAgentID: "agent-other"})
	require.ErrorContains(t, err, "ownerAgentId does not belong")

	require.NoError(t, srcModel.Create(ctx, &model.DBSource{Name: "scoped", Driver: "mysql", DSN: goodDSN,
		Kind: model.DBSourceKindSelf, GameID: "demo", Env: "prod", Enabled: true}))
	items, err := srcModel.List(ctx)
	require.NoError(t, err)
	otherGame := "other"
	_, err = dbmonSvc.UpdateSource(ctx, &SourceUpdateRequest{
		ID: fmt.Sprintf("%d", items[0].ID), OwnerAgentID: &[]string{"agent-other"}[0],
		GameID: &otherGame,
	})
	require.ErrorContains(t, err, "ownerAgentId does not belong")
}

func TestDBMonSource_AllowsOwnerMatchingSourceScope(t *testing.T) {
	dbmonSvc, _, _ := newDBMonFixture(t)
	require.NoError(t, registry.MigrateAgentSessions(dbmonSvc.svcCtx.DB))
	ctx := context.Background()
	require.NoError(t, registry.NewAgentSessionModel(dbmonSvc.svcCtx.DB).Upsert(ctx, &registry.AgentSession{
		AgentID: "agent-demo", GameID: "demo", Env: "prod",
		ExpireAt: time.Now().Add(time.Hour), LastSeen: time.Now(),
	}))

	resp, err := dbmonSvc.CreateSource(ctx, &SourceUpsertRequest{Name: "scoped", Driver: "mysql", DSN: goodDSN,
		GameID: "demo", Env: "prod", OwnerAgentID: "agent-demo"})
	require.NoError(t, err)
	assert.Equal(t, "agent-demo", resp.OwnerAgentId)
}

func TestDBMonSource_OwnerAgentRoundTrip(t *testing.T) {
	r, dbmonSvc, _, _ := newDBMonTestEnv(t)
	require.NoError(t, registry.MigrateAgentSessions(dbmonSvc.svcCtx.DB))
	for _, agent := range []struct{ id string }{
		{"agent-sidecar-1"}, {"agent-devops-2"}, {"agent-sidecar-3"},
	} {
		require.NoError(t, registry.NewAgentSessionModel(dbmonSvc.svcCtx.DB).Upsert(context.Background(), &registry.AgentSession{
			AgentID: agent.id, GameID: "demo", Env: "prod",
			ExpireAt: time.Now().Add(time.Hour), LastSeen: time.Now(),
		}))
	}

	// Create with ownership.
	rec := doDBMonReq(r, http.MethodPost, "/dbmon/sources", fmt.Sprintf(`{
		"name":"归属主库","driver":"mysql","kind":"self","dsn":%q,
		"gameId":"demo","env":"prod","ownerAgentId":"agent-sidecar-1"
	}`, goodDSN))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var created SourceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "agent-sidecar-1", created.OwnerAgentId)

	// Rebind the owning agent.
	rec = doDBMonReq(r, http.MethodPut, fmt.Sprintf("/dbmon/sources/%d", created.Id),
		`{"ownerAgentId":"agent-devops-2"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var updated SourceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.Equal(t, "agent-devops-2", updated.OwnerAgentId)

	// Empty string = unbind ownership (fall back to the server diagnostic
	// endpoint as the fallback path).
	rec = doDBMonReq(r, http.MethodPut, fmt.Sprintf("/dbmon/sources/%d", created.Id),
		`{"ownerAgentId":""}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// Fresh variable: the clear response omits the ownerAgentId key because of
	// omitempty, so reusing updated would leave the old value in place.
	var cleared SourceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cleared))
	assert.Empty(t, cleared.OwnerAgentId)

	// Ownership field not submitted: the current value is left unchanged
	// (pointer semantics, distinct from "empty string = unbind").
	rec = doDBMonReq(r, http.MethodPut, fmt.Sprintf("/dbmon/sources/%d", created.Id),
		`{"ownerAgentId":"agent-sidecar-3","name":"改名不丢归属"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var rebound SourceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rebound))
	assert.Equal(t, "agent-sidecar-3", rebound.OwnerAgentId)
	assert.Equal(t, "改名不丢归属", rebound.Name)
}
