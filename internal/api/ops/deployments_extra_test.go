package ops

// Extra coverage for deployments.go: the multipart success surface of the upload
// handler (the open/read/oversize branches are honestly skipped at the end of this
// file), package list / deployment CRUD handler round trips, service-layer
// validation branches (name/version/role sanitization, empty payload, object
// store / registration failures), list and query errors, the full Start chain
// (stage/apply/health-gate failure points + success completion) and Rollback
// (missing package / missing order / rollback package missing / no-op / success /
// failure), plus currentUsername real-name and fallback.
//
// Honestly skipped (unreachable / oversized): DeployPackageUpload handler's 133
// (a fileHeader.Open failure requires faking multipart internals), 141 (in-memory
// multipart reads never fail), 145 (a payload over 500 MiB would need a real half-GB
// allocation, not something a unit test can bear).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/ops/deploy"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeDeployAgent is a deploy.Agent stand-in whose stage/apply/health-gate
// outcomes can be injected.
type fakeDeployAgent struct {
	stageErr error
	applyErr error
	healthy  bool
	reason   string
}

func (a *fakeDeployAgent) StagePackage(context.Context, string, *model.DeployPackage) (string, error) {
	if a.stageErr != nil {
		return "", a.stageErr
	}
	return "/tmp/croupier-deploy-staged", nil
}

func (a *fakeDeployAgent) ApplyUpdate(context.Context, string, string, string, string) (string, error) {
	if a.applyErr != nil {
		return "", a.applyErr
	}
	return "fp-1", nil
}

func (a *fakeDeployAgent) Healthy(context.Context, string) (bool, string) { return a.healthy, a.reason }

// deployFixture assembles a deployment service with a complete DB surface: the
// orchestrator uses the DeploymentModel directly as the deploy.Store (isomorphic
// to production), with an injectable stand-in agent.
func deployFixture(t *testing.T, agent *fakeDeployAgent) *Service {
	t.Helper()
	ctx := context.Background()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/deploy_svc.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Deployment{}, &model.DeployPackage{}))
	objStore, err := objstore.OpenFile(ctx, objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	orch := deploy.New(model.NewDeploymentModel(db), agent)
	orch.HealthRetry = time.Millisecond
	orch.HealthRetries = 1
	svcCtx := &svc.ServiceContext{
		DB:                 db,
		DeployPackageModel: model.NewDeployPackageModel(db),
		DeploymentModel:    model.NewDeploymentModel(db),
		Deployments:        orch,
		ObjectStore:        objStore,
	}
	return NewService(svcCtx)
}

// seedDeployment directly creates a target-carrying draft deployment (bypassing
// orchestrator validation, solely for the Start/Rollback lookup paths).
func seedDeployment(t *testing.T, s *Service, id, packageID string, targets ...model.DeploymentTarget) {
	t.Helper()
	targetsJSON := []byte("[]")
	if len(targets) > 0 {
		var err error
		targetsJSON, err = json.Marshal(targets)
		require.NoError(t, err)
	}
	require.NoError(t, s.svcCtx.DeploymentModel.Create(context.Background(), &model.Deployment{
		DeploymentID: id, PackageID: packageID, Name: "n", GameID: "demo", Env: "prod",
		Role: "game-server", ProcessName: "game", BatchPercent: 100, BatchCount: 1,
		State: model.DeploymentStateDraft, TargetsJSON: model.JSON(targetsJSON),
	}))
}

func seedPackage(t *testing.T, s *Service, id, objectKey string) {
	t.Helper()
	require.NoError(t, s.svcCtx.DeployPackageModel.Create(context.Background(), &model.DeployPackage{
		PackageID: id, Name: "game", Version: "1.0.0", Role: "game-server", ObjectKey: objectKey,
	}))
}

