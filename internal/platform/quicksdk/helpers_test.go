package quicksdk

import (
	"testing"
	"time"
)

func TestInt64Value(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want int64
	}{
		{"int", int(42), 42},
		{"int8", int8(7), 7},
		{"int16", int16(1000), 1000},
		{"int32", int32(100000), 100000},
		{"int64", int64(999), 999},
		{"uint", uint(42), 42},
		{"uint8", uint8(255), 255},
		{"uint16", uint16(65535), 65535},
		{"uint32", uint32(100000), 100000},
		{"uint64", uint64(123), 123},
		{"float64", float64(3.14), 3},
		{"string_valid", "123", 123},
		{"string_invalid", "abc", 0},
		{"string_empty", "", 0},
		{"nil", nil, 0},
		{"bool", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Int64Value(tt.in)
			if got != tt.want {
				t.Errorf("Int64Value(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	tests := []struct {
		name string
		in   interface{}
		want time.Time
	}{
		{"int64", now.Unix(), now},
		{"float64", float64(now.Unix()), now},
		{"string", "1700000000", time.Unix(1700000000, 0)},
		{"string_invalid", "abc", time.Unix(0, 0)},
		{"nil", nil, time.Unix(0, 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTimestamp(tt.in)
			if !got.Equal(tt.want) {
				t.Errorf("ParseTimestamp(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	m := map[string]interface{}{
		"str":   "hello",
		"num":   42,
		"float": 3.14,
		"bool":  true,
	}

	tests := []struct {
		key  string
		want string
	}{
		{"str", "hello"},
		{"num", "42"},
		{"float", "3.14"},
		{"bool", "true"},
		{"missing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := getString(m, tt.key)
			if got != tt.want {
				t.Errorf("getString(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	m := map[string]interface{}{
		"int":      42,
		"float64":  float64(3.9),
		"str":      "100",
		"str_bad":  "abc",
		"bool":     true,
		"nil":      nil,
	}

	tests := []struct {
		key  string
		want int
	}{
		{"int", 42},
		{"float64", 3},
		{"str", 100},
		{"str_bad", 0},
		{"missing", 0},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := getInt(m, tt.key)
			if got != tt.want {
				t.Errorf("getInt(%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

func TestGetInt64(t *testing.T) {
	m := map[string]interface{}{
		"int":     42,
		"int64":   int64(999),
		"float64": float64(3.9),
		"str":     "100",
		"str_bad": "abc",
	}

	tests := []struct {
		key  string
		want int64
	}{
		{"int", 42},
		{"int64", 999},
		{"float64", 3},
		{"str", 100},
		{"str_bad", 0},
		{"missing", 0},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := getInt64(m, tt.key)
			if got != tt.want {
				t.Errorf("getInt64(%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	m := map[string]interface{}{
		"bool_true":  true,
		"bool_false": false,
		"str_1":      "1",
		"str_true":   "true",
		"str_false":  "false",
		"str_0":      "0",
		"float_zero": float64(0),
		"float_one":  float64(1),
		"nil":        nil,
	}

	tests := []struct {
		key  string
		want bool
	}{
		{"bool_true", true},
		{"bool_false", false},
		{"str_1", true},
		{"str_true", true},
		{"str_false", false},
		{"str_0", false},
		{"float_zero", false},
		{"float_one", true},
		{"missing", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := getBool(m, tt.key)
			if got != tt.want {
				t.Errorf("getBool(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
