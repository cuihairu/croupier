// 密码箱装配（vault-provider 设计 §4）：config→vault.SourceOptions 逐项
// 映射 + 启动解锁预检。enabled=false 或提供商构建失败 → ctx.Vault=nil
// （调用方零改动维持现状）；预检失败不阻断启动（fail-soft，与 outlet
// token 缺失同口径——调用方持 ErrLocked/ErrPassphraseMissing 自行兜底）。
package svc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/vault"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
)

// newVaultProvider 从配置构建密码箱提供商（todo 4251 装配顺序：DB DSN 等
// 接入点可能是 vault:// 引用，provider 必须先于 openDatabase 就位）。
// enabled=false → nil（存量内联部署零行为变化）；构建失败 → nil + warn。
// 解锁预检 fail-soft：口令缺位/不匹配在启动日志暴露，而非首次取凭据时。
func newVaultProvider(c config.Config) vault.Provider {
	cfg := c.Vault
	if !cfg.Enabled {
		return nil
	}
	p, err := buildVaultProvider(cfg)
	if err != nil {
		slog.WarnContext(context.Background(), "vault enabled but provider build failed; vault stays unwired",
			"provider", cfg.Provider, "error", err)
		return nil
	}
	if err := p.Unlock(context.Background()); err != nil {
		slog.WarnContext(context.Background(), "vault unlock failed at startup; callers get ErrLocked until unlock",
			"provider", p.Name(), "error", err)
	}
	return p
}

// wireVault 把 vault 配置装配成 ctx.Vault / ctx.Secrets。提供商切换=只改
// 配置 vault.provider 键（验收口径①：业务调用面走 Provider/Resolver 接口
// 零改动）。
func wireVault(ctx *ServiceContext) {
	p := newVaultProvider(ctx.Config)
	if p == nil {
		return
	}
	ctx.Vault = p
	if ctx.Secrets == nil {
		ctx.Secrets = vault.NewResolver(p)
	}
	slog.InfoContext(context.Background(), "vault provider wired", "provider", p.Name())
}

// resolveJWTSecret 展开 JWT 密钥值内引用后交 ResolveSecret 判定（todo 4251）。
// 引用解析失败返回错误——调用方必须硬失败，不得回落 DevSecret（fail-closed）。
func resolveJWTSecret(cfg config.Config, secrets vault.Resolver) (string, error) {
	s, err := secrets(context.Background(), cfg.Auth.JWTSecret)
	if err != nil {
		return "", fmt.Errorf("jwt secret resolve: %w", err)
	}
	cfg.Auth.JWTSecret = s
	return jwtutil.ResolveSecret(cfg)
}

// resolveSecretOrPassthrough 展开凭据值内引用（todo 4251）：resolver 为
// nil 或值非引用时原样返回（存量直通）。
func resolveSecretOrPassthrough(ctx context.Context, resolve vault.Resolver, value string) (string, error) {
	if resolve == nil || !vault.IsVaultRef(value) {
		return value, nil
	}
	return resolve(ctx, value)
}

// buildVaultProvider config→SourceOptions 逐项映射（缺省 provider=local）。
func buildVaultProvider(cfg config.VaultConfig) (vault.Provider, error) {
	provider := cfg.Provider
	if provider == "" {
		provider = vault.ProviderLocal
	}
	src := cfg.Providers[provider]
	return vault.Build(provider, vault.SourceOptions{
		Path:          src.Path,
		PassphraseEnv: src.PassphraseEnv,
		Prefix:        src.Prefix,
		SessionTTL:    time.Duration(cfg.SessionTtlSeconds) * time.Second,
	})
}
