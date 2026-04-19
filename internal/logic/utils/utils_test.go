package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
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

// TestParseUintID tests parsing uint ID
func TestParseUintID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		label   string
		want    uint
		wantErr bool
	}{
		{
			name:    "valid ID",
			id:      "123",
			label:   "ID",
			want:    123,
			wantErr: false,
		},
		{
			name:    "empty ID",
			id:      "",
			label:   "ID",
			want:    0,
			wantErr: true,
		},
		{
			name:    "whitespace only ID",
			id:      "   ",
			label:   "ID",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid non-numeric ID",
			id:      "abc",
			label:   "ID",
			want:    0,
			wantErr: true,
		},
		{
			name:    "zero ID",
			id:      "0",
			label:   "ID",
			want:    0,
			wantErr: true,
		},
		{
			name:    "negative ID",
			id:      "-1",
			label:   "ID",
			want:    0,
			wantErr: true,
		},
		{
			name:    "ID with spaces",
			id:      "  456  ",
			label:   "ID",
			want:    0,
			wantErr: true, // strconv.ParseUint doesn't trim spaces
		},
		{
			name:    "large ID",
			id:      "18446744073709551615",
			label:   "ID",
			want:    18446744073709551615,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUintID(tt.id, tt.label)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseUintID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseUintID() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidatePassword tests password validation with enhanced security rules
func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		wantErr     bool
		errContains string
	}{
		// Valid passwords (2+ character varieties)
		{
			name:     "valid: lowercase + digits",
			password: "password123",
			wantErr:  false,
		},
		{
			name:     "valid: uppercase + lowercase",
			password: "Password",
			wantErr:  false,
		},
		{
			name:     "valid: uppercase + digits",
			password: "PASSWORD123",
			wantErr:  false,
		},
		{
			name:     "valid: lowercase + special",
			password: "password!",
			wantErr:  false,
		},
		{
			name:     "valid: all 4 varieties",
			password: "Pass123!",
			wantErr:  false,
		},
		{
			name:     "valid: exactly 8 chars, 2 varieties",
			password: "Pass123",
			wantErr:  false,
		},
		{
			name:     "valid: long password with special chars",
			password: "MySecure@Password#2023",
			wantErr:  false,
		},
		{
			name:     "valid: mixed case and numbers",
			password: "Abc123xyz",
			wantErr:  false,
		},
		{
			name:     "valid: chinese characters + digits",
			password: "密码123456",
			wantErr:  false,
		},

		// Empty password
		{
			name:        "invalid: empty password",
			password:    "",
			wantErr:     true,
			errContains: "密码不能为空",
		},

		// Whitespace violations
		{
			name:        "invalid: space in middle",
			password:    "pass word",
			wantErr:     true,
			errContains: "密码不能包含空格",
		},
		{
			name:        "invalid: tab character",
			password:    "pass\tword",
			wantErr:     true,
			errContains: "密码不能包含空格",
		},
		{
			name:        "invalid: newline character",
			password:    "pass\nword",
			wantErr:     true,
			errContains: "密码不能包含空格",
		},
		{
			name:        "invalid: trailing space",
			password:    "password ",
			wantErr:     true,
			errContains: "密码不能包含空格",
		},
		{
			name:        "invalid: leading space",
			password:    " password",
			wantErr:     true,
			errContains: "密码不能包含空格",
		},

		// Length violations
		{
			name:        "invalid: 7 chars (too short)",
			password:    "Pass12",
			wantErr:     true,
			errContains: "密码长度至少为8个字符",
		},
		{
			name:        "invalid: 1 char (too short)",
			password:    "a",
			wantErr:     true,
			errContains: "密码长度至少为8个字符",
		},
		{
			name:        "invalid: 129 chars (too long)",
			password:    strings.Repeat("a", 129),
			wantErr:     true,
			errContains: "密码长度不能超过128个字符",
		},
		{
			name:        "invalid: 200 chars (too long)",
			password:    strings.Repeat("a", 200),
			wantErr:     true,
			errContains: "密码长度不能超过128个字符",
		},

		// Weak password violations
		{
			name:        "invalid: common password 'password'",
			password:    "password",
			wantErr:     true,
			errContains: "密码过于简单",
		},
		{
			name:        "invalid: common password '12345678'",
			password:    "12345678",
			wantErr:     true,
			errContains: "密码过于简单",
		},
		{
			name:        "invalid: common password 'qwerty123'",
			password:    "qwerty123",
			wantErr:     true,
			errContains: "密码过于简单",
		},
		{
			name:        "invalid: common password 'admin'",
			password:    "admin",
			wantErr:     true,
			errContains: "密码过于简单",
		},
		{
			name:        "invalid: common password 'welcome'",
			password:    "welcome",
			wantErr:     true,
			errContains: "密码过于简单",
		},

		// Character variety violations (less than 2 of 4 categories)
		{
			name:        "invalid: only lowercase",
			password:    "password",
			wantErr:     true,
			errContains: "密码必须包含大写字母、小写字母、数字、特殊字符中的至少两种",
		},
		{
			name:        "invalid: only uppercase",
			password:    "PASSWORD",
			wantErr:     true,
			errContains: "密码必须包含大写字母、小写字母、数字、特殊字符中的至少两种",
		},
		{
			name:        "invalid: only digits",
			password:    "12345678",
			wantErr:     true,
			errContains: "密码必须包含大写字母、小写字母、数字、特殊字符中的至少两种",
		},
		{
			name:        "invalid: only special chars",
			password:    "!@#$%^&*",
			wantErr:     true,
			errContains: "密码必须包含大写字母、小写字母、数字、特殊字符中的至少两种",
		},
		{
			name:        "invalid: single variety - all lowercase with 8 chars",
			password:    "abcdefgh",
			wantErr:     true,
			errContains: "密码必须包含大写字母、小写字母、数字、特殊字符中的至少两种",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if tt.errContains != "" && err != nil {
					if !strings.Contains(err.Error(), tt.errContains) {
						t.Errorf("ValidatePassword() error = %v, should contain %v", err, tt.errContains)
					}
				}
			} else {
				if got != tt.password {
					t.Errorf("ValidatePassword() = %v, want %v", got, tt.password)
				}
			}
		})
	}
}

