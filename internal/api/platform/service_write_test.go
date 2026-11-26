package platform

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

// TestWriteListPlatformsResponse_NilResponse tests the nil response path
func TestWriteListPlatformsResponse_NilResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/test", func(c *gin.Context) {
		writeListPlatformsResponse(c, nil)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 for nil response, got %d", w.Code)
	}
}

// TestWriteCallResponse_NilResponse tests the nil response path

// TestWriteCallResponse_NilResponse tests the nil response path
func TestWriteCallResponse_NilResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/test", func(c *gin.Context) {
		writeCallResponse(c, nil)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 for nil response, got %d", w.Code)
	}
}

// TestWriteCallResponse_ErrorCode tests the error code path

// TestWriteCallResponse_ErrorCode tests the error code path
func TestWriteCallResponse_ErrorCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/test", func(c *gin.Context) {
		resp := &CallPlatformResponse{
			Code:    http.StatusBadRequest,
			Message: "bad request",
		}
		writeCallResponse(c, resp)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for error response, got %d", w.Code)
	}
}

// TestWriteListMethodsResponse_NilResponse tests the nil response path

// TestWriteListMethodsResponse_NilResponse tests the nil response path
func TestWriteListMethodsResponse_NilResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/test", func(c *gin.Context) {
		writeListMethodsResponse(c, nil)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 for nil response, got %d", w.Code)
	}
}
