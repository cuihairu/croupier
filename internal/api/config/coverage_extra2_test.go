package config

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// splitCSV 纯函数：空串 / 逗号分隔 / 空白项过滤。
func TestSplitCSV_Branches(t *testing.T) {
	assert.Nil(t, splitCSV(""))
	assert.Nil(t, splitCSV("   "))
	assert.Equal(t, []string{"a", "b"}, splitCSV("a,b"))
	assert.Equal(t, []string{"a", "b"}, splitCSV(" a , b ,,"), "空白项应被过滤")
	assert.Equal(t, []string{"only"}, splitCSV("only,"))
}

func newImportExcelRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ConfigVersion{}))
	svcCtx := &svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}
	h := NewHandler(NewService(svcCtx), svcCtx)
	r := gin.New()
	r.POST("/configs/excel/import", h.ImportExcel)
	return r, db
}

func postMultipart(r *gin.Engine, path, filename string, content []byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return httptest.NewRecorder()
	}
	_, _ = fw.Write(content)
	_ = writer.Close()

	req, _ := http.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ImportExcel：缺 file 字段 → 400；超 4MB → 400。
func TestHandler_ImportExcel_Guards(t *testing.T) {
	r, _ := newImportExcelRouter(t)

	req, _ := http.NewRequest(http.MethodPost, "/configs/excel/import", bytes.NewReader([]byte("not multipart")))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=none")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "缺少 file")

	big := make([]byte, 4*1024*1024+1)
	w = postMultipart(r, "/configs/excel/import", "big.xlsx", big)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "4MB")
}

// List 的 service 错误分支：删表后 → 500。
func TestHandler_List_ServiceError(t *testing.T) {
	_, db := newImportExcelRouter(t)
	svcCtx := &svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}
	h := NewHandler(NewService(svcCtx), svcCtx)
	r2 := gin.New()
	r2.GET("/configs", h.List)
	require.NoError(t, db.Migrator().DropTable("config_versions"))

	w := httptest.NewRecorder()
	r2.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/configs", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	_ = context.Background()
}

// ImportExcel 成功路径：multipart 上传合法 xlsx（复用 buildXLSX 构造器）。
func TestHandler_ImportExcel_Success(t *testing.T) {
	r, _ := newImportExcelRouter(t)
	data := buildXLSX(t, map[string][][]string{
		"HeroCfg": {
			{"id", "atk", "rare"},
			{"#type", "", "bool"},
			{"h1", "99", "true"},
		},
	})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw, err := writer.CreateFormFile("file", "hero.xlsx")
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("gameId", "demo"))
	require.NoError(t, writer.WriteField("env", "prod"))
	require.NoError(t, writer.WriteField("message", "导入英雄表"))
	require.NoError(t, writer.Close())

	req, _ := http.NewRequest(http.MethodPost, "/configs/excel/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"rows":1`)
}
