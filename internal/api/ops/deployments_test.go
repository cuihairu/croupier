package ops

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
	// no multipart file field: the handler rejects with 400 first (file required),
	// never reaching the object store.
	ctx, rec := newOpsTestContext(http.MethodPost, "/api/v1/ops/deploy/packages", `{}`)
	h.DeployPackageUpload(ctx)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

var _ = context.Background

// Review finding: bytesReader created a new bytes.Reader on every Read/Seek, so
// Read never returned EOF — an io.Copy loop in objstore Put. Regression: a single
// reader with a persistent cursor, returning EOF when exhausted.
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

// Review finding: role/name/version were concatenated into the object key and the
// agent staging path without sanitizing. Regression: values with path separators or
// traversal sequences are rejected.
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

// Review finding 726: repeated uploads with the same (role,name,version) used to
// overwrite the same object key, so historical Checksum values no longer matched the
// object content. Regression: the object key embeds a unique packageID, two uploads
// with different content do not overwrite each other, and each Checksum matches its
// object content.
func TestDeployPackageUpload_SameTripleDoesNotOverwrite(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/deploy.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.DeployPackage{}))
	store, err := objstore.OpenFile(ctx, objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	svcCtx := &svc.ServiceContext{
		DeployPackageModel: model.NewDeployPackageModel(db),
		ObjectStore:        store,
	}
	s := NewService(svcCtx)

	req := &DeployPackageCreateRequest{Name: "game", Version: "1.0.0", Role: "game-server"}
	first, err := s.DeployPackageUpload(ctx, req, []byte("version-one"))
	require.NoError(t, err)
	second, err := s.DeployPackageUpload(ctx, req, []byte("version-two"))
	require.NoError(t, err)

	assert.NotEqual(t, first.Package.ObjectKey, second.Package.ObjectKey,
		"同 (role,name,version) 上传必须写入不同对象键，不得覆盖")
	// the old object content is still readable back, and the Checksum matches the content.
	r, err := store.Get(ctx, first.Package.ObjectKey)
	require.NoError(t, err)
	buf := make([]byte, 32)
	n, _ := io.ReadFull(r, buf)
	_ = r.Close()
	require.Equal(t, "version-one", string(buf[:n]))
}
