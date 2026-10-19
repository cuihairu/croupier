package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	synconfig "github.com/cuihairu/croupier/agents/synthetics/config"
	synrunner "github.com/cuihairu/croupier/agents/synthetics/runner"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSyntheticsAgentSessionUploadsAndPersistsResult(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:probe-agent-e2e?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ProbeEvent{}))
	svc := NewControlService(registry.NewStore(), nil)
	svc.SetProbeEventModel(model.NewProbeEventModel(db))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer httpServer.Close()
	listener, err := NewTCPListener(&TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}, nil, registry.NewStore(), slog.Default())
	require.NoError(t, err)
	defer listener.Close()
	listener.SetHandler(svc)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = listener.Serve(ctx) }()
	agent := synrunner.New(synconfig.Config{
		Agent:      synconfig.Agent{ID: "synthetics-e2e", ServerAddr: listener.Addr(), GameID: "demo", Env: "staging"},
		Synthetics: synconfig.Synthetics{Interval: time.Second, Targets: []synconfig.Target{{Name: "local-http", URL: httpServer.URL, Method: "GET", Timeout: time.Second, ExpectedStatus: http.StatusNoContent}}},
	}, slog.Default())
	go func() { _ = agent.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	var rows []model.ProbeEvent
	for time.Now().Before(deadline) {
		rows, err = model.NewProbeEventModel(db).Recent(context.Background(), "demo", "staging", "synthetics-e2e", 10)
		require.NoError(t, err)
		if len(rows) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NotEmpty(t, rows, "agent session should upload a result to durable storage")
	require.Equal(t, "local-http", rows[0].Target)
	require.Equal(t, "probe.detail", rows[0].Kind)
	require.Equal(t, "success", rows[0].Status)
	require.Empty(t, rows[0].ErrorClass)
	time.Sleep(100 * time.Millisecond) // let the one-way uplink write finish before closing the session
	otherScope, err := model.NewProbeEventModel(db).Recent(context.Background(), "other", "staging", "", 10)
	require.NoError(t, err)
	require.Empty(t, otherScope)
}
