package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/gin-gonic/gin"
)

// setupTestContext creates a gin context for testing
func setupTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

// TestSuccess 测试成功响应
func TestSuccess(t *testing.T) {
	c, w := setupTestContext()
	data := map[string]string{"message": "hello"}

	Success(c, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["message"] != "hello" {
		t.Errorf("Expected message 'hello', got '%s'", response["message"])
	}
}

// TestSuccess_NilData 测试空数据
func TestSuccess_NilData(t *testing.T) {
	c, w := setupTestContext()

	Success(c, nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestSuccess_ComplexData 测试复杂数据
func TestSuccess_ComplexData(t *testing.T) {
	c, w := setupTestContext()
	data := map[string]interface{}{
		"id":       123,
		"name":     "test",
		"active":   true,
		"tags":     []string{"a", "b", "c"},
		"metadata": map[string]string{"key": "value"},
	}

	Success(c, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", response["name"])
	}
}

// TestCreated 测试创建成功响应
func TestCreated(t *testing.T) {
	c, w := setupTestContext()
	data := map[string]string{"id": "123"}

	Created(c, data)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["id"] != "123" {
		t.Errorf("Expected id '123', got '%s'", response["id"])
	}
}

// TestNoContent 测试无内容响应
func TestNoContent(t *testing.T) {
	c, w := setupTestContext()

	NoContent(c)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	if w.Body.Len() != 0 {
		t.Errorf("Expected empty body, got %d bytes", w.Body.Len())
	}
}

// TestError 测试错误响应
func TestError(t *testing.T) {
	c, w := setupTestContext()
	err := errorx.NewBadRequest("invalid input")

	Error(c, err)

	// Verify we got an error response
	if w.Code == 0 && w.Body.Len() == 0 {
		t.Error("Error should write some response")
	}
}

// TestError_NotFound 测试 404 错误
func TestError_NotFound(t *testing.T) {
	c, w := setupTestContext()
	err := errorx.NewNotFound("resource not found")

	Error(c, err)

	// Verify we got an error response
	if w.Code == 0 && w.Body.Len() == 0 {
		t.Error("Error should write some response")
	}
}

// TestError_Validation 测试验证错误
func TestError_Validation(t *testing.T) {
	c, w := setupTestContext()
	details := map[string]string{
		"email": "invalid format",
		"age":   "must be positive",
	}
	err := errorx.NewValidationErrorWithDetails("validation failed", details)

	Error(c, err)

	// Verify we got an error response
	if w.Code == 0 && w.Body.Len() == 0 {
		t.Error("Error should write some response")
	}
}

// TestSuccessList 测试列表响应
func TestSuccessList(t *testing.T) {
	c, w := setupTestContext()
	items := []string{"item1", "item2", "item3"}

	SuccessList(c, items, 100, 1, 10)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response ListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Total != 100 {
		t.Errorf("Expected total 100, got %d", response.Total)
	}

	if response.Page != 1 {
		t.Errorf("Expected page 1, got %d", response.Page)
	}

	if response.Size != 10 {
		t.Errorf("Expected size 10, got %d", response.Size)
	}

	// 验证 items（由于 Items 是 interface{}，需要特殊处理）
	itemsBytes, _ := json.Marshal(response.Items)
	var unmarshaledItems []string
	json.Unmarshal(itemsBytes, &unmarshaledItems)

	if len(unmarshaledItems) != 3 {
		t.Errorf("Expected 3 items, got %d", len(unmarshaledItems))
	}
}

// TestSuccessList_Empty 测试空列表响应
func TestSuccessList_Empty(t *testing.T) {
	c, w := setupTestContext()
	items := []string{}

	SuccessList(c, items, 0, 1, 10)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response ListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Total != 0 {
		t.Errorf("Expected total 0, got %d", response.Total)
	}
}

// TestSuccessList_NilItems 测试 nil items
func TestSuccessList_NilItems(t *testing.T) {
	c, w := setupTestContext()

	SuccessList(c, nil, 0, 1, 10)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestListResponse_JSONFields 测试 ListResponse JSON 字段名
func TestListResponse_JSONFields(t *testing.T) {
	response := ListResponse{
		Items: []string{"a", "b"},
		Total: 100,
		Page:  1,
		Size:  10,
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// 验证 JSON 字段名
	if _, ok := result["items"]; !ok {
		t.Error("Response should have 'items' field")
	}

	if _, ok := result["total"]; !ok {
		t.Error("Response should have 'total' field")
	}

	if _, ok := result["page"]; !ok {
		t.Error("Response should have 'page' field")
	}

	if _, ok := result["pageSize"]; !ok {
		t.Error("Response should have 'pageSize' field")
	}
}

// TestResponseContentType 测试响应内容类型
func TestResponseContentType(t *testing.T) {
	tests := []struct {
		name     string
		response func(c *gin.Context)
	}{
		{"Success", func(c *gin.Context) { Success(c, map[string]string{"test": "data"}) }},
		{"Created", func(c *gin.Context) { Created(c, map[string]string{"id": "123"}) }},
		{"SuccessList", func(c *gin.Context) { SuccessList(c, []string{}, 0, 1, 10) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := setupTestContext()
			tt.response(c)

			contentType := w.Header().Get("Content-Type")
			if contentType == "" {
				t.Error("Content-Type header should be set")
			}
		})
	}
}

// TestMultipleWrites 测试多次写入
func TestMultipleWrites(t *testing.T) {
	c, w := setupTestContext()

	// 第一次写入
	Success(c, map[string]string{"first": "response"})

	// 验证第一次写入成功
	if w.Code != http.StatusOK {
		t.Errorf("First write: expected status %d, got %d", http.StatusOK, w.Code)
	}

	// 尝试第二次写入
	c2, w2 := setupTestContext()
	Success(c2, map[string]string{"second": "response"})

	if w2.Code != http.StatusOK {
		t.Errorf("Second write: expected status %d, got %d", http.StatusOK, w2.Code)
	}
}

// TestSuccessWithLargeData 测试大数据响应
func TestSuccessWithLargeData(t *testing.T) {
	c, w := setupTestContext()

	// 创建大型数据集
	largeData := make([]int, 10000)
	for i := 0; i < 10000; i++ {
		largeData[i] = i
	}

	Success(c, largeData)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.Len() == 0 {
		t.Error("Body should not be empty for large data")
	}
}

// BenchmarkSuccess 性能基准测试
func BenchmarkSuccess(b *testing.B) {
	data := map[string]string{"message": "hello"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, _ := setupTestContext()
		Success(c, data)
	}
}

// BenchmarkSuccessList 性能基准测试
func BenchmarkSuccessList(b *testing.B) {
	items := make([]string, 100)
	for i := 0; i < 100; i++ {
		items[i] = "item"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, _ := setupTestContext()
		SuccessList(c, items, 1000, 1, 100)
	}
}

// BenchmarkError 性能基准测试
func BenchmarkError(b *testing.B) {
	err := errorx.NewBadRequest("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, _ := setupTestContext()
		Error(c, err)
	}
}
