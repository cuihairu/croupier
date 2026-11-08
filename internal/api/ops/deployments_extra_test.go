package ops

// deployments.go 补测：上传 handler 的 multipart 成功面（打开/读取/超限分支
// 见文末诚实跳过）、包列表/发版单 CRUD handler 往返、service 层校验分支
// （name/version/role 净化、空载荷、对象存储/登记失败）、列表与查询错误、
// Start 全链（stage/apply/健康门禁三败点 + 成功完成）与 Rollback（缺包/
// 缺单/回滚包缺失/空转/成功/失败）、currentUsername 实名与兜底。
//
// 诚实跳过（不可达/超重）：DeployPackageUpload handler 的 133（fileHeader.Open
// 失败需伪造 multipart 内部结构）、141（内存 multipart 读取不失败）、
// 145（超 500 MiB 载荷需真实分配半 GB 内存，非单测可承受）。

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

// fakeDeployAgent 可注入 stage/apply/健康门禁成败的 deploy.Agent 替身。
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

// deployFixture 组装 DB 面齐备的部署 service：编排器直接以 DeploymentModel
// 充当 deploy.Store（生产同构），agent 用可注入替身。
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

// seedDeployment 直接落一条带目标的 draft 发版单（绕过编排校验，专供
// Start/Rollback 的查找路径）。
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

	// name/version 缺失
	_, err := s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game"}, []byte("x"))
	require.ErrorContains(t, err, "name 与 version 必填")

	// name/version/role 净化失败三类
	_, err = s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "a/b", Version: "v1"}, []byte("x"))
	require.ErrorContains(t, err, "name:")
	_, err = s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game", Version: ".."}, []byte("x"))
	require.ErrorContains(t, err, "version:")
	_, err = s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game", Version: "v1", Role: "x\\y"}, []byte("x"))
	require.ErrorContains(t, err, "role:")

	// 载荷为空
	_, err = s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game", Version: "v1"}, nil)
	require.ErrorContains(t, err, "file 内容为空")

	// 对象存储 Put 失败
	s.svcCtx.ObjectStore = &stubObjectStore{err: errors.New("obj down")}
	_, err = s.DeployPackageUpload(ctx, &DeployPackageCreateRequest{Name: "game", Version: "v1"}, []byte("x"))
	require.ErrorContains(t, err, "上传对象存储失败")
	s.svcCtx.ObjectStore = nil

	// 登记包失败（deploy_packages 表缺失，对象存储走真实文件驱动）
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

	// DeployPackageList：未装配 503 / 成功 / 查询失败
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

	// DeploymentCreate：非法批次 / 默认批次 / 成功（ctx 带用户名）
	_, err := s.DeploymentCreate(ctx, &DeploymentCreateRequest{PackageID: "p", ProcessName: "game", BatchPercent: 33, Targets: []string{"a1"}})
	require.ErrorContains(t, err, "invalid batch percent")
	userCtx := context.WithValue(ctx, "username", "alice") //nolint:staticcheck // 与 utils.CurrentUsername 键契约一致
	created, err := s.DeploymentCreate(userCtx, &DeploymentCreateRequest{PackageID: "p", ProcessName: "game", Targets: []string{"a1"}})
	require.NoError(t, err)
	require.Equal(t, 100, created.Deployment.BatchPercent, "batch 0 must default to 100")
	require.Equal(t, "alice", created.Deployment.CreatedBy)

	// DeploymentGet：未命中 404 / 命中
	_, err = s.DeploymentGet(ctx, "ghost")
	require.ErrorContains(t, err, "发版单不存在")
	got, err := s.DeploymentGet(ctx, created.Deployment.DeploymentID)
	require.NoError(t, err)
	require.Equal(t, created.Deployment.DeploymentID, got.Deployment.DeploymentID)
	require.NotNil(t, got.Deployment.Targets, "empty TargetsJSON must render as empty slice")

	// DeploymentList：成功
	lists, err := s.DeploymentList(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, lists.Total)

	// List/Find 错误面（drop deployments 表）
	require.NoError(t, s.svcCtx.DB.Migrator().DropTable("deployments"))
	_, err = s.DeploymentList(ctx)
	require.ErrorContains(t, err, "查询发版单失败")
	_, err = s.DeploymentGet(ctx, created.Deployment.DeploymentID)
	require.ErrorContains(t, err, "查询发版单失败")
}

