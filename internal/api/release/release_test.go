package release

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

var relDBSeq uint64

func newReleaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("rel_%d", atomic.AddUint64(&relDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

type releaseFixture struct {
	db  *gorm.DB
	svc *Service
}

func newFixture(t *testing.T) *releaseFixture {
	db := newReleaseTestDB(t)
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: t.TempDir()})
	require.NoError(t, err)
	svcCtx := &svc.ServiceContext{
		ReleaseModel: model.NewGameReleaseModel(db),
		ObjectStore:  store,
	}
	return &releaseFixture{db: db, svc: NewService(svcCtx)}
}

func (f *releaseFixture) seedRelease(t *testing.T, status string, version string, gray int) *model.GameRelease {
	t.Helper()
	rel := &model.GameRelease{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		Version: version, Type: model.ReleaseTypeFull, Status: status,
		GrayPercent: gray, GraySeed: "seed",
	}
	require.NoError(t, f.db.Create(rel).Error)
	return rel
}

func TestReleaseLifecycle_Transitions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	rel := f.seedRelease(t, model.ReleaseStatusDraft, "1.0.0", 0)
	m := model.NewGameReleaseModel(f.db)

	// testing without artifact is rejected (draft must upload first: the
	// upload endpoint drives draft → uploading).
	_, err := m.Transition(ctx, rel.ID, model.ReleaseStatusTesting, nil)
	require.ErrorContains(t, err, "invalid transition")

	// Upload artifact (draft → uploading) then testing.
	_, err = f.svc.UploadArtifact(ctx, &UploadArtifactRequest{
		ID: fmt.Sprint(rel.ID), Data: strings.NewReader("package-bytes"),
		Size: 13, ContentType: "application/octet-stream",
	})
	require.NoError(t, err)
	stored, err := m.FindOne(ctx, rel.ID)
	require.NoError(t, err)
	require.Equal(t, model.ReleaseStatusUploading, stored.Status)

	_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusTesting, nil)
	require.NoError(t, err)

	pct := 20
	_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusGray, &pct)
	require.NoError(t, err)

	// Decreasing gray percent is rejected (rollback semantics).
	lower := 10
	_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusGray, &lower)
	require.ErrorContains(t, err, "only increase")

	pct = 60
	_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusGray, &pct)
	require.NoError(t, err)

	full, err := m.Transition(ctx, rel.ID, model.ReleaseStatusFull, nil)
	require.NoError(t, err)
	assert.Equal(t, 100, full.GrayPercent)

	// Illegal: full back to gray.
	_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusGray, &pct)
	require.ErrorContains(t, err, "invalid transition")
}

func TestReleaseSingleFullInvariant(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	m := model.NewGameReleaseModel(f.db)

	v1 := f.seedRelease(t, model.ReleaseStatusDraft, "1.0.0", 0)
	v2 := f.seedRelease(t, model.ReleaseStatusDraft, "1.1.0", 0)
	for _, rel := range []*model.GameRelease{v1, v2} {
		_, err := f.svc.UploadArtifact(ctx, &UploadArtifactRequest{
			ID: fmt.Sprint(rel.ID), Data: strings.NewReader("pkg"), Size: 3,
			ContentType: "application/octet-stream",
		})
		require.NoError(t, err)
		_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusTesting, nil)
		require.NoError(t, err)
		full100 := 100
		_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusGray, &full100)
		require.NoError(t, err)
		_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusFull, nil)
		require.NoError(t, err)
	}

	// Promoting v2 to full archived v1.
	v1After, err := m.FindOne(ctx, v1.ID)
	require.NoError(t, err)
	assert.Equal(t, model.ReleaseStatusArchived, v1After.Status)
}

func TestCheckUpdate_GrayBucketStability(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	rel := f.seedRelease(t, model.ReleaseStatusGray, "2.0.0", 50)
	rel.ObjectKey = "releases/x"
	require.NoError(t, f.db.Save(rel).Error)

	// Same device must always resolve the same way across repeated checks.
	device := "device-42"
	first := deviceSees(ctx, t, f, device)
	for range 10 {
		assert.Equal(t, first, deviceSees(ctx, t, f, device))
	}

	// 100% gray: every device sees it.
	require.NoError(t, f.db.Model(&model.GameRelease{}).Where("id = ?", rel.ID).
		Update("gray_percent", 100).Error)
	assert.True(t, deviceSees(ctx, t, f, device))

	// 0%: nobody sees it.
	require.NoError(t, f.db.Model(&model.GameRelease{}).Where("id = ?", rel.ID).
		Update("gray_percent", 0).Error)
	assert.False(t, deviceSees(ctx, t, f, device))
}

