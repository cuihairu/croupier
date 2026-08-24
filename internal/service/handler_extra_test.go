package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newHandlerTestContext builds a gin test context for the given request.
func newHandlerTestContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	return c, w
}

func newClosedServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return db
}

func TestContractHandlers_SuccessAndFailure(t *testing.T) {
	ctx := proposalTestContext()
	db := setupTestDB(t)
	handler := NewContractHandler(NewContractService(db))

	// ListContracts success.
	c, w := newHandlerTestContext(http.MethodGet, "/api/contracts")
	c.Request = c.Request.WithContext(ctx)
	handler.ListContracts(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// GetContract for an unknown function id surfaces a 404 error response.
	c, w = newHandlerTestContext(http.MethodGet, "/api/contracts/player.query")
	c.Params = gin.Params{{Key: "functionId", Value: "player.query"}}
	c.Request = c.Request.WithContext(ctx)
	handler.GetContract(c)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// ListResourceCapabilities success.
	c, w = newHandlerTestContext(http.MethodGet, "/api/resource-capabilities")
	c.Request = c.Request.WithContext(ctx)
	handler.ListResourceCapabilities(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Failure paths via a closed database pool.
	broken := NewContractHandler(NewContractService(newClosedServiceDB(t)))

	c, w = newHandlerTestContext(http.MethodGet, "/api/contracts")
	c.Request = c.Request.WithContext(ctx)
	broken.ListContracts(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	c, w = newHandlerTestContext(http.MethodGet, "/api/contracts/player.query")
	c.Params = gin.Params{{Key: "functionId", Value: "player.query"}}
	c.Request = c.Request.WithContext(ctx)
	broken.GetContract(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	c, w = newHandlerTestContext(http.MethodGet, "/api/resource-capabilities")
	c.Request = c.Request.WithContext(ctx)
	broken.ListResourceCapabilities(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestProposalHandlers_BasicFlows(t *testing.T) {
	ctx := proposalTestContext()
	db := setupTestDB(t)
	handler := NewProposalHandler(NewProposalService(db))

	// Empty inbox/list succeed.
	for _, tc := range []struct {
		name   string
		do     func(h *ProposalHandler, c *gin.Context)
		params []gin.Param
		path   string
	}{
		{"list", func(h *ProposalHandler, c *gin.Context) { h.ListProposals(c) }, nil, "/api/proposals"},
		{"inbox", func(h *ProposalHandler, c *gin.Context) { h.Inbox(c) }, nil, "/api/proposals/inbox"},
	} {
		c, w := newHandlerTestContext(http.MethodGet, tc.path)
		c.Request = c.Request.WithContext(ctx)
		tc.do(handler, c)
		assert.Equal(t, http.StatusOK, w.Code, tc.name)
	}

	// Unknown proposal keys produce error responses.
	notFound := []struct {
		name   string
		do     func(h *ProposalHandler, c *gin.Context)
		path   string
		status int
	}{
		{"get", func(h *ProposalHandler, c *gin.Context) { h.GetProposal(c) }, "/api/proposals/nope", http.StatusNotFound},
		{"accept", func(h *ProposalHandler, c *gin.Context) { h.AcceptProposal(c) }, "/accept", http.StatusNotFound},
		{"publish", func(h *ProposalHandler, c *gin.Context) { h.AcceptAndPublishProposal(c) }, "/publish", http.StatusNotFound},
	}
	for _, tc := range notFound {
		c, w := newHandlerTestContext(http.MethodPost, tc.path)
		c.Params = gin.Params{{Key: "proposalKey", Value: "missing-key"}}
		c.Request = c.Request.WithContext(ctx)
		tc.do(handler, c)
		assert.Equal(t, tc.status, w.Code, tc.name)
	}

	// Closed database turns every handler into an error response.
	broken := NewProposalHandler(NewProposalService(newClosedServiceDB(t)))
	failing := []struct {
		name string
		do   func(h *ProposalHandler, c *gin.Context)
	}{
		{"list", func(h *ProposalHandler, c *gin.Context) { h.ListProposals(c) }},
		{"inbox", func(h *ProposalHandler, c *gin.Context) { h.Inbox(c) }},
	}
	for _, tc := range failing {
		c, w := newHandlerTestContext(http.MethodGet, "/api/proposals")
		c.Request = c.Request.WithContext(ctx)
		tc.do(broken, c)
		assert.Equal(t, http.StatusInternalServerError, w.Code, tc.name)
	}
}

func TestExportHandler_PagesAndFailures(t *testing.T) {
	ctx := proposalTestContext()
	db := setupTestDB(t)
	handler := NewExportHandler(NewDataExportService(db))

	c, w := newHandlerTestContext(http.MethodGet, "/api/export/pages")
	c.Request = c.Request.WithContext(ctx)
	handler.ExportPages(c)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=page-export-")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	broken := NewExportHandler(NewDataExportService(newClosedServiceDB(t)))
	c, w = newHandlerTestContext(http.MethodGet, "/api/export/pages")
	c.Request = c.Request.WithContext(ctx)
	broken.ExportPages(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Direct service error branch for parity.
	svc := NewDataExportService(newClosedServiceDB(t))
	_, err := svc.ExportToJSON(context.Background(), "demo-game", "development")
	assert.Error(t, err)
}
