package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityPolicy_DefaultsOff 账号安全策略五键默认全关（内置基线兜底）。
func TestSecurityPolicy_DefaultsOff(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	policy := l.SecurityPolicy()
	assert.False(t, policy.MFARequired)
	assert.Equal(t, 0, policy.PasswordMinLength)
	assert.False(t, policy.PasswordRequireUppercase)
	assert.False(t, policy.PasswordRequireSpecial)
	assert.Equal(t, 0, policy.PasswordMaxAgeDays)
}

// TestSecurityPolicy_L3Overrides 五键经 L3 数据库覆盖热生效 + 类型校验拒绝。
func TestSecurityPolicy_L3Overrides(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	set := func(key, raw string) {
		require.NoError(t, store.Set(context.Background(), key, json.RawMessage(raw), "tester"))
	}
	set(KeySecurityMFARequired, `true`)
	set(KeySecurityPasswordMinLength, `12`)
	set(KeySecurityPasswordRequireUpper, `true`)
	set(KeySecurityPasswordRequireSpecial, `true`)
	set(KeySecurityPasswordMaxAgeDays, `90`)
	l.Reload(context.Background(), store)

	policy := l.SecurityPolicy()
	assert.True(t, policy.MFARequired)
	assert.Equal(t, 12, policy.PasswordMinLength)
	assert.True(t, policy.PasswordRequireUppercase)
	assert.True(t, policy.PasswordRequireSpecial)
	assert.Equal(t, 90, policy.PasswordMaxAgeDays)

	// Clear → 回默认全关
	require.NoError(t, store.Clear(context.Background(), KeySecurityMFARequired))
	l.Reload(context.Background(), store)
	assert.False(t, l.SecurityPolicy().MFARequired)
}

// TestApprovalStepUpOTP_DefaultOn #61 审批 step-up 总开关：缺省 true
// （fail-safe，与 #75 强制语义一致），L3 false 覆盖生效、Clear 回默认开。
func TestApprovalStepUpOTP_DefaultOn(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	assert.True(t, l.GetBool(KeySecurityApprovalStepUpOTP, true), "未配置时必须取调用方默认 true")
	assert.True(t, l.SecurityPolicy().ApprovalStepUpOTP, "快照缺省也必须是开")

	require.NoError(t, store.Set(context.Background(), KeySecurityApprovalStepUpOTP, json.RawMessage(`false`), "tester"))
	l.Reload(context.Background(), store)
	assert.False(t, l.GetBool(KeySecurityApprovalStepUpOTP, true))
	assert.False(t, l.SecurityPolicy().ApprovalStepUpOTP)

	require.NoError(t, store.Clear(context.Background(), KeySecurityApprovalStepUpOTP))
	l.Reload(context.Background(), store)
	assert.True(t, l.SecurityPolicy().ApprovalStepUpOTP)
}
