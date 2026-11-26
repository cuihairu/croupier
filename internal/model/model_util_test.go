package model

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"testing"
	"time"
)

// Split from model_test.go; test bodies moved verbatim.

func TestPaginationOptions_Normalize(t *testing.T) {
	tests := []struct {
		name         string
		opts         PaginationOptions
		expectedPage int
		expectedSize int
	}{
		{"default values", PaginationOptions{}, 1, 20},
		{"negative page", PaginationOptions{Page: -1, PageSize: 10}, 1, 10},
		{"zero page", PaginationOptions{Page: 0, PageSize: 10}, 1, 10},
		{"negative size", PaginationOptions{Page: 1, PageSize: -1}, 1, 20},
		{"zero size", PaginationOptions{Page: 1, PageSize: 0}, 1, 20},
		{"large size", PaginationOptions{Page: 1, PageSize: 200}, 1, 100},
		{"valid values", PaginationOptions{Page: 2, PageSize: 50}, 2, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.Normalize()
			assert.Equal(t, tt.expectedPage, opts.Page)
			assert.Equal(t, tt.expectedSize, opts.PageSize)
		})
	}
}

func TestPaginationOptions_Offset(t *testing.T) {
	tests := []struct {
		name     string
		opts     PaginationOptions
		expected int
	}{
		{"page 1", PaginationOptions{Page: 1, PageSize: 10}, 0},
		{"page 2", PaginationOptions{Page: 2, PageSize: 10}, 10},
		{"page 3", PaginationOptions{Page: 3, PageSize: 20}, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.opts.Offset())
		})
	}
}

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		status int
		valid  bool
	}{
		{0, true},
		{1, true},
		{2, true},
		{-1, false},
		{3, false},
		{100, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.valid, IsValidStatus(tt.status))
		})
	}
}

func TestIsValidGameStatus(t *testing.T) {
	tests := []struct {
		status string
		valid  bool
	}{
		{GameStatusDev, true},
		{GameStatusTest, true},
		{GameStatusRunning, true},
		{GameStatusOnline, true},
		{GameStatusOffline, true},
		{GameStatusMaintenance, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.valid, IsValidGameStatus(tt.status))
		})
	}
}

func TestNowUTC(t *testing.T) {
	now := NowUTC()
	assert.False(t, now.IsZero())
	assert.WithinDuration(t, time.Now().UTC(), now, time.Second)
}

// ===== AlertModel Tests =====

func TestEncodeData(t *testing.T) {
	// Test encoding nil
	data, err := EncodeData(nil)
	require.NoError(t, err)
	assert.Equal(t, JSON([]byte("null")), data)

	// Test encoding map
	input := map[string]interface{}{"key": "value", "number": 123}
	data, err = EncodeData(input)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// ===== NodeModel Tests =====

func TestNormalizeJSONMapUpdate(t *testing.T) {
	tests := []struct {
		name     string
		updates  map[string]interface{}
		key      string
		wantType string
	}{
		{
			name:     "nil updates",
			updates:  nil,
			key:      "test",
			wantType: "nil",
		},
		{
			name:     "missing key",
			updates:  map[string]interface{}{"other": "value"},
			key:      "test",
			wantType: "missing",
		},
		{
			name:     "nil value",
			updates:  map[string]interface{}{"test": nil},
			key:      "test",
			wantType: "nil",
		},
		{
			name:     "JSONMap type",
			updates:  map[string]interface{}{"test": datatypes.JSONMap{"key": "value"}},
			key:      "test",
			wantType: "jsonmap",
		},
		{
			name:     "map type",
			updates:  map[string]interface{}{"test": map[string]interface{}{"key": "value"}},
			key:      "test",
			wantType: "jsonmap",
		},
		{
			name:     "byte slice",
			updates:  map[string]interface{}{"test": []byte(`{"key":"value"}`)},
			key:      "test",
			wantType: "jsonmap",
		},
		{
			name:     "string",
			updates:  map[string]interface{}{"test": `{"key":"value"}`},
			key:      "test",
			wantType: "jsonmap",
		},
		{
			name:     "invalid JSON bytes",
			updates:  map[string]interface{}{"test": []byte(`{invalid}`)},
			key:      "test",
			wantType: "bytes_invalid_orig",
		},
		{
			name:     "invalid JSON string",
			updates:  map[string]interface{}{"test": `{invalid}`},
			key:      "test",
			wantType: "string_invalid_orig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizeJSONMapUpdate(tt.updates, tt.key)
			if tt.updates == nil || tt.wantType == "nil" || tt.wantType == "missing" {
				return
			}
			val := tt.updates[tt.key]
			switch tt.wantType {
			case "jsonmap":
				if _, ok := val.(datatypes.JSONMap); !ok {
					t.Errorf("expected datatypes.JSONMap, got %T", val)
				}
			case "bytes_invalid_orig", "string_invalid_orig":
				// Original value is preserved when JSON unmarshal fails
			}
		})
	}
}

func TestMustJSON(t *testing.T) {
	// nil
	result := MustJSON(nil)
	assert.Equal(t, "null", string(result))

	// struct
	type TestStruct struct {
		Name string `json:"name"`
	}
	result = MustJSON(TestStruct{Name: "test"})
	assert.Contains(t, string(result), "test")

	// map
	result = MustJSON(map[string]string{"key": "value"})
	assert.Contains(t, string(result), "value")

	// slice
	result = MustJSON([]int{1, 2, 3})
	assert.Contains(t, string(result), "1")
}

// TestDeploymentModelCRUD 覆盖 OPS-004 部署包/发版单模型 CRUD 与表名。
