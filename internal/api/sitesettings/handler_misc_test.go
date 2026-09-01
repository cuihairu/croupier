package sitesettings

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// GET 快照端点的基本可达性（此前 GetNotification/GetPublic 0% 覆盖）。
func TestGetNotificationAndPublicSnapshots(t *testing.T) {
	h := newAuthTestHandler(t)
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name string
		call func(*gin.Context)
	}{
		{"notification", h.GetNotification},
		{"public", h.GetPublic},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/site/x", nil)
		tc.call(c)
		if w.Code != 200 {
			t.Fatalf("%s = %d %s", tc.name, w.Code, w.Body.String())
		}
	}
}

func TestClearKey(t *testing.T) {
	h := newAuthTestHandler(t)
	gin.SetMode(gin.TestMode)

	setAuthKey(t, h, "site.name", `"demo-site"`)

	clear := func(key string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "key", Value: key}}
		c.Request = httptest.NewRequest("DELETE", "/site/"+key, nil)
		h.ClearKey(c)
		return w
	}

	if w := clear("site.name"); w.Code != 200 {
		t.Fatalf("clear valid = %d %s", w.Code, w.Body.String())
	}
	if w := clear("not.a.key"); w.Code == 200 {
		t.Fatalf("clear invalid key should fail")
	}
}

func TestValidateValueBranches(t *testing.T) {
	for _, tc := range []struct {
		name    string
		key     string
		raw     string
		wantErr bool
	}{
		{"bool ok", "features.dev", `true`, false},
		{"bool bad", "features.dev", `"yes"`, true},
		{"int ok", "notification.smtpPort", `465`, false},
		{"int bad type", "notification.smtpPort", `"465"`, true},
		{"int out of range", "notification.smtpPort", `70000`, true},
		{"string ok", "site.name", `"abc"`, false},
		{"string bad", "site.name", `123`, true},
		{"footer links array", "footer.links", `[{"label":"a","url":"https://x"}]`, false},
		{"obs url ok", "obs.grafanaExploreUrl", `"https://grafana.example.com"`, false},
		{"obs url bad", "obs.grafanaExploreUrl", `"not-a-url"`, true},
		{"notify url bad", "notification.dingtalkUrl", `"ftp://x"`, true},
		{"notify webhook empty ok", "notification.webhookUrl", `""`, false},
	} {
		err := validateValue(tc.key, []byte(tc.raw))
		if (err != nil) != tc.wantErr {
			t.Fatalf("%s: err=%v, wantErr=%v", tc.name, err, tc.wantErr)
		}
	}
}

func TestCurrentUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	if got := currentUsername(c); got != "system" {
		t.Fatalf("no username = %q, want system fallback", got)
	}
	c.Set("username", 42) // 非字符串类型
	if got := currentUsername(c); got != "system" {
		t.Fatalf("non-string username = %q, want system fallback", got)
	}
	c.Set("username", "admin")
	if got := currentUsername(c); got != "admin" {
		t.Fatalf("username = %q", got)
	}
}
