package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// brokenStore 返回一个 List 必然失败的 store（表被删除），
// 用于覆盖 L3 加载/Reload 失败分支。
func brokenStore(t *testing.T) *model.PlatformSettingModel {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/broken.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, db.Migrator().DropTable("platform_settings"))
	return model.NewPlatformSettingModel(db)
}

func TestKeyClassificationHelpers(t *testing.T) {
	assert.True(t, IsSecretKey(KeyNotifySMTPPassword))
	assert.True(t, IsSecretKey(KeyNotifyDingtalkSecret))
	assert.True(t, IsSecretKey(KeyNotifyWebhookSecret))
	assert.False(t, IsSecretKey(KeyNotifySMTPHost))
	assert.True(t, IsIntKey(KeyNotifySMTPPort))
	assert.False(t, IsIntKey(KeySiteName))
}

func TestCurrent_BeforeAndAfterInit(t *testing.T) {
	resetForTest()
	assert.Nil(t, Current())
	l := InitLayered(context.Background(), &ConfigInput{}, newStore(t))
	assert.Equal(t, l, Current())
	ResetForTest()
	assert.Nil(t, Current())
}

func TestInitLayered_NilConfigAndStoreLoadFailure(t *testing.T) {
	resetForTest()
	l := InitLayered(context.Background(), nil, nil)
	assert.NotNil(t, l)
	_, src, ok := l.GetString(context.Background(), KeySiteName)
	assert.False(t, ok)
	assert.Equal(t, "default", src)

	// L3 加载失败：停留在 L2，不标记 l3Loaded。
	resetForTest()
	l = InitLayered(context.Background(), &ConfigInput{SiteName: "l2-name"}, brokenStore(t))
	v, src, ok := l.GetString(context.Background(), KeySiteName)
	require.True(t, ok)
	assert.Equal(t, "l2-name", v)
	assert.Equal(t, "config", src)

	// Reload 失败同样保持 L2 且不 panic。
	l.Reload(context.Background(), brokenStore(t))
	v, src, _ = l.GetString(context.Background(), KeySiteName)
	assert.Equal(t, "l2-name", v)
	assert.Equal(t, "config", src)

	// nil store / nil receiver 的 Reload 是 no-op。
	l.Reload(context.Background(), nil)
	var nilLayered *Layered
	nilLayered.Reload(context.Background(), newStore(t))
}

func TestGetString_FallbackBranches(t *testing.T) {
	resetForTest()
	l := InitLayered(context.Background(), &ConfigInput{SiteName: "l2-name"}, newStore(t))

	// 非法 key → source 标注 invalid。
	_, src, ok := l.GetString(context.Background(), "database.dsn")
	assert.False(t, ok)
	assert.Equal(t, "invalid", src)

	// L3 存了非字符串 JSON（数字）：解析失败回落 L2。
	l.l3 = map[string]json.RawMessage{KeySiteName: json.RawMessage(`123`)}
	l.l3Loaded = true
	v, src, ok := l.GetString(context.Background(), KeySiteName)
	require.True(t, ok)
	assert.Equal(t, "l2-name", v)
	assert.Equal(t, "config", src)
}

func TestGetBool_FallbackBranches(t *testing.T) {
	resetForTest()
	l := InitLayered(context.Background(), &ConfigInput{}, newStore(t))

	// L3 存了非 bool JSON（字符串）：回落默认值。
	l.l3 = map[string]json.RawMessage{KeyFeatureDev: json.RawMessage(`"yes"`)}
	l.l3Loaded = true
	assert.False(t, l.GetBool(KeyFeatureDev, false))
}

func TestLayered_NilReceiver(t *testing.T) {
	var l *Layered
	ctx := context.Background()

	v, src, ok := l.GetString(ctx, KeySiteName)
	assert.Empty(t, v)
	assert.Equal(t, "default", src)
	assert.False(t, ok)
	assert.True(t, l.GetBool(KeyFeatureDev, true))
	assert.False(t, l.GetBool(KeyFeatureDev, false))
	assert.Equal(t, 25, l.GetInt(KeyNotifySMTPPort, 25))
	assert.True(t, l.FeatureEnabled("dev"))
	assert.False(t, l.hasL3(KeySiteName))

	snap := l.SiteSnapshot()
	assert.Equal(t, "Croupier", snap.SiteName)
	assert.Equal(t, "default", snap.Sources[KeySiteName])

	obs := l.ObsSnapshot()
	assert.Equal(t, "default", obs.Sources[KeyObsJaegerURL])

	notify := l.NotificationSnapshot()
	assert.False(t, notify.EmailEnabled)
	assert.True(t, notify.InAppEnabled)
	assert.False(t, notify.SMTPPasswordSet)

	feat := l.FeatureSnapshot()
	assert.Len(t, feat.Domains, len(featureFlagNames))
	for name, st := range feat.Domains {
		assert.True(t, st.Enabled, name)
		assert.False(t, st.TrimmedByConfig, name)
		assert.False(t, st.Overridden, name)
	}
}

