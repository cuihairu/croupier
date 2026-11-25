// Coverage target: the DB error branches of Service.List/Upsert/Delete
// (injected via gorm callbacks).
package terms

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newTermsFailService builds an isolated in-memory database and injects
// callback errors (ops names the callback class that should fail, e.g.
// "create"/"delete").
func newTermsFailService(t *testing.T, ops ...string) *Service {
	t.Helper()
	name := fmt.Sprintf("terms_fail_%s", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	fail := func(tx *gorm.DB) { _ = tx.AddError(errors.New("forced db failure")) }
	for _, op := range ops {
		switch op {
		case "query":
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_query", fail))
		case "create":
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_create", fail))
		case "delete":
			require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("test:fail_delete", fail))
		default:
			t.Fatalf("unsupported op %q", op)
		}
	}
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove("test:fail_query")
		_ = db.Callback().Create().Remove("test:fail_create")
		_ = db.Callback().Delete().Remove("test:fail_delete")
	})
	return NewService(&svc.ServiceContext{TermDictModel: model.NewTermDictionaryModel(db)})
}

func TestService_List_DBError(t *testing.T) {
	s := newTermsFailService(t, "query")
	_, err := s.List(context.Background(), &TermsListRequest{Domain: "resource"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced db failure")
}

func TestService_Upsert_DBError(t *testing.T) {
	s := newTermsFailService(t, "create")
	_, err := s.Upsert(context.Background(), &TermUpsertRequest{
		Domain: "resource", TermKey: "player", Alias: "玩家",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced db failure")
}

func TestService_Delete_DBError(t *testing.T) {
	s := newTermsFailService(t, "delete")
	_, err := s.Delete(context.Background(), &TermDeleteRequest{Domain: "resource", Alias: "玩家"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced db failure")
}

func TestHandler_List_ServiceError(t *testing.T) {
	s := newTermsFailService(t, "query")
	h := NewHandler(s)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/terms?domain=resource", nil)
	h.List(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
