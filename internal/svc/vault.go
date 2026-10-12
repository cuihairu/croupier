// 密码箱装配（vault-provider 设计 §4）：config→vault.SourceOptions 逐项
// 映射 + 启动解锁预检。enabled=false 或提供商构建失败 → ctx.Vault=nil
// （调用方零改动维持现状）；预检失败不阻断启动（fail-soft，与 outlet
// token 缺失同口径——调用方持 ErrLocked/ErrPassphraseMissing 自行兜底）。
package svc

import (
	"context"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/vault"
)

// wireVault 把 vault 配置装配成 ctx.Vault。提供商切换=只改配置
// vault.provider 键（验收口径①：业务调用面走 Provider 接口零改动）。
func wireVault(ctx *ServiceContext) {
	cfg := ctx.Config.Vault
	if !cfg.Enabled {
		return
	}
	p, err := buildVaultProvider(cfg)
	if err != nil {
		slog.WarnContext(context.Background(), "vault enabled but provider build failed; vault stays unwired",
			"provider", cfg.Provider, "error", err)
		return
	}
	// 启动解锁预检：口令缺位/不匹配在部署时暴露，而非首次取凭据时。
	if err := p.Unlock(context.Background()); err != nil {
		slog.WarnContext(context.Background(), "vault unlock failed at startup; callers get ErrLocked until unlock",
			"provider", p.Name(), "error", err)
	}
	ctx.Vault = p
	slog.InfoContext(context.Background(), "vault provider wired", "provider", p.Name())
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
