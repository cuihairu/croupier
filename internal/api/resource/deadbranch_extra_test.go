package resource

import (
	"net/http"
	"net/http/httptest"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 以下测试文档化本包中不可达的防御分支（对齐 internal/analytics/mq/deadbranch_doc_test.go 惯例）：
//
//  1. handler.go List（:23）错误分支：ResourceListRequest 仅含 Category/Query
//     两个 string form 字段，ShouldBindQuery 对纯 string 绑定恒成功，错误
//     分支不可触发。
//  2. service.go List 排序（:49）：resourceSpecFromCapability 构造
//     ResourceCategorySpec 时从不设置 Order（恒为 0），Order 不等分支不可
//     触发，排序恒走 Key tie-break。
//  3. service.go loadPersistentResources（:96）：ListByScope 使用 GORM
//     Find(&[]*ResourceCapability)，结果元素恒非 nil，cap==nil 分支不可触发。
//  4. service.go humanizeKey（:289）：strings.FieldsFunc 不产生空字符串段，
//     parts[i]=="" 分支不可触发。

func TestHandlerListQueryBindNeverFailsV11(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")

	// 纯 string form 字段对任意查询串（含奇怪编码/重复参数）都不会绑定失败。
	for _, target := range []string{
		"/api/v1/resources?category=player&q=guild",
		"/api/v1/resources?category=%E4%B8&category=x&q[]=",
		"/api/v1/resources?=v&category",
	} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		req := httptest.NewRequest(http.MethodGet, target, nil)
		c.Request = req.WithContext(ctx)

		var reqDTO ResourceListRequest
		require.NoError(t, c.ShouldBindQuery(&reqDTO), "string-only form binding must never fail: %s", target)
	}
}

func TestHumanizeKeyNeverSeesEmptyPartsV11(t *testing.T) {
	assert.Equal(t, "", humanizeKey("...___---"))
	assert.Equal(t, "Player Guild", humanizeKey("player.guild"))
	assert.Equal(t, "A B", humanizeKey("a--b"))
}
