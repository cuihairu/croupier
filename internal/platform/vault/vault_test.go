package vault

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 密码箱批红线进用例：密钥不进仓库、不落盘（密文文件不含明文）；解锁
// 口令与明文凭据不落日志、不出现在错误消息。

func localTestProvider(t *testing.T, path string, ttl time.Duration, now func() time.Time) *LocalProvider {
	t.Helper()
	p, err := NewLocalProvider(SourceOptions{Path: path, PassphraseEnv: "VAULT_TEST_PASS", SessionTTL: ttl, Now: now})
	if err != nil {
		t.Fatalf("NewLocalProvider: %v", err)
	}
	return p
}

func unlockOK(t *testing.T, p Provider) {
	t.Helper()
	if err := p.Unlock(context.Background()); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestLocalUnlockCreatesStore(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "correct horse battery staple")
	path := filepath.Join(t.TempDir(), "vault.json")
	p := localTestProvider(t, path, 0, nil)
	unlockOK(t, p)
	if p.Locked() {
		t.Fatal("unlocked provider reports Locked")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("store file: %v", err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("store file perm = %v, want 0600", fi.Mode().Perm())
	}
	for _, probe := range []string{"correct horse", "staple", `"` + DefaultPassphraseEnv} {
		if bytes.Contains(raw, []byte(probe)) {
			t.Fatalf("store file leaks passphrase-like bytes (probe %q)", probe)
		}
	}
}

func TestLocalUnlockMissingPassphrase(t *testing.T) {
	// 不 Setenv：口令环境变量缺位必须显式报错（不静默降级）。
	p := localTestProvider(t, filepath.Join(t.TempDir(), "vault.json"), 0, nil)
	if err := p.Unlock(context.Background()); !errors.Is(err, ErrPassphraseMissing) {
		t.Fatalf("Unlock err = %v, want ErrPassphraseMissing", err)
	}
}

func TestLocalUnlockWrongPassphrase(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "right-pass")
	path := filepath.Join(t.TempDir(), "vault.json")
	p := localTestProvider(t, path, 0, nil)
	unlockOK(t, p)
	p.Write(context.Background(), "smtp/password", "s3cret")
	p.Lock()

	t.Setenv("VAULT_TEST_PASS", "WRONG-pass-guess")
	err := p.Unlock(context.Background())
	if !errors.Is(err, ErrPassphraseMismatch) {
		t.Fatalf("Unlock err = %v, want ErrPassphraseMismatch", err)
	}
	// 红线：错误消息不得回显口令。
	if strings.Contains(err.Error(), "WRONG-pass-guess") {
		t.Fatalf("error message leaks passphrase: %q", err.Error())
	}
	if _, err := p.Read(context.Background(), "smtp/password"); !errors.Is(err, ErrLocked) {
		t.Fatalf("Read after failed unlock err = %v, want ErrLocked", err)
	}
}

func TestLocalRoundtripAndNoPlaintextOnDisk(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "master-pass-1")
	path := filepath.Join(t.TempDir(), "vault.json")
	p := localTestProvider(t, path, 0, nil)
	unlockOK(t, p)

	secret := "merchant-key-DO-NOT-LEAK-9001"
	if err := p.Write(context.Background(), "channels/payment", secret); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := p.Read(context.Background(), "channels/payment")
	if err != nil || got != secret {
		t.Fatalf("Read = %q, %v; want %q", got, err, secret)
	}

	raw, _ := os.ReadFile(path)
	if bytes.Contains(raw, []byte(secret)) {
		t.Fatal("store file contains plaintext secret")
	}
	// 覆盖写生效。
	if err := p.Write(context.Background(), "channels/payment", "rotated"); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if got, _ := p.Read(context.Background(), "channels/payment"); got != "rotated" {
		t.Fatalf("rotated read = %q", got)
	}
}

func TestLocalPersistenceAcrossInstances(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "master-pass-1")
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")
	p1 := localTestProvider(t, path, 0, nil)
	unlockOK(t, p1)
	if err := p1.Write(context.Background(), "db/dsn", "mysql://u:p@host/db"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	p1.Lock()

	// 新实例（模拟进程重启）：同一存储 + 同一口令即可解。
	p2 := localTestProvider(t, path, 0, nil)
	unlockOK(t, p2)
	got, err := p2.Read(context.Background(), "db/dsn")
	if err != nil || got != "mysql://u:p@host/db" {
		t.Fatalf("Read after restart = %q, %v", got, err)
	}
	metas, err := p2.List(context.Background())
	if err != nil || len(metas) != 1 || metas[0].Path != "db/dsn" {
		t.Fatalf("List = %+v, %v", metas, err)
	}
}

