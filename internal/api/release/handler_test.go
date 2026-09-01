package release

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
)

func ginCtx(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
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

func TestHandlerList(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)
	f.seedRelease(t, model.ReleaseStatusDraft, "1.0.0", 0)

	c, w := ginCtx("GET", "/releases?gameId=demo&env=prod", "")
	h.List(c)
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"total":1`)

	// 非法 query 参数（pageSize 非数字）走 bind 错误分支
	c2, w2 := ginCtx("GET", "/releases?pageSize=abc", "")
	h.List(c2)
	assert.NotEqual(t, 200, w2.Code)
}

func TestHandlerCreate(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)

	c, w := ginCtx("POST", "/releases", `{"gameId":"demo","env":"prod","channel":"official","platform":"android","version":"1.0.0"}`)
	h.Create(c)
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"version":"1.0.0"`)

	// 坏 JSON → bind 错误
	c3, w3 := ginCtx("POST", "/releases", `{bad`)
	h.Create(c3)
	assert.NotEqual(t, 200, w3.Code)
}

func TestHandlerTransition(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)
	rel := f.seedRelease(t, model.ReleaseStatusDraft, "1.0.0", 0)

	// 缺 artifact 的 draft → testing 被拒（状态机保护）
	c, w := ginCtx("POST", fmt.Sprintf("/releases/%d/transition", rel.ID), `{"action":"testing"}`)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(rel.ID)}}
	h.Transition(c)
	assert.NotEqual(t, 200, w.Code)

	// 非法 action
	c2, w2 := ginCtx("POST", fmt.Sprintf("/releases/%d/transition", rel.ID), `{"action":"explode"}`)
	c2.Params = gin.Params{{Key: "id", Value: fmt.Sprint(rel.ID)}}
	h.Transition(c2)
	assert.NotEqual(t, 200, w2.Code)

	// 坏 JSON
	c3, w3 := ginCtx("POST", fmt.Sprintf("/releases/%d/transition", rel.ID), `{bad`)
	c3.Params = gin.Params{{Key: "id", Value: fmt.Sprint(rel.ID)}}
	h.Transition(c3)
	assert.NotEqual(t, 200, w3.Code)
}

func TestHandlerUploadArtifact(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)
	rel := f.seedRelease(t, model.ReleaseStatusDraft, "1.0.0", 0)

	// 构造 multipart 表单
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "pkg.bin")
	require.NoError(t, err)
	_, err = fw.Write([]byte("package-bytes"))
	require.NoError(t, err)
	require.NoError(t, mw.WriteField("manifest", `{"files":["a.so"]}`))
	require.NoError(t, mw.Close())

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(rel.ID)}}
	req := httptest.NewRequest("POST", fmt.Sprintf("/releases/%d/artifact", rel.ID), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.Request = req
	h.UploadArtifact(c)
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"status":"uploading"`)

	// 非草稿状态禁止上传
	c2, w2 := ginCtx("POST", fmt.Sprintf("/releases/%d/artifact", rel.ID), "")
	c2.Params = gin.Params{{Key: "id", Value: fmt.Sprint(rel.ID)}}
	h.UploadArtifact(c2) // 无 multipart → 缺少 file 字段
	assert.NotEqual(t, 200, w2.Code)
}

func TestHandlerCheckUpdate(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)

	// 无 release → update:false
	c, w := ginCtx("POST", "/releases/check", `{"gameId":"demo","env":"prod","platform":"android","deviceId":"d1","currentVersion":"9.9.9"}`)
	h.CheckUpdate(c)
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"update":false`)

	// 坏 JSON
	c2, w2 := ginCtx("POST", "/releases/check", `{bad`)
	h.CheckUpdate(c2)
	assert.NotEqual(t, 200, w2.Code)
}

func TestWhitelistHit(t *testing.T) {
	assert.False(t, whitelistHit(nil, "d1"))
	assert.False(t, whitelistHit(model.JSON(`{not json`), "d1"))
	assert.True(t, whitelistHit(model.JSON(`["d1","d2"]`), "d1"))
	assert.True(t, whitelistHit(model.JSON(`[" d1 "]`), "d1")) // TrimSpace
	assert.False(t, whitelistHit(model.JSON(`["d2"]`), "d1"))
}

