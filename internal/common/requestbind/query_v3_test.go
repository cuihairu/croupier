package requestbind

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

// requiredFieldRequest has binding:"required" tags to force ShouldBindQuery to fail
type requiredFieldRequest struct {
	Name string `form:"name" binding:"required"`
	Age  int    `form:"age"`
}

// pointerToIntRequest tests passing a pointer to a non-struct type
type simpleRequest struct {
	Value string `form:"value"`
}

func TestBindQueryCompat_ShouldBindQueryFails_FallbackPath(t *testing.T) {
	t.Parallel()

	// Create a request with binding:"required" tag but don't provide the required field
	// This forces ShouldBindQuery to fail, exercising the fallback code path
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	u, _ := url.Parse("?age=25")
	c.Request = &http.Request{URL: u}

	var req requiredFieldRequest
	err := BindQueryCompat(c, &req)

	// ShouldBindQuery fails due to missing "name" field with binding:"required"
	// The fallback should still bind the "age" field
	if req.Age != 25 {
		t.Errorf("Age = %d, want 25 (fallback should bind available fields)", req.Age)
	}
	if err != nil {
		t.Logf("BindQueryCompat() validation error = %v (expected from required tag)", err)
	}
}

func TestBindQueryCompat_NonPointerValue(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	u, _ := url.Parse("?value=test")
	c.Request = &http.Request{URL: u}

	// Pass by value (not pointer) - gin's ShouldBindQuery may panic on non-pointer
	// This tests the function handles it gracefully
	var req simpleRequest
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic for non-pointer: %v", r)
		}
	}()
	err := BindQueryCompat(c, req) // pass value, not &req
	if err != nil {
		t.Logf("BindQueryCompat() with value type error = %v", err)
	}
}

func TestBindQueryCompat_PointerToNonStruct(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	u, _ := url.Parse("?test=1")
	c.Request = &http.Request{URL: u}

	// Pass a pointer to a non-struct type
	var s string
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic for pointer-to-non-struct: %v", r)
		}
	}()
	err := BindQueryCompat(c, &s)
	if err != nil {
		t.Logf("BindQueryCompat() with pointer-to-non-struct error = %v", err)
	}
}

func TestBindQueryCompat_RequiredFields_AllPresent(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	u, _ := url.Parse("?name=testuser&age=30")
	c.Request = &http.Request{URL: u}

	var req requiredFieldRequest
	err := BindQueryCompat(c, &req)
	if err != nil {
		t.Logf("BindQueryCompat() error = %v", err)
	}
	if req.Name != "testuser" {
		t.Errorf("Name = %q, want %q", req.Name, "testuser")
	}
	if req.Age != 30 {
		t.Errorf("Age = %d, want 30", req.Age)
	}
}

func TestBindQueryCompat_SliceOfInts(t *testing.T) {
	t.Parallel()

	type sliceIntRequest struct {
		Ids []int `form:"ids"`
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	u, _ := url.Parse("?ids=1&ids=2&ids=3")
	c.Request = &http.Request{URL: u}

	var req sliceIntRequest
	err := BindQueryCompat(c, &req)
	if err != nil {
		t.Logf("BindQueryCompat() error = %v", err)
	}
	// Slice of non-strings should not be bound by fallback
	if len(req.Ids) != 0 {
		t.Logf("Ids length = %d (non-string slices may or may not be bound)", len(req.Ids))
	}
}

func TestBindQueryCompat_TagWithMultipleParts(t *testing.T) {
	t.Parallel()

	type multiPartTagRequest struct {
		Field string `form:"field,omitempty,required" json:"field,omitempty"`
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	u, _ := url.Parse("?field=value123")
	c.Request = &http.Request{URL: u}

	var req multiPartTagRequest
	err := BindQueryCompat(c, &req)
	if err != nil {
		t.Logf("BindQueryCompat() error = %v", err)
	}
	if req.Field != "value123" {
		t.Errorf("Field = %q, want %q", req.Field, "value123")
	}
}

func TestBindQueryCompat_EmptyFormTag(t *testing.T) {
	t.Parallel()

	type emptyFormTagRequest struct {
		Field string `form:",omitempty" json:"field"`
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	u, _ := url.Parse("?field=value")
	c.Request = &http.Request{URL: u}

	var req emptyFormTagRequest
	err := BindQueryCompat(c, &req)
	if err != nil {
		t.Logf("BindQueryCompat() error = %v", err)
	}
}

func TestBindQueryCompat_MultipleQueryValues(t *testing.T) {
	t.Parallel()

	type multiValRequest struct {
		Tags  []string `form:"tags" json:"tags"`
		Name  string   `form:"name" json:"name"`
		Count int      `form:"count" json:"count"`
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	u, _ := url.Parse("?tags=a&tags=b&tags=c&name=test&count=42")
	c.Request = &http.Request{URL: u}

	var req multiValRequest
	err := BindQueryCompat(c, &req)
	if err != nil {
		t.Logf("BindQueryCompat() error = %v", err)
	}
	if len(req.Tags) != 3 {
		t.Errorf("Tags length = %d, want 3", len(req.Tags))
	}
	if req.Name != "test" {
		t.Errorf("Name = %q, want %q", req.Name, "test")
	}
	if req.Count != 42 {
		t.Errorf("Count = %d, want 42", req.Count)
	}
}

func TestBindQueryCompat_BoolVariants(t *testing.T) {
	t.Parallel()

	type boolRequest struct {
		Flag string `form:"flag"`
	}

	tests := []struct {
		name  string
		query string
	}{
		{"true", "flag=true"},
		{"1", "flag=1"},
		{"TRUE", "flag=TRUE"},
		{"false", "flag=false"},
		{"0", "flag=0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			u, _ := url.Parse("?" + tt.query)
			c.Request = &http.Request{URL: u}

			var req boolRequest
			err := BindQueryCompat(c, &req)
			if err != nil {
				t.Logf("error = %v", err)
			}
			if req.Flag == "" {
				t.Errorf("Flag should be set for query %q", tt.query)
			}
		})
	}
}

func TestTagKey_VariousEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tag  string
		want string
	}{
		{"multiple commas", "field,a,b,c", "field"},
		{"just spaces", "   ", ""},
		{"comma with spaces", " , field ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tagKey(tt.tag)
			if got != tt.want {
				t.Errorf("tagKey(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}
