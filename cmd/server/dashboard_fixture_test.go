package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// TestRealDashboardFixtureHealth verifies the named real-dashboard fixture:
// a genuine Server, Agent, Go SDK and /players OpenAPI provider boot with a
// clean dedicated scope, become healthy, and clean up only their own scope.
func TestRealDashboardFixtureHealth(t *testing.T) {
	fixture, err := StartDashboardFixture(context.Background(), DashboardFixtureOptions{
		BaseDir:  t.TempDir(),
		HTTPAddr: "127.0.0.1:0",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	require.NoError(t, fixture.WaitReady(ctx))

	// The fixture scope is also valid UI metadata, so the real dashboard can
	// select the exact scope where the SDK and provider registered.
	game, err := fixture.ServiceContext().GameModel.FindByGameIDString(ctx, fixture.GameID)
	require.NoError(t, err)
	require.Equal(t, fixture.GameID, game.GameID)
	binding, err := fixture.ServiceContext().GameModel.FindEnvBinding(ctx, fixture.GameID, fixture.Env)
	require.NoError(t, err)
	require.NotNil(t, binding)
	admin, err := fixture.ServiceContext().AdminModel.FindByUsername(ctx, "admin")
	require.NoError(t, err)
	require.Equal(t, fixture.GameID, admin.LastGameID)
	require.Equal(t, fixture.Env, admin.LastEnv)

	// Server HTTP health endpoint responds.
	resp, err := http.Get(fmt.Sprintf("http://%s/healthz", fixture.HTTPAddr))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// The /players provider serves the deterministic seed records.
	resp, err = http.Get(fmt.Sprintf("http://%s/players", fixture.ProviderAddr))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(body, &list))
	require.Equal(t, 2, list.Total)
	require.Equal(t, "p-001", list.Items[0].ID)
	require.Equal(t, "Ada", list.Items[0].Name)
	require.Equal(t, "p-002", list.Items[1].ID)

	// The provider publishes its OpenAPI document.
	resp, err = http.Get(fmt.Sprintf("http://%s/openapi.json", fixture.ProviderAddr))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// The real agent session is registered in the server registry with the
	// SDK function snapshot.
	agent, ok := fixture.ServiceContext().RegistryStore.AgentsUnsafe()["real-dashboard-agent"]
	require.True(t, ok, "agent session not registered")
	require.Contains(t, agent.Functions, "mail.send")

	// The unannotated SDK function materialized exactly one standalone
	// Operation proposal and no resource proposal (I-002 semantics).
	db := fixture.DB()
	proposal, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, fixture.GameID, fixture.Env, "operation:mail.send")
	require.NoError(t, err)
	require.Equal(t, "operation--mail.send", proposal.PageKey)
	require.Equal(t, "operation", proposal.PageType)
	var resourceCount int64
	require.NoError(t, db.Model(&model.PageProposal{}).
		Where("game_id = ? AND env = ? AND page_type = ?", fixture.GameID, fixture.Env, "resource").
		Count(&resourceCount).Error)
	require.Zero(t, resourceCount, "unannotated SDK function must not produce resource proposals")

	// Fixture control API reports health.
	resp, err = http.Get(fmt.Sprintf("http://%s/__fixture__/health", fixture.FixtureAddr))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var health fixtureHealthResponse
	require.NoError(t, json.Unmarshal(body, &health))
	require.Equal(t, "ok", health.Status)
	require.True(t, health.Agent)
	require.Contains(t, health.Functions, "mail.send")

	// Canary rows in a different scope must survive fixture scope cleanup.
	canary := model.FunctionContract{
		GameID:      "other-game",
		Env:         "other-env",
		FunctionID:  "canary.fn",
		Version:     "1.0.0",
		Enabled:     true,
		InputSchema: datatypes.JSON(`{"type":"object"}`),
	}
	require.NoError(t, db.Create(&canary).Error)

	require.NoError(t, fixture.CleanupScope(ctx))

	var remaining int64
	require.NoError(t, db.Model(&model.FunctionContract{}).
		Where("game_id = ? AND env = ?", fixture.GameID, fixture.Env).
		Count(&remaining).Error)
	require.Zero(t, remaining, "fixture scope contracts must be cleaned")
	require.NoError(t, db.Model(&model.PageProposal{}).
		Where("game_id = ? AND env = ?", fixture.GameID, fixture.Env).
		Count(&remaining).Error)
	require.Zero(t, remaining, "fixture scope proposals must be cleaned")
	require.NoError(t, db.Model(&reg.AgentSessionDB{}).
		Where("game_id = ? AND env = ?", fixture.GameID, fixture.Env).
		Count(&remaining).Error)
	require.Zero(t, remaining, "fixture scope agent sessions must be cleaned")
	require.NoError(t, db.Model(&model.FunctionContract{}).
		Where("game_id = ? AND env = ? AND function_id = ?", "other-game", "other-env", "canary.fn").
		Count(&remaining).Error)
	require.Equal(t, int64(1), remaining, "other scopes must survive fixture cleanup")
	_, err = fixture.ServiceContext().GameModel.FindByGameIDString(ctx, fixture.GameID)
	require.Error(t, err, "fixture-created game metadata must be cleaned")
	binding, err = fixture.ServiceContext().GameModel.FindEnvBinding(ctx, fixture.GameID, fixture.Env)
	require.NoError(t, err)
	require.Nil(t, binding, "fixture-created environment binding must be cleaned")
	admin, err = fixture.ServiceContext().AdminModel.FindByUsername(ctx, "admin")
	require.NoError(t, err)
	require.Empty(t, admin.LastGameID, "fixture admin scope must be restored")
	require.Empty(t, admin.LastEnv, "fixture admin environment must be restored")

	require.NoError(t, fixture.Close(ctx))
}
