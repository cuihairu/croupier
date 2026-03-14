package utils

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
)

// TestValidateFunctionID tests function ID validation
func TestValidateFunctionID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		want    string
		wantErr bool
	}{
		{
			name:    "valid ID",
			id:      "prom.query",
			want:    "prom.query",
			wantErr: false,
		},
		{
			name:    "ID with spaces",
			id:      "  prom.query  ",
			want:    "prom.query",
			wantErr: false,
		},
		{
			name:    "empty ID",
			id:      "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "whitespace only ID",
			id:      "   ",
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateFunctionID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFunctionID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateFunctionID() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildFunctionDTO tests building function DTO from model
func TestBuildFunctionDTO(t *testing.T) {
	fn := &model.Function{
		FunctionID:  "test.function",
		Name:        "Test Function",
		Description: "Test Description",
		Category:    "test",
		GameID:      "game1",
		Status:      1,
		Version:     "1.0.0",
		Instances:   3,
		SpecFormat:  "openapi3.0.3",
	}

	result := BuildFunctionDTO(fn)

	if result.Id != "test.function" {
		t.Errorf("BuildFunctionDTO() Id = %v, want %v", result.Id, "test.function")
	}
	if result.Name != "Test Function" {
		t.Errorf("BuildFunctionDTO() Name = %v, want %v", result.Name, "Test Function")
	}
	if result.GameId != "game1" {
		t.Errorf("BuildFunctionDTO() GameId = %v, want %v", result.GameId, "game1")
	}
	if result.Instances != 3 {
		t.Errorf("BuildFunctionDTO() Instances = %v, want %v", result.Instances, 3)
	}
	if result.SpecFormat != "openapi3.0.3" {
		t.Errorf("BuildFunctionDTO() SpecFormat = %v, want %v", result.SpecFormat, "openapi3.0.3")
	}
}

// TestBuildFunctionInstances tests building function instances
func TestBuildFunctionInstances(t *testing.T) {
	instances := []model.FunctionInstance{
		{
			AgentID:   "agent-1",
			AgentName: "Agent One",
			Status:    "online",
		},
		{
			AgentID:   "agent-2",
			AgentName: "Agent Two",
			Status:    "offline",
		},
	}

	result := BuildFunctionInstances(instances)

	if len(result) != 2 {
		t.Fatalf("BuildFunctionInstances() returned %d items, want 2", len(result))
	}
	if result[0].AgentId != "agent-1" {
		t.Errorf("BuildFunctionInstances()[0].AgentId = %v, want %v", result[0].AgentId, "agent-1")
	}
	if result[1].Status != "offline" {
		t.Errorf("BuildFunctionInstances()[1].Status = %v, want %v", result[1].Status, "offline")
	}
}

// TestBuildFunctionInstances_Empty tests empty instances
func TestBuildFunctionInstances_Empty(t *testing.T) {
	result := BuildFunctionInstances([]model.FunctionInstance{})
	if result == nil {
		t.Error("BuildFunctionInstances() returned nil, want empty slice")
	}
	if len(result) != 0 {
		t.Errorf("BuildFunctionInstances() returned %d items, want 0", len(result))
	}
}

// TestBuildFunctionPermissions tests building function permissions
func TestBuildFunctionPermissions(t *testing.T) {
	perms := []model.FunctionPermission{
		{
			Resource: "prom.query",
			Actions:  datatypes.JSON([]byte(`["read", "execute"]`)),
			Roles:    datatypes.JSON([]byte(`["admin", "user"]`)),
		},
	}

	result := BuildFunctionPermissions(perms)

	if len(result) != 1 {
		t.Fatalf("BuildFunctionPermissions() returned %d items, want 1", len(result))
	}
	if result[0].Resource != "prom.query" {
		t.Errorf("BuildFunctionPermissions()[0].Resource = %v, want %v", result[0].Resource, "prom.query")
	}
	if len(result[0].Actions) != 2 {
		t.Errorf("BuildFunctionPermissions()[0].Actions length = %d, want 2", len(result[0].Actions))
	}
}

// TestBuildInvokeRequest tests building invoke request
func TestBuildInvokeRequest(t *testing.T) {
	functionID := "test.function"
	payload := []byte(`{"key": "value"}`)
	metadata := map[string]string{"game_id": "game1", "env": "prod"}

	result := BuildInvokeRequest(functionID, payload, metadata)

	if result.FunctionId != "test.function" {
		t.Errorf("BuildInvokeRequest() FunctionId = %v, want %v", result.FunctionId, "test.function")
	}
	if string(result.Payload) != string(payload) {
		t.Errorf("BuildInvokeRequest() Payload = %v, want %v", string(result.Payload), string(payload))
	}
	if len(result.Metadata) != 2 {
		t.Errorf("BuildInvokeRequest() Metadata length = %d, want 2", len(result.Metadata))
	}
}

// TestBuildInvokeRequest_NilMetadata tests with nil metadata
func TestBuildInvokeRequest_NilMetadata(t *testing.T) {
	result := BuildInvokeRequest("test.function", []byte("{}"), nil)
	if result.Metadata != nil {
		t.Errorf("BuildInvokeRequest() Metadata = %v, want nil", result.Metadata)
	}
}

// TestConvertFunctionPermissions tests converting API permissions to model
func TestConvertFunctionPermissions(t *testing.T) {
	perms := []FunctionPermission{
		{
			Resource: "prom.query",
			Actions:  []string{"read", "execute"},
			Roles:    []string{"admin", "user"},
		},
	}

	result, err := ConvertFunctionPermissions("func1", perms)

	if err != nil {
		t.Fatalf("ConvertFunctionPermissions() error = %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("ConvertFunctionPermissions() returned %d items, want 1", len(result))
	}
	if result[0].FunctionID != "func1" {
		t.Errorf("ConvertFunctionPermissions()[0].FunctionID = %v, want %v", result[0].FunctionID, "func1")
	}
	if result[0].Resource != "prom.query" {
		t.Errorf("ConvertFunctionPermissions()[0].Resource = %v, want %v", result[0].Resource, "prom.query")
	}
}

// TestConvertFunctionPermissions_EmptyResource tests error on empty resource
func TestConvertFunctionPermissions_EmptyResource(t *testing.T) {
	perms := []FunctionPermission{
		{
			Resource: "",
			Actions:  []string{"read"},
		},
	}

	_, err := ConvertFunctionPermissions("func1", perms)
	if err == nil {
		t.Error("ConvertFunctionPermissions() expected error for empty resource, got nil")
	}
	if !strings.Contains(err.Error(), "权限资源名称不能为空") {
		t.Errorf("ConvertFunctionPermissions() error = %v, want contain '权限资源名称不能为空'", err)
	}
}

// TestConvertFunctionPermissions_WhitespaceResource tests error on whitespace resource
func TestConvertFunctionPermissions_WhitespaceResource(t *testing.T) {
	perms := []FunctionPermission{
		{
			Resource: "   ",
			Actions:  []string{"read"},
		},
	}

	_, err := ConvertFunctionPermissions("func1", perms)
	if err == nil {
		t.Error("ConvertFunctionPermissions() expected error for whitespace resource, got nil")
	}
}

// TestDecodeStringSlice tests decoding JSON array to string slice
func TestDecodeStringSlice(t *testing.T) {
	tests := []struct {
		name string
		data datatypes.JSON
		want []string
	}{
		{
			name: "valid JSON array",
			data: datatypes.JSON([]byte(`["read", "write", "execute"]`)),
			want: []string{"read", "write", "execute"},
		},
		{
			name: "empty JSON array",
			data: datatypes.JSON([]byte(`[]`)),
			want: []string{},
		},
		{
			name: "nil data",
			data: datatypes.JSON(nil),
			want: nil,
		},
		{
			name: "empty data",
			data: datatypes.JSON([]byte{}),
			want: nil,
		},
		{
			name: "invalid JSON",
			data: datatypes.JSON([]byte(`not valid json`)),
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecodeStringSlice(tt.data)
			if tt.want == nil {
				if got != nil {
					t.Errorf("DecodeStringSlice() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("DecodeStringSlice() length = %d, want %d", len(got), len(tt.want))
				return
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("DecodeStringSlice()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestEncodeStringSlice tests encoding string slice to JSON
func TestEncodeStringSlice(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{
			name:   "normal slice",
			values: []string{"read", "write"},
			want:   `["read","write"]`,
		},
		{
			name:   "empty slice",
			values: []string{},
			want:   `[]`,
		},
		{
			name:   "nil slice",
			values: nil,
			want:   `null`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeStringSlice(tt.values)
			// Normalize JSON for comparison
			var gotJSON interface{}
			json.Unmarshal(got, &gotJSON)
			var wantJSON interface{}
			json.Unmarshal([]byte(tt.want), &wantJSON)
			gotNormalized, _ := json.Marshal(gotJSON)
			wantNormalized, _ := json.Marshal(wantJSON)
			if string(gotNormalized) != string(wantNormalized) {
				t.Errorf("EncodeStringSlice() = %v, want %v", string(gotNormalized), string(wantNormalized))
			}
		})
	}
}

// TestHasAdminRole tests checking for admin role
func TestHasAdminRole(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  bool
	}{
		{
			name:  "has admin role",
			roles: []string{"user", "admin"},
			want:  true,
		},
		{
			name:  "has super_admin role",
			roles: []string{"user", "super_admin"},
			want:  true,
		},
		{
			name:  "no admin role",
			roles: []string{"user", "guest"},
			want:  false,
		},
		{
			name:  "empty roles",
			roles: []string{},
			want:  false,
		},
		{
			name:  "nil roles",
			roles: nil,
			want:  false,
		},
		{
			name:  "admin with spaces",
			roles: []string{"  admin  "},
			want:  true,
		},
		{
			name:  "ADMIN uppercase",
			roles: []string{"ADMIN"},
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasAdminRole(tt.roles); got != tt.want {
				t.Errorf("HasAdminRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHasPermissionID tests checking permission ID in list
func TestHasPermissionID(t *testing.T) {
	tests := []struct {
		name          string
		permissionIDs []string
		want          string
		expect        bool
	}{
		{
			name:          "permission exists",
			permissionIDs: []string{"functions.read", "functions.write"},
			want:          "functions.read",
			expect:        true,
		},
		{
			name:          "permission not exists",
			permissionIDs: []string{"functions.read", "functions.write"},
			want:          "functions.delete",
			expect:        false,
		},
		{
			name:          "case insensitive match",
			permissionIDs: []string{"functions.READ"},
			want:          "functions.read",
			expect:        true,
		},
		{
			name:          "empty want",
			permissionIDs: []string{"functions.read"},
			want:          "",
			expect:        false,
		},
		{
			name:          "empty permission list",
			permissionIDs: []string{},
			want:          "functions.read",
			expect:        false,
		},
		{
			name:          "whitespace want",
			permissionIDs: []string{"functions.read"},
			want:          "  ",
			expect:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPermissionID(tt.permissionIDs, tt.want); got != tt.expect {
				t.Errorf("HasPermissionID() = %v, want %v", got, tt.expect)
			}
		})
	}
}

// TestValidatePermissionID tests permission ID validation
func TestValidatePermissionID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		want    string
		wantErr bool
	}{
		{
			name:    "valid ID",
			id:      "functions.read",
			want:    "functions.read",
			wantErr: false,
		},
		{
			name:    "ID with spaces",
			id:      "  functions.read  ",
			want:    "functions.read",
			wantErr: false,
		},
		{
			name:    "empty ID",
			id:      "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			id:      "   ",
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatePermissionID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePermissionID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidatePermissionID() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildPermission tests building permission DTO
func TestBuildPermission(t *testing.T) {
	perm := &model.Permission{
		ID:          "functions.read",
		Name:        "functions.read",
		Description: "Read functions",
		Resource:    "functions",
		Action:      "read",
		Category:    "functions",
	}

	result := BuildPermission(perm)

	if result.Id != "functions.read" {
		t.Errorf("BuildPermission() Id = %v, want %v", result.Id, "functions.read")
	}
	if result.Name != "functions.read" {
		t.Errorf("BuildPermission() Name = %v, want %v", result.Name, "functions.read")
	}
	if result.Resource != "functions" {
		t.Errorf("BuildPermission() Resource = %v, want %v", result.Resource, "functions")
	}
}

// TestRoleNamesFromModels tests extracting role names
func TestRoleNamesFromModels(t *testing.T) {
	roles := []model.Role{
		{Name: "admin"},
		{Name: "user"},
		{Name: "guest"},
	}

	result := RoleNamesFromModels(roles)

	if len(result) != 3 {
		t.Fatalf("RoleNamesFromModels() returned %d items, want 3", len(result))
	}
	if result[0] != "admin" {
		t.Errorf("RoleNamesFromModels()[0] = %v, want %v", result[0], "admin")
	}
}

// TestRoleNamesFromModels_Empty tests empty roles
func TestRoleNamesFromModels_Empty(t *testing.T) {
	result := RoleNamesFromModels([]model.Role{})
	if result == nil {
		t.Error("RoleNamesFromModels() returned nil, want empty slice")
	}
	if len(result) != 0 {
		t.Errorf("RoleNamesFromModels() returned %d items, want 0", len(result))
	}
}