func TestCheckUpdate_VersionComparison(t *testing.T) {
	f := newFixture(t)
	// Client already on the newest version: no update.
	f.seedRelease(t, model.ReleaseStatusFull, "2.0.0", 100)
	f.db.Exec("UPDATE game_releases SET object_key = 'k'")

	resp, err := f.svc.CheckUpdate(context.Background(), &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		DeviceID: "d1", CurrentVersion: "2.0.0",
	})
	require.NoError(t, err)
	assert.False(t, resp.Update)

	// Older client: update.
	resp, err = f.svc.CheckUpdate(context.Background(), &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		DeviceID: "d1", CurrentVersion: "1.9.9",
	})
	require.NoError(t, err)
	assert.True(t, resp.Update)
	assert.Equal(t, "2.0.0", resp.Version)
}

func TestHandler_UploadArtifact_MissingFile(t *testing.T) {
	f := newFixture(t)
	rel := f.seedRelease(t, model.ReleaseStatusDraft, "3.0.0", 0)
	h := NewHandler(f.svc)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader("")
	c.Request = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/releases/%d/artifact", rel.ID), body)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(rel.ID)}}
	h.UploadArtifact(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func deviceSees(ctx context.Context, t *testing.T, f *releaseFixture, device string) bool {
	t.Helper()
	resp, err := f.svc.CheckUpdate(ctx, &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		DeviceID: device, CurrentVersion: "1.0.0",
	})
	require.NoError(t, err)
	return resp.Update
}

var _ = time.Minute

// TestCheckUpdate_DeltaDownloads verifies the P2 manifest diff: changed and
// added files are listed, unchanged ones are not.
func TestCheckUpdate_DeltaDownloads(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	m := model.NewGameReleaseModel(f.db)

	// v1.0.0 (archived full) with manifest {a,b,c}.
	v1 := f.seedRelease(t, model.ReleaseStatusDraft, "1.0.0", 0)
	v1.Manifest = model.JSON(`{"a.lua":{"hash":"h1","size":10},"b.lua":{"hash":"h2","size":20},"c.lua":{"hash":"h3","size":30}}`)
	require.NoError(t, f.db.Save(v1).Error)
	v2 := f.seedRelease(t, model.ReleaseStatusDraft, "2.0.0", 0)
	// a unchanged, b changed, c removed, d added.
	v2.Manifest = model.JSON(`{"a.lua":{"hash":"h1","size":10},"b.lua":{"hash":"h2x","size":25},"d.lua":{"hash":"h4","size":40}}`)
	require.NoError(t, f.db.Save(v2).Error)

	for _, rel := range []*model.GameRelease{v1, v2} {
		_, err := f.svc.UploadArtifact(ctx, &UploadArtifactRequest{
			ID: fmt.Sprint(rel.ID), Data: strings.NewReader("pkg"), Size: 3,
			ContentType: "application/octet-stream",
		})
		require.NoError(t, err)
		_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusTesting, nil)
		require.NoError(t, err)
		pct := 100
		_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusGray, &pct)
		require.NoError(t, err)
		_, err = m.Transition(ctx, rel.ID, model.ReleaseStatusFull, nil)
		require.NoError(t, err)
	}

	resp, err := f.svc.CheckUpdate(ctx, &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Channel: "official", Platform: "android",
		DeviceID: "d-delta", CurrentVersion: "1.0.0",
	})
	require.NoError(t, err)
	require.True(t, resp.Update)
	assert.Equal(t, "2.0.0", resp.Version)
	// Changed + added only; c.lua removal is client-side cleanup.
	require.Len(t, resp.DeltaFiles, 2)
	assert.Equal(t, []string{"b.lua", "d.lua"}, resp.DeltaFiles)
	assert.Equal(t, int64(65), resp.DeltaSize)
}
