package game

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01}
var jpegMagic = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
var webpMagic = append(append([]byte("RIFF"), 0, 0, 0, 0), "WEBP"...)

func TestSniffIconKind(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		ext  string
		ct   string
		err  bool
	}{
		{"png", pngMagic, "png", "image/png", false},
		{"jpeg", jpegMagic, "jpg", "image/jpeg", false},
		{"webp", webpMagic, "webp", "image/webp", false},
		{"svg with xml decl", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`), "svg", "image/svg+xml", false},
		{"svg uppercase", []byte(`<SVG/>`), "svg", "image/svg+xml", false},
		{"plain text rejected", []byte("hello world, not an icon"), "", "", true},
		{"empty rejected", nil, "", "", true},
		{"html rejected", []byte("<html><body>x</body></html>"), "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext, ct, err := sniffIconKind(tc.data)
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.ext, ext)
			assert.Equal(t, tc.ct, ct)
		})
	}
}

// iconTestService assembles a service with file object storage (permission cases reuse the
// seeding that already exists in this package).
func iconTestService(t *testing.T, driver string) (*Service, string) {
	t.Helper()
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	baseDir := t.TempDir()
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: baseDir})
	require.NoError(t, err)
	svcCtx.ObjectStore = store
	svcCtx.Config = config.Config{Storage: config.StorageConfig{Driver: driver, BaseDir: baseDir}}
	return &Service{svcCtx: svcCtx}, baseDir
}

func TestService_UploadIcon_PermissionDenied(t *testing.T) {
	svc, _ := iconTestService(t, "file")
	// Same key as the existing unauthorized case in this package: an unknown username also
	// takes the permission-denied exit.
	ctx := context.WithValue(context.Background(), "username", "unauthorized") //nolint:staticcheck,revive
	_, err := svc.UploadIcon(ctx, bytes.NewReader(pngMagic), int64(len(pngMagic)), "a.png")
	require.Error(t, err)
}

func TestService_UploadIcon_StoreNil(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	svcCtx.Config = config.Config{Storage: config.StorageConfig{Driver: "file"}}
	svc := &Service{svcCtx: svcCtx}

	ctx, _ := createTestAdminWithContext(t, db, "iconadmin", "password123", "admin")
	_, err := svc.UploadIcon(ctx, bytes.NewReader(pngMagic), int64(len(pngMagic)), "a.png")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "对象存储未初始化")
}

func TestService_UploadIcon_DriverGuard(t *testing.T) {
	svc, _ := iconTestService(t, "s3")
	ctx, _ := createTestAdminWithContext(t, svc.svcCtx.DB, "iconadmin2", "password123", "admin")
	_, err := svc.UploadIcon(ctx, bytes.NewReader(pngMagic), int64(len(pngMagic)), "a.png")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "仅支持本地存储")
}

func TestService_UploadIcon_SizeGuard(t *testing.T) {
	svc, _ := iconTestService(t, "file")
	ctx, _ := createTestAdminWithContext(t, svc.svcCtx.DB, "iconadmin3", "password123", "admin")

	_, err := svc.UploadIcon(ctx, bytes.NewReader(pngMagic), 0, "a.png")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "为空")

	big := bytes.Repeat(pngMagic, 100)
	_, err = svc.UploadIcon(ctx, bytes.NewReader(big), IconMaxBytes+1, "a.png")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "超过大小上限")
}

func TestService_UploadIcon_BadContent(t *testing.T) {
	svc, _ := iconTestService(t, "file")
	ctx, _ := createTestAdminWithContext(t, svc.svcCtx.DB, "iconadmin4", "password123", "admin")

	// Text content disguised with a .png extension: rejected by content sniffing, the
	// extension is not trusted.
	_, err := svc.UploadIcon(ctx, strings.NewReader("not an icon at all"), 18, "a.png")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "仅支持")
	// Rejecting content outside the allowlist must be 400 (the client sent the wrong file);
	// it once drifted to a 500 internal error.
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr, "坏内容须为 errorx CodeError（映射 4xx），不可裸 error 归 internal_error 500")
}

func TestService_UploadIcon_ContentAddressed(t *testing.T) {
	svc, baseDir := iconTestService(t, "file")
	ctx, _ := createTestAdminWithContext(t, svc.svcCtx.DB, "iconadmin5", "password123", "admin")

	resp1, err := svc.UploadIcon(ctx, bytes.NewReader(pngMagic), int64(len(pngMagic)), "logo.png")
	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^icons/games/[0-9a-f]{16}\.png$`), resp1.Key)
	assert.Equal(t, "/uploads/"+resp1.Key, resp1.URL)

	// Re-uploading identical content: the same key is overwritten idempotently (same file
	// bytes).
	resp2, err := svc.UploadIcon(ctx, bytes.NewReader(pngMagic), int64(len(pngMagic)), "other-name.png")
	require.NoError(t, err)
	assert.Equal(t, resp1.Key, resp2.Key)

	// Different content: a different key, and the two do not overwrite each other.
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`)
	resp3, err := svc.UploadIcon(ctx, bytes.NewReader(svg), int64(len(svg)), "logo.svg")
	require.NoError(t, err)
	assert.NotEqual(t, resp1.Key, resp3.Key)
	assert.Regexp(t, regexp.MustCompile(`\.svg$`), resp3.Key)

	// On-disk verification: under the file driver, baseDir/icons/games/<key file name> matches
	// the uploaded content.
	written, err := os.ReadFile(filepath.Join(baseDir, filepath.FromSlash(resp1.Key)))
	require.NoError(t, err)
	assert.Equal(t, pngMagic, written)
}

func iconHandlerMultipartRequest(t *testing.T, filename string, content []byte, fieldName string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if content != nil {
		fw, err := mw.CreateFormFile(fieldName, filename)
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/games/icons", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestHandler_UploadIcon_Multipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	baseDir := t.TempDir()
	store, err := objstore.OpenFile(context.Background(), objstore.Config{BaseDir: baseDir})
	require.NoError(t, err)
	svcCtx.ObjectStore = store
	svcCtx.Config = config.Config{Storage: config.StorageConfig{Driver: "file", BaseDir: baseDir}}
	h := NewHandler(&Service{svcCtx: svcCtx})

	ctx, _ := createTestAdminWithContext(t, db, "iconhandler", "password123", "admin")

	// Happy path: 200 plus key/url, and the file lands on disk.
	req := iconHandlerMultipartRequest(t, "logo.png", pngMagic, "file").WithContext(ctx)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.UploadIcon(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp GameIconUploadResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	assert.Regexp(t, regexp.MustCompile(`^icons/games/[0-9a-f]{16}\.png$`), resp.Key)
	assert.Equal(t, "/uploads/"+resp.Key, resp.URL)
	_, statErr := os.Stat(filepath.Join(baseDir, filepath.FromSlash(resp.Key)))
	assert.NoError(t, statErr, "图标应已落盘")

	// Missing file field -> 400
	req2 := iconHandlerMultipartRequest(t, "", nil, "").WithContext(ctx)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = req2
	h.UploadIcon(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	// Failure injection at the Open seam -> 500
	orig := openIconMultipartFileHeader
	openIconMultipartFileHeader = func(_ *multipart.FileHeader) (multipart.File, error) {
		return nil, os.ErrInvalid
	}
	defer func() { openIconMultipartFileHeader = orig }()
	req3 := iconHandlerMultipartRequest(t, "logo.png", pngMagic, "file").WithContext(ctx)
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = req3
	h.UploadIcon(c3)
	assert.Equal(t, http.StatusInternalServerError, w3.Code)
}
