// 密码箱装配层覆盖（vault-provider 设计 §4）：enabled=false 不装配、
// 提供商经注册表键构建、启动解锁预检 fail-soft、未知提供商拒绝。
// 验收口径①（todo 4252 批头「真实切换 provider 只改配置」）在
// TestWireVaultProviderSwitchConfigOnly 留证：同一取凭据调用面，仅换
// 配置 provider 键，local/env 两提供商先后交付同一凭据。
package svc

import (
	"context"
	"path/filepath"
	"strings"
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

// ---- 凭据值内引用接入点（todo 4251）----

// refStubProvider 最小 Provider 桩：Read 命中固定表，其余只读报错。
type refStubProvider struct{ entries map[string]string }

func (p *refStubProvider) Name() string                 { return "stub" }
func (p *refStubProvider) Unlock(context.Context) error { return nil }
func (p *refStubProvider) Locked() bool                 { return false }
func (p *refStubProvider) Lock()                        {}
func (p *refStubProvider) Read(_ context.Context, path string) (string, error) {
	if v, ok := p.entries[path]; ok {
		return v, nil
	}
	return "", vault.ErrNotFound
}
func (p *refStubProvider) Write(context.Context, string, string) error { return vault.ErrReadOnly }
func (p *refStubProvider) List(context.Context) ([]vault.EntryMeta, error) {
	return nil, vault.ErrReadOnly
}
func (p *refStubProvider) Delete(context.Context, string) error { return vault.ErrReadOnly }

// DSN 引用展开：展开后的 scheme 驱动 auto 判定（postgres://）。
func TestResolveDriverAndDSN_VaultRefExpanded(t *testing.T) {
	secrets := vault.NewResolver(&refStubProvider{entries: map[string]string{
		"db/dsn": "postgres://u:p@dbhost:5432/gamedb",
	}})
	driver, dsn, err := resolveDriverAndDSN(config.Config{
		Database: config.DatabaseConfig{DataSource: "vault://db/dsn"},
	}, secrets)
	if err != nil {
		t.Fatalf("resolveDriverAndDSN: %v", err)
	}
	if driver != "postgres" {
		t.Fatalf("driver = %q, want postgres (auto-detect on expanded DSN)", driver)
	}
	if dsn != "postgres://u:p@dbhost:5432/gamedb" {
		t.Fatalf("dsn = %q, want expanded", dsn)
	}
}

// nil resolver（CLI 面）：引用原样直通，不展开不报错。
func TestResolveDriverAndDSN_VaultRefNilResolverPassthrough(t *testing.T) {
	driver, dsn, err := resolveDriverAndDSN(config.Config{
		Database: config.DatabaseConfig{DataSource: "vault://db/dsn"},
	}, nil)
	if err != nil {
		t.Fatalf("resolveDriverAndDSN: %v", err)
	}
	if dsn != "vault://db/dsn" {
		t.Fatalf("dsn = %q, want raw ref passthrough", dsn)
	}
	if driver != "sqlite" {
		t.Fatalf("driver = %q, want sqlite (ref has no scheme)", driver)
	}
}

// 解析失败 = 显式错误（fail-closed）：不把引用串当真 DSN 发往驱动。
func TestResolveDriverAndDSN_VaultRefFailureFailsClosed(t *testing.T) {
	secrets := vault.NewResolver(nil)
	_, _, err := resolveDriverAndDSN(config.Config{
		Database: config.DatabaseConfig{DataSource: "vault://db/dsn"},
	}, secrets)
	if err == nil {
		t.Fatal("ref resolve failure must error (fail closed)")
	}
	if !strings.Contains(err.Error(), "db/dsn") {
		t.Fatalf("error = %q, want reference path in message", err.Error())
	}
}

// JWT 密钥内联直通（存量零行为变化）。
func TestResolveJWTSecret_InlinePassthrough(t *testing.T) {
	secret, err := resolveJWTSecret(config.Config{
		Auth: config.AuthConfig{JWTSecret: "inline-jwt-secret"},
	}, vault.NewResolver(nil))
	if err != nil || secret != "inline-jwt-secret" {
		t.Fatalf("secret = %q err = %v, want passthrough", secret, err)
	}
}

// JWT 密钥引用展开：明文来自密码箱（仅内存传递）。
func TestResolveJWTSecret_RefExpanded(t *testing.T) {
	secret, err := resolveJWTSecret(config.Config{
		Auth: config.AuthConfig{JWTSecret: "vault://auth/jwt"},
	}, vault.NewResolver(&refStubProvider{entries: map[string]string{"auth/jwt": "vaulted-jwt-secret"}}))
	if err != nil || secret != "vaulted-jwt-secret" {
		t.Fatalf("secret = %q err = %v, want expanded", secret, err)
	}
}

// JWT 密钥引用解析失败 = 硬错误（fail-closed）：绝不回落 DevSecret。
func TestResolveJWTSecret_RefFailureFailsClosed(t *testing.T) {
	_, err := resolveJWTSecret(config.Config{
		Auth: config.AuthConfig{JWTSecret: "vault://auth/jwt"},
	}, vault.NewResolver(nil))
	if err == nil {
		t.Fatal("ref resolve failure must error, never fall back to DevSecret")
	}
	if strings.Contains(err.Error(), "DevSecret") || err == nil {
		t.Fatalf("error = %v, want explicit resolve failure", err)
	}
}

// wireVault 装配 Resolver：enabled 时 ctx.Vault/ctx.Secrets 同时就位。
func TestWireVaultWiresSecretsResolver(t *testing.T) {
	t.Setenv("CROUPIER_TEST_VAULT_PASS", "svc-pass-2")
	dir := t.TempDir()
	src := map[string]config.VaultSourceConfig{
		"local": {Path: filepath.Join(dir, "vault.json"), PassphraseEnv: "CROUPIER_TEST_VAULT_PASS"},
	}
	ctx := &ServiceContext{Config: vaultConfig(vault.ProviderLocal, src)}
	wireVault(ctx)
	if ctx.Vault == nil || ctx.Secrets == nil {
		t.Fatalf("Vault=%v Secrets=%v, want both wired when enabled", ctx.Vault, ctx.Secrets)
	}
	// 经 Resolver 读引用（写入→读取回环）。
	if err := ctx.Vault.Write(context.Background(), "auth/jwt", "jwt-value"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := ctx.Secrets(context.Background(), "vault://auth/jwt")
	if err != nil || got != "jwt-value" {
		t.Fatalf("Secrets(ref) = %q, %v; want jwt-value", got, err)
	}

	// disabled：Secrets 保持 nil（NewServiceContext 顶部另行装配恒非 nil）。
	ctx2 := &ServiceContext{}
	wireVault(ctx2)
	if ctx2.Secrets != nil {
		t.Fatal("disabled vault must not wire Secrets")
	}
}
