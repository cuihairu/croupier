// Coverage target (coverage final):
//  1. The query bind failure branch of handler.List (binding.Validator failure injected).
//  2. The ListAll re-read failure branch in service.Update (error injected via the
//     gorm query callback; Updates goes through the update callback and is unaffected).
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

// failValidator makes the validate step of any Bind return an error, used to
// cover the bind-failure branch that a normal request cannot trigger because
// every DTO field is an optional string.
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

	// Updates goes through the update callback, while ListAll (Find/SELECT)
	// goes through the query callback: failing the query alone leaves Update
	// itself succeeding but the ListAll re-read failing.
	require.NoError(t, db.Callback().Query().Before("gorm.query").
		Register("test:tool_fail_listall", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("listall boom"))
		}))

	_, err = s.Update(context.Background(), &ToolUpdateRequest{ID: fmt.Sprintf("%d", created.Id), Name: "renamed"})
	require.Error(t, err)
}