func TestDeployServiceValidationBranches(t *testing.T) {
	ctx := context.Background()
	s := deployFixture(t, &fakeDeployAgent{})

	// name/version missing
	_, err := s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game"}, []byte("x"))
	require.ErrorContains(t, err, "name 与 version 必填")

	// the three name/version/role sanitization failure cases
	_, err = s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "a/b", Version: "v1"}, []byte("x"))
	require.ErrorContains(t, err, "name:")
	_, err = s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game", Version: ".."}, []byte("x"))
	require.ErrorContains(t, err, "version:")
	_, err = s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game", Version: "v1", Role: "x\\y"}, []byte("x"))
	require.ErrorContains(t, err, "role:")

	// empty payload
	_, err = s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game", Version: "v1"}, nil)
	require.ErrorContains(t, err, "file 内容为空")

	// object store Put failure
	s.svcCtx.ObjectStore = &stubObjectStore{err: errors.New("obj down")}
	_, err = s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game", Version: "v1"}, []byte("x"))
	require.ErrorContains(t, err, "上传对象存储失败")
	s.svcCtx.ObjectStore = nil

	// package registration failure (the deploy_packages table is missing, object
	// storage uses the real file driver)
	objStore, err := objstore.OpenFile(ctx, objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/dep_bad.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.DeployPackage{}))
	require.NoError(t, db.Migrator().DropTable("deploy_packages"))
	bad := NewService(&svc.ServiceContext{
		DeployPackageModel: model.NewDeployPackageModel(db),
		ObjectStore:        objStore,
	})
	_, err = bad.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game", Version: "v1"}, []byte("x"))
	require.ErrorContains(t, err, "登记包失败")

	// DeployPackageList: not wired → 503 / success / query failure
	_, err = NewService(&svc.ServiceContext{}).DeployPackageList(ctx)
	require.Error(t, err, errDeployNotConfigured.Message)
	lists, err := s.DeployPackageList(ctx)
	require.NoError(t, err)
	require.Empty(t, lists.Items)
	require.NoError(t, s.svcCtx.DB.Migrator().DropTable("deploy_packages"))
	_, err = s.DeployPackageList(ctx)
	require.ErrorContains(t, err, "查询包失败")
}

func TestDeployServiceCreateGetListBranches(t *testing.T) {
	ctx := context.Background()
	s := deployFixture(t, &fakeDeployAgent{})

	// DeploymentCreate: invalid batch / default batch / success (ctx carries a username)
	_, err := s.DeploymentCreate(ctx, &DeploymentCreateRequest{PackageID: "p", ProcessName: "game", BatchPercent: 33, Targets: []string{"a1"}})
	require.ErrorContains(t, err, "invalid batch percent")
	userCtx := context.WithValue(ctx, "username", "alice") //nolint:staticcheck // matches the key contract of utils.CurrentUsername
	created, err := s.DeploymentCreate(userCtx, &DeploymentCreateRequest{PackageID: "p", ProcessName: "game", Targets: []string{"a1"}})
	require.NoError(t, err)
	require.Equal(t, 100, created.Deployment.BatchPercent, "batch 0 must default to 100")
	require.Equal(t, "alice", created.Deployment.CreatedBy)

	// DeploymentGet: miss → 404 / hit
	_, err = s.DeploymentGet(ctx, "ghost")
	require.ErrorContains(t, err, "发版单不存在")
	got, err := s.DeploymentGet(ctx, created.Deployment.DeploymentID)
	require.NoError(t, err)
	require.Equal(t, created.Deployment.DeploymentID, got.Deployment.DeploymentID)
	require.NotNil(t, got.Deployment.Targets, "empty TargetsJSON must render as empty slice")

	// DeploymentList: success
	lists, err := s.DeploymentList(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, lists.Total)

	// List/Find error surfaces (drop the deployments table)
	require.NoError(t, s.svcCtx.DB.Migrator().DropTable("deployments"))
	_, err = s.DeploymentList(ctx)
	require.ErrorContains(t, err, "查询发版单失败")
	_, err = s.DeploymentGet(ctx, created.Deployment.DeploymentID)
	require.ErrorContains(t, err, "查询发版单失败")
}

func TestDeployServiceStartBranches(t *testing.T) {
	ctx := context.Background()

	// deployment order missing
	s0 := deployFixture(t, &fakeDeployAgent{})
	_, err := s0.DeploymentStart(ctx, "ghost")
	require.ErrorContains(t, err, "发版单不存在")

	// package missing
	agent := &fakeDeployAgent{}
	s := deployFixture(t, agent)
	seedDeployment(t, s, "d1", "pkg-missing",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetPending})
	_, err = s.DeploymentStart(ctx, "d1")
	require.ErrorContains(t, err, "包不存在")

	seedPackage(t, s, "p1", "deploy/game.bin")

	// stage failure → deployment failed + error passthrough
	agent.stageErr = errors.New("dial refused")
	seedDeployment(t, s, "d2", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetPending})
	staged, err := s.DeploymentStart(ctx, "d2")
	require.ErrorContains(t, err, "stage a1")
	require.Equal(t, model.DeploymentStateFailed, staged.Deployment.State)

	// health gate failure → paused + error passthrough
	agent.stageErr = nil
	agent.healthy = false
	seedDeployment(t, s, "d3", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetPending})
	paused, err := s.DeploymentStart(ctx, "d3")
	require.ErrorContains(t, err, "health gate failed")
	require.Equal(t, model.DeploymentStatePaused, paused.Deployment.State)

	// apply failure → failed
	agent.healthy = true
	agent.applyErr = errors.New("checksum mismatch")
	seedDeployment(t, s, "d4", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetPending})
	failed, err := s.DeploymentStart(ctx, "d4")
	require.ErrorContains(t, err, "apply a1")
	require.Equal(t, model.DeploymentStateFailed, failed.Deployment.State)

	// full chain success → complete
	agent.applyErr = nil
	seedDeployment(t, s, "d5", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetPending})
	done, err := s.DeploymentStart(ctx, "d5")
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateComplete, done.Deployment.State)
}