func TestSeekWrapSeek(t *testing.T) {
	sw := &seekWrap{r: strings.NewReader("x")}
	n, err := sw.Seek(0, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

func TestUploadArtifactInvalidManifest(t *testing.T) {
	f := newFixture(t)
	rel := f.seedRelease(t, model.ReleaseStatusDraft, "1.0.0", 0)
	_, err := f.svc.UploadArtifact(t.Context(), &UploadArtifactRequest{
		ID: fmt.Sprint(rel.ID), Data: strings.NewReader("bytes"),
		Size: 5, ContentType: "application/octet-stream",
		Manifest: []byte(`{not json`),
	})
	require.ErrorContains(t, err, "manifest")
}

func TestUploadArtifactNotDraft(t *testing.T) {
	f := newFixture(t)
	rel := f.seedRelease(t, model.ReleaseStatusFull, "1.0.0", 0)
	_, err := f.svc.UploadArtifact(t.Context(), &UploadArtifactRequest{
		ID: fmt.Sprint(rel.ID), Data: strings.NewReader("bytes"),
		Size: 5, ContentType: "application/octet-stream",
	})
	require.ErrorContains(t, err, "草稿")
}

func TestUploadArtifactBadID(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.UploadArtifact(t.Context(), &UploadArtifactRequest{
		ID: "not-a-number", Data: strings.NewReader("x"), Size: 1,
	})
	require.Error(t, err)
}

func TestCheckUpdatePositivePaths(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()

	// full 状态的高版本 → 所有设备可见
	full := f.seedRelease(t, model.ReleaseStatusFull, "2.0.0", 0)
	full.ObjectKey = "releases/demo/prod/official/android/2.0.0-1.bin"
	full.Size = 100
	full.Checksum = "abc123"
	full.Manifest = model.JSON(`{"files":["a.so"]}`)
	full.Whitelist = model.JSON(`["vip-device"]`)
	require.NoError(t, f.db.Save(full).Error)

	resp, err := f.svc.CheckUpdate(ctx, &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Platform: "android",
		DeviceID: "d1", CurrentVersion: "1.0.0",
	})
	require.NoError(t, err)
	assert.True(t, resp.Update)
	assert.Equal(t, "2.0.0", resp.Version)
	assert.NotEmpty(t, resp.FullManifest) // buildReleaseDTO manifest 分支

	// testing 状态：白名单命中才可见
	f2 := newFixture(t)
	testing_ := f2.seedRelease(t, model.ReleaseStatusTesting, "2.1.0", 0)
	testing_.Whitelist = model.JSON(`["vip-device"]`)
	require.NoError(t, f2.db.Save(testing_).Error)

	resp2, err := f2.svc.CheckUpdate(ctx, &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Platform: "android",
		DeviceID: "vip-device", CurrentVersion: "1.0.0",
	})
	require.NoError(t, err)
	assert.True(t, resp2.Update)

	// 白名单未命中 → 不可见
	resp3, err := f2.svc.CheckUpdate(ctx, &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Platform: "android",
		DeviceID: "random-device", CurrentVersion: "1.0.0",
	})
	require.NoError(t, err)
	assert.False(t, resp3.Update)

	// gray 状态：灰度桶命中
	f3 := newFixture(t)
	gray := f3.seedRelease(t, model.ReleaseStatusGray, "2.2.0", 100) // 100% 灰度必中
	require.NoError(t, f3.db.Save(gray).Error)
	resp4, err := f3.svc.CheckUpdate(ctx, &CheckUpdateRequest{
		GameID: "demo", Env: "prod", Platform: "android",
		DeviceID: "any", CurrentVersion: "1.0.0",
	})
	require.NoError(t, err)
	assert.True(t, resp4.Update)

	// deviceId 空 → 400
	_, err = f.svc.CheckUpdate(ctx, &CheckUpdateRequest{GameID: "demo", Platform: "android"})
	require.Error(t, err)
}
