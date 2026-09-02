// 覆盖目标：tool service 的 Update 全字段/无字段/不存在、Delete、
// handler 的 bind 失败与服务错误路径。
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newToolSvc(t *testing.T) *Service {
	db := newToolTestDB(t)
	return NewService(&svc.ServiceContext{ToolModel: model.NewToolLinkModel(db)})
}

func mustCreateTool(t *testing.T, s *Service) string {
	t.Helper()
	resp, err := s.Create(context.Background(), &ToolCreateRequest{
		Name: "wiki", URL: "https://wiki", Category: "docs",
	})
	require.NoError(t, err)
	return fmt.Sprintf("%d", resp.Tool.Id)
}

func TestToolService_Update_AllFields(t *testing.T) {
	s := newToolSvc(t)
	id := mustCreateTool(t, s)

	url := "https://wiki2"
	desc := "d"
	cat := "dev"
	icon := "i"
	sort := 9
	enabled := false
	gameID := "demo"
	env := "prod"
	resp, err := s.Update(context.Background(), &ToolUpdateRequest{
		ID: id, URL: &url, Description: &desc, Category: &cat,
		Icon: &icon, Sort: &sort, Enabled: &enabled, GameID: &gameID, Env: &env,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://wiki2", resp.Tool.Url)
	assert.False(t, resp.Tool.Enabled)
}

func TestToolService_Update_NoFields_BadRequest(t *testing.T) {
	s := newToolSvc(t)
	id := mustCreateTool(t, s)
	_, err := s.Update(context.Background(), &ToolUpdateRequest{ID: id})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "需要更新")
}

func TestToolService_Update_UnknownTool(t *testing.T) {
	s := newToolSvc(t)
	name := "renamed"
	_, err := s.Update(context.Background(), &ToolUpdateRequest{ID: "99999", Name: name})
	require.Error(t, err)
}

func TestToolService_Update_InvalidID(t *testing.T) {
	s := newToolSvc(t)
	_, err := s.Update(context.Background(), &ToolUpdateRequest{ID: "abc"})
	require.Error(t, err)
}

func TestToolService_Delete_RoundTrip(t *testing.T) {
	s := newToolSvc(t)
	id := mustCreateTool(t, s)

	require.NoError(t, s.Delete(context.Background(), &ToolDeleteRequest{ID: id}))
	resp, err := s.List(context.Background(), &ToolListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	require.Error(t, s.Delete(context.Background(), &ToolDeleteRequest{ID: "not-a-number"}))
}

func TestToolHandler_CRUDThroughHandler(t *testing.T) {
	db := newToolTestDB(t)
	h := newToolHandler(db)

	// Create
	c, w := toolRequest(http.MethodPost, "/tools", `{"name":"h1","url":"https://h","category":"docs"}`)
	h.Create(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// List（含关键字/分页 query）
	c, w = toolRequest(http.MethodGet, "/tools?keyword=h1&page=1&pageSize=10", "")
	h.List(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "h1")

	var listResp ToolListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Len(t, listResp.Items, 1)
	id := fmt.Sprintf("%d", listResp.Items[0].Id)

	// Update
	c, w = toolRequest(http.MethodPut, "/tools/"+id, `{"name":"h2"}`)
	c.Params = gin.Params{{Key: "id", Value: id}}
	h.Update(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "h2")

	// Delete
	c, w = toolRequest(http.MethodDelete, "/tools/"+id, "")
	c.Params = gin.Params{{Key: "id", Value: id}}
	h.Delete(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestToolHandler_Update_MalformedJSON(t *testing.T) {
	db := newToolTestDB(t)
	h := newToolHandler(db)

	c, w := toolRequest(http.MethodPut, "/tools/1", `{bad`)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	h.Update(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestToolHandler_Delete_InvalidID(t *testing.T) {
	db := newToolTestDB(t)
	h := newToolHandler(db)

	c, w := toolRequest(http.MethodDelete, "/tools/abc", "")
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	h.Delete(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestToolHandler_List_StoreError(t *testing.T) {
	db := newToolTestDB(t)
	h := newToolHandler(db)
	require.NoError(t, db.Migrator().DropTable("tool_links"))

	c, w := toolRequest(http.MethodGet, "/tools", "")
	h.List(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestToolHandler_Create_InvalidCategory(t *testing.T) {
	db := newToolTestDB(t)
	h := newToolHandler(db)

	c, w := toolRequest(http.MethodPost, "/tools", `{"name":"x","url":"https://x","category":"bogus"}`)
	h.Create(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid tool category")
}

func TestToolService_Create_InvalidURL(t *testing.T) {
	s := newToolSvc(t)
	_, err := s.Create(context.Background(), &ToolCreateRequest{Name: "x", URL: "ftp://bad", Category: "ci"})
	require.Error(t, err)

	_, err = s.Create(context.Background(), &ToolCreateRequest{Name: "  ", URL: "https://ok", Category: "ci"})
	require.Error(t, err)
}

func TestToolService_List_ScopeFilter(t *testing.T) {
	s := newToolSvc(t)
	// 全局 + scoped 两条
	_, err := s.Create(context.Background(), &ToolCreateRequest{Name: "global1", URL: "https://g", Category: "ci"})
	require.NoError(t, err)
	_, err = s.Create(context.Background(), &ToolCreateRequest{Name: "scoped1", URL: "https://s", Category: "ci", GameID: "demo", Env: "prod"})
	require.NoError(t, err)

	globalOnly, err := s.List(context.Background(), &ToolListRequest{})
	require.NoError(t, err)
	assert.Len(t, globalOnly.Items, 1, "无 scope 只见全局")

	scoped, err := s.List(context.Background(), &ToolListRequest{GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.Len(t, scoped.Items, 2, "scope 内可见全局+scoped")
}
