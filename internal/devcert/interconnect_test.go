package devcert

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureInterconnectCert_DualEKUAndSAN(t *testing.T) {
	dir := t.TempDir()
	caCrt, caKey, err := EnsureDevCA(dir)
	if err != nil {
		t.Fatalf("EnsureDevCA: %v", err)
	}
	crt, key, err := EnsureInterconnectCert(filepath.Join(dir, "ic"), caCrt, caKey, "server-a", []string{"127.0.0.1", "cluster.local"})
	if err != nil {
		t.Fatalf("EnsureInterconnectCert: %v", err)
	}
	if crt == "" || key == "" {
		t.Fatal("empty paths")
	}
	// 双 EKU：ServerAuth + ClientAuth（server 角色，区分 Agent 证书）。
	data, err := os.ReadFile(crt)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("no pem block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	if cert.Subject.CommonName != "server-a" {
		t.Errorf("CommonName = %q, want server-a", cert.Subject.CommonName)
	}
	if cert.Subject.CommonName == "" {
		t.Error("empty CommonName")
	}
	if !hasUsage(cert, x509.ExtKeyUsageServerAuth) || !hasUsage(cert, x509.ExtKeyUsageClientAuth) {
		t.Errorf("expected dual EKU, got %v", cert.ExtKeyUsage)
	}
	if len(cert.IPAddresses) == 0 && len(cert.DNSNames) == 0 {
		t.Error("expected SAN entries")
	}
}

func hasUsage(cert *x509.Certificate, usage x509.ExtKeyUsage) bool {
	for _, u := range cert.ExtKeyUsage {
		if u == usage {
			return true
		}
	}
	return false
}