func TestDeployServiceStartBranches(t *testing.T) {
	ctx := context.Background()

	// 发版单缺失
	s0 := deployFixture(t, &fakeDeployAgent{})
	_, err := s0.DeploymentStart(ctx, "ghost")
	require.ErrorContains(t, err, "发版单不存在")

	// 包缺失
	agent := &fakeDeployAgent{}
	s := deployFixture(t, agent)
	seedDeployment(t, s, "d1", "pkg-missing",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetPending})
	_, err = s.DeploymentStart(ctx, "d1")
	require.ErrorContains(t, err, "包不存在")

	seedPackage(t, s, "p1", "deploy/game.bin")

	// stage 失败 → 发版单 failed + 错误透传
	agent.stageErr = errors.New("dial refused")
	seedDeployment(t, s, "d2", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetPending})
	staged, err := s.DeploymentStart(ctx, "d2")
	require.ErrorContains(t, err, "stage a1")
	require.Equal(t, model.DeploymentStateFailed, staged.Deployment.State)

	// 健康门禁失败 → paused + 错误透传
	agent.stageErr = nil
	agent.healthy = false
	seedDeployment(t, s, "d3", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetPending})
	paused, err := s.DeploymentStart(ctx, "d3")
	require.ErrorContains(t, err, "health gate failed")
	require.Equal(t, model.DeploymentStatePaused, paused.Deployment.State)

	// apply 失败 → failed
	agent.healthy = true
	agent.applyErr = errors.New("checksum mismatch")
	seedDeployment(t, s, "d4", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetPending})
	failed, err := s.DeploymentStart(ctx, "d4")
	require.ErrorContains(t, err, "apply a1")
	require.Equal(t, model.DeploymentStateFailed, failed.Deployment.State)

	// 全链成功 → complete
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

	// 发版单缺失
	_, err := s.DeploymentRollback(ctx, "ghost", &DeploymentRollbackRequest{RollbackPackageID: "p1"})
	require.ErrorContains(t, err, "发版单不存在")

	// rollbackPackageId 缺失
	seedDeployment(t, s, "r1", "p1")
	_, err = s.DeploymentRollback(ctx, "r1", &DeploymentRollbackRequest{})
	require.ErrorContains(t, err, "rollbackPackageId 必填")

	// 回滚包缺失
	_, err = s.DeploymentRollback(ctx, "r1", &DeploymentRollbackRequest{RollbackPackageID: "pkg-missing"})
	require.ErrorContains(t, err, "回滚包不存在")

	// 无待回滚目标（全 pending）→ 空转成功
	rolled, err := s.DeploymentRollback(ctx, "r1", &DeploymentRollbackRequest{RollbackPackageID: "p1"})
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateRolledBack, rolled.Deployment.State)

	// 目标 healthy + apply 成功 → rolled back
	seedDeployment(t, s, "r2", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetHealthy})
	rolled, err = s.DeploymentRollback(ctx, "r2", &DeploymentRollbackRequest{RollbackPackageID: "p1"})
	require.NoError(t, err)
	require.Equal(t, model.DeploymentStateRolledBack, rolled.Deployment.State)

	// 目标 failed + apply 失败 → rollback failed + 错误透传
	agent.applyErr = errors.New("apply boom")
	seedDeployment(t, s, "r3", "p1",
		model.DeploymentTarget{AgentID: "a1", Batch: 1, State: model.DeploymentTargetFailed})
	_, err = s.DeploymentRollback(ctx, "r3", &DeploymentRollbackRequest{RollbackPackageID: "p1"})
	require.ErrorContains(t, err, "rollback apply agent a1")
}

// ---- handler 往返 ----

// multipartUpload 构造带文件与表单字段的 multipart 请求并投递 handler。
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

// deployCtx 构造带路由参数的测试 gin 上下文（跑 handler 前置）。
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

	// 上传：无文件 → 400；成功 → 200
	rec := multipartUpload(t, h, map[string]string{"name": "game"}, []byte("x"))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = multipartUpload(t, h, map[string]string{
		"name": "game", "version": "1.0.0", "role": "game-server",
		"gameId": "demo", "env": "prod", "contentType": "application/tar",
	}, []byte("payload-bytes"))
	require.Equal(t, http.StatusOK, rec.Code)

	// 包列表 → 200
	c, rec := deployCtx(http.MethodGet, "/api/v1/ops/deploy/packages", "")
	h.DeployPackageList(c)
	require.Equal(t, http.StatusOK, rec.Code)

	// 发版单创建：坏 JSON 400 / 成功 200
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

	// 列表 / 详情 → 200
	c, rec = deployCtx(http.MethodGet, "/api/v1/ops/deployments", "")
	h.DeploymentList(c)
	require.Equal(t, http.StatusOK, rec.Code)

	c, rec = deployCtx(http.MethodGet, "/api/v1/ops/deployments/"+depID, "",
		gin.Param{Key: "id", Value: depID})
	h.DeploymentGet(c)
	require.Equal(t, http.StatusOK, rec.Code)

	// 启动（fake agent 全绿）→ 200 complete
	c, rec = deployCtx(http.MethodPost, "/api/v1/ops/deployments/"+depID+"/start", "",
		gin.Param{Key: "id", Value: depID})
	h.DeploymentStart(c)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), model.DeploymentStateComplete)

	// 回滚：坏 JSON 400 / 成功 200
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
	// svcCtx/模型未装配 → errDeployNotConfigured（233/340/355）
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
	// 表结构在册但物理表缺失 → handler service 错误分支（163/186/195/218）
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

	// 回滚 service 错误分支（218）
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

	// deployments 表缺失 → 创建落库失败（333）
	require.NoError(t, s.svcCtx.DB.Migrator().DropTable("deployments"))
	_, err := s.DeploymentCreate(ctx, &DeploymentCreateRequest{PackageID: "p1", ProcessName: "game", Targets: []string{"a1"}})
	require.ErrorContains(t, err, "创建发版单失败")

	// TargetsJSON 非法（null）→ 视图兜底空切片（435）
	view := buildDeploymentView(&model.Deployment{DeploymentID: "d9", TargetsJSON: model.JSON([]byte("null"))})
	require.NotNil(t, view.Targets)
	require.Empty(t, view.Targets)
}

func TestBytesReaderRoundTrip(t *testing.T) {
	// mu 保护的 ReadSeeker 包装（476/481）：Read/Seek 行为与底层 reader 一致
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
