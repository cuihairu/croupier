package devcert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// genCAV9 生成自签 CA 的 PEM 内容。notAfter 为结束时间。
func genCAV9(t *testing.T, cn string, notAfter time.Time) (crtPEM, keyPEM []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-2 * time.Hour),
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	crtPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return crtPEM, keyPEM
}

// writeExpiredCAV9 在 dir 写入一对已过期的 CA 文件。
func writeExpiredCAV9(t *testing.T, dir string) {
	t.Helper()
	crtPEM, keyPEM := genCAV9(t, "expired-ca", time.Now().Add(-time.Hour))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), crtPEM, 0o644); err != nil {
		t.Fatalf("write ca.crt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ca.key"), keyPEM, 0o600); err != nil {
		t.Fatalf("write ca.key: %v", err)
	}
}

// writeMismatchedCAV9 写入 ca.crt（keyA 自签）与 ca.key（keyB），
// 两者解析均合法但签名时公私钥不匹配，可触发 x509.CreateCertificate 错误。
func writeMismatchedCAV9(t *testing.T, dir string) (caCrt, caKey string) {
	t.Helper()
	crtPEM, _ := genCAV9(t, "ca-a", time.Now().Add(time.Hour))
	_, keyPEM := genCAV9(t, "ca-b", time.Now().Add(time.Hour))
	caCrt = filepath.Join(dir, "ca.crt")
	caKey = filepath.Join(dir, "ca.key")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(caCrt, crtPEM, 0o644); err != nil {
		t.Fatalf("write ca.crt: %v", err)
	}
	if err := os.WriteFile(caKey, keyPEM, 0o600); err != nil {
		t.Fatalf("write ca.key: %v", err)
	}
	return caCrt, caKey
}

// writeBadDERPEMV9 写入 PEM 块格式合法但 DER 内容非法的文件。
func writeBadDERPEMV9(t *testing.T, path, blockType string) {
	t.Helper()
	bad := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: []byte("this is not DER data")})
	if err := os.WriteFile(path, bad, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestWriteFileV9_MkdirAllFails(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
		t.Fatalf("prepare blocker: %v", err)
	}
	// blocker 是文件，无法作为目录创建。
	err := writeFile(filepath.Join(blocker, "ca.crt"), []byte("data"), 0o644)
	if err == nil {
		t.Error("expected MkdirAll error when parent path is a file")
	}
}

func TestEnsureDevCAV9_RegeneratesExpiredCA(t *testing.T) {
	dir := t.TempDir()
	writeExpiredCAV9(t, dir)

	caCrt, caKey, err := EnsureDevCA(dir)
	if err != nil {
		t.Fatalf("EnsureDevCA() error = %v", err)
	}
	if caCrt != filepath.Join(dir, "ca.crt") || caKey != filepath.Join(dir, "ca.key") {
		t.Fatalf("unexpected paths: %s, %s", caCrt, caKey)
	}

	// 新证书必须是 10 年有效期的 CA。
	der, _ := os.ReadFile(caCrt)
	block, _ := pem.Decode(der)
	if block == nil {
		t.Fatal("regenerated ca.crt is not valid PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse regenerated cert: %v", err)
	}
	if !cert.IsCA {
		t.Error("regenerated cert should be a CA")
	}
	if time.Until(cert.NotAfter) < 9*365*24*time.Hour {
		t.Errorf("regenerated cert should be valid ~10 years, NotAfter=%v", cert.NotAfter)
	}
}

func TestEnsureDevCAV9_WriteCertFails(t *testing.T) {
	dir := t.TempDir()
	// ca.crt 是目录：stat 成功、ReadFile 失败 → 视为过期 → 重生成时写失败。
	if err := os.MkdirAll(filepath.Join(dir, "ca.crt"), 0o755); err != nil {
		t.Fatalf("mkdir ca.crt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ca.key"), []byte("key"), 0o600); err != nil {
		t.Fatalf("write ca.key: %v", err)
	}

	_, _, err := EnsureDevCA(dir)
	if err == nil {
		t.Error("expected error when ca.crt path is a directory")
	}
}

