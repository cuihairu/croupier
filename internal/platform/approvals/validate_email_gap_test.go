package approvals

// validate_email_gap_test.go 补齐 validateEmailAddress 的 Display Name
// 拒绝分支（notification.go:828-830）：mail.ParseAddress 能成功解析
// "Name <addr>" 形态，但规范地址与原始输入不一致时必须拒绝——显示名
// 可能携带 SMTP 命令注入载荷。
//
// 本包其余未满分支的可达性论证（详见交付报告）：
//   - model.go:113-115 encodeMetadataJSON Marshal 失败：入参
//     map[string]string 恒可 JSON 序列化，死分支
//   - notification.go:617/862/912 三个 Send 的 Marshal 失败：payload
//     顶层与嵌套值全部为 string，恒可序列化，死分支
//   - workflow_store.go:140-142 History Marshal 失败：
//     []WorkflowHistoryEntry 仅含 time/string 字段，恒可序列化，死分支
//   - workflow_store.go:261-263 Permissions Marshal 失败：
//     []DelegationPermission 为 string 枚举切片，恒可序列化，死分支

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEmailAddressRejectsDisplayName_I(t *testing.T) {
	cases := []string{
		// 显示名形态：ParseAddress 成功但规范地址 != 原始输入。
		`Display Name <ops@example.com>`,
		`<ops@example.com>`,
	}
	for _, addr := range cases {
		got, err := validateEmailAddress(addr)
		require.Error(t, err, "display-name form must be rejected: %q", addr)
		assert.Contains(t, err.Error(), "display name not allowed")
		assert.Empty(t, got)
	}

	// 裸地址仍被接受且原样返回（回归保护）。
	got, err := validateEmailAddress("ops@example.com")
	require.NoError(t, err)
	assert.Equal(t, "ops@example.com", got)
}