func TestLayered_GetInt(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	assert.Equal(t, 25, l.GetInt(KeyNotifySMTPPort, 25))

	require.NoError(t, store.Set(context.Background(), KeyNotifySMTPPort, json.RawMessage(`587`), "admin"))
	l.Reload(context.Background(), store)
	assert.Equal(t, 587, l.GetInt(KeyNotifySMTPPort, 25))

	// 容错：数值被存成字符串。
	require.NoError(t, store.Set(context.Background(), KeyNotifySMTPPort, json.RawMessage(`"465"`), "admin"))
	l.Reload(context.Background(), store)
	assert.Equal(t, 465, l.GetInt(KeyNotifySMTPPort, 25))

	// 非数字字符串与非法 key 都回落默认值。
	require.NoError(t, store.Set(context.Background(), KeyNotifySMTPPort, json.RawMessage(`"smtp"`), "admin"))
	l.Reload(context.Background(), store)
	assert.Equal(t, 25, l.GetInt(KeyNotifySMTPPort, 25))
	assert.Equal(t, 7, l.GetInt("server.port", 7))
}

func TestLayered_NotificationSnapshot(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	set := func(key string, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "admin"))
	}
	set(KeyNotifyEmailEnabled, `true`)
	set(KeyNotifySMTPHost, `"smtp.example.com"`)
	set(KeyNotifySMTPPort, `587`)
	set(KeyNotifySMTPUser, `"croupier"`)
	set(KeyNotifySMTPFrom, `"croupier@example.com"`)
	set(KeyNotifySMTPPassword, `"super-secret-9999"`) // >4 尾 4 位回显
	set(KeyNotifyDingtalkURL, `"https://oapi.dingtalk.me"`)
	set(KeyNotifyDingtalkSecret, `"abc"`) // <=4 全掩码
	set(KeyNotifyWebhookURL, `"https://hook.example.me"`)
	l.Reload(context.Background(), store)

	snap := l.NotificationSnapshot()
	assert.True(t, snap.EmailEnabled)
	assert.Equal(t, "smtp.example.com", snap.SMTPHost)
	assert.Equal(t, 587, snap.SMTPPort)
	assert.Equal(t, "croupier", snap.SMTPUser)
	assert.Equal(t, "croupier@example.com", snap.SMTPFrom)
	assert.True(t, snap.SMTPPasswordSet)
	assert.Equal(t, "****9999", snap.SMTPPasswordMasked)
	assert.True(t, snap.DingtalkSecretSet)
	assert.Equal(t, "****", snap.DingtalkSecretMasked)
	assert.False(t, snap.WebhookSecretSet)
	assert.Empty(t, snap.WebhookSecretMasked)
	assert.True(t, snap.InAppEnabled) // 未设置默认开
	// 注意：Sources 目前仅记录三个密钥 key 的 provenance，普通字段不记录。
	assert.NotContains(t, snap.Sources, KeyNotifySMTPHost)
	assert.Equal(t, "database", snap.Sources[KeyNotifySMTPPassword])

	// 清除覆盖后回默认（邮件关、InApp 开）。
	require.NoError(t, store.Clear(context.Background(), KeyNotifyEmailEnabled))
	require.NoError(t, store.Clear(context.Background(), KeyNotifySMTPPassword))
	l.Reload(context.Background(), store)
	snap = l.NotificationSnapshot()
	assert.False(t, snap.EmailEnabled)
	assert.False(t, snap.SMTPPasswordSet)
}

func TestLayered_NotifySMTP_Unmasked(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)
	require.NoError(t, store.Set(context.Background(), KeyNotifySMTPPassword, json.RawMessage(`"raw-pass"`), "admin"))
	require.NoError(t, store.Set(context.Background(), KeyNotifySMTPHost, json.RawMessage(`"smtp.example.com"`), "admin"))
	l.Reload(context.Background(), store)

	smtp := l.NotifySMTP()
	assert.True(t, smtp.Enabled == false) // 未设置 EmailEnabled 默认关
	assert.Equal(t, "smtp.example.com", smtp.Host)
	assert.Equal(t, "raw-pass", smtp.Password) // 内部读取不脱敏
}

func TestLayered_NotifyChannels(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)
	require.NoError(t, store.Set(context.Background(), KeyNotifyDingtalkURL, json.RawMessage(`"https://ding"`), "admin"))
	require.NoError(t, store.Set(context.Background(), KeyNotifyWebhookSecret, json.RawMessage(`"wh-secret"`), "admin"))
	l.Reload(context.Background(), store)

	ch := l.NotifyChannels()
	assert.Equal(t, "https://ding", ch.DingtalkURL)
	assert.Equal(t, "wh-secret", ch.WebhookSecret) // 未脱敏
	assert.True(t, ch.InAppEnabled)
	assert.False(t, ch.EmailEnabled)
}

