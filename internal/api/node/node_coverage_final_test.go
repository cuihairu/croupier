// 覆盖目标（coverage final）：
//  1. handler.List / handler.ListCommands 的 query 绑定失败分支
//     （DTO 全为可选 string，注入失败 Validator 触发）。
//  2. service.UpdateMeta 中 NodeModel.UpdateMeta 失败分支
//     （gorm update callback 注错，前置 FindByNodeID 为 SELECT 不受影响）。
package node

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// nodeFailValidator 让任意 Bind 的 validate 步骤返回错误。
type nodeFailValidator struct{}

func (nodeFailValidator) ValidateStruct(any) error { return errors.New("injected validate failure") }
func (nodeFailValidator) Engine() any              { return nil }

func nodeWithFailingValidator(t *testing.T) {
	t.Helper()
	orig := binding.Validator
	binding.Validator = nodeFailValidator{}
	t.Cleanup(func() { binding.Validator = orig })
}

func TestNodeHandler_ListAndListCommands_BindValidatorFailure(t *testing.T) {
	nodeWithFailingValidator(t)
	db := newNodeTestDB(t)
	h := newNodeHandler(db)

	c, w := newNodeRequest(http.MethodGet, "/nodes?type=agent", "")
	h.List(c)
	assert.NotEqual(t, http.StatusOK, w.Code)

	c, w = newNodeRequest(http.MethodGet, "/nodes/commands", "")
	h.ListCommands(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestNodeService_UpdateMeta_StoreFailure(t *testing.T) {
	db := newNodeTestDB(t)
	seedNode(t, db, "agent-metafail")
	svc := NewService(nodeSvcCtxOf(db))

	require.NoError(t, db.Callback().Update().Before("gorm:update").
		Register("test:node_fail_update", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("update meta boom"))
		}))

	_, err := svc.UpdateMeta(context.Background(), &NodeMetaUpdateRequest{
		ID:   "agent-metafail",
		Meta: map[string]interface{}{"k": "v"},
	})
	require.Error(t, err)
}
