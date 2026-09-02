// 覆盖目标：RegisterPlayerRoutes（0%）、queryInt/clampLevel/decodeFAQTags
// 纯函数矩阵、ListFAQs 无模型分支。
package support

import (
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterPlayerRoutes_Wired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	RegisterPlayerRoutes(v1, &svc.ServiceContext{})

	routes := map[string]bool{}
	for _, rt := range r.Routes() {
		routes[rt.Method+" "+rt.Path] = true
	}
	assert.True(t, routes["GET /api/v1/public/support/faqs"])
	assert.True(t, routes["POST /api/v1/public/support/tickets"])
	assert.True(t, routes["GET /api/v1/public/support/tickets"])
}

func TestListFAQs_NilModels_EmptyItems(t *testing.T) {
	h := NewPlayerHandler(&svc.ServiceContext{})

	c, w := playerReq(http.MethodGet, "/public/support/faqs", "")
	h.ListFAQs(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "items")
}

func TestQueryInt_Matrix(t *testing.T) {
	c, _ := playerReq(http.MethodGet, "/x?page=12", "")
	assert.Equal(t, 12, queryInt(c, "page", 1))

	c2, _ := playerReq(http.MethodGet, "/x?page=abc", "")
	assert.Equal(t, 7, queryInt(c2, "page", 7), "非数字回落默认值")

	c3, _ := playerReq(http.MethodGet, "/x", "")
	assert.Equal(t, 5, queryInt(c3, "page", 5), "缺参回落默认值")

	c4, _ := playerReq(http.MethodGet, "/x?page=%2020%20", "")
	assert.Equal(t, 20, queryInt(c4, "page", 1), "空白容错")
}

func TestClampLevel_Bounds(t *testing.T) {
	assert.Equal(t, 0, clampLevel(-1))
	assert.Equal(t, 5, clampLevel(5))
	assert.Equal(t, 10000, clampLevel(99999))
}

func TestDecodeFAQTags_Variants(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, decodeFAQTags([]string{"a", "b"}))
	assert.Equal(t, []string{"x"}, decodeFAQTags([]interface{}{"x", 42, nil}))
	assert.Nil(t, decodeFAQTags("not-a-slice"))
	assert.Nil(t, decodeFAQTags(nil))
}

func TestCreateTicket_Validation(t *testing.T) {
	h := newPlayerHandler(t)

	// 缺 playerID
	c, w := playerReq(http.MethodPost, "/public/support/tickets", `{"title":"t","content":"c"}`)
	h.CreateTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 空 body 绑定失败
	c2, w2 := playerReq(http.MethodPost, "/public/support/tickets", `not-json`)
	h.CreateTicket(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestCreateTicket_Success_ThenListMine(t *testing.T) {
	h := newPlayerHandler(t)

	c, w := playerReq(http.MethodPost, "/public/support/tickets",
		`{"title":"无法登录","content":"闪退","playerId":"p-1","category":"","playerLevel":20000}`)
	h.CreateTicket(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "open")

	// playerId 缺失 → 400
	c2, w2 := playerReq(http.MethodGet, "/public/support/tickets", "")
	h.ListMyTickets(c2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	// 查询自己的工单
	c3, w3 := playerReq(http.MethodGet, "/public/support/tickets?playerId=p-1&page=1&pageSize=10", "")
	h.ListMyTickets(c3)
	require.Equal(t, http.StatusOK, w3.Code, w3.Body.String())
	assert.Contains(t, w3.Body.String(), "无法登录")
}

func TestCreateTicket_NilModel_Disabled(t *testing.T) {
	h := NewPlayerHandler(&svc.ServiceContext{})
	c, w := playerReq(http.MethodPost, "/public/support/tickets", `{"title":"t","content":"c","playerId":"p"}`)
	h.CreateTicket(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c2, w2 := playerReq(http.MethodGet, "/public/support/tickets", "")
	h.ListMyTickets(c2)
	assert.Equal(t, http.StatusOK, w2.Code, "无模型返回空列表")
}
