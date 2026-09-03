package config

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func newConfigSvcDBV9(t *testing.T) (*gorm.DB, *svc.ServiceContext) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ConfigVersion{}))
	return db, &svc.ServiceContext{ConfigVersionModel: model.NewConfigVersionModel(db)}
}

// CompileXLSX：sheet 数量超过上限（默认 50）。
func TestCompileXLSX_TooManySheetsV9(t *testing.T) {
	f := excelize.NewFile()
	for i := 1; i < 51; i++ {
		_, err := f.NewSheet(fmt.Sprintf("s%d", i))
		require.NoError(t, err)
	}
	data, err := f.WriteToBuffer()
	require.NoError(t, err)

	_, err = CompileXLSX(data.Bytes())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "上限")
}

// CompileXLSX：编译错误透传（重复字段）。
func TestCompileXLSX_CompileErrorV9(t *testing.T) {
	data := buildXLSX(t, map[string][][]string{
		"Bad": {{"id", "id"}},
	})
	_, err := CompileXLSX(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "重复")
}

// CompileXLSX：所有 sheet 均为空。
func TestCompileXLSX_AllSheetsEmptyV9(t *testing.T) {
	data := buildXLSX(t, map[string][][]string{
		"Empty": {{"   "}},
	})
	_, err := CompileXLSX(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "均为空")
}

