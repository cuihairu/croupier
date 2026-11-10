package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
)

// LocalEnabled 报告账号密码登录是否启用（auth.local.enabled，默认启用）。
func (s *Service) LocalEnabled() bool {
	s.providersMu.RLock()
	defer s.providersMu.RUnlock()
	return !s.localDisabled
}

// RegisterEnabled 报告自助注册是否开启（auth.register.enabled，默认关闭）。
func (s *Service) RegisterEnabled() bool {
	s.providersMu.RLock()
	defer s.providersMu.RUnlock()
	return s.registerEnabled
}

// RegisterRequest 是自助注册入参（POST /api/v1/auth/register，匿名可访问）。
type RegisterRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	ClientIP  string `json:"-"`
	UserAgent string `json:"-"`
}

// Register 自助注册本地账号（OPEN-ISSUES #51b，默认关闭）。密码经
// security.* 策略校验（与建号/改密同链）；成功后赋默认角色。不签发
// token——注册完成走正常登录（mustChangePassword/MFA 门控照常生效）。
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*model.Admin, error) {
	if !s.RegisterEnabled() {
		return nil, errors.New("自助注册未开启")
	}
	username := strings.TrimSpace(req.Username)
	if !isValidRegisterUsername(username) {
		return nil, errors.New("用户名须为 3-32 位字母、数字、下划线或连字符")
	}
	if err := utils.ValidatePasswordForUser(req.Password, username); err != nil {
		return nil, err
	}
	if existing, err := s.adminModel.FindByUsername(ctx, username); err == nil && existing != nil {
		return nil, fmt.Errorf("用户名已存在")
	}
	email := strings.TrimSpace(req.Email)
	if emailVerificationRequired() && email == "" {
		// required 开启后邮箱从选填变必填：没有收件地址就没有可验证之物。
		return nil, errorx.NewBadRequest("已开启注册邮箱验证，注册必须填写邮箱")
	}
	if email != "" {
		if err := s.checkRegisterEmailPolicy(ctx, email); err != nil {
			return nil, err
		}
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = username
	}
	admin := &model.Admin{
		Username: username,
		Nickname: nickname,
		Email:    email,
		Status:   1,
	}
	if err := s.adminModel.Create(ctx, admin, req.Password); err != nil {
		// 并发重名：唯一索引兜底为失败
		s.recordLoginAudit(ctx, username, "auth.register_failed", "failed",
			&LoginRequest{ClientIP: req.ClientIP, UserAgent: req.UserAgent}, "create_failed", "")
		return nil, fmt.Errorf("注册失败：用户名可能已存在")
	}
	s.assignRegisterRoles(ctx, admin.ID)
	// #51c 第二批：required 开启且填了邮箱 → 签发验证令牌并发信。失败仅
	// 日志（令牌已落库，用户可走重发端点），不影响注册结果。
	s.maybeSendVerificationOnRegister(ctx, admin)
	s.recordLoginAudit(ctx, username, "auth.register", "success",
		&LoginRequest{ClientIP: req.ClientIP, UserAgent: req.UserAgent}, "", "")
	slog.Default().Info("self-registered admin account", "username", username)
	return admin, nil
}

// checkRegisterEmailPolicy 注册邮箱策略（OPEN-ISSUES #51c）。email 非空时
// 依次执行：①基础形态校验（恰好一个 @、域含点）；②域后缀白名单
// （auth.email.domainWhitelist，空 = 不限）；③别名限制
// （auth.email.aliasRestriction）：拒绝 + 别名形态，并按 local 去点小写
// 归一与同域既有账号比对（u.s@example.com 视同 us@example.com）。
// 每次注册实时读 L3——保存即生效，无需接入热刷新装配链。
func (s *Service) checkRegisterEmailPolicy(ctx context.Context, email string) error {
	local, domain, hasAt := strings.Cut(email, "@")
	if !hasAt || local == "" || !strings.Contains(domain, ".") || strings.ContainsAny(email, " \t") {
		return fmt.Errorf("邮箱格式无效")
	}
	cur := settings.Current()
	if cur == nil {
		return nil
	}
	if wl, _, ok := cur.GetString(ctx, settings.KeyAuthEmailDomainWhitelist); ok && strings.TrimSpace(wl) != "" {
		domain = strings.ToLower(strings.TrimSuffix(domain, "."))
		allowed := false
		for _, entry := range strings.Split(wl, ",") {
			entry = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(entry), ".")))
			if entry == "" {
				continue
			}
			if domain == entry || strings.HasSuffix(domain, "."+entry) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("邮箱域名 %s 不在允许清单内", domain)
		}
	}
	if cur.GetBool(settings.KeyAuthEmailAliasRestriction, false) {
		if strings.Contains(local, "+") {
			return fmt.Errorf("邮箱别名（+tag 形态）不允许用于注册")
		}
		canonical := strings.ReplaceAll(strings.ToLower(local), ".", "")
		existing, err := s.adminModel.FindEmailsByDomain(ctx, domain)
		if err == nil {
			for _, e := range existing {
				el, ed, ok2 := strings.Cut(strings.TrimSpace(e), "@")
				if !ok2 {
					continue
				}
				if strings.EqualFold(ed, domain) && strings.ReplaceAll(strings.ToLower(el), ".", "") == canonical {
					return fmt.Errorf("该邮箱（忽略别名与点号后）已被其他账号使用")
				}
			}
		}
	}
	return nil
}

// isValidRegisterUsername 校验注册用户名形态（3-32 位字母/数字/_/-）。
func isValidRegisterUsername(u string) bool {
	if len(u) < 3 || len(u) > 32 {
		return false
	}
	for _, r := range u {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// assignRegisterRoles 赋注册默认角色；角色名不存在时告警跳过（与 JIT 同策略）。
func (s *Service) assignRegisterRoles(ctx context.Context, adminID uint) {
	s.providersMu.RLock()
	names := s.registerRoles
	s.providersMu.RUnlock()
	for _, name := range names {
		role, ok := s.findRoleByName(ctx, name)
		if !ok {
			slog.Default().Warn("register default role not found", "role", name)
			continue
		}
		if err := s.adminModel.AssignRole(ctx, adminID, role.ID); err != nil {
			slog.Default().Warn("assign register default role failed",
				"role", name, "adminID", adminID, "error", err)
		}
	}
}