func TestEnsureDevCAV9_WriteKeyFails(t *testing.T) {
	dir := t.TempDir()
	// ca.crt 是真实过期证书（会先被成功覆盖），ca.key 是目录 → 第二次写失败。
	writeExpiredCAV9(t, dir)
	if err := os.Remove(filepath.Join(dir, "ca.key")); err != nil {
		t.Fatalf("remove ca.key: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "ca.key"), 0o755); err != nil {
		t.Fatalf("mkdir ca.key: %v", err)
	}

	_, _, err := EnsureDevCA(dir)
	if err == nil {
		t.Error("expected error when ca.key path is a directory")
	}
}

func TestEnsureServerCertV9_BadCACertDER(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey := filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key")
	_, keyPEM := genCAV9(t, "ca", time.Now().Add(time.Hour))
	writeBadDERPEMV9(t, caCrt, "CERTIFICATE")
	if err := os.WriteFile(caKey, keyPEM, 0o600); err != nil {
		t.Fatalf("write ca.key: %v", err)
	}

	_, _, err := EnsureServerCert(dir, caCrt, caKey, []string{"localhost"})
	if err == nil {
		t.Error("expected ParseCertificate error for garbage CA cert DER")
	}
}

func TestEnsureServerCertV9_BadCAKeyDER(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey := filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key")
	crtPEM, _ := genCAV9(t, "ca", time.Now().Add(time.Hour))
	if err := os.WriteFile(caCrt, crtPEM, 0o644); err != nil {
		t.Fatalf("write ca.crt: %v", err)
	}
	writeBadDERPEMV9(t, caKey, "RSA PRIVATE KEY")

	_, _, err := EnsureServerCert(dir, caCrt, caKey, []string{"localhost"})
	if err == nil {
		t.Error("expected ParsePKCS1PrivateKey error for garbage CA key DER")
	}
}

func TestEnsureServerCertV9_KeyCertMismatch(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey := writeMismatchedCAV9(t, dir)

	_, _, err := EnsureServerCert(dir, caCrt, caKey, []string{"localhost"})
	if err == nil {
		t.Error("expected CreateCertificate error when CA key does not match CA cert")
	}
}

func TestEnsureServerCertV9_WriteCrtFails(t *testing.T) {
	dir := t.TempDir()
	writeCAOnlyV9(t, dir)
	// server.crt 是目录且 server.key 缺失 → 绕过 reuse 检查后写 crt 失败。
	if err := os.MkdirAll(filepath.Join(dir, "server.crt"), 0o755); err != nil {
		t.Fatalf("mkdir server.crt: %v", err)
	}

	_, _, err := EnsureServerCert(dir, filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key"), []string{"localhost"})
	if err == nil {
		t.Error("expected error when server.crt path is a directory")
	}
}

func TestEnsureServerCertV9_WriteKeyFails(t *testing.T) {
	dir := t.TempDir()
	writeCAOnlyV9(t, dir)
	// server.crt 缺失（跳过 reuse），server.key 是目录 → 写 key 失败。
	if err := os.MkdirAll(filepath.Join(dir, "server.key"), 0o755); err != nil {
		t.Fatalf("mkdir server.key: %v", err)
	}

	_, _, err := EnsureServerCert(dir, filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key"), []string{"localhost"})
	if err == nil {
		t.Error("expected error when server.key path is a directory")
	}
}

// writeCAOnlyV9 写一对正常的 CA 文件（仅 ca.crt/ca.key）。
func writeCAOnlyV9(t *testing.T, dir string) (string, string) {
	t.Helper()
	crtPEM, keyPEM := genCAV9(t, "ca-only", time.Now().Add(24*time.Hour))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	caCrt := filepath.Join(dir, "ca.crt")
	caKey := filepath.Join(dir, "ca.key")
	if err := os.WriteFile(caCrt, crtPEM, 0o644); err != nil {
		t.Fatalf("write ca.crt: %v", err)
	}
	if err := os.WriteFile(caKey, keyPEM, 0o600); err != nil {
		t.Fatalf("write ca.key: %v", err)
	}
	return caCrt, caKey
}

func TestEnsureAgentCertV9_Reuse(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey := writeCAOnlyV9(t, dir)

	crt1, key1, err := EnsureAgentCert(dir, caCrt, caKey, "agent-v9")
	if err != nil {
		t.Fatalf("first EnsureAgentCert() error = %v", err)
	}

	crt2, key2, err := EnsureAgentCert(dir, caCrt, caKey, "agent-v9")
	if err != nil {
		t.Fatalf("second EnsureAgentCert() error = %v", err)
	}
	if crt1 != crt2 || key1 != key2 {
		t.Errorf("expected reused paths, got (%s,%s) vs (%s,%s)", crt1, key1, crt2, key2)
	}
}

func TestEnsureAgentCertV9_BadCACertDER(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey := filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key")
	_, keyPEM := genCAV9(t, "ca", time.Now().Add(time.Hour))
	writeBadDERPEMV9(t, caCrt, "CERTIFICATE")
	if err := os.WriteFile(caKey, keyPEM, 0o600); err != nil {
		t.Fatalf("write ca.key: %v", err)
	}

	_, _, err := EnsureAgentCert(dir, caCrt, caKey, "agent-v9")
	if err == nil {
		t.Error("expected ParseCertificate error for garbage CA cert DER")
	}
}

func TestEnsureAgentCertV9_BadCAKeyDER(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey := filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key")
	crtPEM, _ := genCAV9(t, "ca", time.Now().Add(time.Hour))
	if err := os.WriteFile(caCrt, crtPEM, 0o644); err != nil {
		t.Fatalf("write ca.crt: %v", err)
	}
	writeBadDERPEMV9(t, caKey, "RSA PRIVATE KEY")

	_, _, err := EnsureAgentCert(dir, caCrt, caKey, "agent-v9")
	if err == nil {
		t.Error("expected ParsePKCS1PrivateKey error for garbage CA key DER")
	}
}

func TestEnsureAgentCertV9_KeyCertMismatch(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey := writeMismatchedCAV9(t, dir)

	_, _, err := EnsureAgentCert(dir, caCrt, caKey, "agent-v9")
	if err == nil {
		t.Error("expected CreateCertificate error when CA key does not match CA cert")
	}
}

func TestEnsureAgentCertV9_WriteCrtFails(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey := writeCAOnlyV9(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "agent.crt"), 0o755); err != nil {
		t.Fatalf("mkdir agent.crt: %v", err)
	}

	_, _, err := EnsureAgentCert(dir, caCrt, caKey, "agent-v9")
	if err == nil {
		t.Error("expected error when agent.crt path is a directory")
	}
}

func TestEnsureAgentCertV9_WriteKeyFails(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey := writeCAOnlyV9(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "agent.key"), 0o755); err != nil {
		t.Fatalf("mkdir agent.key: %v", err)
	}

	_, _, err := EnsureAgentCert(dir, caCrt, caKey, "agent-v9")
	if err == nil {
		t.Error("expected error when agent.key path is a directory")
	}
}

func TestIsCertExpiredV9_BadDERContent(t *testing.T) {
	dir := t.TempDir()
	badDER := filepath.Join(dir, "bad-der.crt")
	writeBadDERPEMV9(t, badDER, "CERTIFICATE")

	if !isCertExpired(badDER) {
		t.Error("valid PEM block with non-DER body should be considered expired")
	}
}
