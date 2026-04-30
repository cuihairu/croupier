// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestResponseStruct(t *testing.T) {
	t.Parallel()

	resp := Response{
		Code:    0,
		Message: "success",
		Data:    "test data",
	}

	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf(`Message = %s, want "success"`, resp.Message)
	}
	if resp.Data != "test data" {
		t.Errorf(`Data = %s, want "test data"`, resp.Data)
	}
}

func TestSuccess(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]string{"key": "value"}
	Success(c, data)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf(`Message = %s, want "success"`, resp.Message)
	}
	if resp.Data == nil {
		t.Error("Data should not be nil")
	}
}

func TestSuccessWithMessage(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	customMsg := "operation completed"
	data := "result"
	SuccessWithMessage(c, customMsg, data)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if resp.Message != customMsg {
		t.Errorf(`Message = %s, want "%s"`, resp.Message, customMsg)
	}
}

func TestSuccessWithNilData(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Success(c, nil)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// When data is nil, Data field should be omitted or null
	if _, ok := resp["data"]; !ok {
		// Data field omitted when nil is acceptable
		return
	}
	// If present, it should be null
	if resp["data"] != nil {
		t.Error("Data should be nil when nil is passed")
	}
}

func TestError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    int
		message string
	}{
		{"error 400", 400, "bad request"},
		{"error 401", 401, "unauthorized"},
		{"error 404", 404, "not found"},
		{"error 500", 500, "internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			Error(c, tt.code, tt.message)

			if w.Code != http.StatusOK {
				t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
			}

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if resp.Code != tt.code {
				t.Errorf("Code = %d, want %d", resp.Code, tt.code)
			}
			if resp.Message != tt.message {
				t.Errorf(`Message = %s, want "%s"`, resp.Message, tt.message)
			}
		})
	}
}

func TestErrorWithData(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	code := 422
	message := "validation failed"
	data := map[string]string{"field": "email", "error": "invalid format"}

	ErrorWithData(c, code, message, data)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Code != code {
		t.Errorf("Code = %d, want %d", resp.Code, code)
	}
	if resp.Message != message {
		t.Errorf(`Message = %s, want "%s"`, resp.Message, message)
	}
	if resp.Data == nil {
		t.Error("Data should not be nil")
	}
}

func TestBadRequest(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	BadRequest(c, "invalid input")

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Code != 400 {
		t.Errorf("Code = %d, want 400", resp.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Unauthorized(c, "please login")

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Code != 401 {
		t.Errorf("Code = %d, want 401", resp.Code)
	}
}

func TestForbidden(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Forbidden(c, "access denied")

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Code != 403 {
		t.Errorf("Code = %d, want 403", resp.Code)
	}
}

func TestNotFound(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	NotFound(c, "resource not found")

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Code != 404 {
		t.Errorf("Code = %d, want 404", resp.Code)
	}
}

func TestInternalServerError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	InternalServerError(c, "something went wrong")

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Code != 500 {
		t.Errorf("Code = %d, want 500", resp.Code)
	}
}

func TestResponseJSONMarshal(t *testing.T) {
	t.Parallel()

	resp := Response{
		Code:    0,
		Message: "success",
		Data:    map[string]string{"foo": "bar"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal Response: %v", err)
	}

	var unmarshaled Response
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal Response: %v", err)
	}

	if unmarshaled.Code != resp.Code {
		t.Errorf("Unmarshaled Code = %d, want %d", unmarshaled.Code, resp.Code)
	}
	if unmarshaled.Message != resp.Message {
		t.Errorf(`Unmarshaled Message = %s, want "%s"`, unmarshaled.Message, resp.Message)
	}
}

func TestResponseWithComplexData(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	complexData := map[string]interface{}{
		"items":  []string{"item1", "item2"},
		"count":  2,
		"nested": map[string]string{"key": "value"},
	}

	Success(c, complexData)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Code = %d, want 0", resp.Code)
	}
	if resp.Data == nil {
		t.Fatal("Data should not be nil")
	}

	// Verify complex data structure
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Data should be a map")
	}

	if items, ok := dataMap["items"].([]interface{}); ok {
		if len(items) != 2 {
			t.Errorf("items length = %d, want 2", len(items))
		}
	} else {
		t.Error("items should be an array")
	}
}
