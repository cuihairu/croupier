package ops

import (
	"context"
	"io"
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

// 审核发现：bytesReader 每次 Read/Seek 新建 bytes.Reader，Read 永不返回 EOF
// ——objstore Put 的 io.Copy 死循环。回归：单 reader 持久游标，读尽返回 EOF。
func TestBytesReader_ReadsToEOF(t *testing.T) {
	r := newBytesReader([]byte("hello"))
	buf := make([]byte, 3)
	n, err := r.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, "hel", string(buf[:n]))
	n, err = r.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.Equal(t, "lo", string(buf[:n]))
	n, err = r.Read(buf)
	require.Equal(t, 0, n)
	require.ErrorIs(t, err, io.EOF, "读尽必须返回 EOF（旧实现永不 EOF 致 io.Copy 死循环）")
}

// 审核发现：role/name/version 未净化即拼接对象键与 agent 暂存路径。回归：
// 含路径分隔符/遍历序列被拒。
func TestSafeObjectPathPart_RejectsTraversal(t *testing.T) {
	for _, bad := range []string{"../x", "a/b", "a\\b", "..", ".", "", "v1/../x"} {
		_, err := safeObjectPathPart(bad)
		require.Error(t, err, "must reject %q", bad)
	}
	for _, good := range []string{"agent", "v1.2.3", "game-server"} {
		got, err := safeObjectPathPart(good)
		require.NoError(t, err)
		require.Equal(t, good, got)
	}
}
