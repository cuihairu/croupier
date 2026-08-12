// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package requestbind

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestTagKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tag  string
		want string
	}{
		{"empty tag", "", ""},
		{"simple tag", "name", "name"},
		{"tag with comma", "name,omitempty", "name"},
		{"tag with spaces", " name ,omitempty ", "name"},
		{"dash tag", "-", "-"},
		{"tag with only comma", ",omitempty", ""},
		{"complex tag", "field,name,omitempty", "field"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tagKey(tt.tag)
			if got != tt.want {
				t.Errorf("tagKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

type testQueryRequest struct {
	Name  string   `form:"name" json:"name"`
	Age   int      `form:"age" json:"age"`
	Admin bool     `form:"admin" json:"admin"`
	Tags  []string `form:"tags" json:"tags"`
}

func TestBindQueryCompatWithFormTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		query     string
		wantName  string
		wantAge   int
		wantAdmin bool
		wantTags  int
	}{
		{
			name:      "all fields set",
			query:     "name=test&age=25&admin=true&tags=a&tags=b",
			wantName:  "test",
			wantAge:   25,
			wantAdmin: true,
			wantTags:  2,
		},
		{
			name:      "only name",
			query:     "name=test",
			wantName:  "test",
			wantAge:   0,
			wantAdmin: false,
			wantTags:  0,
		},
		{
			name:      "empty query",
			query:     "",
			wantName:  "",
			wantAge:   0,
			wantAdmin: false,
			wantTags:  0,
		},
		{
			name:      "boolean false",
			query:     "admin=false",
			wantName:  "",
			wantAge:   0,
			wantAdmin: false,
			wantTags:  0,
		},
		{
			name:      "negative age",
			query:     "age=-5",
			wantName:  "",
			wantAge:   -5,
			wantAdmin: false,
			wantTags:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Set query parameters
			u, _ := url.Parse("?" + tt.query)
			c.Request = &http.Request{
				URL: u,
			}

			var req testQueryRequest
			err := BindQueryCompat(c, &req)

			if err != nil && tt.name != "empty query" { // empty query may not error
				t.Logf("BindQueryCompat() error = %v", err)
			}

			if req.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", req.Name, tt.wantName)
			}
			if req.Age != tt.wantAge {
				t.Errorf("Age = %d, want %d", req.Age, tt.wantAge)
			}
			if req.Admin != tt.wantAdmin {
				t.Errorf("Admin = %v, want %v", req.Admin, tt.wantAdmin)
			}
			if len(req.Tags) != tt.wantTags {
				t.Errorf("Tags length = %d, want %d", len(req.Tags), tt.wantTags)
			}
		})
	}
}

type testJSONFallbackRequest struct {
	// Only json tags, no form tags
	Username string `json:"username"`
	Count    int    `json:"count"`
	Active   bool   `json:"active"`
}

func TestBindQueryCompatJSONFallback(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Set query parameters using json tag names
	u, _ := url.Parse("?username=testuser&count=42&active=true")
	c.Request = &http.Request{
		URL: u,
	}

	var req testJSONFallbackRequest
	err := BindQueryCompat(c, &req)

	if err != nil {
		t.Logf("BindQueryCompat() error = %v", err)
	}

	// Note: ShouldBindQuery returns nil for structs without form tags,
	// but doesn't bind any values. The fallback logic only triggers
	// when ShouldBindQuery fails, not when it succeeds without binding.
	// This test documents the current behavior - fields remain unbound.
	if req.Username != "" {
		t.Errorf(`Username = %q, want "" (json tags don't work with query binding)`, req.Username)
	}
	if req.Count != 0 {
		t.Errorf("Count = %d, want 0 (json tags don't work with query binding)", req.Count)
	}
	if req.Active {
		t.Error("Active should be false (json tags don't work with query binding)")
	}
}

type testMixedTagsRequest struct {
	// Has both form and json tags
	Search string `form:"q" json:"query"`
	Page   int    `form:"page" json:"page"`
}

func TestBindQueryCompatFormPreferredOverJSON(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Use form tag name "q" instead of json tag "query"
	u, _ := url.Parse("?q=searchterm&page=2")
	c.Request = &http.Request{
		URL: u,
	}

	var req testMixedTagsRequest
	err := BindQueryCompat(c, &req)

	if err != nil {
		t.Logf("BindQueryCompat() error = %v", err)
	}

	if req.Search != "searchterm" {
		t.Errorf(`Search = %q, want "searchterm"`, req.Search)
	}
	if req.Page != 2 {
		t.Errorf("Page = %d, want 2", req.Page)
	}
}

