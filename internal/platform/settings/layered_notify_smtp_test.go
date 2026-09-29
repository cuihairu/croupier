package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotifySMTP_TransportKeys SMTP 传输细节键 L3 读写（OPEN-ISSUES #55）：
// 默认空串/false（自动加密 + PLAIN + 校验证书），覆盖后 NotifySMTP 与
// NotificationSnapshot 同步透出。
func TestNotifySMTP_TransportKeys(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	cfg := l.NotifySMTP()
	assert.Equal(t, "", cfg.Encryption)
	assert.Equal(t, "", cfg.AuthType)
	assert.False(t, cfg.InsecureSkipVerify)

	set := func(key string, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	set(KeyNotifySMTPEncryption, `"ssl"`)
	set(KeyNotifySMTPAuthType, `"login"`)
	set(KeyNotifySMTPInsecureSkipVerify, `true`)
	l.Reload(context.Background(), store)

	cfg = l.NotifySMTP()
	assert.Equal(t, "ssl", cfg.Encryption)
	assert.Equal(t, "login", cfg.AuthType)
	assert.True(t, cfg.InsecureSkipVerify)

	snap := l.NotificationSnapshot()
	assert.Equal(t, "ssl", snap.SMTPEncryption)
	assert.Equal(t, "login", snap.SMTPAuthType)
	assert.True(t, snap.SMTPInsecureSkipVerify)
}
