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

// MFA 管理：本地账号自助绑定/解绑 TOTP。仅对 local provider 账号开放；
// 外部身份源账号的 MFA 由 IdP 负责，平台侧不维护。

type MFASetupResponse struct {
	// Secret 同时以原文与 otpauth URL 形式返回，便于手动录入与扫码。
	Secret      string `json:"secret"`
	OtpauthURL  string `json:"otpauthUrl"`
	AlreadyDone bool   `json:"alreadyEnabled"`
}

type MFAConfirmRequest struct {
	Code string `json:"code" binding:"required"`
}

type MFADisableRequest struct {
	Code     string `json:"code" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// MFASetup 为当前登录的本地账号生成 TOTP secret（未启用，待 confirm）。
func (s *Service) MFASetup(ctx context.Context, username string) (*MFASetupResponse, error) {
	admin, err := s.requireLocalAdmin(ctx, username)
	if err != nil {
		return nil, err
	}
	if admin.OTPEnabled {
		return &MFASetupResponse{AlreadyDone: true}, nil
	}
	secret, err := otp.GenerateSecret()
	if err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}
	if err := s.adminModel.SetOTPSecret(ctx, admin.ID, secret); err != nil {
		return nil, fmt.Errorf("保存密钥失败: %w", err)
	}
	return &MFASetupResponse{
		Secret:     secret,
		OtpauthURL: fmt.Sprintf("otpauth://totp/Croupier:%s?secret=%s&issuer=Croupier", username, secret),
	}, nil
}

// MFAConfirm 用首个有效验证码确认启用。启用后登录必须携带 totpCode。
func (s *Service) MFAConfirm(ctx context.Context, username, code string) error {
	admin, err := s.requireLocalAdmin(ctx, username)
	if err != nil {
		return err
	}
	if admin.OTPSecret == "" {
		return errors.New("请先获取二次验证密钥")
	}
	if !otp.VerifyTOTP(admin.OTPSecret, strings.TrimSpace(code), 1) {
		return errors.New("验证码错误")
	}
	if err := s.adminModel.EnableOTP(ctx, admin.ID); err != nil {
		return fmt.Errorf("启用失败: %w", err)
	}
	s.recordMfaAudit(ctx, username, audit.EventMFAEnabled)
	return nil
}

// MFADisable 校验验证码 + 密码双重确认后关闭 TOTP。
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
	s.recordMfaAudit(ctx, username, audit.EventMFADisabled)
	return nil
}

// requireLocalAdmin 确认目标账号是本地 provider 管理的账号。
// 判定依据：本地账号必有 password_hash（外部 JIT 影子账号密码为空）。
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
