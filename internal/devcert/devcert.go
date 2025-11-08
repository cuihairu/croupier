package devcert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// writeFile creates file with given mode if not exists; overwrites otherwise.
func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// EnsureDevCA ensures a development CA exists under dir and returns paths.
func EnsureDevCA(dir string) (caCrt, caKey string, err error) {
	caCrt = filepath.Join(dir, "ca.crt")
	caKey = filepath.Join(dir, "ca.key")
	// If both exist, assume OK
	if _, e1 := os.Stat(caCrt); e1 == nil {
		if _, e2 := os.Stat(caKey); e2 == nil {
			return caCrt, caKey, nil
		}
	}

	// Create CA
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)
	tmpl := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: "croupier-dev-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := writeFile(caCrt, crtPEM, 0o644); err != nil {
		return "", "", err
	}
	if err := writeFile(caKey, keyPEM, 0o600); err != nil {
		return "", "", err
	}
	return caCrt, caKey, nil
}

// EnsureServerCert ensures a server cert signed by CA; hosts may include DNS/IP.
func EnsureServerCert(dir, caCrtPath, caKeyPath string, hosts []string) (crtPath, keyPath string, err error) {
	crtPath = filepath.Join(dir, "server.crt")
	keyPath = filepath.Join(dir, "server.key")
	if _, e1 := os.Stat(crtPath); e1 == nil {
		if _, e2 := os.Stat(keyPath); e2 == nil {
			return crtPath, keyPath, nil
		}
	}
	caCrtBytes, err := os.ReadFile(caCrtPath)
	if err != nil {
		return "", "", err
	}
	caKeyBytes, err := os.ReadFile(caKeyPath)
	if err != nil {
		return "", "", err
	}
	caCrtBlock, _ := pem.Decode(caCrtBytes)
	caKeyBlock, _ := pem.Decode(caKeyBytes)
	if caCrtBlock == nil || caKeyBlock == nil {
		return "", "", fmt.Errorf("invalid CA pem files")
	}
	caCert, err := x509.ParseCertificate(caCrtBlock.Bytes)
	if err != nil {
		return "", "", err
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return "", "", err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)
	tmpl := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: "croupier-server"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return "", "", err
	}
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := writeFile(crtPath, crtPEM, 0o644); err != nil {
		return "", "", err
	}
	if err := writeFile(keyPath, keyPEM, 0o600); err != nil {
		return "", "", err
	}
	return crtPath, keyPath, nil
}

// EnsureAgentCert ensures a client cert for agent signed by CA.
func EnsureAgentCert(dir, caCrtPath, caKeyPath, commonName string) (crtPath, keyPath string, err error) {
	crtPath = filepath.Join(dir, "agent.crt")
	keyPath = filepath.Join(dir, "agent.key")
	if _, e1 := os.Stat(crtPath); e1 == nil {
		if _, e2 := os.Stat(keyPath); e2 == nil {
			return crtPath, keyPath, nil
		}
	}
	caCrtBytes, err := os.ReadFile(caCrtPath)
	if err != nil {
		return "", "", err
	}
	caKeyBytes, err := os.ReadFile(caKeyPath)
	if err != nil {
		return "", "", err
	}
	caCrtBlock, _ := pem.Decode(caCrtBytes)
	caKeyBlock, _ := pem.Decode(caKeyBytes)
	if caCrtBlock == nil || caKeyBlock == nil {
		return "", "", fmt.Errorf("invalid CA pem files")
	}
	caCert, err := x509.ParseCertificate(caCrtBlock.Bytes)
	if err != nil {
		return "", "", err
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return "", "", err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)
	tmpl := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return "", "", err
	}
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := writeFile(crtPath, crtPEM, 0o644); err != nil {
		return "", "", err
	}
	if err := writeFile(keyPath, keyPEM, 0o600); err != nil {
		return "", "", err
	}
	return crtPath, keyPath, nil
}