func TestLocalSessionTTLSlidingWindow(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "ttl-pass")
	now := time.Unix(1_800_000_000, 0)
	clock := now
	p := localTestProvider(t, filepath.Join(t.TempDir(), "vault.json"), 60*time.Second, func() time.Time { return clock })
	unlockOK(t, p)
	if err := p.Write(context.Background(), "smtp/password", "v1"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// 空闲 45s：窗口内可读，且滑动续期。
	clock = now.Add(45 * time.Second)
	if _, err := p.Read(context.Background(), "smtp/password"); err != nil {
		t.Fatalf("read within window: %v", err)
	}
	// 续期后距上次操作 45s（距解锁 90s）仍可读。
	clock = now.Add(90 * time.Second)
	if _, err := p.Read(context.Background(), "smtp/password"); err != nil {
		t.Fatalf("sliding window should extend session: %v", err)
	}
	// 空闲超时：锁定，读报 ErrLocked（不静默返回旧值）。
	clock = now.Add(90*time.Second + 61*time.Second)
	if _, err := p.Read(context.Background(), "smtp/password"); !errors.Is(err, ErrLocked) {
		t.Fatalf("expired session err = %v, want ErrLocked", err)
	}
	// 重新解锁恢复。
	t.Setenv("VAULT_TEST_PASS", "ttl-pass") // 同一进程 env 未变；显式重申意图
	if err := p.Unlock(context.Background()); err != nil {
		t.Fatalf("re-Unlock: %v", err)
	}
	if got, err := p.Read(context.Background(), "smtp/password"); err != nil || got != "v1" {
		t.Fatalf("after re-unlock = %q, %v", got, err)
	}
}

func TestLocalLockClearsSession(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "lock-pass")
	p := localTestProvider(t, filepath.Join(t.TempDir(), "vault.json"), 0, nil)
	unlockOK(t, p)
	if err := p.Write(context.Background(), "a/b", "v"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	p.Lock()
	if !p.Locked() {
		t.Fatal("Locked() = false after Lock()")
	}
	for name, call := range map[string]func() error{
		"read":   func() error { _, err := p.Read(context.Background(), "a/b"); return err },
		"write":  func() error { return p.Write(context.Background(), "a/c", "v") },
		"list":   func() error { _, err := p.List(context.Background()); return err },
		"delete": func() error { return p.Delete(context.Background(), "a/b") },
	} {
		if err := call(); !errors.Is(err, ErrLocked) {
			t.Fatalf("%s after Lock err = %v, want ErrLocked", name, err)
		}
	}
}

func TestLocalListAndDelete(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "list-pass")
	p := localTestProvider(t, filepath.Join(t.TempDir(), "vault.json"), 0, nil)
	unlockOK(t, p)
	ctx := context.Background()
	for _, path := range []string{"b/c", "a/b", "a/c"} {
		if err := p.Write(ctx, path, "v-"+path); err != nil {
			t.Fatalf("Write %s: %v", path, err)
		}
	}
	metas, err := p.List(ctx)
	if err != nil || len(metas) != 3 {
		t.Fatalf("List = %+v, %v", metas, err)
	}
	if metas[0].Path != "a/b" || metas[1].Path != "a/c" || metas[2].Path != "b/c" {
		t.Fatalf("List not sorted: %+v", metas)
	}
	if err := p.Delete(ctx, "a/b"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := p.Delete(ctx, "a/b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("double Delete err = %v, want ErrNotFound", err)
	}
	if _, err := p.Read(ctx, "a/b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read deleted err = %v, want ErrNotFound", err)
	}
}

func TestLocalInvalidPath(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "path-pass")
	p := localTestProvider(t, filepath.Join(t.TempDir(), "vault.json"), 0, nil)
	unlockOK(t, p)
	for _, path := range []string{"", " spaced", "trailing ", "..", "a/../b", "a//b", "/abs"} {
		if err := p.Write(context.Background(), path, "v"); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("Write path %q err = %v, want ErrInvalidPath", path, err)
		}
	}
}

