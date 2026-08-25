package reqinfo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddlewareInjectsClientIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())

	var got Info
	var ok bool
	r.GET("/probe", func(c *gin.Context) {
		got, ok = FromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("User-Agent", "reqinfo-test/1.0")
	// httptest.NewRequest sets RemoteAddr to 192.0.2.1:1234 by default.
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !ok {
		t.Fatal("FromContext: info not injected")
	}
	if got.IP == "" {
		t.Fatal("IP not captured")
	}
	if got.UserAgent != "reqinfo-test/1.0" {
		t.Fatalf("UserAgent = %q", got.UserAgent)
	}
}

func TestFromContextAbsent(t *testing.T) {
	if _, ok := FromContext(nil); ok {
		t.Fatal("nil context must not carry info")
	}
}
