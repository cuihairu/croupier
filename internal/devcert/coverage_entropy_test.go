package devcert

import (
	"crypto/rand"
	"errors"
	"io"
	"testing"
)

// failingRandReader 模拟熵源读取失败。
type failingRandReader struct{}

func (failingRandReader) Read(p []byte) (int, error) { return 0, errors.New("entropy unavailable") }

// swapRandReader 用 r 替换 crypto/rand.Reader，测试结束后恢复。
// Go 1.26 语义：crypto/rand.Prime（rsa.GenerateKey 内部）默认忽略自定义
// reader、改用系统熵源，仅当 GODEBUG=cryptocustomrand=1 时才透传自定义
// reader；x509.CreateCertificate 生成序列号则始终直接 io.ReadFull(random)。
// 两类行为组合可分别注入到密钥生成与证书签发两个失败点。
func swapRandReader(t *testing.T, r io.Reader) {
	t.Helper()
	prev := rand.Reader
	rand.Reader = r
	t.Cleanup(func() { rand.Reader = prev })
}

// TestEnsureDevCA_CreateCertificateEntropyFailure 覆盖 devcert.go:66-68：
// 不设置 GODEBUG 时 rsa.GenerateKey 仍用系统默认熵成功生成密钥，但
// rand.Int 失败使 SerialNumber 为 nil，x509.CreateCertificate 随后直接从
// 失败 reader 读取序列号而报错。
func TestEnsureDevCA_CreateCertificateEntropyFailure(t *testing.T) {
	swapRandReader(t, failingRandReader{})

	caCrt, caKey, err := EnsureDevCA(t.TempDir())
	if err == nil {
		t.Fatal("expected CreateCertificate error when serial-number entropy read fails")
	}
	if caCrt != "" || caKey != "" {
		t.Errorf("error path must return empty paths, got (%q, %q)", caCrt, caKey)
	}
}

// TestEnsureDevCA_GenerateKeyEntropyFailure 覆盖 devcert.go:62-64：
// GODEBUG=cryptocustomrand=1 使 rsa.GenerateKey 透传失败 reader，
// 密钥生成直接报错。
func TestEnsureDevCA_GenerateKeyEntropyFailure(t *testing.T) {
	t.Setenv("GODEBUG", "cryptocustomrand=1")
	swapRandReader(t, failingRandReader{})

	caCrt, caKey, err := EnsureDevCA(t.TempDir())
	if err == nil {
		t.Fatal("expected GenerateKey error when custom random reader fails")
	}
	if caCrt != "" || caKey != "" {
		t.Errorf("error path must return empty paths, got (%q, %q)", caCrt, caKey)
	}
}

// TestEnsureServerCert_GenerateKeyEntropyFailure 覆盖 devcert.go:130-132。
func TestEnsureServerCert_GenerateKeyEntropyFailure(t *testing.T) {
	caDir := t.TempDir()
	caCrt, caKey, err := EnsureDevCA(caDir)
	if err != nil {
		t.Fatalf("prepare real CA: %v", err)
	}

	t.Setenv("GODEBUG", "cryptocustomrand=1")
	swapRandReader(t, failingRandReader{})

	crtPath, keyPath, err := EnsureServerCert(t.TempDir(), caCrt, caKey, []string{"localhost"})
	if err == nil {
		t.Fatal("expected GenerateKey error when custom random reader fails")
	}
	if crtPath != "" || keyPath != "" {
		t.Errorf("error path must return empty paths, got (%q, %q)", crtPath, keyPath)
	}
}

// TestEnsureAgentCert_GenerateKeyEntropyFailure 覆盖 devcert.go:191-193。
func TestEnsureAgentCert_GenerateKeyEntropyFailure(t *testing.T) {
	caDir := t.TempDir()
	caCrt, caKey, err := EnsureDevCA(caDir)
	if err != nil {
		t.Fatalf("prepare real CA: %v", err)
	}

	t.Setenv("GODEBUG", "cryptocustomrand=1")
	swapRandReader(t, failingRandReader{})

	crtPath, keyPath, err := EnsureAgentCert(t.TempDir(), caCrt, caKey, "agent-entropy-fail")
	if err == nil {
		t.Fatal("expected GenerateKey error when custom random reader fails")
	}
	if crtPath != "" || keyPath != "" {
		t.Errorf("error path must return empty paths, got (%q, %q)", crtPath, keyPath)
	}
}
