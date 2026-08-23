package meta

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Root_WithProfiles(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Server: config.ServerConfig{Mode: "test"},
			Profiles: map[string]config.ProfileConfig{
				"dev":  {},
				"prod": {},
			},
		},
	}
	svc := NewService(svcCtx)

	resp, err := svc.Root(nil)
	require.NoError(t, err)
	assert.Equal(t, "croupier-server", resp.Service)
	assert.Equal(t, "test", resp.Environment)
	assert.Equal(t, []string{"dev", "prod"}, resp.Profiles)
	assert.Contains(t, resp.Links, "docs")
	assert.Contains(t, resp.Links, "status")
	assert.Contains(t, resp.Links, "health")
}

func TestService_Root_EmptyProfiles(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Server:   config.ServerConfig{Mode: "prod"},
			Profiles: nil,
		},
	}
	svc := NewService(svcCtx)

	resp, err := svc.Root(nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Profiles)
	assert.Equal(t, "prod", resp.Environment)
}

func TestReadVersionFile_Exists(t *testing.T) {
	// Create a temp directory and write a VERSION file
	tmpDir := t.TempDir()
	versionFile := filepath.Join(tmpDir, "VERSION")
	require.NoError(t, os.WriteFile(versionFile, []byte("1.2.3\n"), 0644))

	// We can't directly test readVersionFile with custom path,
	// but we can test the behavior by changing to the temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	result := readVersionFile()
	assert.Equal(t, "1.2.3", result)
}

func TestReadVersionFile_NotExists(t *testing.T) {
	// Change to a directory without VERSION file
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	result := readVersionFile()
	assert.Equal(t, "", result)
}

func TestCurrentAPIVersion_EnvVar(t *testing.T) {
	// Reset the singleton
	versionOnce = sync.Once{}
	apiVersion = ""

	t.Setenv("CROUPIER_VERSION", "3.0.0")
	result := currentAPIVersion()
	assert.Equal(t, "3.0.0", result)
}

func TestCurrentAPIVersion_Default(t *testing.T) {
	// Reset the singleton
	versionOnce = sync.Once{}
	apiVersion = ""

	// Unset the env var
	os.Unsetenv("CROUPIER_VERSION")

	// Change to a directory without VERSION file
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	result := currentAPIVersion()
	assert.Equal(t, "dev", result)
}

func TestCurrentAPIVersion_VersionFile(t *testing.T) {
	// Create a temp directory with VERSION file
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "VERSION"), []byte("2.5.0"), 0644))

	// Reset the singleton
	versionOnce = sync.Once{}
	apiVersion = ""

	// Unset the env var
	os.Unsetenv("CROUPIER_VERSION")

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	result := currentAPIVersion()
	assert.Equal(t, "2.5.0", result)
}

func TestHandler_Root_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Server:   config.ServerConfig{Mode: "test"},
			Profiles: nil,
		},
	}
	handler := NewHandler(NewService(svcCtx))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1", nil)

	handler.Root(ctx)

	// Service.Root doesn't return errors normally, so this should succeed
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp RootResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "croupier-server", resp.Service)
}

func TestNewHandler(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Server: config.ServerConfig{Mode: "test"},
		},
	}
	service := NewService(svcCtx)
	handler := NewHandler(service)
	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}
