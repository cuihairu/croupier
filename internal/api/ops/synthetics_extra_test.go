package ops

// synthetics_results.go 补测：缺少 gameId/env 查询参数的错误面、DB 未装配的
// 查询错误透传、以及 window 事件的执行时快照分支（LastReason 与窗口级
// TargetCategory 兜底）。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

func TestSyntheticsResultsMissingParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{}))
	req := httptest.NewRequest(http.MethodGet, "/?env=prod", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.SyntheticsResults(c)
	require.NotEqual(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "required")
}

func TestSyntheticsResultsDBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{})) // DB nil → gorm.ErrInvalidDB
	req := httptest.NewRequest(http.MethodGet, "/?gameId=demo&env=prod", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.SyntheticsResults(c)
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestSyntheticsResultsWindowSnapshot(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/syn.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ProbeEvent{}))

	// window 事件：执行时类别快照只在 Window 内，且窗口级兜底 TargetCategory
	winPayload, err := proto.Marshal(&opsv1.AlertEvent{
		Kind: "probe.window", Window: &opsv1.ProbeWindow{
			LastReason: "unavailable: timeout", TargetCategory: "third_party",
		},
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.ProbeEvent{
		AgentID: "a1", GameID: "demo", Env: "prod", Kind: "probe.window",
		Target: "redis", Status: "failed", ErrorClass: "timeout", Payload: winPayload,
	}).Error)

	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{DB: db}))
	req := httptest.NewRequest(http.MethodGet, "/?gameId=demo&env=prod", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.SyntheticsResults(c)
	require.Equal(t, http.StatusOK, w.Code)

	var resp SyntheticsResultsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	require.Equal(t, "unavailable: timeout", resp.Items[0].Error)
	require.Equal(t, "third_party", resp.Items[0].TargetCategory)
}
