package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRoutesTestContext(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, nil)
	return ctx, rec
}

func TestHandler_GetRoutes_Success(t *testing.T) {
	handler := NewHandler(NewService())

	ctx, rec := newRoutesTestContext(http.MethodGet, "/api/v1/routes")
	handler.GetRoutes(ctx)

	assert.Equal(t, http.StatusOK, rec.Code)

	var routes []RouteItem
	err := json.Unmarshal(rec.Body.Bytes(), &routes)
	require.NoError(t, err)
	assert.NotEmpty(t, routes, "routes list should not be empty")

	// Verify each route has the expected shape.
	for _, r := range routes {
		assert.NotEmpty(t, r.Path)
		assert.NotEmpty(t, r.Name)
		assert.NotEmpty(t, r.Component)
	}
}

func TestHandler_GetRoutes_ReturnsKnownRoute(t *testing.T) {
	handler := NewHandler(NewService())

	ctx, rec := newRoutesTestContext(http.MethodGet, "/api/v1/routes")
	handler.GetRoutes(ctx)

	require.Equal(t, http.StatusOK, rec.Code)

	var routes []RouteItem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &routes))

	found := false
	for _, r := range routes {
		if r.Path == "/api/v1/admin" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected the admin route to be present")
}

func TestService_GetRoutes_NoError(t *testing.T) {
	svc := NewService()
	resp, err := svc.GetRoutes(nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Greater(t, len(*resp), 0)
}
