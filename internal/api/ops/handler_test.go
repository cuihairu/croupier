package ops

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newOpsTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func TestBindOpsRequestUsesQueryForGet(t *testing.T) {
	t.Parallel()

	ctx, _ := newOpsTestContext(http.MethodGet, "/api/v1/ops/metrics?gameId=tower&env=prod&metric=cpu", "")
	var req OpsMetricsRequest
	if err := bindOpsRequest(ctx, &req); err != nil {
		t.Fatalf("bindOpsRequest() error = %v", err)
	}
	if req.GameId != "tower" {
		t.Fatalf("expected gameId=tower, got %q", req.GameId)
	}
	if req.Metric != "cpu" {
		t.Fatalf("expected metric=cpu, got %q", req.Metric)
	}
}

func TestBindOpsRequestUsesJSONForPost(t *testing.T) {
	t.Parallel()

	ctx, _ := newOpsTestContext(http.MethodPost, "/api/v1/ops/node/drain", `{"nodeId":"n1"}`)
	var req OpsNodeCommandsRequest
	if err := bindOpsRequest(ctx, &req); err != nil {
		t.Fatalf("bindOpsRequest() error = %v", err)
	}
	if req.NodeId != "n1" {
		t.Fatalf("expected nodeId=n1, got %q", req.NodeId)
	}
}

func TestOpsHandlersRejectMalformedJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(&Service{})
	cases := []struct {
		name string
		fn   func(*gin.Context)
	}{
		{name: "OpsAgentsList", fn: h.OpsAgentsList},
		{name: "OpsAgentMeta", fn: h.OpsAgentMeta},
		{name: "OpsAgentMetrics", fn: h.OpsAgentMetrics},
		{name: "OpsAgentProcesses", fn: h.OpsAgentProcesses},
		{name: "OpsAgentSystemInfo", fn: h.OpsAgentSystemInfo},
		{name: "OpsAgentExecCommand", fn: h.OpsAgentExecCommand},
		{name: "OpsAgentProcessStart", fn: h.OpsAgentProcessStart},
		{name: "OpsAgentProcessStop", fn: h.OpsAgentProcessStop},
		{name: "OpsAgentProcessRestart", fn: h.OpsAgentProcessRestart},
		{name: "OpsBackupsList", fn: h.OpsBackupsList},
		{name: "OpsBackupCreate", fn: h.OpsBackupCreate},
		{name: "OpsBackupDelete", fn: h.OpsBackupDelete},
		{name: "OpsBackupDownload", fn: h.OpsBackupDownload},
		{name: "OpsAlerts", fn: h.OpsAlerts},
		{name: "OpsAlertSilence", fn: h.OpsAlertSilence},
		{name: "OpsSilenceDelete", fn: h.OpsSilenceDelete},
		{name: "OpsSilences", fn: h.OpsSilences},
		{name: "OpsNodes", fn: h.OpsNodes},
		{name: "OpsNodeCommands", fn: h.OpsNodeCommands},
		{name: "OpsNodeDrain", fn: h.OpsNodeDrain},
		{name: "OpsNodeMeta", fn: h.OpsNodeMeta},
		{name: "OpsNodeRestart", fn: h.OpsNodeRestart},
		{name: "OpsNodeUndrain", fn: h.OpsNodeUndrain},
		{name: "OpsHealthGet", fn: h.OpsHealthGet},
		{name: "OpsHealthRun", fn: h.OpsHealthRun},
		{name: "OpsHealthUpdate", fn: h.OpsHealthUpdate},
		{name: "OpsMaintenanceGet", fn: h.OpsMaintenanceGet},
		{name: "OpsMaintenanceUpdate", fn: h.OpsMaintenanceUpdate},
		{name: "OpsMetrics", fn: h.OpsMetrics},
		{name: "OpsConfig", fn: h.OpsConfig},
		{name: "OpsNotificationsGet", fn: h.OpsNotificationsGet},
		{name: "OpsNotificationsUpdate", fn: h.OpsNotificationsUpdate},
		{name: "OpsServices", fn: h.OpsServices},
		{name: "OpsFunctions", fn: h.OpsFunctions},
		{name: "OpsMQ", fn: h.OpsMQ},
		{name: "AliasAgentsList", fn: h.AgentsList},
		{name: "AliasMetrics", fn: h.Metrics},
		{name: "AliasMQ", fn: h.MQ},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops", "{")
			tc.fn(ctx)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("expected status=500, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
