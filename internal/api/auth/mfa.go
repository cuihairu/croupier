package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/otp"
)

// MFA management: local accounts self-bind/unbind TOTP. Only open to local
// provider accounts; MFA for external identity provider accounts is the
// IdP's responsibility and is not maintained on the platform side.

type MFASetupResponse struct {
	// Secret is returned both as raw text and as an otpauth URL, to make
	// manual entry and QR scanning easy.
	Secret     string `json:"secret"`
	OtpauthURL string `json:"otpauthUrl"`
	// Issuer is the issuer name written into the otpauth URI; the frontend
	// renders the caption text under the QR code from it.
	Issuer      string `json:"issuer"`
	AlreadyDone bool   `json:"alreadyEnabled"`
}

// MFAConfirmResponse hands back one-time backup recovery codes after enabling
// is confirmed.
//
// The plaintext is **returned exactly once, right here**: only hashes are
// stored. A user who loses their authenticator app can log in with any code,
// so this step must not merely say "save these yourself" without providing
// them.
type MFAConfirmResponse struct {
	RecoveryCodes []string `json:"recoveryCodes"`
}

type MFAConfirmRequest struct {
	Code string `json:"code" binding:"required"`
}

type MFADisableRequest struct {
	Code     string `json:"code" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// recoveryCodeModel lets tests inject a no-op implementation; production gets
// it from ServiceContext.
func (s *Service) recoveryCodeModel() *model.AdminOTPRecoveryCodeModel {
	if s.otpRecoveryModel == nil {
		return model.NewAdminOTPRecoveryCodeModel(s.recoveryDB)
	}
	return s.otpRecoveryModel
}

// MFASetup generates a TOTP secret for the logged-in local account (not yet
// enabled, awaiting confirm).
func (s *Service) MFASetup(ctx context.Context, username string) (*MFASetupResponse, error) {
	admin, err := s.requireLocalAdmin(ctx, username)
	if err != nil {
		return nil, err
	}
	if admin.OTPEnabled {
		return &MFASetupResponse{AlreadyDone: true, Issuer: otp.DefaultIssuer}, nil
	}
	// The only error source of otp.GenerateSecret is crypto/rand.Read, which
	// since Go 1.24 never returns an error (a failure is a fatal in-process
	// condition); the error branch is unreachable and has been removed.
	secret, _ := otp.GenerateSecret()
	if err := s.adminModel.SetOTPSecret(ctx, admin.ID, secret); err != nil {
		return nil, fmt.Errorf("保存密钥失败: %w", err)
	}
	// Use otp.OtpauthURI instead of a hand-written string: the hand-rolled
	// version does not percent-encode the account name, so usernames containing
	// '@', '/', or non-ASCII characters make verifier apps fail to parse it; it
	// also omits the algorithm/digits/period parameters, and some apps fall
	// back to their own defaults.
	uri, err := otp.OtpauthURI(otp.DefaultIssuer, username, secret)
	if err != nil {
		return nil, fmt.Errorf("生成绑定链接失败: %w", err)
	}
	return &MFASetupResponse{
		Secret:     secret,
		OtpauthURL: uri,
		Issuer:     otp.DefaultIssuer,
	}, nil
}

// MFAConfirm confirms enabling with the first valid code and issues one-time
// backup recovery codes.
func (s *Service) MFAConfirm(ctx context.Context, username, code string) (*MFAConfirmResponse, error) {
	admin, err := s.requireLocalAdmin(ctx, username)
	if err != nil {
		return nil, err
	}
	if admin.OTPSecret == "" {
		return nil, errors.New("请先获取二次验证密钥")
	}
	if !otp.VerifyTOTP(admin.OTPSecret, strings.TrimSpace(code), 1) {
		return nil, errors.New("验证码错误")
	}
	codes, err := otp.GenerateRecoveryCodes()
	if err != nil {
		return nil, fmt.Errorf("生成备用恢复码失败: %w", err)
	}
	// Land in the database first, then flip the switch: if the recovery codes
	// fail to write while MFA is already enabled, the user will never get a
	// recovery code.
	hashes := make([]string, 0, len(codes))
	for _, c := range codes {
		hashes = append(hashes, otp.HashRecoveryCode(c))
	}
	if err := s.recoveryCodeModel().Replace(ctx, admin.ID, hashes); err != nil {
		return nil, fmt.Errorf("保存备用恢复码失败: %w", err)
	}
	if err := s.adminModel.EnableOTP(ctx, admin.ID); err != nil {
		return nil, fmt.Errorf("启用失败: %w", err)
	}
	s.recordMfaAudit(ctx, username, audit.EventMFAEnabled)
	return &MFAConfirmResponse{RecoveryCodes: codes}, nil
}

// MFADisable verifies the code + password double confirmation and then disables
// TOTP, clearing the recovery codes.
func (s *Service) MFADisable(ctx context.Context, username, code, password string) error {
	admin, err := s.requireLocalAdmin(ctx, username)
	if err != nil {
		return err
	}
	if !admin.OTPEnabled {
		return errors.New("未启用二次验证")
	}
	if !otp.VerifyTOTP(admin.OTPSecret, strings.TrimSpace(code), 1) {
		return errors.New("验证码错误")
	}
	if _, err := s.adminModel.ValidatePassword(ctx, username, password); err != nil {
		return errors.New("密码错误")
	}
	if err := s.adminModel.DisableOTP(ctx, admin.ID); err != nil {
		return fmt.Errorf("关闭失败: %w", err)
	}
	// Recovery codes must be invalidated together with MFA: otherwise the old
	// codes could still be used by the login verification path after it is off.
	if err := s.recoveryCodeModel().DeleteAll(ctx, admin.ID); err != nil {
		// Do not block the disable flow, but make it visible to operators —
		// leftover codes are a security hole.
		slog.Default().Error("关闭两步验证后清理恢复码失败", "username", username, "error", err)
	}
	s.recordMfaAudit(ctx, username, audit.EventMFADisabled)
	return nil
}

// ConsumeRecoveryCode validates a recovery code and marks it spent.
//
// Returns whether the code was valid. A malformed code is rejected before any
// database access.
func (s *Service) ConsumeRecoveryCode(ctx context.Context, adminID uint, code string) (bool, error) {
	candidate := strings.TrimSpace(code)
	if !otp.IsWellFormedRecoveryCode(candidate) {
		return false, nil
	}
	ok, err := s.recoveryCodeModel().Consume(ctx, adminID, otp.HashRecoveryCode(candidate))
	if err != nil {
		return false, err
	}
	return ok, nil
}

// CountRecoveryCodes returns how many unused recovery codes remain. Used by the
// settings UI to warn the user when they are running low.
func (s *Service) CountRecoveryCodes(ctx context.Context, adminID uint) (int, error) {
	n, err := s.recoveryCodeModel().CountUsable(ctx, adminID)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// requireLocalAdmin confirms the target account is managed by the local
// provider. The tell is that a local account always has a password_hash
// (external JIT shadow accounts have an empty password).
func (s *Service) requireLocalAdmin(ctx context.Context, username string) (*model.Admin, error) {
	a, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil || a == nil {
		return nil, errors.New("用户不存在")
	}
	if a.PasswordHash == "" {
		return nil, errors.New("外部身份源账号的二次验证由身份提供方管理")
	}
	return a, nil
}

func (s *Service) recordMfaAudit(ctx context.Context, username string, event audit.AuditEventType) {
	if s.auditSvc == nil {
		return
	}
	_, err := s.auditSvc.Log(ctx, event,
		audit.WithActorID(username, "admin", username),
		audit.WithDetails(map[string]interface{}{"provider": "local"}),
	)
	if err != nil {
		slog.Default().Warn("mfa audit failed", "event", event, "username", username, "error", err)
	}
}

// verifySecondFactor accepts either a valid TOTP code or an unspent recovery
// code. Returns true when the login may proceed.
//
// TOTP first: nearly every login takes the dynamic code path, and a recovery
// code only hits when the user explicitly types one, so this leaves no
// bypassable window for recovery-code consumption.
func (s *Service) verifySecondFactor(ctx context.Context, admin *model.Admin, req *LoginRequest) bool {
	if code := strings.TrimSpace(req.TOTPCode); code != "" {
		if otp.VerifyTOTP(admin.OTPSecret, code, 1) {
			return true
		}
	}
	// Input that is not six pure digits cannot be a recovery code (recovery
	// codes are ten alphanumeric characters), so fail immediately and avoid
	// needless hashing and a query.
	candidate := strings.TrimSpace(req.TOTPCode)
	if !otp.IsWellFormedRecoveryCode(candidate) {
		return false
	}
	ok, err := s.ConsumeRecoveryCode(ctx, admin.ID, candidate)
	if err != nil {
		slog.Default().Warn("恢复码校验出错", "username", admin.Username, "error", err)
		return false
	}
	return ok
}

// MFAStatusResponse reports the current account's TOTP enablement status.
// local=false means an external identity provider account (MFA is managed by
// the IdP and is not configurable on the platform side).
type MFAStatusResponse struct {
	Enabled bool `json:"enabled"`
	Local   bool `json:"local"`
	// RecoveryCodesRemaining lets the settings page warn when the remaining
	// codes run low.
	RecoveryCodesRemaining int `json:"recoveryCodesRemaining"`
	// RecoveryCodeTotal is the total number issued (convenient for computing
	// how many have been used).
	RecoveryCodeTotal int `json:"recoveryCodeTotal"`
}

// MFAStatus queries the current account's two-factor verification status.
// The implementation has no error path (a missing account simply yields the
// not-enabled status), so the signature was tightened to drop the error return.
func (s *Service) MFAStatus(ctx context.Context, username string) *MFAStatusResponse {
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil || admin == nil {
		return &MFAStatusResponse{}
	}
	local := admin.PasswordHash != ""
	enabled := local && admin.OTPEnabled
	out := &MFAStatusResponse{Enabled: enabled, Local: local}
	if enabled {
		// a counting failure must not fail the status query: the remaining
		// count is only a hint.
		if n, err := s.CountRecoveryCodes(ctx, admin.ID); err != nil {
			slog.Default().Warn("统计剩余恢复码失败", "username", username, "error", err)
		} else {
			out.RecoveryCodesRemaining = n
			out.RecoveryCodeTotal = otp.RecoveryCodeCount
		}
	}
	return out
}
