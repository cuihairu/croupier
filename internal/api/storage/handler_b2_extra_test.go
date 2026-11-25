package storage

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandlerUploadObject_PreParsedFormTempFileRemoved pre-parses the multipart form and
// removes its temporary file. http.Request.FormFile tries fh.Open() and fails before
// the handler runs, so the handler takes the "no file" branch and answers 400.
func TestHandlerUploadObject_PreParsedFormTempFileRemoved(t *testing.T) {
	handler := setupHandler(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/storage/objects", handler.UploadObject)

	var buf strings.Builder
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "gone.txt")
	require.NoError(t, err)
	_, err = fw.Write([]byte("0123456789"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	body := []byte(buf.String())
	boundary := w.Boundary()
	mr := multipart.NewReader(strings.NewReader(string(body)), boundary)
	form, err := mr.ReadForm(1)
	require.NoError(t, err)
	require.Len(t, form.File["file"], 1)
	require.NoError(t, form.RemoveAll())

	req := httptest.NewRequest(http.MethodPost, "/storage/objects", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.MultipartForm = form

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