func TestDeployServiceRollbackBranches(t *testing.T) {
	ctx := context.Background()
	agent := &fakeDeployAgent{healthy: true}
	s := deployFixture(t, agent)
	seedPackage(t, s, "p1", "deploy/game.bin")

	// deployment order missing
	_, err := s.DeploymentRollback(ctx, "ghost", &DeploymentRollbackRequest{RollbackPackageID: "p1"})
	require.ErrorContains(t, err, "发版单不存在")

	// rollbackPackageId missing
	seedDeployment(t, s, "r1", "p1")
	_, err = s.DeploymentRollback(ctx, "r1", &DeploymentRollbackRequest{})
	require.ErrorContains(t, err, "rollbackPackageId 必填")

	// rollback package missing
	_, err = s.DeploymentRollback(ctx, "r1", &DeploymentRollbackRequest{RollbackPackageID: "pkg-missing"})
	require.ErrorContains(t, err, "回滚包不存在")

	// no rollback targets (empty / all pending) → must not falsely report
	// rolled_back (AUD-023: a zero-target operation must fail explicitly; the
	// previous "no-op success" assertion was itself the defect, now replaced
	// with an assertion that it is rejected).
	_, err = s.DeploymentRollback(ctx, "r1", &DeploymentRollbackRequest{RollbackPackageID: "p1"})
	require.ErrorContains(t, err, "nothing to rollback")

	// target healthy + apply success → rolled back
	seedDeployment(t, s, "r2", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetHealthy})
	rolled, err := s.DeploymentRollback(ctx, "r2", &DeploymentRollbackRequest{RollbackPackageID: "p1"})
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateRolledBack, rolled.Deployment.State)

	// target failed + apply failure → rollback failed + error passthrough
	agent.applyErr = errors.New("apply boom")
	seedDeployment(t, s, "r3", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetFailed})
	_, err = s.DeploymentRollback(ctx, "r3", &DeploymentRollbackRequest{RollbackPackageID: "p1"})
	require.ErrorContains(t, err, "rollback apply agent a1")
}

// ---- handler round trips ----

// multipartUpload builds a multipart request carrying a file plus form fields and
// dispatches it to the handler.
func multipartUpload(t *testing.T, h *Handler, fields map[string]string, payload []byte) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for k, v := range fields {
		require.NoError(t, mw.WriteField(k, v))
	}
	fw, err := mw.CreateFormFile("file", "pkg.bin")
	require.NoError(t, err)
	_, err = fw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ops/deploy/packages", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	h.DeployPackageUpload(c)
	return rec
}

// deployCtx builds a test gin context carrying route parameters (run before the handler).
func deployCtx(method, target, body string, params ...gin.Param) (*gin.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, target, nil)
	if body != "" {
		req = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = params
	return c, rec
}

func TestDeployHandlersRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	agent := &fakeDeployAgent{healthy: true}
	s := deployFixture(t, agent)
	seedPackage(t, s, "p1", "deploy/game.bin")
	h := NewHandler(s)

	// upload: no file → 400; success → 200
	rec := multipartUpload(t, h, map[string]string{"name": "game"}, []byte("x"))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = multipartUpload(t, h, map[string]string{
		"name": "game", "version": "1.0.0", "role": "game-server",
		"gameId": "demo", "env": "prod", "contentType": "application/tar",
	}, []byte("payload-bytes"))
	require.Equal(t, http.StatusOK, rec.Code)

	// package list → 200
	c, rec := deployCtx(http.MethodGet, "/api/v1/ops/deploy/packages", "")
	h.DeployPackageList(c)
	require.Equal(t, http.StatusOK, rec.Code)

	// deployment creation: bad JSON 400 / success 200
	c, rec = deployCtx(http.MethodPost, "/api/v1/ops/deployments", "{bad")
	h.DeploymentCreate(c)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	c, rec = deployCtx(http.MethodPost, "/api/v1/ops/deployments",
		`{"packageId":"p1","processName":"game","targets":["a1"]}`)
	h.DeploymentCreate(c)
	require.Equal(t, http.StatusOK, rec.Code)
	var created DeploymentCreateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	depID := created.Deployment.DeploymentID

	// list / details → 200
	c, rec = deployCtx(http.MethodGet, "/api/v1/ops/deployments", "")
	h.DeploymentList(c)
	require.Equal(t, http.StatusOK, rec.Code)

	c, rec = deployCtx(http.MethodGet, "/api/v1/ops/deployments/"+depID, "",
		gin.Param{Key: "id", Value: depID})
	h.DeploymentGet(c)
	require.Equal(t, http.StatusOK, rec.Code)

	// start (fake agent all green) → 200 complete
	c, rec = deployCtx(http.MethodPost, "/api/v1/ops/deployments/"+depID+"/start", "",
		gin.Param{Key: "id", Value: depID})
	h.DeploymentStart(c)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), model.DeploymentStateComplete)

	// rollback: bad JSON 400 / success 200
	c, rec = deployCtx(http.MethodPost, "/api/v1/ops/deployments/"+depID+"/rollback", "{bad",
		gin.Param{Key: "id", Value: depID})
	h.DeploymentRollback(c)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	c, rec = deployCtx(http.MethodPost, "/api/v1/ops/deployments/"+depID+"/rollback",
		`{"rollbackPackageId":"p1"}`, gin.Param{Key: "id", Value: depID})
	h.DeploymentRollback(c)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestDeployServiceNotConfiguredBranches(t *testing.T) {
	// svcCtx/model not wired → errDeployNotConfigured (233/340/355)
	s := NewService(&svc.ServiceContext{})
	_, err := s.DeployPackageUpload(context.Background(), &DeployPackageCreateRequest{Name: "game", Version: "v1"}, []byte("x"))
	require.ErrorIs(t, err, errDeployNotConfigured)
	_, err = s.DeploymentList(context.Background())
	require.ErrorIs(t, err, errDeployNotConfigured)
	_, err = s.DeploymentGet(context.Background(), "d1")
	require.ErrorIs(t, err, errDeployNotConfigured)
}

func TestDeployHandlerErrorBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// schema registered but physical tables missing → handler service error branches (163/186/195/218)
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/dep_hb.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Deployment{}, &model.DeployPackage{}))
	require.NoError(t, db.Migrator().DropTable("deploy_packages", "deployments"))
	h := NewHandler(NewService(&svc.ServiceContext{
		DB:                 db,
		DeployPackageModel: model.NewDeployPackageModel(db),
		DeploymentModel:    model.NewDeploymentModel(db),
	}))

	c, w := deployCtx(http.MethodGet, "/api/v1/ops/deploy/packages", "")
	h.DeployPackageList(c)
	require.NotEqual(t, http.StatusOK, w.Code)

	c, w = deployCtx(http.MethodGet, "/api/v1/ops/deployments", "")
	h.DeploymentList(c)
	require.NotEqual(t, http.StatusOK, w.Code)

	c, w = deployCtx(http.MethodGet, "/api/v1/ops/deployments/ghost", "",
		gin.Param{Key: "id", Value: "ghost"})
	h.DeploymentGet(c)
	require.NotEqual(t, http.StatusOK, w.Code)

	c, w = deployCtx(http.MethodPost, "/api/v1/ops/deployments/ghost/start", "",
		gin.Param{Key: "id", Value: "ghost"})
	h.DeploymentStart(c)
	require.NotEqual(t, http.StatusOK, w.Code)

	// rollback service error branches (218)
	c, w = deployCtx(http.MethodPost, "/api/v1/ops/deployments/ghost/rollback",
		`{"rollbackPackageId":"p1"}`, gin.Param{Key: "id", Value: "ghost"})
	h.DeploymentRollback(c)
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestDeployCreateStoreErrorAndViewDefaults(t *testing.T) {
	ctx := context.Background()
	agent := &fakeDeployAgent{}
	s := deployFixture(t, agent)
	seedPackage(t, s, "p1", "deploy/game.bin")

	// deployments table missing → create fails to land (333)
	require.NoError(t, s.svcCtx.DB.Migrator().DropTable("deployments"))
	_, err := s.DeploymentCreate(ctx, &DeploymentCreateRequest{PackageID: "p1", ProcessName: "game", Targets: []string{"a1"}})
	require.ErrorContains(t, err, "创建发版单失败")

	// TargetsJSON invalid (null) → the view falls back to an empty slice (435)
	view := buildDeploymentView(&model.Deployment{DeploymentID: "d9", TargetsJSON: model.JSON([]byte("null"))})
	require.NotNil(t, view.Targets)
	require.Empty(t, view.Targets)
}

func TestBytesReaderRoundTrip(t *testing.T) {
	// mutex-protected ReadSeeker wrapper (476/481): Read/Seek behave like the underlying reader
	br := newBytesReader([]byte("hello"))
	buf := make([]byte, 3)
	n, err := br.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, "hel", string(buf))

	off, err := br.Seek(1, io.SeekStart)
	require.NoError(t, err)
	require.Equal(t, int64(1), off)
	rest, err := io.ReadAll(br)
	require.NoError(t, err)
	require.Equal(t, "ello", string(rest))
}