// CompileSnapshot：非法 JSON / 无 sheet / 超 sheet 上限 / 全空 sheet。
func TestCompileSnapshot_GuardsV9(t *testing.T) {
	_, err := CompileSnapshot([]byte(`{bad json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "快照格式无效")

	_, err = CompileSnapshot([]byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "没有任何 sheet")

	var sb strings.Builder
	sb.WriteString(`{"sheets":{`)
	for i := 0; i < 51; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `"s%d":{"cellData":{"0":{"0":{"v":"a"}}}}`, i)
	}
	sb.WriteString(`}}`)
	_, err = CompileSnapshot([]byte(sb.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "上限")

	_, err = CompileSnapshot([]byte(`{"sheets":{"E":{"cellData":{"0":{"0":{"v":"  "}}}}}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "均为空")
}

// snapshotToRows：无效行/列 key 被跳过；全无效 key 返回 nil（该 sheet 被跳过）。
func TestCompileSnapshot_InvalidRowColKeysV9(t *testing.T) {
	snap := `{"sheets":{
		"A":{"cellData":{
			"x":{"0":{"v":"skip"}},
			"0":{"y":{"v":"skip"},"0":{"v":"id"},"1":{"v":"name"}},
			"1":{"0":{"v":"r1"}}
		}},
		"B":{"cellData":{"bad":{"also":{"v":"1"}}}}
	}}`
	wb, err := CompileSnapshot([]byte(snap))
	require.NoError(t, err)
	require.NotNil(t, wb.Sheets["A"])
	assert.Equal(t, []string{"id", "name"}, wb.Sheets["A"].Fields)
	assert.Len(t, wb.Sheets["A"].Rows, 1)
	_, hasB := wb.Sheets["B"]
	assert.False(t, hasB, "全无效 key 的 sheet 应被跳过")
}

// compileRows 直测：空数据/无字段名/类型行过短/非法类型/超行数/空单元格/仅表头。
func TestCompileRows_BranchesV9(t *testing.T) {
	opts := CompileOptions{MaxRows: 100, MaxSheets: 10}

	sheet, err := compileRows("S", [][]string{{" ", " "}, {"1", "2"}}, opts)
	require.NoError(t, err)
	assert.Nil(t, sheet, "全空行应被过滤后返回 nil")

	sheet, err = compileRows("S", [][]string{{"a", "b", "c"}, {"#type"}, {"1", "2", "3"}}, opts)
	require.NoError(t, err)
	assert.NotNil(t, sheet)
	assert.Equal(t, map[string]string{}, sheet.Types, "类型行过短时应提前 break")

	_, err = compileRows("S", [][]string{{"a", "b"}, {"#type", "decimal"}, {"1", "2"}}, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效")

	_, err = compileRows("S", [][]string{{"a"}, {"#type", "int"}, {"1"}, {"2"}}, CompileOptions{MaxRows: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "行数超过上限")

	sheet, err = compileRows("S", [][]string{{"a", "b"}, {"1", ""}}, opts)
	require.NoError(t, err)
	require.Len(t, sheet.Rows, 1)
	_, ok := sheet.Rows[0]["b"]
	assert.False(t, ok, "空单元格应跳过该字段")

	sheet, err = compileRows("S", [][]string{{"a", "b"}}, opts)
	require.NoError(t, err)
	assert.Nil(t, sheet, "仅表头无数据行应返回 nil")
}

// splitIdentifiers / coerceCell 补充分支。
func TestExcelPureHelpersV9(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, splitIdentifiers([]string{"a", " ", "b"}))

	v, err := coerceCell("1.5", "")
	require.NoError(t, err)
	assert.Equal(t, float64(1.5), v)

	v, err = coerceCell("42", "")
	require.NoError(t, err)
	assert.Equal(t, int64(42), v)

	_, err = coerceCell("x", "int")
	assert.ErrorContains(t, err, "不是 int")

	_, err = coerceCell("x", "float")
	assert.ErrorContains(t, err, "不是 float")

	_, err = coerceCell("x", "bool")
	assert.ErrorContains(t, err, "不是 bool")
}

// ExcelService.CompileSnapshot：空快照 / 默认 key / register 落库失败。
func TestExcelService_CompileSnapshotV9(t *testing.T) {
	db, svcCtx := newConfigSvcDBV9(t)
	s := NewExcelService(svcCtx)
	ctx := context.Background()

	_, err := s.CompileSnapshot(ctx, &CompileSnapshotRequest{})
	require.ErrorContains(t, err, "缺少快照内容")

	snap := `{"sheets":{"A":{"cellData":{"0":{"0":{"v":"id"},"1":{"v":"n"}},"1":{"0":{"v":"1"},"1":{"v":"x"}}}}}}`
	resp, err := s.CompileSnapshot(ctx, &CompileSnapshotRequest{Snapshot: []byte(snap)})
	require.NoError(t, err)
	assert.Equal(t, "excel.workbook", resp.Key)
	assert.Equal(t, 1, resp.Rows)

	require.NoError(t, db.Migrator().DropTable("config_versions"))
	_, err = s.CompileSnapshot(ctx, &CompileSnapshotRequest{Snapshot: []byte(snap), Key: "k"})
	require.Error(t, err)
}

// ImportExcel：上传非 xlsx 内容 → 编译失败 400。
func TestHandler_ImportExcel_CompileErrorV9(t *testing.T) {
	r, _ := newImportExcelRouter(t)
	w := postMultipart(r, "/configs/excel/import", "bad.xlsx", []byte("this is not an xlsx"))
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// CompileExcelSnapshot：非法 JSON body → 400。
func TestHandler_CompileExcelSnapshot_BadJSONV9(t *testing.T) {
	_, svcCtx := newConfigSvcDBV9(t)
	h := NewHandler(NewService(svcCtx), svcCtx)
	c, w := excelReq(http.MethodPost, "/configs/excel/compile", `{bad`, true)
	h.CompileExcelSnapshot(c)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// Service 数据库错误分支：Upsert / SaveConfig / ListVersions。
func TestService_DBErrorBranchesV9(t *testing.T) {
	db, svcCtx := newConfigSvcDBV9(t)
	s := NewService(svcCtx)
	ctx := context.Background()
	require.NoError(t, db.Migrator().DropTable("config_versions"))

	_, err := s.Upsert(ctx, &UpsertRequest{Key: "k", Value: "1"})
	require.Error(t, err)

	_, err = s.SaveConfig(ctx, "k", &SaveConfigRequest{Content: "1"})
	require.Error(t, err)

	_, err = s.ListVersions(ctx, &ListVersionsRequest{Key: "k"})
	require.Error(t, err)
}

// configScopeValue：ctx 携带 game scope 时优先于 fallback。
func TestConfigScopeValueV9(t *testing.T) {
	base := context.Background()
	gameCtx := svc.WithGameScope(base, svc.GameScope{GameID: "g1"})
	envCtx := svc.WithGameScope(base, svc.GameScope{Env: "prod"})
	fullCtx := svc.WithGameScope(base, svc.GameScope{GameID: "g1", Env: "prod"})

	assert.Equal(t, "g1", configScopeValue(gameCtx, "fb", true))
	assert.Equal(t, "fb", configScopeValue(gameCtx, "fb", false))
	assert.Equal(t, "prod", configScopeValue(envCtx, "dev", false))
	assert.Equal(t, "dev", configScopeValue(envCtx, "dev", true))
	assert.Equal(t, "g1", configScopeValue(fullCtx, "fb", true))
	assert.Equal(t, "prod", configScopeValue(fullCtx, "fb", false))
	assert.Equal(t, "fb", configScopeValue(base, " fb ", true))
}

// 带 game scope 的查询：GetConfig/ListVersions/GetVersion 走 ByScope 模型方法。
func TestService_ScopedQueriesV9(t *testing.T) {
	db, svcCtx := newConfigSvcDBV9(t)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Key: "scoped.key", Version: 2, Value: "v2", GameID: "g1", Env: "prod", Format: "json",
	}).Error)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Key: "scoped.key", Version: 1, Value: "v1", GameID: "g1", Env: "prod", Format: "json",
	}).Error)

	s := NewService(svcCtx)
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g1", Env: "prod"})

	resp, err := s.GetConfig(ctx, &GetConfigRequest{ID: "scoped.key"})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Version)
	assert.Equal(t, "v2", resp.Content)

	lv, err := s.ListVersions(ctx, &ListVersionsRequest{Key: "scoped.key"})
	require.NoError(t, err)
	assert.Equal(t, 2, lv.Total)
	assert.Equal(t, 2, lv.Versions[0].Version)

	gv, err := s.GetVersion(ctx, &GetVersionRequest{Key: "scoped.key", Version: 1})
	require.NoError(t, err)
	assert.Equal(t, "v1", gv.Version.Value)
}

