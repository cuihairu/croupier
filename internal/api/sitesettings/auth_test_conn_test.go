package sitesettings

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/cuihairu/croupier/internal/platform/settings"
)

// postTestAuth 调用 POST /site/auth/test 并返回解析后的响应体与状态码。
func postTestAuth(t *testing.T, h *Handler, bodyJSON string) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/site/auth/test", strings.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	h.TestAuthConnection(c)
	var parsed map[string]any
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("response not json: %s", w.Body.String())
		}
	}
	return w.Code, parsed
}

func setAuthKey(t *testing.T, h *Handler, key, jsonValue string) {
	t.Helper()
	if w := putSiteKey(t, h, key, jsonValue); w.Code != 200 {
		t.Fatalf("put %s = %d %s", key, w.Code, w.Body.String())
	}
}

func TestGetAuthReturnsSnapshot(t *testing.T) {
	h := newAuthTestHandler(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/site/auth", nil)
	h.GetAuth(c)
	if w.Code != 200 {
		t.Fatalf("GetAuth = %d %s", w.Code, w.Body.String())
	}
	var snap settings.AuthSnapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatalf("snapshot not json: %v", err)
	}
}

func TestTestAuthConnectionBadJSON(t *testing.T) {
	h := newAuthTestHandler(t)
	code, _ := postTestAuth(t, h, `{bad json`)
	if code == 200 {
		t.Fatalf("bad json should not return 200")
	}
}

func TestTestAuthConnectionInvalidKind(t *testing.T) {
	h := newAuthTestHandler(t)
	code, body := postTestAuth(t, h, `{"kind":"saml"}`)
	if code != 400 {
		t.Fatalf("invalid kind = %d, want 400", code)
	}
	if !strings.Contains(body["message"].(string), "ldap 或 oidc") {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestTestAuthConnectionLDAPNotEnabled(t *testing.T) {
	h := newAuthTestHandler(t)
	setAuthKey(t, h, "auth.ldap.enabled", `false`)
	code, body := postTestAuth(t, h, `{"kind":"ldap"}`)
	if code == 200 && body["ok"] == true {
		t.Fatalf("ldap disabled should not succeed: %v", body)
	}
	if !strings.Contains(body["message"].(string), "未启用") {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestTestAuthConnectionOIDCNotEnabled(t *testing.T) {
	h := newAuthTestHandler(t)
	setAuthKey(t, h, "auth.oidc.enabled", `false`)
	_, body := postTestAuth(t, h, `{"kind":"oidc"}`)
	if !strings.Contains(body["message"].(string), "未启用") {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestTestAuthConnectionLDAPAddrEmpty(t *testing.T) {
	h := newAuthTestHandler(t)
	setAuthKey(t, h, "auth.ldap.enabled", `true`)
	setAuthKey(t, h, "auth.ldap.addr", `""`)
	_, body := postTestAuth(t, h, `{"kind":"ldap"}`)
	if body["ok"] != false {
		t.Fatalf("want ok=false, got %v", body)
	}
	if !strings.Contains(body["message"].(string), "addr 未配置") {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestTestAuthConnectionLDAPUnreachable(t *testing.T) {
	h := newAuthTestHandler(t)
	setAuthKey(t, h, "auth.ldap.enabled", `true`)
	setAuthKey(t, h, "auth.ldap.addr", `"127.0.0.1:1"`)
	_, body := postTestAuth(t, h, `{"kind":"ldap"}`)
	if body["ok"] != false {
		t.Fatalf("want ok=false, got %v", body)
	}
	if !strings.Contains(body["message"].(string), "失败") {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestTestAuthConnectionLDAPReachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	h := newAuthTestHandler(t)
	setAuthKey(t, h, "auth.ldap.enabled", `true`)
	addrJSON, _ := json.Marshal(ln.Addr().String())
	setAuthKey(t, h, "auth.ldap.addr", string(addrJSON))
	_, body := postTestAuth(t, h, `{"kind":"LDAP"}`) // 大小写不敏感
	if body["ok"] != true {
		t.Fatalf("want ok=true, got %v", body)
	}
	setAuthKey(t, h, "auth.ldap.enabled", `false`)
}

func TestTestAuthConnectionOIDCIssuerEmpty(t *testing.T) {
	h := newAuthTestHandler(t)
	setAuthKey(t, h, "auth.oidc.enabled", `true`)
	setAuthKey(t, h, "auth.oidc.issuer", `""`)
	_, body := postTestAuth(t, h, `{"kind":"oidc"}`)
	if body["ok"] != false {
		t.Fatalf("want ok=false, got %v", body)
	}
	if !strings.Contains(body["message"].(string), "issuer 未配置") {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestTestAuthConnectionOIDCDiscoveryOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issuer":"x"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	h := newAuthTestHandler(t)
	setAuthKey(t, h, "auth.oidc.enabled", `true`)
	issuerJSON, _ := json.Marshal(srv.URL + "/") // 尾部斜杠应被 TrimRight 处理
	setAuthKey(t, h, "auth.oidc.issuer", string(issuerJSON))
	_, body := postTestAuth(t, h, `{"kind":"oidc"}`)
	if body["ok"] != true {
		t.Fatalf("want ok=true, got %v", body)
	}
}

func TestTestAuthConnectionOIDCDiscoveryBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	h := newAuthTestHandler(t)
	setAuthKey(t, h, "auth.oidc.enabled", `true`)
	issuerJSON, _ := json.Marshal(srv.URL)
	setAuthKey(t, h, "auth.oidc.issuer", string(issuerJSON))
	_, body := postTestAuth(t, h, `{"kind":"oidc"}`)
	if body["ok"] != false {
		t.Fatalf("want ok=false, got %v", body)
	}
	if !strings.Contains(body["message"].(string), "500") {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestTestAuthConnectionOIDCUnreachable(t *testing.T) {
	h := newAuthTestHandler(t)
	setAuthKey(t, h, "auth.oidc.enabled", `true`)
	setAuthKey(t, h, "auth.oidc.issuer", `"http://127.0.0.1:1"`)
	_, body := postTestAuth(t, h, `{"kind":"oidc"}`)
	if body["ok"] != false {
		t.Fatalf("want ok=false, got %v", body)
	}
	if !strings.Contains(body["message"].(string), "不可达") {
		t.Fatalf("message = %v", body["message"])
	}
	setAuthKey(t, h, "auth.oidc.enabled", `false`)
}
