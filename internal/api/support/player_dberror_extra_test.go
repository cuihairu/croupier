// 覆盖目标：ListFAQs / CreateTicket / ListMyTickets 的 DB 错误分支
// （gorm 回调注入 Query/Create 错误）。
package support

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newPlayerFailHandler 建独立内存库并按 op 注入 gorm 回调错误
// （op 取 "Query" 或 "Create"）。
func newPlayerFailHandler(t *testing.T, op string) *PlayerHandler {
	t.Helper()
	name := fmt.Sprintf("support_fail_%s", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	fail := func(tx *gorm.DB) { _ = tx.AddError(errors.New("forced db failure")) }
	cbName := "test:fail_" + op
	switch op {
	case "Query":
		require.NoError(t, db.Callback().Query().Before("gorm:query").Register(cbName, fail))
	case "Create":
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register(cbName, fail))
	default:
		t.Fatalf("unsupported op %q", op)
	}
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(cbName)
		_ = db.Callback().Create().Remove(cbName)
	})
	return NewPlayerHandler(&svc.ServiceContext{
		FAQModel:    model.NewFAQModel(db),
		TicketModel: model.NewTicketModel(db),
	})
}

func TestListFAQs_DBError(t *testing.T) {
	h := newPlayerFailHandler(t, "Query")
	c, w := playerReq(http.MethodGet, "/public/support/faqs", "")
	h.ListFAQs(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "forced db failure")
}

func TestCreateTicket_DBError(t *testing.T) {
	h := newPlayerFailHandler(t, "Create")
	c, w := playerReq(http.MethodPost, "/public/support/tickets",
		`{"title":"t","content":"c","playerId":"p-1"}`)
	h.CreateTicket(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "forced db failure")
}

func TestListMyTickets_DBError(t *testing.T) {
	h := newPlayerFailHandler(t, "Query")
	c, w := playerReq(http.MethodGet, "/public/support/tickets?playerId=p-1", "")
	h.ListMyTickets(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "forced db failure")
}
