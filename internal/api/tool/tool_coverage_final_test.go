// 覆盖目标（coverage final）：
//  1. handler.List 的 query 绑定失败分支（binding.Validator 注入失败）。
//  2. service.Update 中 ListAll 回读失败分支（gorm query callback 注错，
//     Updates 走 update callback 不受影响）。
package tool

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// failValidator 让任意 Bind 的 validate 步骤返回错误，
// 用于覆盖「DTO 全为可选 string、正常请求无法触发」的绑定失败分支。
type failValidator struct{}

func (failValidator) ValidateStruct(any) error { return errors.New("injected validate failure") }
func (failValidator) Engine() any              { return nil }

func withFailingValidator(t *testing.T) {
	t.Helper()
	orig := binding.Validator
	binding.Validator = failValidator{}
	t.Cleanup(func() { binding.Validator = orig })
}

func TestToolHandler_List_BindValidatorFailure(t *testing.T) {
	withFailingValidator(t)
	db := newToolTestDB(t)
	h := newToolHandler(db)

	c, w := toolRequest(http.MethodGet, "/tools?gameId=demo", "")
	h.List(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestToolService_Update_ListAllFailure(t *testing.T) {
	db := newToolTestDB(t)
	s := NewService(&svc.ServiceContext{ToolModel: model.NewToolLinkModel(db)})

	created, err := s.Create(context.Background(), &ToolCreateRequest{Name: "n1", URL: "https://u", Category: "docs"})
	require.NoError(t, err)

	// Updates 走 update callback，ListAll（Find/SELECT）走 query callback：
	// 仅令 query 失败 → Update 成功、ListAll 回读失败。
	require.NoError(t, db.Callback().Query().Before("gorm.query").
		Register("test:tool_fail_listall", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("listall boom"))
		}))

	_, err = s.Update(context.Background(), &ToolUpdateRequest{ID: fmt.Sprintf("%d", created.Tool.Id), Name: "renamed"})
	require.Error(t, err)
}
