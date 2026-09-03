package release

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- v9 helpers ----

var v9RelDBSeq uint64

func v9TableOf(tx *gorm.DB) string {
	if tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	if tx.Statement.Schema != nil {
		return tx.Statement.Schema.Table
	}
	return ""
}

// v9FailQueryOn makes queries touching table fail from the from-th hit (1-based).
func v9FailQueryOn(db *gorm.DB, table string, from int) {
	var hits int
	db.Callback().Query().Before("gorm:query").Register("v9_fail_query_"+table, func(tx *gorm.DB) {
		if v9TableOf(tx) != table {
			return
		}
		hits++
		if hits >= from {
			tx.AddError(errors.New("v9 forced query error on " + table))
		}
	})
}

func v9FailCreateOn(db *gorm.DB, table string) {
	db.Callback().Create().Before("gorm:create").Register("v9_fail_create_"+table, func(tx *gorm.DB) {
		if v9TableOf(tx) == table {
			tx.AddError(errors.New("v9 forced create error on " + table))
		}
	})
}

// v9FailUpdateOn fails updates on table; when destKey != "" only updates whose
// destination map contains that key fail (distinguishes column updates).
func v9FailUpdateOn(db *gorm.DB, table, destKey string) {
	db.Callback().Update().Before("gorm:update").Register("v9_fail_update_"+table+"_"+destKey, func(tx *gorm.DB) {
		if v9TableOf(tx) != table {
			return
		}
		if destKey != "" {
			m, ok := tx.Statement.Dest.(map[string]interface{})
			if !ok || m[destKey] == nil {
				return
			}
		}
		tx.AddError(errors.New("v9 forced update error on " + table))
	})
}

type v9ReleaseFixture struct {
	db  *gorm.DB
	svc *Service
}

func newV9ReleaseDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("relv9_%d", atomic.AddUint64(&v9RelDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newV9ReleaseFixture(t *testing.T, store objstore.Store) *v9ReleaseFixture {
	t.Helper()
	db := newV9ReleaseDB(t)
	svcCtx := &svc.ServiceContext{ReleaseModel: model.NewGameReleaseModel(db), ObjectStore: store}
	return &v9ReleaseFixture{db: db, svc: NewService(svcCtx)}
}

func (f *v9ReleaseFixture) seedV9(t *testing.T, status, version string, gray int) *model.GameRelease {
	t.Helper()
	rel := &model.GameRelease{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		Version: version, Type: model.ReleaseTypeFull, Status: status,
		GrayPercent: gray, GraySeed: "seed",
	}
	require.NoError(t, f.db.Create(rel).Error)
	return rel
}

func v9GinCtx(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	return c, w
}

// failStoreV9 is an objstore.Store whose Put always fails.
type failStoreV9 struct{}

func (failStoreV9) Put(context.Context, string, objstore.ReadSeeker, int64, string) error {
	return errors.New("v9 put boom")
}
func (failStoreV9) SignedURL(context.Context, string, string, time.Duration) (string, error) {
	return "", errors.New("v9 url boom")
}
func (failStoreV9) Delete(context.Context, string) error { return errors.New("v9 del boom") }
func (failStoreV9) List(context.Context, string, string, string, int) (objstore.ListResult, error) {
	return objstore.ListResult{}, errors.New("v9 list boom")
}
func (failStoreV9) CreatePrefix(context.Context, string) error { return errors.New("v9 cp boom") }
func (failStoreV9) RenamePrefix(context.Context, string, string) error {
	return errors.New("v9 rp boom")
}

// ---- List ----

func TestV9ListQueryErrorAndHandlerError(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	f.seedV9(t, model.ReleaseStatusDraft, "1.0.0", 0)
	v9FailQueryOn(f.db, "game_releases", 1)

	_, err = f.svc.List(t.Context(), &ReleaseListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)

	h := NewHandler(f.svc)
	c, w := v9GinCtx("GET", "/releases", "")
	h.List(c)
	assert.NotEqual(t, 200, w.Code)
}

// ---- Create ----

func TestV9CreateValidationBranches(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)

	_, err = f.svc.Create(t.Context(), &ReleaseCreateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "gameboy", Version: "1.0.0",
	})
	require.ErrorContains(t, err, "无效的平台")

	_, err = f.svc.Create(t.Context(), &ReleaseCreateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android", Version: "  ",
	})
	require.ErrorContains(t, err, "版本号不能为空")
}

