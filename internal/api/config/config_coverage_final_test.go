// 覆盖目标（coverage final）：
//  1. CompileXLSX：工作簿无 sheet（DeleteSheet 移除唯一 sheet）。
//  2. CompileXLSX：GetRows 解析失败（重打包 zip，损坏 worksheet XML）。
//  3. ExcelService.register：json.Marshal 失败（untyped "NaN" 单元格）。
//  4. handler List/Validate/ListVersions/GetVersion 的 bind 失败分支（Validator 注入）。
//  5. handler ImportExcel 的 file.Read 失败分支（反射注入 /dev/full tmpfile）。
package config

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/cuihairu/croupier/internal/common/requestbind"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"mime/multipart"
)

type cfgFailValidator struct{}

func (cfgFailValidator) ValidateStruct(any) error { return errors.New("injected validate failure") }
func (cfgFailValidator) Engine() any              { return nil }

func cfgWithFailingValidator(t *testing.T) {
	t.Helper()
	orig := binding.Validator
	binding.Validator = cfgFailValidator{}
	t.Cleanup(func() { binding.Validator = orig })
}

// Bind 兼容层在 Validator 为 nil 时跳过校验，这里确认注入失败可传导。
var _ = requestbind.BindQueryCompat

func newCfgFinalFixture(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ConfigVersion{}))
	svcCtx := &svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}
	return NewHandler(NewService(svcCtx), svcCtx), db
}

func TestCompileXLSX_WorkbookWithoutSheets(t *testing.T) {
	data := buildEmptyWorkbookXLSX(t)
	_, err := CompileXLSX(data)
	require.ErrorContains(t, err, "没有任何 sheet")
}

func TestCompileXLSX_GetRowsFailure(t *testing.T) {
	data := renameSheetTooLongXLSX(t, buildXLSX(t, map[string][][]string{
		"Solo": {{"id"}, {"1"}},
	}))
	_, err := CompileXLSX(data)
	require.ErrorContains(t, err, "读取 sheet")
}

// renameSheetTooLongXLSX 重打包 xlsx，把 workbook.xml 中的 sheet 名改为
// 超长（>31 字符）名字：GetSheetList 不校验名字仍会返回，而 GetRows 内
// checkSheetName 拒绝超长名 → 覆盖 GetRows 错误分支。
func renameSheetTooLongXLSX(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	longName := strings.Repeat("a", 32)
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, zf := range zr.File {
		w, err := zw.Create(zf.Name)
		require.NoError(t, err)
		if zf.Name == "xl/workbook.xml" {
			rc, err := zf.Open()
			require.NoError(t, err)
			raw, err := io.ReadAll(rc)
			require.NoError(t, err)
			_ = rc.Close()
			replaced := strings.Replace(string(raw), `name="Solo"`, `name="`+longName+`"`, 1)
			_, err = w.Write([]byte(replaced))
			require.NoError(t, err)
			continue
		}
		rc, err := zf.Open()
		require.NoError(t, err)
		_, err = io.Copy(w, rc)
		require.NoError(t, err)
		_ = rc.Close()
	}
	require.NoError(t, zw.Close())
	return out.Bytes()
}

func TestExcelService_Register_MarshalFailure(t *testing.T) {
	h, db := newCfgFinalFixture(t)
	svcExcel := NewExcelService(&svc.ServiceContext{
		ConfigVersionModel: model.NewConfigVersionModel(db),
	})
	_ = h
	// untyped 单元格 "NaN"：ParseFloat 成功 → float64 NaN → json.Marshal 失败。
	snap := `{"sheets":{"N":{"cellData":{
		"0":{"0":{"v":"id"},"1":{"v":"x"}},
		"1":{"0":{"v":"1"},"1":{"v":"NaN"}}
	}}}}`
	_, err := svcExcel.CompileSnapshot(context.Background(), &CompileSnapshotRequest{
		Snapshot: []byte(snap), Key: "nan.sheet",
	})
	require.Error(t, err)
}

func TestConfigHandler_BindValidatorFailures(t *testing.T) {
	cfgWithFailingValidator(t)
	h, _ := newCfgFinalFixture(t)
	r := gin.New()
	r.GET("/configs", h.List)
	r.POST("/configs/x/validate", h.Validate)
	r.GET("/configs/versions", h.ListVersions)
	r.GET("/configs/version", h.GetVersion)

	do := func(method, path string) int {
		var body io.Reader
		if method == http.MethodPost {
			body = strings.NewReader(`{"format":"json","content":"{}"}`)
		}
		req := httptest.NewRequest(method, path, body)
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	assert.Equal(t, http.StatusBadRequest, do(http.MethodGet, "/configs?gameId=demo"), "List bind 失败")
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPost, "/configs/x/validate"), "Validate bind 失败")
	assert.Equal(t, http.StatusBadRequest, do(http.MethodGet, "/configs/versions?key=k"), "ListVersions bind 失败")
	assert.Equal(t, http.StatusBadRequest, do(http.MethodGet, "/configs/version?key=k&version=1"), "GetVersion bind 失败")
}

func TestConfigHandler_ImportExcel_ReadFailure(t *testing.T) {
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full not available on this platform")
	}
	h, _ := newCfgFinalFixture(t)
	r := gin.New()
	r.POST("/configs/excel/import", h.ImportExcel)

	req := httptest.NewRequest(http.MethodPost, "/configs/excel/import", nil)
	// 预填已解析的 MultipartForm：FileHeader 的 tmpfile 指向目录
	// （os.Open 成功，read(2) 返回 EISDIR），覆盖单次 file.Read(data) 错误分支。
	dir := t.TempDir()
	fh := &multipart.FileHeader{Filename: "boom.xlsx", Size: 64}
	setFileHeaderTmpfile(fh, dir)
	req.MultipartForm = &multipart.Form{
		Value: map[string][]string{},
		File:  map[string][]*multipart.FileHeader{"file": {fh}},
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "读取文件失败")
}

// setFileHeaderTmpfile 通过反射写入 multipart.FileHeader 的私有 tmpfile 字段。
func setFileHeaderTmpfile(fh *multipart.FileHeader, path string) {
	v := reflect.ValueOf(fh).Elem().FieldByName("tmpfile")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().SetString(path)
}

// buildEmptyWorkbookXLSX 生成保留合法 workbook.xml 但不含任何 sheet 的 xlsx。
func buildEmptyWorkbookXLSX(t *testing.T) []byte {
	t.Helper()
	f := buildXLSX(t, map[string][][]string{"Solo": {{"id"}, {"1"}}})
	zr, err := zip.NewReader(bytes.NewReader(f), int64(len(f)))
	require.NoError(t, err)

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, zf := range zr.File {
		// 跳过 worksheet 本体；workbook.xml 替换为空 sheets 声明。
		if strings.Contains(zf.Name, "worksheets") {
			continue
		}
		w, err := zw.Create(zf.Name)
		require.NoError(t, err)
		if zf.Name == "xl/workbook.xml" {
			_, err = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets/></workbook>`))
			require.NoError(t, err)
			continue
		}
		rc, err := zf.Open()
		require.NoError(t, err)
		_, err = io.Copy(w, rc)
		require.NoError(t, err)
		_ = rc.Close()
	}
	require.NoError(t, zw.Close())
	return out.Bytes()
}
