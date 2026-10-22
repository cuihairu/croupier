package settings

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/vault"
)

// 凭据值内引用（vault://）在分层读边界的展开语义（todo 4251）：
// 引用经注入的 Resolver 展开；内联值直通（存量零行为变化）；解析失败
// 置空 + fail-closed；未注入 Resolver 时原样透传（CLI/测试面）。

// fakeSecretResolver 展开 vault://<p> → "expanded[<p>]"；inline 原样返回。
func fakeSecretResolver(_ context.Context, value string) (string, error) {
	if !vault.IsVaultRef(value) {
		return value, nil
	}
	return "expanded[" + strings.TrimPrefix(value, vault.VaultRefPrefix) + "]", nil
}

func setL3(t *testing.T, l *Layered, store *model.PlatformSettingModel, key, raw string) {
	t.Helper()
	require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	l.Reload(context.Background(), store)
}

func TestSecretStrRefExpandedNotifySMTP(t *testing.T) {
	resetForTest()
	defer resetForTest()
	SetSecretResolver(fakeSecretResolver)
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	setL3(t, l, store, KeyNotifySMTPPassword, `"vault://smtp/password"`)
	assert.Equal(t, "expanded[smtp/password]", l.NotifySMTP().Password)
}

func TestSecretStrRefExpandedNotifyChannels(t *testing.T) {
	resetForTest()
	defer resetForTest()
	SetSecretResolver(fakeSecretResolver)
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	setL3(t, l, store, KeyNotifyDingtalkSecret, `"vault://channels/dingtalk"`)
	setL3(t, l, store, KeyNotifyWebhookSecret, `"vault://channels/webhook"`)
	setL3(t, l, store, KeyNotifyFeishuSecret, `"vault://channels/feishu"`)
	ch := l.NotifyChannels()
	assert.Equal(t, "expanded[channels/dingtalk]", ch.DingtalkSecret)
	assert.Equal(t, "expanded[channels/webhook]", ch.WebhookSecret)
	assert.Equal(t, "expanded[channels/feishu]", ch.FeishuSecret)
}

func TestSecretStrRefExpandedAuthProviderConfig(t *testing.T) {
	resetForTest()
	defer resetForTest()
	SetSecretResolver(fakeSecretResolver)
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	setL3(t, l, store, KeyAuthLdapBindPassword, `"vault://auth/ldap-bind"`)
	setL3(t, l, store, KeyAuthOidcClientSecret, `"vault://auth/oidc"`)
	cfg := l.AuthProviderConfig()
	assert.Equal(t, "expanded[auth/ldap-bind]", cfg.LDAP.BindPassword)
	assert.Equal(t, "expanded[auth/oidc]", cfg.OIDC.ClientSecret)
}

// 内联值直通：Resolver 已注入，存量明文/环境变量形态不受影响。
func TestSecretStrInlinePassthrough(t *testing.T) {
	resetForTest()
	defer resetForTest()
	SetSecretResolver(fakeSecretResolver)
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	setL3(t, l, store, KeyNotifySMTPPassword, `"inline-secret"`)
	assert.Equal(t, "inline-secret", l.NotifySMTP().Password)
}

// 未注入 Resolver（CLI/测试面）：引用原样透传，不误吞。
func TestSecretStrNoResolverPassthrough(t *testing.T) {
	resetForTest()
	defer resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	setL3(t, l, store, KeyNotifySMTPPassword, `"vault://smtp/password"`)
	assert.Equal(t, "vault://smtp/password", l.NotifySMTP().Password)
}

// 解析失败 → 空串（fail-closed：消费侧按「未配置」降级，不把引用串当
// 真凭据外发）。
func TestSecretStrResolveFailureYieldsEmpty(t *testing.T) {
	resetForTest()
	defer resetForTest()
	SetSecretResolver(func(context.Context, string) (string, error) {
		return "", vault.ErrLocked
	})
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	setL3(t, l, store, KeyNotifySMTPPassword, `"vault://smtp/password"`)
	assert.Equal(t, "", l.NotifySMTP().Password)
}

// 展开只发生在消费面 getter；快照/掩码读路径保持 raw——否则掩尾 4 字
// 会泄露明文尾 4 字（密码箱红线）。raw "vault://smtp/password" 掩码尾
// 4 字是 "word"；若误用展开值（尾字 "]"）即此断言炸。
func TestSecretSnapshotStaysRaw(t *testing.T) {
	resetForTest()
	defer resetForTest()
	SetSecretResolver(fakeSecretResolver)
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	setL3(t, l, store, KeyNotifySMTPPassword, `"vault://smtp/password"`)
	snap := l.NotificationSnapshot()
	assert.True(t, snap.SMTPPasswordSet)
	assert.Equal(t, "****word", snap.SMTPPasswordMasked)
}
