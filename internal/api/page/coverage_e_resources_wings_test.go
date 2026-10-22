package page

// 覆盖率巡检第十三轮（wt-api）：api/page/service.go 残余 5 块收口——
// Resources 两道守卫错误翼（无读权限、缺 game scope）与 ListByScope
// 存储错误翼；functionResourceIndex 的 nil 链早退翼（Service/svcCtx/DB）
// 与 function_contracts 读取失败翼。复用 newPageTestService 夹具。

import (
	"context"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResources_GuardWings(t *testing.T) {
	// 无任何页面权限：requirePageRead 直接拒绝（226 翼）
	service, ctx, _ := newPageTestService(t)
	_, err := service.Resources(ctx)
	require.Error(t, err, "无 pages:read 族权限应被拒")

	// 有读权限、但请求上下文缺 game scope（230 翼）：
	// fixture 的 admin 需仍可加载（username 在库），仅剥离 scope 值
	service2, _, _ := newPageTestService(t, "pages:read")
	noScope := context.WithValue(context.Background(), "username", "page_tester")
	_, err = service2.Resources(noScope)
	require.ErrorContains(t, err, "X-Game-ID")

	// scope 与权限齐备、page_specs 缺表：ListByScope 存储错误透传（235 翼）
	service3, ctx3, _ := newPageTestService(t, "pages:read")
	require.NoError(t, service3.svcCtx.DB.Migrator().DropTable("page_specs"))
	_, err = service3.Resources(ctx3)
	require.Error(t, err, "缺表时应直传存储错误")
}

func TestFunctionResourceIndex_Wings(t *testing.T) {
	// nil Service / 无 DB 连线：空映射早退（91 翼）
	var nilSvc *Service
	assert.Empty(t, nilSvc.functionResourceIndex(context.Background(), "demo", "dev"))
	bare := NewService(&svc.ServiceContext{})
	assert.Empty(t, bare.functionResourceIndex(context.Background(), "demo", "dev"))

	// function_contracts 缺表：ListByScope 出错按空索引降级（95 翼）
	service, ctx, _ := newPageTestService(t, "pages:read")
	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("function_contracts"))
	assert.Empty(t, service.functionResourceIndex(ctx, "demo-game", "development"))
}

// TestResourcesHandler_ErrorWing handler 层错误翼：service 返回错误时
// response.Error 写 4xx 而非 panic。
func TestResourcesHandler_ErrorWing(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:read")
	noScope := context.WithValue(context.Background(), "username", "page_tester")

	ginCtx, rec := newTestContext(http.MethodGet, "/api/v1/pages/resources", "")
	ginCtx.Request = ginCtx.Request.WithContext(noScope)
	NewHandler(service).Resources(ginCtx)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "缺 scope 应 400")
}
