package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFunctionProviderTypeFromString(t *testing.T) {
	tests := []struct {
		input string
		want  FunctionProviderType
	}{
		{"openapi", FunctionProviderTypeOpenAPI},
		{"OpenAPI", FunctionProviderTypeOpenAPI},
		{"proto", FunctionProviderTypeProto},
		{"Proto", FunctionProviderTypeProto},
		{"pack", FunctionProviderTypePack},
		{"Pack", FunctionProviderTypePack},
		{"unknown", FunctionProviderTypeUnknown},
		{"", FunctionProviderTypeUnknown},
		{"random", FunctionProviderTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := FunctionProviderTypeFromString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFunctionProviderType_String(t *testing.T) {
	tests := []struct {
		providerType FunctionProviderType
		want         string
	}{
		{FunctionProviderTypeOpenAPI, "openapi"},
		{FunctionProviderTypeProto, "proto"},
		{FunctionProviderTypePack, "pack"},
		{FunctionProviderTypeUnknown, "unknown"},
		{FunctionProviderType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.providerType.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLooksLikeJSON(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		want  bool
	}{
		{"empty", []byte{}, false},
		{"whitespace only", []byte("   "), false},
		{"json object", []byte(`{"key": "value"}`), true},
		{"json array", []byte(`[1, 2, 3]`), true},
		{"json object with whitespace", []byte(`  { "key": "value" }`), true},
		{"json array with whitespace", []byte(`  [1, 2, 3]`), true},
		{"yaml", []byte("key: value"), false},
		{"plain text", []byte("hello"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeJSON(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestYamlToJSON(t *testing.T) {
	tests := []struct {
		name     string
		yamlData []byte
		wantErr  bool
	}{
		{
			name:     "simple yaml",
			yamlData: []byte("key: value"),
			wantErr:  false,
		},
		{
			name:     "nested yaml",
			yamlData: []byte("parent:\n  child: value"),
			wantErr:  false,
		},
		{
			name:     "invalid yaml",
			yamlData: []byte("invalid: [unclosed"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := yamlToJSON(tt.yamlData)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}
