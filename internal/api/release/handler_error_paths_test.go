package release

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
)

// uri 绑定失败（缺少 :id 参数）应直接 400，不触达 service。
func TestHandlerUploadArtifact_BadUriBinding(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)
	rel := f.seedRelease(t, model.ReleaseStatusDraft, "1.0.0", 0)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "pkg.bin")
	require.NoError(t, err)
	_, err = fw.Write([]byte("bytes"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/releases/%d/artifact", rel.ID), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.Request = req
	// 故意不设置 c.Params：ShouldBindUri 找不到 id → 绑定失败
	h.UploadArtifact(c)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// multipart 上传携带非法 manifest 时，service 错误应经 handler 统一回错。
func TestHandlerUploadArtifact_ServiceErrorThroughHandler(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)
	rel := f.seedRelease(t, model.ReleaseStatusDraft, "1.0.0", 0)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "pkg.bin")
	require.NoError(t, err)
	_, err = fw.Write([]byte("package-bytes"))
	require.NoError(t, err)
	require.NoError(t, mw.WriteField("manifest", `{not json`))
	require.NoError(t, mw.Close())

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(rel.ID)}}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/releases/%d/artifact", rel.ID), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.Request = req
	h.UploadArtifact(c)

	assert.NotEqual(t, http.StatusOK, w.Code, "invalid manifest should not upload successfully")
	assert.True(t, strings.Contains(w.Body.String(), "error"), w.Body.String())
}
