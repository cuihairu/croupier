package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

var excelSeq uint64

func newExcelFixture(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	name := fmt.Sprintf("excel_%d", atomic.AddUint64(&excelSeq, 1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	svcCtx := &svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}
	return NewHandler(NewService(svcCtx), svcCtx), db
}

func buildXLSX(t *testing.T, sheets map[string][][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	first := true
	for name, rows := range sheets {
		if first {
			f.SetSheetName("Sheet1", name)
			first = false
		} else {
			_, err := f.NewSheet(name)
			require.NoError(t, err)
		}
		for ri, row := range rows {
			for ci, cell := range row {
				require.NoError(t, f.SetCellStr(name, fmt.Sprintf("%c%d", rune('A'+ci), ri+1), cell))
			}
		}
	}
	data, err := f.WriteToBuffer()
	require.NoError(t, err)
	return data.Bytes()
}

func excelReq(method, target string, body string, isJSON bool) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if isJSON {
		c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
	} else {
		c.Request = httptest.NewRequest(method, target, nil)
	}
	return c, w
}

func TestCompileSnapshot_RoundTrip(t *testing.T) {
	h, db := newExcelFixture(t)
	// Univer-style snapshot: header + #type row + 2 data rows.
	snap := `{"sheets":{"ItemCfg":{"cellData":{
		"0":{"0":{"v":"id"},"1":{"v":"name"},"2":{"v":"price"}},
		"1":{"0":{"v":"#type"},"1":{"v":"string"},"2":{"v":"int"}},
		"2":{"0":{"v":"1001"},"1":{"v":"宝石包"},"2":{"v":"60"}},
		"3":{"0":{"v":"1002"},"1":{"v":"金币包"},"2":{"v":"30"}}
	}}}}`
	c, w := excelReq(http.MethodPost, "/configs/excel/compile",
		fmt.Sprintf(`{"snapshot":%s,"key":"shop.items","message":"上线商城"}`, snap), true)
	h.CompileExcelSnapshot(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var firstResp ExcelCompileResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &firstResp))
	assert.Equal(t, 1, firstResp.Sheets)
	assert.Equal(t, 2, firstResp.Rows)
	assert.GreaterOrEqual(t, firstResp.Version, 1)

	// Stored as gameplay namespace with typed values.
	var rec model.ConfigVersion
	require.NoError(t, db.Where("key = ? AND version = ?", "shop.items", firstResp.Version).First(&rec).Error)
	assert.Equal(t, model.ConfigNamespaceGameplay, rec.Namespace)
	var wb ExcelWorkbook
	require.NoError(t, json.Unmarshal([]byte(rec.Value), &wb))
	sheet := wb.Sheets["ItemCfg"]
	require.NotNil(t, sheet)
	require.Len(t, sheet.Rows, 2)
	// JSON round-trip makes numbers float64 on read; compare numerically.
	price, ok := sheet.Rows[0]["price"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(60), price)
	assert.Equal(t, "宝石包", sheet.Rows[0]["name"])
	assert.Equal(t, "int", sheet.Types["price"])

	// Second compile bumps the version.
	c, w = excelReq(http.MethodPost, "/configs/excel/compile",
		fmt.Sprintf(`{"snapshot":%s,"key":"shop.items"}`, snap), true)
	h.CompileExcelSnapshot(c)
	require.Equal(t, http.StatusOK, w.Code)
	var secondResp ExcelCompileResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &secondResp))
	assert.Equal(t, firstResp.Version+1, secondResp.Version)
}

func TestCompileSnapshot_TypeError(t *testing.T) {
	h, _ := newExcelFixture(t)
	snap := `{"sheets":{"Bad":{"cellData":{
		"0":{"0":{"v":"id"},"1":{"v":"price"}},
		"1":{"0":{"v":"#type"},"1":{"v":"int"}},
		"2":{"0":{"v":"1"},"1":{"v":"not-a-number"}}
	}}}}`
	c, w := excelReq(http.MethodPost, "/configs/excel/compile",
		fmt.Sprintf(`{"snapshot":%s,"key":"bad"}`, snap), true)
	h.CompileExcelSnapshot(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "price")
}

func TestImportXLSX_RoundTrip(t *testing.T) {
	_, db := newExcelFixture(t)
	data := buildXLSX(t, map[string][][]string{
		"HeroCfg": {
			{"id", "atk", "rare"},
			{"#type", "", "bool"},
			{"h1", "99", "true"},
			{"h2", "120", "false"},
		},
	})
	svcExcel := NewExcelService(&svc.ServiceContext{
		ConfigVersionModel: model.NewConfigVersionModel(db),
	})
	resp, err := svcExcel.ImportXLSX(context.Background(), &ImportXLSXRequest{
		Data: data, GameID: "demo", Env: "prod", Message: "导入英雄表",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Version, 1)
	assert.Equal(t, 2, resp.Rows)

	// Same compiler output shape as the snapshot path.
	var rec model.ConfigVersion
	require.NoError(t, db.Where("key = ? AND version = ?", "excel", resp.Version).First(&rec).Error)
	assert.Equal(t, model.ConfigNamespaceGameplay, rec.Namespace)
	var wb ExcelWorkbook
	require.NoError(t, json.Unmarshal([]byte(rec.Value), &wb))
	require.NotNil(t, wb.Sheets["HeroCfg"])
	assert.Equal(t, true, wb.Sheets["HeroCfg"].Rows[0]["rare"])
	atk, _ := wb.Sheets["HeroCfg"].Rows[0]["atk"].(float64)
	assert.Equal(t, float64(99), atk)
}

func TestCompile_DuplicateFieldRejected(t *testing.T) {
	h, _ := newExcelFixture(t)
	snap := `{"sheets":{"Dup":{"cellData":{
		"0":{"0":{"v":"id"},"1":{"v":"id"}}
	}}}}`
	c, w := excelReq(http.MethodPost, "/configs/excel/compile",
		fmt.Sprintf(`{"snapshot":%s,"key":"dup"}`, snap), true)
	h.CompileExcelSnapshot(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "重复")
}
