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

// A uri binding failure (missing :id param) should return 400 directly,
// without reaching the service.
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
	// c.Params is deliberately left unset: ShouldBindUri cannot find the id
	// -> binding fails
	h.UploadArtifact(c)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// For a multipart upload carrying an invalid manifest, the service error must
// come back through the handler's unified error path.
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
