package ops

import (
	"context"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDeploymentDTOBuild(t *testing.T) {
	d := &model.Deployment{
		DeploymentID: "dep-1", PackageID: "pkg-1", Name: "n", GameID: "g", Env: "prod",
		Role: "game-server", ProcessName: "game", BatchPercent: 100, BatchCount: 1,
		State: model.DeploymentStateDraft, CreatedBy: "admin",
		TargetsJSON: model.JSON([]byte(`[{"agentId":"a","batch":1,"state":"pending"}]`)),
	}
	v := buildDeploymentView(d)
	require.Equal(t, "dep-1", v.DeploymentID)
	require.Len(t, v.Targets, 1)
	require.Equal(t, "a", v.Targets[0].AgentID)
}

func TestDeploymentCreateRequiresOrchestrator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Service{}
	h := NewHandler(s)
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/deployments", `{"packageId":"p","processName":"game","batchPercent":100,"targets":["a"]}`)
	h.DeploymentCreate(ctx)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestDeploymentStartRequiresOrchestrator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Service{}
	h := NewHandler(s)
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/deployments/dep-1/start", "")
	h.DeploymentStart(ctx)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestDeployPackageUploadEmptyNoObjectStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Service{}
	h := NewHandler(s)
	// 无 multipart 文件字段：handler 先拦 400（file 必填），不触达对象存储。
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/deploy/packages", `{}`)
	h.DeployPackageUpload(ctx)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

var _ = context.Background