func TestEnvProviderMapping(t *testing.T) {
	p := NewEnvProvider(SourceOptions{Prefix: "TEST_VAULT_"})
	t.Setenv("TEST_VAULT_SMTP_PASSWORD", "mail-pass")
	t.Setenv("TEST_VAULT_CHANNELS_PAYMENT", "pay-key")

	for _, path := range []string{"smtp/password", "smtp_password"} {
		got, err := p.Read(context.Background(), path)
		if err != nil || got != "mail-pass" {
			t.Fatalf("Read %q = %q, %v", path, got, err)
		}
	}
	if _, err := p.Read(context.Background(), "missing/entry"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing err = %v, want ErrNotFound", err)
	}
	if err := p.Write(context.Background(), "x/y", "v"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Write err = %v, want ErrReadOnly", err)
	}
	if err := p.Delete(context.Background(), "x/y"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Delete err = %v, want ErrReadOnly", err)
	}

	t.Setenv("TEST_OTHER_UNRELATED", "no") // 无前缀变量不得出现在 List
	metas, err := p.List(context.Background())
	if err != nil || len(metas) != 2 {
		t.Fatalf("List = %+v, %v; want 2 entries", metas, err)
	}
	if metas[0].Path != "channels_payment" || metas[1].Path != "smtp_password" {
		t.Fatalf("List paths = %+v", metas)
	}
	// 恒解锁。
	if p.Locked() {
		t.Fatal("env provider reports Locked")
	}
	if err := p.Unlock(context.Background()); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestRegistryBuildAndUnknown(t *testing.T) {
	for _, name := range []string{ProviderLocal, ProviderEnv} {
		if Lookup(name) == nil {
			t.Fatalf("provider %q not registered", name)
		}
	}
	if _, err := Build("no-such-provider", SourceOptions{}); err == nil ||
		!strings.Contains(err.Error(), "no-such-provider") {
		t.Fatalf("Build unknown err = %v, want named error", err)
	}
	// 未注册键报错时点名已知键（诊断不瞎猜）。
	if _, err := Build("no-such-provider", SourceOptions{}); err != nil {
		for _, known := range Names() {
			if !strings.Contains(err.Error(), known) {
				t.Fatalf("Build error %q missing known provider %q", err.Error(), known)
			}
		}
	}
	// 同名覆盖（测试注入替身通道）。
	RegisterFactory("test-dup", func(SourceOptions) (Provider, error) {
		return NewEnvProvider(SourceOptions{Prefix: "DUP_"}), nil
	})
	t.Cleanup(func() { RegisterFactory("test-dup", nil) })
	if p, err := Build("test-dup", SourceOptions{}); err != nil || p.Name() != ProviderEnv {
		t.Fatalf("override build = %v, %v", p, err)
	}
}

// ---- 值内引用解析（todo 4251：vault:// 统一凭据引用形态）----

// stubProvider 最小 Provider 桩：只支持 Read 命中固定表，其余报错。
type stubProvider struct{ entries map[string]string }

func (s *stubProvider) Name() string                 { return "stub" }
func (s *stubProvider) Unlock(context.Context) error { return nil }
func (s *stubProvider) Locked() bool                 { return false }
func (s *stubProvider) Lock()                        {}
func (s *stubProvider) Read(_ context.Context, path string) (string, error) {
	if v, ok := s.entries[path]; ok {
		return v, nil
	}
	return "", ErrNotFound
}
func (s *stubProvider) Write(context.Context, string, string) error { return ErrReadOnly }
func (s *stubProvider) List(context.Context) ([]EntryMeta, error)   { return nil, ErrReadOnly }
func (s *stubProvider) Delete(context.Context, string) error        { return ErrReadOnly }

func TestIsVaultRef(t *testing.T) {
	for _, v := range []string{"vault://smtp/password", "vault://x"} {
		if !IsVaultRef(v) {
			t.Fatalf("IsVaultRef(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"", "inline-secret", " Vault://x", "mysql://root@host/db"} {
		if IsVaultRef(v) {
			t.Fatalf("IsVaultRef(%q) = true, want false", v)
		}
	}
}

// 内联值直通（存量部署零行为变化）：provider 为 nil 也不得改动内联值。
func TestResolveInlinePassthrough(t *testing.T) {
	got, err := Resolve(context.Background(), nil, "plain-secret")
	if err != nil || got != "plain-secret" {
		t.Fatalf("Resolve(inline) = %q, %v; want passthrough", got, err)
	}
	got, err = Resolve(context.Background(), nil, "")
	if err != nil || got != "" {
		t.Fatalf("Resolve(empty) = %q, %v; want passthrough", got, err)
	}
}

// 引用展开：ref 经 Provider.Read 展开为明文（仅内存传递）。
func TestResolveRefReadsProvider(t *testing.T) {
	p := &stubProvider{entries: map[string]string{"smtp/password": "s3cret"}}
	got, err := Resolve(context.Background(), p, VaultRefPrefix+"smtp/password")
	if err != nil || got != "s3cret" {
		t.Fatalf("Resolve(ref) = %q, %v; want s3cret", got, err)
	}
	// Resolver 适配层同语义。
	r := NewResolver(p)
	got, err = r(context.Background(), VaultRefPrefix+"smtp/password")
	if err != nil || got != "s3cret" {
		t.Fatalf("Resolver(ref) = %q, %v; want s3cret", got, err)
	}
}

// nil provider + 引用 = fail-closed：显式报错，绝不把引用串当真凭据外发。
func TestResolveRefNilProviderFailsClosed(t *testing.T) {
	_, err := Resolve(context.Background(), nil, VaultRefPrefix+"smtp/password")
	if err == nil {
		t.Fatal("Resolve(ref, nil provider) must error")
	}
	// 红线：错误消息只含引用路径，不含明文（此处无明文可泄，验证消息形态）。
	if !strings.Contains(err.Error(), "smtp/password") || !strings.Contains(err.Error(), "no vault provider") {
		t.Fatalf("error message = %q, want path + fail-closed hint", err.Error())
	}
}

// 引用无条目 / 会话锁定：透传 Provider 错误闭集，不吞不裹明文。
func TestResolveRefProviderErrors(t *testing.T) {
	p := &stubProvider{entries: map[string]string{}}
	if _, err := Resolve(context.Background(), p, VaultRefPrefix+"nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