func TestLayered_FeatureSnapshot(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{
		FeatureFlags: map[string]bool{"analytics": false},
	}, store)

	// L3 软关 dev；L2 物理裁剪 analytics。
	require.NoError(t, store.Set(context.Background(), KeyFeatureDev, json.RawMessage(`false`), "admin"))
	l.Reload(context.Background(), store)

	snap := l.FeatureSnapshot()
	require.Len(t, snap.Domains, 5)

	dev := snap.Domains["dev"]
	assert.False(t, dev.Enabled)
	assert.False(t, dev.TrimmedByConfig)
	assert.True(t, dev.Overridden)

	analytics := snap.Domains["analytics"]
	assert.False(t, analytics.Enabled)
	assert.True(t, analytics.TrimmedByConfig)
	assert.False(t, analytics.Overridden)

	// L2 裁剪 + L3 试图开启：仍为关、但标记 Overridden。
	require.NoError(t, store.Set(context.Background(), KeyFeatureAnalytics, json.RawMessage(`true`), "admin"))
	l.Reload(context.Background(), store)
	analytics = l.FeatureSnapshot().Domains["analytics"]
	assert.False(t, analytics.Enabled)
	assert.True(t, analytics.TrimmedByConfig)
	assert.True(t, analytics.Overridden)

	// 未触碰的域 fail-open。
	ops := snap.Domains["ops"]
	assert.True(t, ops.Enabled)
	assert.False(t, ops.Overridden)
}

func TestSiteSnapshot_AllFieldsAndFooterLinks(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{DefaultLocale: "zh-CN"}, store)

	set := func(key string, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "admin"))
	}
	set(KeySiteName, `"db-name"`)
	set(KeySiteLogoURL, `"https://cdn/logo.png"`)
	set(KeySiteFaviconURL, `"https://cdn/favicon.ico"`)
	set(KeySiteDescription, `"GM platform"`)
	set(KeyFooterCopyright, `"© 2026"`)
	// GetString 只反序列化字符串，footer.links 需双重编码为 JSON 字符串
	// 才能走到 SiteSnapshot 的解析分支（见下方 bug 固化测试）。
	set(KeyFooterLinks, `"[{\"key\":\"gh\",\"title\":\"GitHub\",\"url\":\"https://github.com\"}]"`)
	l.Reload(context.Background(), store)

	snap := l.SiteSnapshot()
	assert.Equal(t, "db-name", snap.SiteName)
	assert.Equal(t, "database", snap.Sources[KeySiteName])
	assert.Equal(t, "https://cdn/logo.png", snap.LogoURL)
	assert.Equal(t, "https://cdn/favicon.ico", snap.FaviconURL)
	assert.Equal(t, "GM platform", snap.Description)
	assert.Equal(t, "© 2026", snap.FooterCopy)
	assert.Equal(t, "zh-CN", snap.DefaultLocale)
	assert.Equal(t, "config", snap.Sources[KeyDefaultLocale])
	require.Len(t, snap.FooterLinks, 1)
	assert.Equal(t, "gh", snap.FooterLinks[0].Key)
	assert.Equal(t, "database", snap.Sources[KeyFooterLinks])

	// footer.links 为空数组 → 保留内置默认链接。
	set(KeyFooterLinks, `"[]"`)
	l.Reload(context.Background(), store)
	snap = l.SiteSnapshot()
	assert.NotEmpty(t, snap.FooterLinks, "empty array should keep built-in default")

	// footer.links 内容非法 JSON → 同样保留默认。
	set(KeyFooterLinks, `"[not-an-array"`)
	l.Reload(context.Background(), store)
	snap = l.SiteSnapshot()
	assert.NotEmpty(t, snap.FooterLinks)
	assert.NotContains(t, snap.Sources, KeyFooterLinks)

	// 未设置的 key 不产生 provenance 条目。
	assert.NotContains(t, snap.Sources, KeyFooterICP)
}

// TestSiteSnapshot_FooterLinks_ArrayFormDoesNotRoundTrip 固化当前行为：
// 写侧（sitesettings validateValue）接受单重编码 JSON 数组，但读侧
// GetString 只解字符串，导致文档形态的 footer.links L3 覆盖永远不生效
// （写入/读取不对称，疑似产品 bug，见任务报告）。
func TestSiteSnapshot_FooterLinks_ArrayFormDoesNotRoundTrip(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)
	require.NoError(t, store.Set(context.Background(), KeyFooterLinks,
		json.RawMessage(`[{"key":"gh","title":"GitHub","url":"https://github.com"}]`), "admin"))
	l.Reload(context.Background(), store)

	snap := l.SiteSnapshot()
	assert.Equal(t, "croupier", snap.FooterLinks[0].Key, "array-form override is silently ignored")
	assert.NotContains(t, snap.Sources, KeyFooterLinks)
}
