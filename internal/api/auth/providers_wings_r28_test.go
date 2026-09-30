package auth

// 覆盖率巡检第二十八轮（wt-api）：api/auth 残余翼——BuildIdentityProviders
// 三处 provider 构造失败的失效降级日志翼、VerifyEmailToken 的未知令牌翼。
//
// 登记不可达（不造假用例、不删防御分支）：
//   - siteServerURL 的 Current()==nil 翼（email_verification.go:78）：
//     settings 单例由服务装配初始化，包外无复位缝（resetForTest 是
//     platform/settings 包内私有），测试进程内不可回归 nil；
//   - newVerificationToken 的 rand.Read 失败翼（87）及其在
//     issueEmailVerification 的透传翼（103）：crypto/rand 在 Linux 走
//     getrandom(2)，引导完成后无失败路径；
//   - mfa.go:95 GenerateRecoveryCodes 失败翼：同 rand 族（恢复码生成
//     纯 crypto/rand）；
//   - service.go:738 的 Cut 失败 continue：入列邮箱经 FindEmailsByDomain
//     的 LIKE '%@'+domain 谓词过滤，匹配行必含 @，strings.Cut 恒成功
//     （自证性双保险；register_verify_gap_test.go 的脏行用例已锁定
//     「无 @ 脏行进不了比对」的 SQL 层前提）。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildIdentityProviders_WhitespaceCtorDegrade 三处 provider 的
// 「guard 查非空、ctor 查 TrimSpace 非空」缝：空白串凭据过 guard、
// 构造失败 → 失效降级（slog 记错 + 该 provider 置空返回，本地与其他
// 源不受影响）。
func TestBuildIdentityProviders_WhitespaceCtorDegrade(t *testing.T) {
	t.Run("github", func(t *testing.T) {
		cfg := config.AuthProvidersConfig{}
		cfg.GitHub.Enabled = true
		cfg.GitHub.ClientID = " " // guard（== ""）放行，ctor（TrimSpace）拒绝
		cfg.GitHub.ClientSecret = "cs"
		cfg.GitHub.RedirectURL = "https://gm.example.com/cb"
		out, err := BuildIdentityProviders(cfg)
		require.NoError(t, err, "构造失败只降级不报错")
		assert.Nil(t, out.github, "GitHub 须被跳过")
	})

	t.Run("wechat", func(t *testing.T) {
		cfg := config.AuthProvidersConfig{}
		cfg.WeChat.Enabled = true
		cfg.WeChat.AppID = " "
		cfg.WeChat.AppSecret = "s"
		cfg.WeChat.RedirectURL = "https://gm.example.com/cb"
		out, err := BuildIdentityProviders(cfg)
		require.NoError(t, err)
		assert.Nil(t, out.wechat, "WeChat 须被跳过")
	})

	t.Run("generic-oauth", func(t *testing.T) {
		cfg := config.AuthProvidersConfig{}
		cfg.GenericOAuth.Enabled = true
		cfg.GenericOAuth.ClientID = " "
		cfg.GenericOAuth.ClientSecret = "cs"
		cfg.GenericOAuth.RedirectURL = "https://gm.example.com/cb"
		cfg.GenericOAuth.AuthURL = "https://sso.example.com/authorize"
		cfg.GenericOAuth.TokenURL = "https://sso.example.com/token"
		cfg.GenericOAuth.UserInfoURL = "https://sso.example.com/user"
		out, err := BuildIdentityProviders(cfg)
		require.NoError(t, err)
		assert.Nil(t, out.genericOAuth, "自定义 OAuth 须被跳过")
	})
}

// TestVerifyEmailToken_UnknownToken 未知令牌（哈希不在表）→ 统一
// 「无效或已过期」400 语义——不区分原因，避免令牌有效性探测。
func TestVerifyEmailToken_UnknownToken(t *testing.T) {
	svc, _, _ := verificationFixture(t)
	err := svc.VerifyEmailToken(context.Background(), "no-such-token-in-table")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "验证链接无效或已过期")
}
