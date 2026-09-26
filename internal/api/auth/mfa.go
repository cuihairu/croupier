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
	Secret     string `json:"secret"`
	OtpauthURL string `json:"otpauthUrl"`
	// Issuer 是写入 otpauth URI 的签发方名，前端据此渲染二维码下方的说明文字。
	Issuer      string `json:"issuer"`
	AlreadyDone bool   `json:"alreadyEnabled"`
}

// MFAConfirmResponse 在确认启用后回传一次性备用恢复码。
//
// 明文**只在此处返回一次**：库里只存哈希。用户丢失验证器 App 时可用任一码
// 登录，因此这一步不能只提示「请自行保存」而不给码。
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

// recoveryCodeModel 允许测试注入空实现；生产由 ServiceContext 注入。
func (s *Service) recoveryCodeModel() *model.AdminOTPRecoveryCodeModel {
	if s.otpRecoveryModel == nil {
		return model.NewAdminOTPRecoveryCodeModel(s.recoveryDB)
	}
	return s.otpRecoveryModel
}

// MFASetup 为当前登录的本地账号生成 TOTP secret（未启用，待 confirm）。
func (s *Service) MFASetup(ctx context.Context, username string) (*MFASetupResponse, error) {
	admin, err := s.requireLocalAdmin(ctx, username)
	if err != nil {
		return nil, err
	}
	if admin.OTPEnabled {
		return &MFASetupResponse{AlreadyDone: true, Issuer: otp.DefaultIssuer}, nil
	}
	// otp.GenerateSecret 的唯一 error 来源是 crypto/rand.Read，Go 1.24 起永
	// 不返回错误（失败即进程内 fatal），error 分支不可达，已删除。
	secret, _ := otp.GenerateSecret()
	if err := s.adminModel.SetOTPSecret(ctx, admin.ID, secret); err != nil {
		return nil, fmt.Errorf("保存密钥失败: %w", err)
	}
	// 走 otp.OtpauthURI 而不是手写串：手写版本没有对账号名做 percent-encoding，
	// 含 '@'、'/' 或非 ASCII 的用户名会让验证器 App 解析失败；同时缺
	// algorithm/digits/period 参数，部分 App 会按自己的默认值解析。
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

// MFAConfirm 用首个有效验证码确认启用，并签发一次性备用恢复码。
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
	// 先落库再开开关：恢复码写失败却已启用 MFA，用户将永远拿不到恢复码。
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

// MFADisable 校验验证码 + 密码双重确认后关闭 TOTP，并清空恢复码。
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
	// 恢复码必须随 MFA 一起失效：否则关闭后旧码仍可用于登录校验路径。
	if err := s.recoveryCodeModel().DeleteAll(ctx, admin.ID); err != nil {
		// 不阻断关闭流程，但要让运维看见——残留码是安全隐患。
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

// verifySecondFactor accepts either a valid TOTP code or an unspent recovery
// code. Returns true when the login may proceed.
//
// TOTP 优先：绝大多数登录走动态码路径，恢复码只在用户明确输入时才会命中，
// 因此这里不会给恢复码消耗留下可被绕过的窗口。
func (s *Service) verifySecondFactor(ctx context.Context, admin *model.Admin, req *LoginRequest) bool {
	if code := strings.TrimSpace(req.TOTPCode); code != "" {
		if otp.VerifyTOTP(admin.OTPSecret, code, 1) {
			return true
		}
	}
	// 非 6 位纯数字的输入不可能是恢复码（恢复码是 10 位字母数字），
	// 直接判失败，避免无谓的哈希与查询。
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

// MFAStatusResponse 报告当前账号的 TOTP 启用状态。
// local=false 表示外部身份源账号（MFA 由 IdP 管理，平台侧不可配置）。
type MFAStatusResponse struct {
	Enabled bool `json:"enabled"`
	Local   bool `json:"local"`
	// RecoveryCodesRemaining 让设置页能提示「剩余恢复码过少」。
	RecoveryCodesRemaining int `json:"recoveryCodesRemaining"`
	// RecoveryCodeTotal 是签发总数（便于算出已用数量）。
	RecoveryCodeTotal int `json:"recoveryCodeTotal"`
}

// MFAStatus 查询当前登录账号的两步验证状态。
// 实现无出错路径（账号缺失即返回未启用状态），已收紧签名去掉 error 返回。
func (s *Service) MFAStatus(ctx context.Context, username string) *MFAStatusResponse {
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil || admin == nil {
		return &MFAStatusResponse{}
	}
	local := admin.PasswordHash != ""
	enabled := local && admin.OTPEnabled
	out := &MFAStatusResponse{Enabled: enabled, Local: local}
	if enabled {
		// 统计失败不应让状态查询失败：剩余数量只是提示。
		if n, err := s.CountRecoveryCodes(ctx, admin.ID); err != nil {
			slog.Default().Warn("统计剩余恢复码失败", "username", username, "error", err)
		} else {
			out.RecoveryCodesRemaining = n
			out.RecoveryCodeTotal = otp.RecoveryCodeCount
		}
	}
	return out
}