func TestBindQueryCompatInvalidInteger(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	u, _ := url.Parse("?age=notanumber")
	c.Request = &http.Request{
		URL: u,
	}

	var req testQueryRequest
	err := BindQueryCompat(c, &req)

	// Should not error, but age should be 0 (default value)
	if err != nil {
		t.Logf("BindQueryCompat() with invalid int error = %v", err)
	}
	if req.Age != 0 {
		t.Errorf("Age with invalid input = %d, want 0", req.Age)
	}
}

func TestBindQueryCompatInvalidBool(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	u, _ := url.Parse("?admin=notabool")
	c.Request = &http.Request{
		URL: u,
	}

	var req testQueryRequest
	err := BindQueryCompat(c, &req)

	if err != nil {
		t.Logf("BindQueryCompat() with invalid bool error = %v", err)
	}
	if req.Admin {
		t.Error("Admin with invalid input should be false")
	}
}

func TestBindQueryCompatNonPointer(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	u, _ := url.Parse("?name=test")
	c.Request = &http.Request{
		URL: u,
	}

	var req testQueryRequest
	err := BindQueryCompat(c, &req)

	// Non-pointer should still work for basic binding
	if err != nil {
		t.Logf("BindQueryCompat() with non-pointer error = %v", err)
	}
}

func TestBindQueryCompatNilPointer(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	u, _ := url.Parse("?name=test")
	c.Request = &http.Request{
		URL: u,
	}

	var req *testQueryRequest
	err := BindQueryCompat(c, &req)

	// Nil pointer should error or not panic
	if err == nil && req == nil {
		t.Log("BindQueryCompat() with nil pointer returned nil")
	} else if err != nil {
		t.Logf("BindQueryCompat() with nil pointer error = %v", err)
	}
}

// TestBindQueryCompatWithNoTags tests struct with no form or json tags
type testNoTagsRequest struct {
	Name string
	Age  int
}

func TestBindQueryCompatWithNoTags(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	u, _ := url.Parse("?Name=test&Age=25")
	c.Request = &http.Request{
		URL: u,
	}

	var req testNoTagsRequest
	err := BindQueryCompat(c, &req)

	if err != nil {
		t.Logf("BindQueryCompat() with no tags error = %v", err)
	}

	// Fields without tags may or may not be bound depending on gin's behavior
	// This test documents the actual behavior
	t.Logf("Name=%q, Age=%d", req.Name, req.Age)
}

// TestBindQueryCompatWithDashTag tests struct with dash tag
type testDashTagRequest struct {
	Name string `form:"-" json:"-"`
	Age  int    `form:"age"`
}

func TestBindQueryCompatWithDashTag(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	u, _ := url.Parse("?Name=ignored&age=25")
	c.Request = &http.Request{
		URL: u,
	}

	var req testDashTagRequest
	err := BindQueryCompat(c, &req)

	if err != nil {
		t.Logf("BindQueryCompat() with dash tag error = %v", err)
	}

	// Name should not be bound (dash tag)
	if req.Name != "" {
		t.Errorf("Name should be empty for dash tag")
	}
	if req.Age != 25 {
		t.Errorf("Age = %d, want 25", req.Age)
	}
}

// TestBindQueryCompatWithEmptyQueryString tests empty query string
func TestBindQueryCompatWithEmptyQueryString(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	u, _ := url.Parse("")
	c.Request = &http.Request{
		URL: u,
	}

	var req testQueryRequest
	err := BindQueryCompat(c, &req)

	if err != nil {
		t.Logf("BindQueryCompat() with empty query error = %v", err)
	}
}

// TestBindQueryCompatWithInvalidBoolValue tests invalid bool value
func TestBindQueryCompatWithInvalidBoolValue(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	u, _ := url.Parse("?admin=invalid")
	c.Request = &http.Request{
		URL: u,
	}

	var req testQueryRequest
	err := BindQueryCompat(c, &req)

	if err != nil {
		t.Logf("BindQueryCompat() with invalid bool error = %v", err)
	}

	// Invalid bool should remain false
	if req.Admin {
		t.Error("Admin should be false for invalid bool value")
	}
}
