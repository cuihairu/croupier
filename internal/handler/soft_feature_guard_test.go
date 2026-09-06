package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSoftFeatureGuardEnabledNilFailOpen covers the fail-open semantics: a
// nil guard (or a guard without layered settings) must never lock out every
// feature domain.
func TestSoftFeatureGuardEnabledNilFailOpen(t *testing.T) {
	var nilGuard *softFeatureGuard
	assert.True(t, nilGuard.enabled("dev"))
	assert.True(t, newSoftFeatureGuard(nil).enabled("dev"))
}

// TestSoftFeatureGuardGuardDisabledBlocks covers the interception branch: a
// domain explicitly disabled at L2 gets 403 feature_disabled.
func TestSoftFeatureGuardGuardDisabledBlocks(t *testing.T) {
	settings.ResetForTest()
	defer settings.ResetForTest()

	layered := settings.InitLayered(context.Background(), &settings.ConfigInput{
		FeatureFlags: map[string]bool{"support": false},
	}, nil)
	require.False(t, layered.FeatureEnabled("support"))

	gin.SetMode(gin.TestMode)
	blocked := false
	r := gin.New()
	r.Use(newSoftFeatureGuard(layered).guard("support"))
	r.GET("/api/v1/faqs", func(c *gin.Context) { blocked = true })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/faqs", nil))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "feature_disabled")
	assert.False(t, blocked, "downstream handler must not run when the domain is disabled")
}

// TestSoftFeatureGuardGuardDisabledMiddlewareDirect covers the same
// interception branch by invoking the middleware function directly.
func TestSoftFeatureGuardGuardDisabledMiddlewareDirect(t *testing.T) {
	settings.ResetForTest()
	defer settings.ResetForTest()

	layered := settings.InitLayered(context.Background(), &settings.ConfigInput{
		FeatureFlags: map[string]bool{"ops": false},
	}, nil)
	require.False(t, layered.FeatureEnabled("ops"))

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	newSoftFeatureGuard(layered).guard("ops")(ctx)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "feature_disabled")
	assert.True(t, ctx.IsAborted())
}

// TestSoftFeatureGuardGuardEnabledPassesThrough covers the c.Next() branch:
// with no layered settings the guard fails open and lets the request through.
func TestSoftFeatureGuardGuardEnabledPassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ran := false
	r := gin.New()
	r.Use(newSoftFeatureGuard(nil).guard("support"))
	r.GET("/ping", func(c *gin.Context) { ran = true; c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, ran, "handler behind an enabled guard must run")
}