// Watch handler：无效 namespace → 400。
func TestWatchHandler_InvalidNamespaceV9(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewWatchHandler(NewWatchService(&svc.ServiceContext{}))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/configs/watch?namespaces=secret", nil)
	h.Watch(c)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// currentVersions / latest：nil 模型时安全返回空结果。
func TestWatchAndPublic_NilSvcCtxV9(t *testing.T) {
	ws := NewWatchService(&svc.ServiceContext{})
	assert.Empty(t, ws.currentVersions(context.Background(), []string{"runtime"}))

	ps := NewPublicService(&svc.ServiceContext{})
	items, err := ps.latest(context.Background(), []string{"runtime"}, nil)
	require.NoError(t, err)
	assert.Empty(t, items)
}

// PublicService.latest：keys 过滤 + 查询错误分支；handler 错误 → 500。
func TestPublicService_Latest_KeysAndDBErrorV9(t *testing.T) {
	db, svcCtx := newConfigSvcDBV9(t)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Namespace: "runtime", Key: "a.key", Version: 1, Value: "1", Format: "json",
	}).Error)
	require.NoError(t, db.Create(&model.ConfigVersion{
		Namespace: "runtime", Key: "b.key", Version: 1, Value: "2", Format: "json",
	}).Error)

	ps := NewPublicService(svcCtx)
	items, err := ps.latest(context.Background(), []string{"runtime"}, []string{"a.key"})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "a.key", items[0]["key"])

	h := NewPublicHandler(ps)
	gin.SetMode(gin.TestMode)
	require.NoError(t, db.Migrator().DropTable("config_versions"))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/public/configs?ns=runtime", nil)
	h.List(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

	_, err = ps.latest(context.Background(), []string{"runtime"}, nil)
	require.Error(t, err)
}
