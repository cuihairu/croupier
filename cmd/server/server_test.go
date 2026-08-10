package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// maskIfSet
// ---------------------------------------------------------------------------

func TestMaskIfSet_Empty(t *testing.T) {
	assert.Equal(t, "未设置", maskIfSet(""))
}

func TestMaskIfSet_Short(t *testing.T) {
	assert.Equal(t, "已设置", maskIfSet("abc"))
	assert.Equal(t, "已设置", maskIfSet("12345678"))
}

func TestMaskIfSet_Long(t *testing.T) {
	result := maskIfSet("my-secret-key-value")
	assert.Contains(t, result, "已设置")
	assert.Contains(t, result, "alue")
	assert.NotContains(t, result, "my-secret")
}

// ---------------------------------------------------------------------------
// loadConfigFile
// ---------------------------------------------------------------------------

func TestLoadConfigFile_EmptyPath(t *testing.T) {
	_, err := loadConfigFile("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "必须指定配置文件")
}

func TestLoadConfigFile_NotFound(t *testing.T) {
	_, err := loadConfigFile("/nonexistent/path/config.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "读取配置文件失败")
}

func TestLoadConfigFile_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.yaml")
	// Use YAML that triggers a parse error (tab in mapping key)
	require.NoError(t, os.WriteFile(path, []byte("server:\n\tport: bad"), 0644))

	_, err := loadConfigFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "解析配置文件失败")
}

func TestLoadConfigFile_Valid(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "server.yaml")
	content := `
server:
  host: 0.0.0.0
  port: 18780
  mode: dev
database:
  driver: sqlite
  dataSource: ":memory:"
auth:
  jwtSecret: test-secret
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	c, err := loadConfigFile(path)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", c.Server.Host)
	assert.Equal(t, 18780, c.Server.Port)
	assert.Equal(t, "dev", c.Server.Mode)
	assert.Equal(t, "sqlite", c.Database.Driver)
	assert.Equal(t, "test-secret", c.Auth.JWTSecret)
}

func TestLoadConfigFile_EnvExpansion(t *testing.T) {
	t.Setenv("CROUPIER_TEST_PORT", "9999")
	tmp := t.TempDir()
	path := filepath.Join(tmp, "server.yaml")
	content := `
server:
  host: 0.0.0.0
  port: ${CROUPIER_TEST_PORT}
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	c, err := loadConfigFile(path)
	require.NoError(t, err)
	assert.Equal(t, 9999, c.Server.Port)
}

// ---------------------------------------------------------------------------
// validateAndAdjustTimeout
// ---------------------------------------------------------------------------

func TestValidateAndAdjustTimeout_NoAdjust(t *testing.T) {
	c := &config.Config{}
	c.Server.Timeout = 600000 // 600s > 30*3=90s
	c.SSE.KeepAliveInterval = 30
	validateAndAdjustTimeout(c)
	assert.Equal(t, int64(600000), c.Server.Timeout)
}

func TestValidateAndAdjustTimeout_AutoAdjust(t *testing.T) {
	c := &config.Config{}
	c.Server.Timeout = 10000 // 10s < 30*3=90s
	c.SSE.KeepAliveInterval = 30
	validateAndAdjustTimeout(c)
	assert.Equal(t, int64(90000), c.Server.Timeout) // 90s * 1000
}

func TestValidateAndAdjustTimeout_CustomKeepAlive(t *testing.T) {
	c := &config.Config{}
	c.Server.Timeout = 5000 // 5s < 10*3=30s
	c.SSE.KeepAliveInterval = 10
	validateAndAdjustTimeout(c)
	assert.Equal(t, int64(30000), c.Server.Timeout) // 30s * 1000
}

func TestValidateAndAdjustTimeout_ExactBoundary(t *testing.T) {
	c := &config.Config{}
	c.Server.Timeout = 90000 // exactly 90s = 30*3
	c.SSE.KeepAliveInterval = 30
	validateAndAdjustTimeout(c)
	assert.Equal(t, int64(90000), c.Server.Timeout) // no change
}

// ---------------------------------------------------------------------------
// applyRuntimeDefaults
// ---------------------------------------------------------------------------

func TestApplyRuntimeDefaults_NilConfig(t *testing.T) {
	// Should not panic
	applyRuntimeDefaults(nil)
}

func TestApplyRuntimeDefaults_DefaultTimeout(t *testing.T) {
	c := &config.Config{}
	applyRuntimeDefaults(c)
	assert.Equal(t, int64(600000), c.Server.Timeout)
}

func TestApplyRuntimeDefaults_PreservesExistingTimeout(t *testing.T) {
	c := &config.Config{}
	c.Server.Timeout = 120000
	c.SSE.KeepAliveInterval = 10 // 10*3=30s < 120s
	applyRuntimeDefaults(c)
	assert.Equal(t, int64(120000), c.Server.Timeout)
}

func TestApplyRuntimeDefaults_FileStorageDefaultDir(t *testing.T) {
	c := &config.Config{}
	c.Storage.Driver = "file"
	applyRuntimeDefaults(c)
	assert.Equal(t, filepath.Join("data", "uploads"), c.Storage.BaseDir)
}

func TestApplyRuntimeDefaults_FileStoragePreservesDir(t *testing.T) {
	c := &config.Config{}
	c.Storage.Driver = "file"
	c.Storage.BaseDir = "/custom/path"
	applyRuntimeDefaults(c)
	assert.Equal(t, "/custom/path", c.Storage.BaseDir)
}

func TestApplyRuntimeDefaults_NonFileStorage(t *testing.T) {
	c := &config.Config{}
	c.Storage.Driver = "s3"
	applyRuntimeDefaults(c)
	assert.Equal(t, "", c.Storage.BaseDir)
}

// ---------------------------------------------------------------------------
// PrintVersionInfo
// ---------------------------------------------------------------------------

func TestPrintVersionInfo_DoesNotPanic(t *testing.T) {
	// Save and restore globals
	oldVersion, oldCommit, oldBuild := Version, GitCommit, BuildTime
	defer func() {
		Version, GitCommit, BuildTime = oldVersion, oldCommit, oldBuild
	}()

	Version = "1.2.3"
	GitCommit = "abc1234"
	BuildTime = "2025-01-01"
	assert.NotPanics(t, PrintVersionInfo)
}

func TestPrintVersionInfo_UnknownCommit(t *testing.T) {
	oldVersion, oldCommit, oldBuild := Version, GitCommit, BuildTime
	defer func() {
		Version, GitCommit, BuildTime = oldVersion, oldCommit, oldBuild
	}()

	Version = "dev"
	GitCommit = "unknown"
	BuildTime = ""
	assert.NotPanics(t, PrintVersionInfo)
}

// ---------------------------------------------------------------------------
// wrapHTTPHandler
// ---------------------------------------------------------------------------

func TestWrapHTTPHandler_NilSvcCtx(t *testing.T) {
	// Should return the original handler without panic
	handler := &mockHTTPHandler{}
	result := wrapHTTPHandler(nil, handler)
	assert.Equal(t, handler, result)
}

func TestWrapHTTPHandler_NilTelemetry(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	handler := &mockHTTPHandler{}
	result := wrapHTTPHandler(svcCtx, handler)
	assert.Equal(t, handler, result)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

type mockHTTPHandler struct{}

func (h *mockHTTPHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
