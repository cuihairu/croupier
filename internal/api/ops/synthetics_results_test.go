package ops

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

func TestSyntheticsResultsHandlerFiltersScopeAndAgent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ops-synthetics?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ProbeEvent{}))
	payload, err := proto.Marshal(&opsv1.AlertEvent{Kind: "probe.detail", Detail: &opsv1.ProbeDetail{Target: "http://local", Err: "timeout", LatencyMs: 42}})
	require.NoError(t, err)
	for _, row := range []model.ProbeEvent{
		{AgentID: "a1", GameID: "demo", Env: "prod", Kind: "probe.detail", Target: "http://local", Status: "failed", LatencyMS: 42, ErrorClass: "timeout", Payload: payload},
		{AgentID: "a2", GameID: "demo", Env: "prod", Kind: "probe.detail", Target: "other", Payload: payload},
		{AgentID: "a1", GameID: "demo", Env: "dev", Kind: "probe.detail", Target: "wrong-env", Payload: payload},
	} {
		require.NoError(t, db.Create(&row).Error)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewHandler(NewService(&svc.ServiceContext{DB: db}))
	router.GET("/", h.SyntheticsResults)
	req := httptest.NewRequest(http.MethodGet, "/?gameId=demo&env=prod&agentId=a1&limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "http://local")
	require.Contains(t, w.Body.String(), "timeout")
	require.NotContains(t, w.Body.String(), "wrong-env")
	require.NotContains(t, w.Body.String(), "other")
}