func TestV9CreateModelErrorAndHandlerError(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	v9FailCreateOn(f.db, "game_releases")

	_, err = f.svc.Create(t.Context(), &ReleaseCreateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android", Version: "1.0.0",
	})
	require.Error(t, err)

	h := NewHandler(f.svc)
	c, w := v9GinCtx("POST", "/releases", `{"gameId":"demo","env":"prod","channel":"official","platform":"android","version":"1.0.0"}`)
	h.Create(c)
	assert.NotEqual(t, 200, w.Code)
}

// ---- UploadArtifact ----

func TestV9UploadFindOneError(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	rel := f.seedV9(t, model.ReleaseStatusDraft, "1.0.0", 0)
	v9FailQueryOn(f.db, "game_releases", 1)

	_, err = f.svc.UploadArtifact(t.Context(), &UploadArtifactRequest{
		ID: fmt.Sprint(rel.ID), Data: strings.NewReader("bytes"), Size: 5,
	})
	require.Error(t, err)
}

func TestV9UploadObjectStoreNil(t *testing.T) {
	f := newV9ReleaseFixture(t, nil)
	rel := f.seedV9(t, model.ReleaseStatusDraft, "1.0.0", 0)

	_, err := f.svc.UploadArtifact(t.Context(), &UploadArtifactRequest{
		ID: fmt.Sprint(rel.ID), Data: strings.NewReader("bytes"), Size: 5,
	})
	require.ErrorContains(t, err, "对象存储未配置")
}

func TestV9UploadPutError(t *testing.T) {
	f := newV9ReleaseFixture(t, failStoreV9{})
	rel := f.seedV9(t, model.ReleaseStatusDraft, "1.0.0", 0)

	_, err := f.svc.UploadArtifact(t.Context(), &UploadArtifactRequest{
		ID: fmt.Sprint(rel.ID), Data: strings.NewReader("bytes"), Size: 5,
		ContentType: "application/octet-stream",
	})
	require.ErrorContains(t, err, "上传对象存储失败")
}

func TestV9UploadUpdateError(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	rel := f.seedV9(t, model.ReleaseStatusDraft, "1.0.0", 0)
	v9FailUpdateOn(f.db, "game_releases", "")

	_, err = f.svc.UploadArtifact(t.Context(), &UploadArtifactRequest{
		ID: fmt.Sprint(rel.ID), Data: strings.NewReader("bytes"), Size: 5,
		ContentType: "application/octet-stream",
	})
	require.Error(t, err)
}

func TestV9UploadRefetchError(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	rel := f.seedV9(t, model.ReleaseStatusDraft, "1.0.0", 0)
	v9FailQueryOn(f.db, "game_releases", 2) // FindOne ok, re-FindOne fails

	_, err = f.svc.UploadArtifact(t.Context(), &UploadArtifactRequest{
		ID: fmt.Sprint(rel.ID), Data: strings.NewReader("bytes"), Size: 5,
		ContentType: "application/octet-stream",
	})
	require.Error(t, err)
}

// ---- Transition ----

func TestV9TransitionArchiveAction(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	rel := f.seedV9(t, model.ReleaseStatusDraft, "1.0.0", 0)

	resp, err := f.svc.Transition(t.Context(), &ReleaseTransitionRequest{ID: uitoa(rel.ID), Action: "archive"})
	require.NoError(t, err)
	assert.Equal(t, model.ReleaseStatusArchived, resp.Release.Status)
}

