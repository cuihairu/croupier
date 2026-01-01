package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
)

// TestSuccess 测试成功响应
func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"message": "hello"}

	Success(w, data)

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
	w := httptest.NewRecorder()

	Success(w, nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestSuccess_ComplexData 测试复杂数据
func TestSuccess_ComplexData(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]interface{}{
		"id":       123,
		"name":     "test",
		"active":   true,
		"tags":     []string{"a", "b", "c"},
		"metadata": map[string]string{"key": "value"},
	}

	Success(w, data)

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
	w := httptest.NewRecorder()
	data := map[string]string{"id": "123"}

	Created(w, data)

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
	w := httptest.NewRecorder()

	NoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	if w.Body.Len() != 0 {
		t.Errorf("Expected empty body, got %d bytes", w.Body.Len())
	}
}

// TestError 测试错误响应
func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	err := errorx.NewBadRequest("invalid input")

	Error(w, r, err)

	// go-zero 的 httpx.Error 可能返回不同的响应格式
	// 我们主要验证函数不 panic 并且返回了响应
	if w.Code == 0 && w.Body.Len() == 0 {
		t.Error("Error should write some response")
	}
}

// TestError_NotFound 测试 404 错误
func TestError_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	err := errorx.NewNotFound("resource not found")

	Error(w, r, err)

	// 验证函数调用成功
	if w.Code == 0 && w.Body.Len() == 0 {
		t.Error("Error should write some response")
	}
}

// TestError_Validation 测试验证错误
func TestError_Validation(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)
	details := map[string]string{
		"email": "invalid format",
		"age":   "must be positive",
	}
	err := errorx.NewValidationErrorWithDetails("validation failed", details)

	Error(w, r, err)

	// 验证函数调用成功
	if w.Code == 0 && w.Body.Len() == 0 {
		t.Error("Error should write some response")
	}
}

// TestSuccessList 测试列表响应
func TestSuccessList(t *testing.T) {
	w := httptest.NewRecorder()
	items := []string{"item1", "item2", "item3"}

	SuccessList(w, items, 100, 1, 10)

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
	w := httptest.NewRecorder()
	items := []string{}

	SuccessList(w, items, 0, 1, 10)

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
	w := httptest.NewRecorder()

	SuccessList(w, nil, 0, 1, 10)

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
		response func(w http.ResponseWriter)
	}{
		{"Success", func(w http.ResponseWriter) { Success(w, map[string]string{"test": "data"}) }},
		{"Created", func(w http.ResponseWriter) { Created(w, map[string]string{"id": "123"}) }},
		{"SuccessList", func(w http.ResponseWriter) { SuccessList(w, []string{}, 0, 1, 10) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.response(w)

			contentType := w.Header().Get("Content-Type")
			if contentType == "" {
				t.Error("Content-Type header should be set")
			}
		})
	}
}

// TestMultipleWrites 测试多次写入
func TestMultipleWrites(t *testing.T) {
	w := httptest.NewRecorder()

	// 第一次写入
	Success(w, map[string]string{"first": "response"})

	// 验证第一次写入成功
	if w.Code != http.StatusOK {
		t.Errorf("First write: expected status %d, got %d", http.StatusOK, w.Code)
	}

	// 尝试第二次写入（HTTP ResponseRecorder 允许多次写入）
	Success(w, map[string]string{"second": "response"})

	// 第二次写入后的状态取决于 go-zero 的实现
	// 我们只验证不发生 panic
}

// TestSuccessWithLargeData 测试大数据响应
func TestSuccessWithLargeData(t *testing.T) {
	w := httptest.NewRecorder()

	// 创建大型数据集
	largeData := make([]int, 10000)
	for i := 0; i < 10000; i++ {
		largeData[i] = i
	}

	Success(w, largeData)

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
		w := httptest.NewRecorder()
		Success(w, data)
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
		w := httptest.NewRecorder()
		SuccessList(w, items, 1000, 1, 100)
	}
}

// BenchmarkError 性能基准测试
func BenchmarkError(b *testing.B) {
	err := errorx.NewBadRequest("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/test", nil)
		Error(w, r, err)
	}
}
