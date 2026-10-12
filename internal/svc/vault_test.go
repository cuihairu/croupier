// 密码箱装配层覆盖（vault-provider 设计 §4）：enabled=false 不装配、
// 提供商经注册表键构建、启动解锁预检 fail-soft、未知提供商拒绝。
// 验收口径①（todo 4252 批头「真实切换 provider 只改配置」）在
// TestWireVaultProviderSwitchConfigOnly 留证：同一取凭据调用面，仅换
// 配置 provider 键，local/env 两提供商先后交付同一凭据。
package svc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/vault"
)

// vaultConsumer 模拟业务接入点：只依赖 Provider 接口，不知道也不关心
// 背后是哪个提供商（4251 接入点改造的目标形态）。
func vaultConsumer(p vault.Provider, path string) (string, error) {
	return p.Read(context.Background(), path)
}

func vaultConfig(provider string, providers map[string]config.VaultSourceConfig) config.Config {
	return config.Config{Vault: config.VaultConfig{
		Enabled:   true,
		Provider:  provider,
		Providers: providers,
	}}
}

func TestWireVaultDisabledStaysNil(t *testing.T) {
	ctx := &ServiceContext{}
	wireVault(ctx)
	if ctx.Vault != nil {
		t.Fatal("disabled vault must stay nil (zero behavior change)")
	}
}

func TestWireVaultLocalRoundtrip(t *testing.T) {
	t.Setenv("CROUPIER_TEST_VAULT_PASS", "svc-pass-1")
	path := filepath.Join(t.TempDir(), "vault.json")
	ctx := &ServiceContext{Config: vaultConfig(vault.ProviderLocal, map[string]config.VaultSourceConfig{
		vault.ProviderLocal: {Path: path, PassphraseEnv: "CROUPIER_TEST_VAULT_PASS"},
	})}
	wireVault(ctx)
	if ctx.Vault == nil {
		t.Fatal("enabled local vault must be wired")
	}
	if err := ctx.Vault.Write(context.Background(), "channels/payment", "merchant-key"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := vaultConsumer(ctx.Vault, "channels/payment")
	if err != nil || got != "merchant-key" {
		t.Fatalf("consumer read = %q, %v", got, err)
	}
}

// 验收口径①留证：同一消费者调用面，仅换配置 provider 键（local → env），
// 凭据照样取到——业务代码零改动。
func TestWireVaultProviderSwitchConfigOnly(t *testing.T) {
	t.Setenv("CROUPIER_TEST_VAULT_PASS", "svc-pass-1")
	t.Setenv("TEST_SWITCH_SMTP_PASSWORD", "smtp-secret-via-env")
	path := filepath.Join(t.TempDir(), "vault.json")

	ctx := &ServiceContext{Config: vaultConfig(vault.ProviderLocal, map[string]config.VaultSourceConfig{
		vault.ProviderLocal: {Path: path, PassphraseEnv: "CROUPIER_TEST_VAULT_PASS"},
	})}
	wireVault(ctx)
	if ctx.Vault == nil || ctx.Vault.Name() != vault.ProviderLocal {
		t.Fatalf("first config should wire local, got %v", ctx.Vault)
	}
	if err := ctx.Vault.Write(context.Background(), "smtp/password", "smtp-secret-via-env"); err != nil {
		t.Fatalf("local write: %v", err)
	}
	got, err := vaultConsumer(ctx.Vault, "smtp/password")
	if err != nil || got != "smtp-secret-via-env" {
		t.Fatalf("local consumer read = %q, %v", got, err)
	}

	// 只改配置：provider 切到 env（凭据同值，来源换成编排层环境变量）。
	ctx2 := &ServiceContext{Config: vaultConfig(vault.ProviderEnv, map[string]config.VaultSourceConfig{
		vault.ProviderEnv: {Prefix: "TEST_SWITCH_"},
	})}
	wireVault(ctx2)
	if ctx2.Vault == nil || ctx2.Vault.Name() != vault.ProviderEnv {
		t.Fatalf("switched config should wire env, got %v", ctx2.Vault)
	}
	got, err = vaultConsumer(ctx2.Vault, "smtp/password")
	if err != nil || got != "smtp-secret-via-env" {
		t.Fatalf("env consumer read = %q, %v", got, err)
	}
}

func TestWireVaultUnknownProviderRejected(t *testing.T) {
	ctx := &ServiceContext{Config: vaultConfig("no-such", nil)}
	wireVault(ctx)
	if ctx.Vault != nil {
		t.Fatal("unknown provider must leave vault unwired")
	}
}

func TestWireVaultStartupUnlockWarnNotFatal(t *testing.T) {
	// 口令环境变量缺位：预检失败但不阻断（fail-soft），provider 仍在位，
	// 调用方拿 ErrLocked 自行兜底（与 outlet token 缺失同口径）。
	path := filepath.Join(t.TempDir(), "vault.json")
	ctx := &ServiceContext{Config: vaultConfig(vault.ProviderLocal, map[string]config.VaultSourceConfig{
		vault.ProviderLocal: {Path: path, PassphraseEnv: "CROUPIER_TEST_VAULT_PASS_MISSING"},
	})}
	wireVault(ctx)
	if ctx.Vault == nil {
		t.Fatal("unlock failure must not unwire the provider")
	}
	if !ctx.Vault.Locked() {
		t.Fatal("vault without passphrase must report Locked")
	}
	if _, err := vaultConsumer(ctx.Vault, "smtp/password"); err == nil {
		t.Fatal("locked vault consumer read must fail")
	}
}
