package auth

// Coverage audit round 28 (wt-api): the remaining flanks of api/auth — the
// "degrade on failure" logging flanks of the three failed provider builds in
// BuildIdentityProviders, and VerifyEmailToken's unknown-token flank.
//
// Registered as unreachable (no fabricated tests, no defense branches removed):
//   - the siteServerURL Current()==nil flank (email_verification.go:78): the
//     settings singleton is initialized by service assembly, with no reset seam
//     outside the package (resetForTest is private to platform/settings), so it
//     cannot return to nil within a test process;
//   - newVerificationToken's rand.Read failure flank (:87) and its pass-through
//     flank in issueEmailVerification (:103): crypto/rand on Linux uses
//     getrandom(2) and has no failure path after boot completes;
//   - the mfa.go:95 GenerateRecoveryCodes failure flank: same rand family
//     (recovery code generation is pure crypto/rand);
//   - the Cut-failure continue at service.go:738: the emails entering the queue
//     are filtered by FindEmailsByDomain's LIKE '%@'+domain predicate, so any
//     matching row contains @ and strings.Cut always succeeds (a
//     self-evidencing double guarantee; the dirty-row case in
//     register_verify_gap_test.go has pinned the SQL-layer premise that "dirty
//     rows without @ never reach the comparison").

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildIdentityProviders_WhitespaceCtorDegrade pins the seam in the three
// providers where "the guard checks non-empty while the ctor checks
// TrimSpace-non-empty": a blank-string credential passes the guard, the ctor
// fails → degrade on failure (slog records the error and the provider is
// returned unset; local and the other sources are unaffected).
func TestBuildIdentityProviders_WhitespaceCtorDegrade(t *testing.T) {
	t.Run("github", func(t *testing.T) {
		cfg := config.AuthProvidersConfig{}
		cfg.GitHub.Enabled = true
		cfg.GitHub.ClientID = " " // guard (== "") lets it through, ctor (TrimSpace) rejects
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

// TestVerifyEmailToken_UnknownToken a token whose hash is not in the table →
// the unified "invalid or expired" 400 semantics — the reason is not
// distinguished, to prevent probing for token validity.
func TestVerifyEmailToken_UnknownToken(t *testing.T) {
	svc, _, _ := verificationFixture(t)
	err := svc.VerifyEmailToken(context.Background(), "no-such-token-in-table")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "验证链接无效或已过期")
}
