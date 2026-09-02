package hotpatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRouter 按生产路由（internal/handler/routes.go registerHotpatchRoutes）
// 组装被测 handler。
func newRouter(t *testing.T) (*gin.Engine, *Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svcSrv, _ := newFixtureWithDB(t)
	h := NewHandler(svcSrv)
	g := r.Group("/hotpatches")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.POST("/:id/package", h.UploadPackage)
	g.POST("/:id/transition", h.Transition)
	g.POST("/:id/results", h.ReportResult)
	return r, svcSrv
}

func doJSON(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
	return out
}

func createViaHTTP(t *testing.T, r *gin.Engine) int64 {
	t.Helper()
	rec := doJSON(t, r, http.MethodPost, "/hotpatches", `{
		"gameId":"demo","env":"prod","framework":"skynet","bugId":42,"title":"修复背包闪退"
	}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	id, ok := decode(t, rec)["id"].(float64)
	require.True(t, ok, rec.Body.String())
	return int64(id)
}

// contentType 三种形态："text/plain"（显式）、"formfile"（CreateFormFile 默认
// octet-stream）、"none"（分片无 Content-Type → handler 回退 octet-stream）。
func multipartBody(t *testing.T, contentType string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="patch.bin"`)
	switch contentType {
	case "text/plain":
		hdr.Set("Content-Type", "text/plain")
	case "formfile":
		fw, err := w.CreateFormFile("file", "patch.bin")
		require.NoError(t, err)
		_, err = fw.Write([]byte("patch-bytes"))
		require.NoError(t, err)
		require.NoError(t, w.Close())
		return &buf, w.FormDataContentType()
	}
	part, err := w.CreatePart(hdr)
	require.NoError(t, err)
	_, err = part.Write([]byte("patch-bytes"))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

func TestHandler_List(t *testing.T) {
	r, _ := newRouter(t)
	createViaHTTP(t, r)

	rec := doJSON(t, r, http.MethodGet, "/hotpatches?gameId=demo&framework=SKYNET&status=draft", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resp := decode(t, rec)
	assert.EqualValues(t, 1, resp["total"])
	items, ok := resp["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	assert.Equal(t, "skynet", item["framework"])
	assert.Equal(t, "draft", item["status"])
	assert.Equal(t, "demo", item["gameId"])

	// Bug4 修复后：page 非数字按契约返回 400
	rec = doJSON(t, r, http.MethodGet, "/hotpatches?page=abc", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateValidation(t *testing.T) {
	r, _ := newRouter(t)

	rec := doJSON(t, r, http.MethodPost, "/hotpatches", `{"framework":"skynet","bugId":1,"title":"x"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 非法 JSON → 400。
	rec = doJSON(t, r, http.MethodPost, "/hotpatches", `{"framework":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 字段校验：缺标题 / 缺 bugId / 非法框架。
	for _, body := range []string{
		`{"framework":"skynet","bugId":1}`,
		`{"framework":"skynet","title":"x"}`,
		`{"framework":"lua","bugId":1,"title":"x"}`,
	} {
		rec = doJSON(t, r, http.MethodPost, "/hotpatches", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.Contains(t, rec.Body.String(), "error", body)
	}
}

func TestHandler_UploadPackage(t *testing.T) {
	r, _ := newRouter(t)
	id := createViaHTTP(t, r)

	upload := func(contentType string) *httptest.ResponseRecorder {
		buf, ct := multipartBody(t, contentType)
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/hotpatches/%d/package", id), buf)
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	rec := upload("text/plain")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	item := decode(t, rec)
	assert.NotEmpty(t, item["checksum"])
	assert.EqualValues(t, len("patch-bytes"), item["size"])

	// 分片无 Content-Type → handler 回退 application/octet-stream。
	rec = upload("none")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// 缺 file 字段 → 400。
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/hotpatches/%d/package", id), strings.NewReader("junk"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 非 uint id / 不存在的 id（需带 multipart body 才能进入 service 分支）。
	uploadTo := func(id string) *httptest.ResponseRecorder {
		buf, ct := multipartBody(t, "text/plain")
		req := httptest.NewRequest(http.MethodPost, "/hotpatches/"+id+"/package", buf)
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	rec = uploadTo("abc")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = uploadTo("999999")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// 非草稿状态再上传 → 409。
	rec = doJSON(t, r, http.MethodPost, fmt.Sprintf("/hotpatches/%d/transition", id), `{"action":"approve"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = upload("text/plain")
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "草稿")
}

func TestHandler_Transition(t *testing.T) {
	r, _ := newRouter(t)
	id := createViaHTTP(t, r)

	// 空请求体（ContentLength=0）→ 跳过绑定，action 为空 → 400。
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/hotpatches/%d/transition", id), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 非法 action → 400。
	rec = doJSON(t, r, http.MethodPost, fmt.Sprintf("/hotpatches/%d/transition", id), `{"action":"explode"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 未传包 approve → 409（守卫分支）。
	rec = doJSON(t, r, http.MethodPost, fmt.Sprintf("/hotpatches/%d/transition", id), `{"action":"approve"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)

	// 先上传补丁包再流转：approve → roll(30)。
	ub, ct := multipartBody(t, "text/plain")
	upReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/hotpatches/%d/package", id), ub)
	upReq.Header.Set("Content-Type", ct)
	upRec := httptest.NewRecorder()
	r.ServeHTTP(upRec, upReq)
	require.Equal(t, http.StatusOK, upRec.Code, upRec.Body.String())

	rec = doJSON(t, r, http.MethodPost, fmt.Sprintf("/hotpatches/%d/transition", id), `{"action":"approve"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = doJSON(t, r, http.MethodPost, fmt.Sprintf("/hotpatches/%d/transition", id), `{"action":"roll","rolloutPercent":30}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "rolling", decode(t, rec)["status"])
	assert.EqualValues(t, 30, decode(t, rec)["rolloutPercent"])

	// 非 uint id → 400。
	rec = doJSON(t, r, http.MethodPost, "/hotpatches/0/transition", `{"action":"approve"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ReportResult(t *testing.T) {
	r, _ := newRouter(t)
	id := createViaHTTP(t, r)

	rec := doJSON(t, r, http.MethodPost, fmt.Sprintf("/hotpatches/%d/results", id),
		`{"agentId":"agent-1","node":"node-a","status":"ok","log":"inject ok"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "已记录")

	// 结果随列表回读。
	rec = doJSON(t, r, http.MethodGet, "/hotpatches", "")
	require.Equal(t, http.StatusOK, rec.Code)
	resp := decode(t, rec)
	items := resp["items"].([]any)
	results := items[0].(map[string]any)["results"].([]any)
	require.Len(t, results, 1)
	res := results[0].(map[string]any)
	assert.Equal(t, "agent-1", res["agentId"])
	assert.NotEmpty(t, res["at"])

	// 缺 body → 400；非法 id → 400。
	rec = doJSON(t, r, http.MethodPost, fmt.Sprintf("/hotpatches/%d/results", id), "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doJSON(t, r, http.MethodPost, "/hotpatches/xyz/results", `{"agentId":"a","status":"ok"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_TransitionBadJSONBody(t *testing.T) {
	r, _ := newRouter(t)
	id := createViaHTTP(t, r)

	// 非法 JSON 且 ContentLength>0 → 400。
	rec := doJSON(t, r, http.MethodPost, fmt.Sprintf("/hotpatches/%d/transition", id), `{"action":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_TransitionAppliedAndFailActions(t *testing.T) {
	r, _ := newRouter(t)
	id := createViaHTTP(t, r)

	// draft → fail。
	rec := doJSON(t, r, http.MethodPost, fmt.Sprintf("/hotpatches/%d/transition", id), `{"action":"fail"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "failed", decode(t, rec)["status"])

	// applied 动作：新草稿走完整链路后触发。
	id2 := createViaHTTP(t, r)
	ub, ct := multipartBody(t, "text/plain")
	upReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/hotpatches/%d/package", id2), ub)
	upReq.Header.Set("Content-Type", ct)
	upRec := httptest.NewRecorder()
	r.ServeHTTP(upRec, upReq)
	require.Equal(t, http.StatusOK, upRec.Code)
	for _, action := range []string{"approve", "roll", "applied"} {
		rec = doJSON(t, r, http.MethodPost, fmt.Sprintf("/hotpatches/%d/transition", id2), `{"action":"`+action+`"}`)
		require.Equal(t, http.StatusOK, rec.Code, action)
	}
	assert.Equal(t, "applied", decode(t, rec)["status"])
}

func TestHandler_ListDBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svcSrv, db := newFixtureWithDB(t)
	h := NewHandler(svcSrv)
	r.GET("/hotpatches", h.List)
	r.POST("/hotpatches", h.Create)
	createViaHTTP(t, r)

	// 模型层错误 → 500。
	require.NoError(t, db.Migrator().DropTable("hotpatches"))
	rec := doJSON(t, r, http.MethodGet, "/hotpatches", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