// TestValidatePasswordForUser tests password validation with username check
func TestValidatePasswordForUser(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		username    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid: password does not contain username",
			password: "SecurePass123!",
			username: "admin",
			wantErr:  false,
		},
		{
			name:     "valid: different case, password contains username",
			password: "SecurePass123!",
			username: "john",
			wantErr:  false,
		},
		{
			name:     "valid: empty username",
			password: "MyPassword123",
			username: "",
			wantErr:  false,
		},
		{
			name:        "invalid: password contains username exact match",
			password:    "john123456",
			username:    "john",
			wantErr:     true,
			errContains: "密码不能包含用户名",
		},
		{
			name:        "invalid: password contains username prefix",
			password:    "johnSecure123",
			username:    "john",
			wantErr:     true,
			errContains: "密码不能包含用户名",
		},
		{
			name:        "invalid: password contains username suffix",
			password:    "Securejohn123",
			username:    "john",
			wantErr:     true,
			errContains: "密码不能包含用户名",
		},
		{
			name:        "invalid: password contains username case insensitive",
			password:    "JOHN123456",
			username:    "john",
			wantErr:     true,
			errContains: "密码不能包含用户名",
		},
		{
			name:        "invalid: password contains username, username uppercase",
			password:    "john123456",
			username:    "JOHN",
			wantErr:     true,
			errContains: "密码不能包含用户名",
		},
		{
			name:        "invalid: password contains username mixed case",
			password:    "JoHn123456",
			username:    "john",
			wantErr:     true,
			errContains: "密码不能包含用户名",
		},
		{
			name:        "invalid: weak password also contains username",
			password:    "admin123456",
			username:    "admin",
			wantErr:     true,
			errContains: "密码过于简单",
		},
		{
			name:        "invalid: password fails basic validation first",
			password:    "short",
			username:    "john",
			wantErr:     true,
			errContains: "密码长度至少为8个字符",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordForUser(tt.password, tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePasswordForUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidatePasswordForUser() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

// TestNormalizeFeedbackRating tests rating normalization
func TestNormalizeFeedbackRating(t *testing.T) {
	tests := []struct {
		name   string
		rating int
		want   int
	}{
		{
			name:   "normal rating",
			rating: 3,
			want:   3,
		},
		{
			name:   "minimum rating",
			rating: 0,
			want:   0,
		},
		{
			name:   "maximum rating",
			rating: 5,
			want:   5,
		},
		{
			name:   "negative rating",
			rating: -1,
			want:   0,
		},
		{
			name:   "very negative rating",
			rating: -100,
			want:   0,
		},
		{
			name:   "above maximum rating",
			rating: 6,
			want:   5,
		},
		{
			name:   "very high rating",
			rating: 100,
			want:   5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeFeedbackRating(tt.rating); got != tt.want {
				t.Errorf("NormalizeFeedbackRating() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildFeedback tests building feedback DTO
func TestBuildFeedback(t *testing.T) {
	fb := &model.Feedback{
		PlayerID: "player1",
		Contact:  "test@example.com",
		Content:  "Great game!",
		Category: "bug",
		Priority: "high",
		Status:   "open",
		Rating:   5,
		Attach:   "screenshot.png",
		GameID:   "game1",
		Env:      "prod",
		Reply:    "Thanks!",
	}
	// Set the embedded model fields
	fb.ID = 123

	result := BuildFeedback(fb)

	if result.Id != 123 {
		t.Errorf("BuildFeedback() Id = %v, want %v", result.Id, 123)
	}
	if result.PlayerId != "player1" {
		t.Errorf("BuildFeedback() PlayerId = %v, want %v", result.PlayerId, "player1")
	}
	if result.Content != "Great game!" {
		t.Errorf("BuildFeedback() Content = %v, want %v", result.Content, "Great game!")
	}
	if result.Rating != 5 {
		t.Errorf("BuildFeedback() Rating = %v, want %v", result.Rating, 5)
	}
}

// TestBuildFeedback_Nil tests nil feedback
func TestBuildFeedback_Nil(t *testing.T) {
	result := BuildFeedback(nil)
	if result.Id != 0 {
		t.Errorf("BuildFeedback() Id = %v, want %v", result.Id, 0)
	}
	if result.PlayerId != "" {
		t.Errorf("BuildFeedback() PlayerId = %v, want empty", result.PlayerId)
	}
}

// TestValidateDomain tests domain validation
func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		want    string
		wantErr bool
	}{
		{
			name:    "valid domain",
			domain:  "example.com",
			want:    "example.com",
			wantErr: false,
		},
		{
			name:    "domain with spaces",
			domain:  "  example.com  ",
			want:    "example.com",
			wantErr: false,
		},
		{
			name:    "empty domain",
			domain:  "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "whitespace only domain",
			domain:  "   ",
			want:    "",
			wantErr: true,
		},
		{
			name:    "domain with subdomain",
			domain:  "api.example.com",
			want:    "api.example.com",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateDomain(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDomain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFormatIssuer tests formatting issuer
func TestFormatIssuer(t *testing.T) {
	tests := []struct {
		name string
		cert *x509.Certificate
		want string
	}{
		{
			name: "nil cert",
			cert: nil,
			want: "",
		},
		{
			name: "cert with CommonName",
			cert: &x509.Certificate{
				Issuer: pkix.Name{
					CommonName: "Example CA",
				},
			},
			want: "Example CA",
		},
		{
			name: "cert with Organization",
			cert: &x509.Certificate{
				Issuer: pkix.Name{
					Organization: []string{"Example Org"},
				},
			},
			want: "Example Org",
		},
		{
			name: "cert with both CommonName and Organization",
			cert: &x509.Certificate{
				Issuer: pkix.Name{
					CommonName:   "Example CA",
					Organization: []string{"Example Org"},
				},
			},
			want: "Example CA",
		},
		{
			name: "cert with no CommonName or Organization",
			cert: &x509.Certificate{
				Issuer: pkix.Name{
					Country: []string{"US"},
				},
			},
			want: "C=US",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatIssuer(tt.cert); got != tt.want {
				t.Errorf("FormatIssuer() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseCertificatePEM tests parsing certificate PEM
func TestParseCertificatePEM(t *testing.T) {
	// Generate a test certificate for testing
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour * 24),
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	tests := []struct {
		name    string
		pemData string
		wantErr bool
	}{
		{
			name:    "valid PEM certificate",
			pemData: string(certPEM),
			wantErr: false,
		},
		{
			name:    "empty PEM",
			pemData: "",
			wantErr: true,
		},
		{
			name:    "invalid PEM",
			pemData: "not a valid PEM",
			wantErr: true,
		},
		{
			name:    "PEM with wrong type",
			pemData: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("data")})),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCertificatePEM(tt.pemData)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCertificatePEM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("ParseCertificatePEM() returned nil certificate")
			}
		})
	}
}

// TestBuildCertificateDTO tests building certificate DTO
func TestBuildCertificateDTO(t *testing.T) {
	now := time.Now()
	cert := &model.Certificate{
		Domain:        "example.com",
		Issuer:        "Example CA",
		ExpiresAt:     now,
		Status:        "valid",
		LastCheckedAt: &now,
		ErrorMessage:  "",
	}
	cert.ID = 1

	result := BuildCertificateDTO(cert)

	if result["id"] != uint(1) {
		t.Errorf("BuildCertificateDTO() id = %v, want %v", result["id"], 1)
	}
	if result["domain"] != "example.com" {
		t.Errorf("BuildCertificateDTO() domain = %v, want %v", result["domain"], "example.com")
	}
	if result["status"] != "valid" {
		t.Errorf("BuildCertificateDTO() status = %v, want %v", result["status"], "valid")
	}
}

// TestUpdateCertificateStatus tests updating certificate status
func TestUpdateCertificateStatus(t *testing.T) {
	tests := []struct {
		name             string
		expiresAt        time.Time
		errorMessage     string
		wantStatus       string
		wantErrorMessage string
	}{
		{
			name:             "active certificate",
			expiresAt:        time.Now().Add(time.Hour * 24 * 35),
			errorMessage:     "",
			wantStatus:       "active",
			wantErrorMessage: "",
		},
		{
			name:             "expiring certificate",
			expiresAt:        time.Now().Add(time.Hour * 24 * 15),
			errorMessage:     "",
			wantStatus:       "expiring",
			wantErrorMessage: "",
		},
		{
			name:             "expired certificate with no error",
			expiresAt:        time.Now().Add(-time.Hour * 24),
			errorMessage:     "",
			wantStatus:       "expired",
			wantErrorMessage: "证书已过期",
		},
		{
			name:             "expired certificate with existing error",
			expiresAt:        time.Now().Add(-time.Hour * 24),
			errorMessage:     "existing error",
			wantStatus:       "expired",
			wantErrorMessage: "existing error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := &model.Certificate{
				ExpiresAt:    tt.expiresAt,
				ErrorMessage: tt.errorMessage,
			}
			UpdateCertificateStatus(cert)
			if cert.Status != tt.wantStatus {
				t.Errorf("UpdateCertificateStatus() status = %v, want %v", cert.Status, tt.wantStatus)
			}
			if cert.ErrorMessage != tt.wantErrorMessage {
				t.Errorf("UpdateCertificateStatus() errorMessage = %v, want %v", cert.ErrorMessage, tt.wantErrorMessage)
			}
			if cert.LastCheckedAt == nil {
				t.Error("UpdateCertificateStatus() LastCheckedAt should be set")
			}
		})
	}
}

// TestBuildBackupDTO tests building backup DTO
func TestBuildBackupDTO(t *testing.T) {
	backup := &model.Backup{
		BackupID: "bkp123",
		Name:     "Daily Backup",
		Size:     1024000,
		Type:     "full",
		Status:   "completed",
		Location: "/backups/daily.tar.gz",
	}
	backup.ID = 1

	result := BuildBackupDTO(backup)

	if result.Id != "bkp123" {
		t.Errorf("BuildBackupDTO() Id = %v, want %v", result.Id, "bkp123")
	}
	if result.Name != "Daily Backup" {
		t.Errorf("BuildBackupDTO() Name = %v, want %v", result.Name, "Daily Backup")
	}
	if result.Size != 1024000 {
		t.Errorf("BuildBackupDTO() Size = %v, want %v", result.Size, 1024000)
	}
	if result.Type != "full" {
		t.Errorf("BuildBackupDTO() Type = %v, want %v", result.Type, "full")
	}
	if result.Status != "completed" {
		t.Errorf("BuildBackupDTO() Status = %v, want %v", result.Status, "completed")
	}
}

// TestBuildBackupDTO_Nil tests nil backup
func TestBuildBackupDTO_Nil(t *testing.T) {
	result := BuildBackupDTO(nil)
	if result.Id != "" {
		t.Errorf("BuildBackupDTO() Id = %v, want empty", result.Id)
	}
}

// TestBuildBackupDTO_EmptyBackupID tests backup with empty BackupID
func TestBuildBackupDTO_EmptyBackupID(t *testing.T) {
	backup := &model.Backup{
		BackupID: "",
		Name:     "Test Backup",
	}
	backup.ID = 123
	result := BuildBackupDTO(backup)
	if result.Id != "123" {
		t.Errorf("BuildBackupDTO() Id = %v, want %v", result.Id, "123")
	}
}

// TestBuildBackupDTO_WhitespaceBackupID tests backup with whitespace BackupID
func TestBuildBackupDTO_WhitespaceBackupID(t *testing.T) {
	backup := &model.Backup{
		BackupID: "  ",
		Name:     "Test Backup",
	}
	backup.ID = 456
	result := BuildBackupDTO(backup)
	if result.Id != "456" {
		t.Errorf("BuildBackupDTO() Id = %v, want %v", result.Id, "456")
	}
}

// TestBuildBackupList tests building backup list
func TestBuildBackupList(t *testing.T) {
	backups := []model.Backup{
		{
			BackupID: "bkp1",
			Name:     "Backup 1",
			Size:     1000,
			Type:     "full",
			Status:   "completed",
		},
		{
			BackupID: "bkp2",
			Name:     "Backup 2",
			Size:     2000,
			Type:     "incremental",
			Status:   "completed",
		},
	}
	backups[0].ID = 1
	backups[1].ID = 2

	result := BuildBackupList(backups)

	if len(result) != 2 {
		t.Fatalf("BuildBackupList() returned %d items, want 2", len(result))
	}
	if result[0].Id != "bkp1" {
		t.Errorf("BuildBackupList()[0].Id = %v, want %v", result[0].Id, "bkp1")
	}
	if result[1].Id != "bkp2" {
		t.Errorf("BuildBackupList()[1].Id = %v, want %v", result[1].Id, "bkp2")
	}
}

// TestBuildBackupList_Empty tests empty backup list
func TestBuildBackupList_Empty(t *testing.T) {
	result := BuildBackupList([]model.Backup{})
	if len(result) != 0 {
		t.Errorf("BuildBackupList() returned %d items, want 0", len(result))
	}
}

// TestGenerateBackupID tests generating backup ID
func TestGenerateBackupID(t *testing.T) {
	id1 := GenerateBackupID()
	id2 := GenerateBackupID()

	// IDs should be different (UUID is unique)
	if id1 == id2 {
		t.Error("GenerateBackupID() should generate unique IDs")
	}

	// ID should start with "bkp_"
	if !strings.HasPrefix(id1, "bkp_") {
		t.Errorf("GenerateBackupID() = %v, should start with 'bkp_'", id1)
	}

	// ID should be 36 characters (bkp_ + 32 char hex UUID without dashes)
	if len(id1) != 36 {
		t.Errorf("GenerateBackupID() length = %d, want 36", len(id1))
	}
}

// TestGuessBackupFilename tests guessing backup filename
func TestGuessBackupFilename(t *testing.T) {
	// Helper function to create a backup with ID set
	makeBackup := func(id uint, backupID, name, location string) *model.Backup {
		b := &model.Backup{
			BackupID: backupID,
			Name:     name,
			Location: location,
		}
		b.ID = id
		return b
	}

	tests := []struct {
		name   string
		backup *model.Backup
		want   string
	}{
		{
			name:   "backup with name",
			backup: makeBackup(1, "bkp123", "my-backup.tar.gz", "/other/path/file.tar.gz"),
			want:   "my-backup.tar.gz",
		},
		{
			name:   "backup with whitespace name",
			backup: makeBackup(1, "bkp123", "  ", "/path/to/backup.tar.gz"),
			want:   "backup.tar.gz",
		},
		{
			name:   "backup with location",
			backup: makeBackup(1, "bkp123", "", "/backups/daily/backup-2023.tar.gz"),
			want:   "backup-2023.tar.gz",
		},
		{
			name:   "backup with BackupID",
			backup: makeBackup(1, "bkp_abc123", "", ""),
			want:   "bkp_abc123.bak",
		},
		{
			name:   "backup with only ID",
			backup: makeBackup(42, "", "", ""),
			want:   "backup-42.bak",
		},
		{
			name:   "nil backup",
			backup: nil,
			want:   "",
		},
		{
			name:   "location with only directory",
			backup: makeBackup(1, "bkp123", "", "/backups/"),
			want:   "backups",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuessBackupFilename(tt.backup); got != tt.want {
				t.Errorf("GuessBackupFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestResolveAnalyticsFiltersPath tests resolving analytics filters path
func TestResolveAnalyticsFiltersPath(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.Config
		contains string
	}{
		{
			name: "explicit filters path",
			cfg: config.Config{
				Registry: config.RegistryConfig{
					AnalyticsFiltersPath: "/custom/path/filters.json",
				},
			},
			contains: "filters.json",
		},
		{
			name: "use schemas dir",
			cfg: config.Config{
				Schemas: config.SchemasConfig{
					Dir: "/schemas",
				},
			},
			contains: "analytics_filters.json",
		},
		{
			name:     "default path",
			cfg:      config.Config{},
			contains: "analytics_filters.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveAnalyticsFiltersPath(tt.cfg)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("ResolveAnalyticsFiltersPath() = %v, should contain %v", result, tt.contains)
			}
		})
	}
}

// TestReadAnalyticsFiltersFile tests reading analytics filters file
func TestReadAnalyticsFiltersFile(t *testing.T) {
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "filters-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := []byte(`{"filters": ["test"]}`)
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	tests := []struct {
		name    string
		path    string
		want    []byte
		wantErr bool
	}{
		{
			name:    "valid file",
			path:    tmpfile.Name(),
			want:    content,
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			want:    []byte{},
			wantErr: false,
		},
		{
			name:    "whitespace path",
			path:    "   ",
			want:    []byte{},
			wantErr: false,
		},
		{
			name:    "non-existent file",
			path:    "/nonexistent/file.json",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadAnalyticsFiltersFile(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadAnalyticsFiltersFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil && string(got) != string(tt.want) {
				t.Errorf("ReadAnalyticsFiltersFile() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}

// TestWriteAnalyticsFiltersFile tests writing analytics filters file
func TestWriteAnalyticsFiltersFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "write to valid path",
			path:    filepath.Join(tmpDir, "filters.json"),
			data:    []byte(`{"filters": []}`),
			wantErr: false,
		},
		{
			name:    "write to nested path",
			path:    filepath.Join(tmpDir, "subdir", "filters.json"),
			data:    []byte(`{"filters": []}`),
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			data:    []byte(`{}`),
			wantErr: false,
		},
		{
			name:    "whitespace path",
			path:    "   ",
			data:    []byte(`{}`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WriteAnalyticsFiltersFile(tt.path, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteAnalyticsFiltersFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.path != "" && strings.TrimSpace(tt.path) != "" {
				// Verify file was written
				content, err := os.ReadFile(tt.path)
				if err != nil {
					t.Errorf("Failed to read written file: %v", err)
				}
				if string(content) != string(tt.data) {
					t.Errorf("File content = %v, want %v", string(content), string(tt.data))
				}
			}
		})
	}
}