func TestV9HandlerTransitionSuccess(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	rel := f.seedV9(t, model.ReleaseStatusGray, "2.0.0", 40)

	h := NewHandler(f.svc)
	c, w := v9GinCtx("POST", fmt.Sprintf("/releases/%d/transition", rel.ID), `{"action":"full"}`)
	c.Params = gin.Params{{Key: "id", Value: uitoa(rel.ID)}}
	h.Transition(c)
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"status":"full"`)
}

// ---- CheckUpdate ----

func TestV9CheckUpdateFindCandidatesErrorAndHandlerError(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	v9FailQueryOn(f.db, "game_releases", 1)

	_, err = f.svc.CheckUpdate(t.Context(), &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Platform: "android", DeviceID: "d1", CurrentVersion: "1.0.0",
	})
	require.Error(t, err)

	// handler service-error branch: empty deviceId
	h := NewHandler(f.svc)
	c, w := v9GinCtx("POST", "/releases/check", `{"gameId":"demo","platform":"android"}`)
	h.CheckUpdate(c)
	assert.NotEqual(t, 200, w.Code)
}

func TestV9CheckUpdateClientAlreadyNewer(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	f.seedV9(t, model.ReleaseStatusFull, "2.0.0", 100)

	resp, err := f.svc.CheckUpdate(t.Context(), &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		DeviceID: "d1", CurrentVersion: "3.0.0",
	})
	require.NoError(t, err)
	assert.False(t, resp.Update) // compareVersion <= 0 → skip candidate
}

func TestV9DeltaBadCurrentManifest(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)

	v1 := f.seedV9(t, model.ReleaseStatusArchived, "1.0.0", 0)
	v1.Manifest = model.JSON(`{not-json`)
	require.NoError(t, f.db.Save(v1).Error)
	v2 := f.seedV9(t, model.ReleaseStatusFull, "2.0.0", 100)
	v2.Manifest = model.JSON(`{"a.lua":{"hash":"h1","size":10}}`)
	require.NoError(t, f.db.Save(v2).Error)

	resp, err := f.svc.CheckUpdate(t.Context(), &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		DeviceID: "d1", CurrentVersion: "1.0.0",
	})
	require.NoError(t, err)
	require.True(t, resp.Update)
	assert.Nil(t, resp.DeltaFiles) // current manifest unparseable → no delta
}

func TestV9DeltaBadMatchedManifest(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)

	v1 := f.seedV9(t, model.ReleaseStatusArchived, "1.0.0", 0)
	v1.Manifest = model.JSON(`{"a.lua":{"hash":"h1","size":10}}`)
	require.NoError(t, f.db.Save(v1).Error)
	v2 := f.seedV9(t, model.ReleaseStatusFull, "2.0.0", 100)
	v2.Manifest = model.JSON(`{not-json`)
	require.NoError(t, f.db.Save(v2).Error)

	resp, err := f.svc.CheckUpdate(t.Context(), &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		DeviceID: "d1", CurrentVersion: "1.0.0",
	})
	require.NoError(t, err)
	require.True(t, resp.Update)
	assert.Nil(t, resp.DeltaFiles) // matched manifest unparseable → no delta
}

func TestV9DeltaEmptyDiff(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	manifest := model.JSON(`{"a.lua":{"hash":"h1","size":10}}`)

	v1 := f.seedV9(t, model.ReleaseStatusArchived, "1.0.0", 0)
	v1.Manifest = manifest
	require.NoError(t, f.db.Save(v1).Error)
	v2 := f.seedV9(t, model.ReleaseStatusFull, "2.0.0", 100)
	v2.Manifest = manifest
	require.NoError(t, f.db.Save(v2).Error)

	resp, err := f.svc.CheckUpdate(t.Context(), &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		DeviceID: "d1", CurrentVersion: "1.0.0",
	})
	require.NoError(t, err)
	require.True(t, resp.Update)
	require.NotNil(t, resp.DeltaFiles) // empty-but-present diff
	assert.Empty(t, resp.DeltaFiles)
	assert.Equal(t, int64(0), resp.DeltaSize)
}

// ---- pure helpers ----

func TestV9CompareVersionAndAtoi(t *testing.T) {
	assert.Equal(t, 0, compareVersion("1.0", "1.0.0")) // equal with padding
	assert.Equal(t, 0, compareVersion("2.3.4", "2.3.4"))
	assert.Equal(t, -1, compareVersion("1", "1.1"))       // av < bv
	assert.Equal(t, -1, compareVersion("1.9", "1.10"))    // numeric compare
	assert.Equal(t, 1, compareVersion("1.0.1", "1"))      // av > bv with padding
	assert.Equal(t, 0, compareVersion("2.0", "2.0-beta")) // atoi stops at non-digit: "0-beta" == 0

	assert.Equal(t, 1, atoi("1"))
	assert.Equal(t, 42, atoi("42"))
	assert.Equal(t, 1, atoi("1x"))  // non-digit stops parsing
	assert.Equal(t, 0, atoi(""))    // empty
	assert.Equal(t, 0, atoi("abc")) // no digits
}

func TestV9CurrentUsernameFallback(t *testing.T) {
	assert.Equal(t, "system", currentUsername(context.Background()))

	ctx := context.WithValue(context.Background(), "username", "alice")
	assert.Equal(t, "alice", currentUsername(ctx))

	blank := context.WithValue(context.Background(), "username", "   ")
	assert.Equal(t, "system", currentUsername(blank))
}

func TestV9BuildReleaseDTOWhitelist(t *testing.T) {
	rel := &model.GameRelease{
		Model: gorm.Model{ID: 7}, GameID: "demo", Env: "prod", Channel: "official",
		Platform: "android", Version: "1.0.0", Type: "full", Status: "draft",
		Whitelist: model.JSON(`["w1"," w2 "]`),
	}
	dto := buildReleaseDTO(rel)
	assert.Equal(t, []string{"w1", " w2 "}, dto.Whitelist)

	bad := &model.GameRelease{Whitelist: model.JSON(`{not-json`)}
	assert.Nil(t, buildReleaseDTO(bad).Whitelist)

	none := &model.GameRelease{}
	assert.Nil(t, buildReleaseDTO(none).Whitelist)
}

// ---- handler upload content-type override ----

func TestV9HandlerUploadContentTypeOverride(t *testing.T) {
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	f := newV9ReleaseFixture(t, store)
	rel := f.seedV9(t, model.ReleaseStatusDraft, "1.0.0", 0)
	h := NewHandler(f.svc)

	mkReq := func(contentType string) (*gin.Context, *httptest.ResponseRecorder) {
		var buf strings.Builder
		mw := multipart.NewWriter(&buf)
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="file"; filename="pkg.bin"`)
		if contentType != "" {
			hdr.Set("Content-Type", contentType)
		}
		pw, err := mw.CreatePart(hdr)
		require.NoError(t, err)
		_, err = pw.Write([]byte("pkg"))
		require.NoError(t, err)
		require.NoError(t, mw.Close())

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest("POST", fmt.Sprintf("/releases/%d/artifact", rel.ID), strings.NewReader(buf.String()))
		req.Header.Set("Content-Type", mw.FormDataContentType())
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: uitoa(rel.ID)}}
		return c, w
	}

	// no part content-type → default octet-stream
	c1, w1 := mkReq("")
	h.UploadArtifact(c1)
	require.Equal(t, 200, w1.Code, w1.Body.String())

	// re-upload in uploading status with multipart/* content-type → override
	c2, w2 := mkReq("multipart/mixed")
	h.UploadArtifact(c2)
	require.Equal(t, 200, w2.Code, w2.Body.String())
	assert.Contains(t, w2.Body.String(), `"status":"uploading"`)
}
