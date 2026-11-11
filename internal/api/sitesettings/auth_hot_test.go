package sitesettings

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
)

// newAuthTestHandler 用测试 store 初始化全局 Layered。每次调用都走
// InitLayered 而非 once 守卫：包内其他测试文件（putkey/testemail/
// handler_extra）会 ResetForTest 清掉全局单例，once 版本此后拿到 nil
// Layered——put 落在 per-test store 而 Reload 被 nil 守卫静默吞掉，
// auth.* 键全部退回默认值（shuffle 下按序随机红，OIDC Discovery 系列
// 实证）。InitLayered 自身是 sync.Once 语义，未重置时重复调用 no-op，
// 生产「只装配一次」的约束不变。
func newAuthTestHandler(t *testing.T) *Handler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/site.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.PlatformSetting{}); err != nil {
		t.Fatal(err)
	}
	store := model.NewPlatformSettingModel(db)
	settings.InitLayered(context.Background(), &settings.ConfigInput{}, store)
	return NewHandler(settings.Current(), store)
}

func putSiteKey(t *testing.T, h *Handler, key, jsonValue string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: key}}
	c.Request = httptest.NewRequest("PUT", "/site/"+key, strings.NewReader(`{"value":`+jsonValue+`}`))
	h.PutKey(c)
	return w
}

// secret 掩码占位符回存沿用旧值（读→存回写不破坏真值）。
func TestPutSecretPlaceholderPreserved(t *testing.T) {
	h := newAuthTestHandler(t)
	if w := putSiteKey(t, h, "auth.oidc.clientSecret", `"real-secret-123"`); w.Code != 200 {
		t.Fatalf("first put = %d %s", w.Code, w.Body.String())
	}
	if w := putSiteKey(t, h, "auth.oidc.clientSecret", `"****t123"`); w.Code != 200 {
		t.Fatalf("mask put = %d %s", w.Code, w.Body.String())
	}
	snap := settings.Current().AuthSnapshot()
	if !snap.OIDC.SecretSet {
		t.Fatal("secret should remain set after mask re-save")
	}
	// 真值未被掩码覆盖：掩码回显仍基于原值尾部
	if !strings.HasSuffix(snap.OIDC.SecretMasked, "-123") {
		t.Fatalf("masked = %q, want suffix t123", snap.OIDC.SecretMasked)
	}
}

// auth.* 保存触发热刷新；无效配置被拒并回滚（登录入口不因坏配置中断）。
func TestPutAuthKeyInvalidConfigRejectedAndRolledBack(t *testing.T) {
	h := newAuthTestHandler(t)
	h.SetAuthChangeCallback(func(cfg config.AuthProvidersConfig) error {
		// 模拟 buildIdentityProviders 拒绝：enabled 但缺 addr
		if cfg.LDAP.Enabled && cfg.LDAP.Addr == "" {
			return context.Canceled
		}
		return nil
	})
	// enabled=true 但 addr 空 → 回调报错 → 400 + 键回滚
	w := putSiteKey(t, h, "auth.ldap.enabled", `true`)
	if w.Code == 200 {
		t.Fatalf("invalid config should be rejected, got 200: %s", w.Body.String())
	}
	// 键应被回滚（读取不到 database 层覆盖）
	if _, src, _ := settings.Current().GetString(context.Background(), settings.KeyAuthLdapEnabled); src == "database" {
		t.Fatal("key should be rolled back")
	}
}
