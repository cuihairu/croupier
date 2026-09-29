// 注册邮箱验证令牌链路（OPEN-ISSUES #51c 第二批）：注册发信、登录拦截、
// 公开验证与重发端点的 service 层。令牌只存 SHA-256 哈希（见
// model.EmailVerification）；每次注册/重发实时读 L3——保存即生效。
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/settings"
)

// ErrEmailNotVerified 表示本地账号邮箱未验证被登录拦截。CodeError 携带
// 403 + StableCode=email_not_verified：经统一 response.Error 输出，前端按
// error 码分支展示「重新发送验证邮件」入口，不读 message。
var ErrEmailNotVerified = &errorx.CodeError{
	Code:       http.StatusForbidden,
	Message:    "邮箱尚未验证，请查收验证邮件或重新发送后再登录",
	StableCode: "email_not_verified",
}

const (
	emailVerificationPurpose = "register"
	// emailVerificationTTL 验证令牌有效期（诚实边界：常量非键控，需要
	// 站点级可配时再升级为 settings 键）。
	emailVerificationTTL = 24 * time.Hour
	// resendVerificationMinInterval 重发频控：同一账号两次发信最小间隔。
	// 公开端点防刷信（对管理员有意义的重发节奏远大于此）。
	resendVerificationMinInterval = time.Minute
)

// sendVerificationEmailFn 可注入缝隙（同 sitesettings.sendTestEmailFn /
// ops.systemUpdateFetchFn 口径）：按 L3 当前 SMTP 配置即时构造 EmailSender
// 真实发信。siteURL 为空时正文降级为纯令牌文本（无法拼前端链接的部署仍可
// 手工传达）。
var sendVerificationEmailFn = func(ctx context.Context, siteURL, to, username, token string) error {
	smtpCfg := settings.Current().NotifySMTP()
	sender := approvals.NewEmailSender(smtpCfg.Host, smtpCfg.Port, smtpCfg.User, smtpCfg.Password, smtpCfg.From).
		WithTransport(smtpCfg.Encryption, smtpCfg.AuthType, smtpCfg.InsecureSkipVerify)
	link := fmt.Sprintf("%s/user/verify-email?token=%s", strings.TrimRight(siteURL, "/"), token)
	if siteURL == "" {
		link = token
	}
	return sender.Send(ctx, to, approvals.NotificationEvent{
		Type:  "email_verification",
		Title: "Croupier 邮箱验证",
		Message: fmt.Sprintf("您好 %s，请在 24 小时内访问以下链接完成邮箱验证：%s（若非本人操作请忽略本邮件）。",
			username, link),
		Priority: "high",
	})
}

// emailVerificationRequired 实时读 L3 开关（settings.Current() nil 安全：
// 未初始化分层设置的测试环境按关闭处理）。
func emailVerificationRequired() bool {
	cur := settings.Current()
	if cur == nil {
		return false
	}
	return cur.GetBool(settings.KeyAuthEmailVerificationRequired, false)
}

// siteServerURL 读站点对外地址（验证邮件链接拼接用；空 = 未配置）。
func siteServerURL() string {
	cur := settings.Current()
	if cur == nil {
		return ""
	}
	return cur.SiteSnapshot().ServerURL
}

// newVerificationToken 生成 64 位 hex 令牌及其 SHA-256 哈希。
func newVerificationToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("生成验证令牌失败: %w", err)
	}
	token = hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

// issueEmailVerification 为账号签发新验证令牌（作废旧令牌）并发信。
// 发信失败仅记日志不回滚——令牌已落库，用户可走重发；注册流程不因 SMTP
// 抖动整体失败（诚实边界：SMTP 未配置/坏掉时收不到邮件，靠重发端点兜底）。
func (s *Service) issueEmailVerification(ctx context.Context, admin *model.Admin) error {
	if s.verificationModel == nil {
		return nil
	}
	token, hash, err := newVerificationToken()
	if err != nil {
		return err
	}
	if err := s.verificationModel.CreateInvalidatingActive(ctx, &model.EmailVerification{
		AdminID:   admin.ID,
		TokenHash: hash,
		Purpose:   emailVerificationPurpose,
		ExpiresAt: time.Now().Add(emailVerificationTTL),
	}); err != nil {
		return err
	}
	if err := sendVerificationEmailFn(ctx, siteServerURL(), admin.Email, admin.Username, token); err != nil {
		slog.Default().Warn("send verification email failed",
			"username", admin.Username, "error", err)
	}
	return nil
}

// maybeSendVerificationOnRegister 注册完成后按需发验证邮件：开关未开或
// 注册时未填邮箱则跳过；required 且邮箱为空在 Register 入口已拦截。
// 发信链路任何失败都不影响注册结果（仅日志）。
func (s *Service) maybeSendVerificationOnRegister(ctx context.Context, admin *model.Admin) {
	if !emailVerificationRequired() || admin.Email == "" {
		return
	}
	if err := s.issueEmailVerification(ctx, admin); err != nil {
		slog.Default().Warn("issue email verification after register failed",
			"username", admin.Username, "error", err)
	}
}

// emailVerificationBlocksLogin 判定该账号是否被邮箱验证门拦截：仅 local
// provider（LDAP/OIDC 的邮箱归属 IdP 职责）、开关开启、账号填过邮箱且
// 尚未验证。存量无邮箱账号不拦——没有可验证之物。
func emailVerificationBlocksLogin(admin *model.Admin) bool {
	return admin.Email != "" && !admin.EmailVerified
}

// VerifyEmailToken 校验令牌并落 email_verified。无效/过期/已用统一返回
// errors.New（handler 映射 400）——不区分原因，避免令牌有效性探测。
func (s *Service) VerifyEmailToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errorx.NewBadRequest("token 不能为空")
	}
	if s.verificationModel == nil {
		return errors.New("验证服务不可用")
	}
	sum := sha256.Sum256([]byte(token))
	row, err := s.verificationModel.FindValidByTokenHash(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		return err
	}
	if row == nil {
		return errorx.NewBadRequest("验证链接无效或已过期，请重新发送验证邮件")
	}
	ok, err := s.verificationModel.Consume(ctx, row)
	if err != nil {
		return err
	}
	if !ok {
		return errorx.NewBadRequest("验证链接无效或已过期，请重新发送验证邮件")
	}
	return nil
}

// ResendVerification 按用户名+邮箱重发验证邮件（匿名端点）。防枚举：
// 用户名不存在/邮箱不匹配/已验证/频控内 一律静默成功（不透出账号状态），
// 仅真实匹配且未验证时发信。
func (s *Service) ResendVerification(ctx context.Context, username, email string) error {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	if username == "" || email == "" {
		return errorx.NewBadRequest("username 与 email 不能为空")
	}
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil || admin == nil || !strings.EqualFold(strings.TrimSpace(admin.Email), email) {
		return nil
	}
	if admin.EmailVerified {
		return nil
	}
	if s.verificationModel != nil {
		last, err := s.verificationModel.LatestCreatedAtFor(ctx, admin.ID, emailVerificationPurpose)
		if err == nil && !last.IsZero() && time.Since(last) < resendVerificationMinInterval {
			return nil // 频控内静默：不发也不报
		}
	}
	return s.issueEmailVerification(ctx, admin)
}
