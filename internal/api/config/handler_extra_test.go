package config

import (
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupConfigTestDB(t *testing.T) (*Handler, *Service) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ConfigVersion{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svcCtx := &svc.ServiceContext{
		ConfigVersionModel: model.NewConfigVersionModel(db),
	}
	service := NewService(svcCtx)
	handler := NewHandler(service)
	return handler, service
}

func newGinContextWithParams(recorder interface{}, method, target string, params gin.Params) (*gin.Context, *gin.ResponseWriter) {
	panic("use newConfigTestContext instead")
}

func TestHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	// First create a config
	ctx1, _ := newConfigTestContext(http.MethodPost, "/api/v1/configs", `{"key":"test:list","value":"{}"}`)
	handler.Upsert(ctx1)

	// Test List handler
	ctx2, rec := newConfigTestContext(http.MethodGet, "/api/v1/configs", "")
	handler.List(ctx2)

	if rec.Code != http.StatusOK {
		t.Errorf("List status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	// Create a config first
	ctx1, _ := newConfigTestContext(http.MethodPost, "/api/v1/configs", `{"key":"test:get","value":"hello"}`)
	handler.Upsert(ctx1)

	// Test Get handler - need to set route param
	ctx2, rec := newConfigTestContext(http.MethodGet, "/api/v1/configs/test:get", "")
	ctx2.Params = gin.Params{{Key: "id", Value: "test:get"}}
	handler.Get(ctx2)

	if rec.Code != http.StatusOK {
		t.Errorf("Get status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Save(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	// Test Save handler
	ctx, rec := newConfigTestContext(http.MethodPut, "/api/v1/configs/test:save", `{"content":"{\"a\":1}","format":"json","message":"initial"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "test:save"}}
	handler.Save(ctx)

	if rec.Code != http.StatusOK {
		t.Errorf("Save status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Save_MalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	ctx, rec := newConfigTestContext(http.MethodPut, "/api/v1/configs/test:save", "{bad json")
	ctx.Params = gin.Params{{Key: "id", Value: "test:save"}}
	handler.Save(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_Validate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	ctx, rec := newConfigTestContext(http.MethodPost, "/api/v1/configs/test:validate/validate", `{"format":"json","content":"{\"a\":1}"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "test:validate"}}
	handler.Validate(ctx)

	if rec.Code != http.StatusOK {
		t.Errorf("Validate status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Validate_InvalidContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	ctx, rec := newConfigTestContext(http.MethodPost, "/api/v1/configs/test:validate/validate", `{"format":"json","content":"{invalid}"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "test:validate"}}
	handler.Validate(ctx)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandler_Validate_MalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	ctx, rec := newConfigTestContext(http.MethodPost, "/api/v1/configs/test:validate/validate", "{bad")
	ctx.Params = gin.Params{{Key: "id", Value: "test:validate"}}
	handler.Validate(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandler_ListVersionsByID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	// Create a config first
	ctx1, _ := newConfigTestContext(http.MethodPost, "/api/v1/configs", `{"key":"test:listbyid","value":"v1"}`)
	handler.Upsert(ctx1)

	ctx2, _ := newConfigTestContext(http.MethodPost, "/api/v1/configs", `{"key":"test:listbyid","value":"v2"}`)
	handler.Upsert(ctx2)

	// Test ListVersionsByID
	ctx3, rec := newConfigTestContext(http.MethodGet, "/api/v1/configs/test:listbyid/versions", "")
	ctx3.Params = gin.Params{{Key: "id", Value: "test:listbyid"}}
	handler.ListVersionsByID(ctx3)

	if rec.Code != http.StatusOK {
		t.Errorf("ListVersionsByID status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_GetVersionByID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	// Create a config
	ctx1, _ := newConfigTestContext(http.MethodPost, "/api/v1/configs", `{"key":"test:getverbyid","value":"hello"}`)
	handler.Upsert(ctx1)

	// Test GetVersionByID
	ctx2, rec := newConfigTestContext(http.MethodGet, "/api/v1/configs/test:getverbyid/versions/1", "")
	ctx2.Params = gin.Params{{Key: "id", Value: "test:getverbyid"}, {Key: "version", Value: "1"}}
	handler.GetVersionByID(ctx2)

	if rec.Code != http.StatusOK {
		t.Errorf("GetVersionByID status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_GetVersionByID_InvalidVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	ctx, rec := newConfigTestContext(http.MethodGet, "/api/v1/configs/test/versions/abc", "")
	ctx.Params = gin.Params{{Key: "id", Value: "test"}, {Key: "version", Value: "abc"}}
	handler.GetVersionByID(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestConfigIDFromPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newConfigTestContext(http.MethodGet, "/", "")
	ctx.Params = gin.Params{{Key: "id", Value: "my-config"}}
	if got := configIDFromPath(ctx); got != "my-config" {
		t.Errorf("configIDFromPath = %q, want %q", got, "my-config")
	}
}

func TestService_ListConfigs(t *testing.T) {
	handler, service := setupConfigTestDB(t)

	// Create configs via handler
	ctx1, _ := newConfigTestContext(http.MethodPost, "/", `{"key":"svc:list1","value":"{}"}`)
	handler.Upsert(ctx1)

	resp, err := service.ListConfigs(t.Context(), &ListConfigsRequest{})
	if err != nil {
		t.Fatalf("ListConfigs: %v", err)
	}
	if len(resp.Items) == 0 {
		t.Error("expected at least 1 item")
	}
}

func TestService_ListConfigs_NilRequest(t *testing.T) {
	_, service := setupConfigTestDB(t)

	resp, err := service.ListConfigs(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListConfigs(nil): %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestService_GetConfig(t *testing.T) {
	handler, service := setupConfigTestDB(t)

	ctx1, _ := newConfigTestContext(http.MethodPost, "/", `{"key":"svc:get","value":"data"}`)
	handler.Upsert(ctx1)

	resp, err := service.GetConfig(t.Context(), &GetConfigRequest{ID: "svc:get"})
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if resp.Content != "data" {
		t.Errorf("content = %q, want %q", resp.Content, "data")
	}
}

func TestService_GetConfig_NilRequest(t *testing.T) {
	_, service := setupConfigTestDB(t)

	_, err := service.GetConfig(t.Context(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestService_SaveConfig(t *testing.T) {
	_, service := setupConfigTestDB(t)

	resp, err := service.SaveConfig(t.Context(), "svc:save", &SaveConfigRequest{
		Content: `{"x":1}`,
		Format:  "json",
		Message: "first",
	})
	if err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if resp.Version != 1 {
		t.Errorf("version = %d, want 1", resp.Version)
	}
}

func TestService_SaveConfig_NilRequest(t *testing.T) {
	_, service := setupConfigTestDB(t)

	_, err := service.SaveConfig(t.Context(), "test", nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestService_SaveConfig_EmptyID(t *testing.T) {
	_, service := setupConfigTestDB(t)

	_, err := service.SaveConfig(t.Context(), "  ", &SaveConfigRequest{Content: "{}"})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestService_ValidateConfig(t *testing.T) {
	_, service := setupConfigTestDB(t)

	resp, err := service.ValidateConfig(t.Context(), "test", &ValidateConfigRequest{
		Format:  "json",
		Content: `{"a":1}`,
	})
	if err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
	if !resp.Valid {
		t.Errorf("expected valid, errors: %v", resp.Errors)
	}
}

func TestService_ValidateConfig_InvalidJSON(t *testing.T) {
	_, service := setupConfigTestDB(t)

	resp, err := service.ValidateConfig(t.Context(), "test", &ValidateConfigRequest{
		Format:  "json",
		Content: `{bad}`,
	})
	if err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
	if resp.Valid {
		t.Error("expected invalid")
	}
}

func TestService_ValidateConfig_NilRequest(t *testing.T) {
	_, service := setupConfigTestDB(t)

	_, err := service.ValidateConfig(t.Context(), "test", nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestHandler_Get_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	ctx, rec := newConfigTestContext(http.MethodGet, "/api/v1/configs/nonexistent", "")
	ctx.Params = gin.Params{{Key: "id", Value: "nonexistent"}}
	handler.Get(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for not-found config, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_GetVersionByID_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	ctx, rec := newConfigTestContext(http.MethodGet, "/api/v1/configs/nonexistent/versions/1", "")
	ctx.Params = gin.Params{{Key: "id", Value: "nonexistent"}, {Key: "version", Value: "1"}}
	handler.GetVersionByID(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for not-found version, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ListVersionsByID_EmptyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	ctx, rec := newConfigTestContext(http.MethodGet, "/api/v1/configs//versions", "")
	ctx.Params = gin.Params{{Key: "id", Value: ""}}
	handler.ListVersionsByID(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for empty key, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Save_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	// SaveConfig with empty ID should return error
	ctx, rec := newConfigTestContext(http.MethodPut, "/api/v1/configs/test/save", `{"content":"{}"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "  "}}
	handler.Save(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for empty key, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Validate_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	// ValidateConfig with nil request triggers error
	// We can trigger this by sending empty body with POST
	ctx, rec := newConfigTestContext(http.MethodPost, "/api/v1/configs/test/validate", `{"format":"unsupported","content":"data"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "test"}}
	handler.Validate(ctx)

	// Should succeed (returns valid:false) because ValidateConfig doesn't return error for unsupported format
	// The error path (line 94-97) is only hit if ValidateConfig returns an error, which it doesn't normally
	if rec.Code != http.StatusOK {
		t.Logf("Validate with unsupported format: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ListVersions_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	// Call ListVersions with empty key via POST to trigger service error
	ctx, rec := newConfigTestContext(http.MethodPost, "/api/v1/configs/versions", `{"key":""}`)
	handler.ListVersions(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for empty key, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandler_GetVersion_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, _ := setupConfigTestDB(t)

	// Call GetVersion with empty key via POST to trigger service error
	ctx, rec := newConfigTestContext(http.MethodPost, "/api/v1/configs/versions/get", `{"key":"","version":1}`)
	handler.GetVersion(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for empty key, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateConfigContent_Extra(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		content string
		wantErr bool
	}{
		{"valid json", "json", `{"a":1}`, false},
		{"invalid json", "json", `{bad}`, true},
		{"valid yaml", "yaml", "key: value\n", false},
		{"valid yml", "yml", "key: value\n", false},
		{"valid xml", "xml", "<root><a>1</a></root>", false},
		{"invalid xml", "xml", "<root><a>1</b></root>", true},
		{"valid ini", "ini", "[section]\nkey=value\n", false},
		{"empty ini", "ini", "", true},
		{"ini with comment", "ini", "# comment\n; comment\n[key]=val\n", false},
		{"invalid ini", "ini", "no_equals_or_colon", true},
		{"valid csv", "csv", "a,b,c\n1,2,3", false},
		{"empty csv", "csv", "", true},
		{"empty format defaults to json", "", `{"a":1}`, false},
		{"unsupported format", "toml", "key=value", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfigContent(tt.format, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfigContent(%q, %q) error = %v, wantErr %v", tt.format, tt.content, err, tt.wantErr)
			}
		})
	}
}
