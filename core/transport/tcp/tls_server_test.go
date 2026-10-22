package tcp

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/devcert"
	"github.com/stretchr/testify/require"
)

// Shared TLS listener construction is consumed directly by the server
// transport and the sidecar local gateway.

func TestBuildServerTLSConfig_RequiresCertAndKey(t *testing.T) {
	_, err := BuildServerTLSConfig("", "", "", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "certFile/keyFile")

	_, err = BuildServerTLSConfig("/tmp/some.crt", "", "", false)
	require.Error(t, err)
}

func TestBuildServerTLSConfig_LoadsCertificate(t *testing.T) {
	caDir := t.TempDir()
	certDir := t.TempDir()
	caCrt, caKey, err := devcert.EnsureDevCA(caDir)
	require.NoError(t, err)
	crt, key, err := devcert.EnsureServerCert(certDir, caCrt, caKey, []string{"127.0.0.1", "localhost"})
	require.NoError(t, err)

	cfg, err := BuildServerTLSConfig(crt, key, "", false)
	require.NoError(t, err)
	require.Len(t, cfg.Certificates, 1)
	require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	// One-way TLS by default: server presents its cert, client certs optional.
	require.Equal(t, tls.NoClientCert, cfg.ClientAuth)
	require.Nil(t, cfg.ClientCAs)
}

func TestBuildServerTLSConfig_CAFileEnablesMutualTLS(t *testing.T) {
	caDir := t.TempDir()
	certDir := t.TempDir()
	caCrt, caKey, err := devcert.EnsureDevCA(caDir)
	require.NoError(t, err)
	crt, key, err := devcert.EnsureServerCert(certDir, caCrt, caKey, []string{"127.0.0.1"})
	require.NoError(t, err)

	cfg, err := BuildServerTLSConfig(crt, key, caCrt, false)
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, cfg.ClientAuth)
	require.NotNil(t, cfg.ClientCAs)
}

func TestBuildServerTLSConfig_InsecureSkipsClientAuth(t *testing.T) {
	caDir := t.TempDir()
	certDir := t.TempDir()
	caCrt, caKey, err := devcert.EnsureDevCA(caDir)
	require.NoError(t, err)
	crt, key, err := devcert.EnsureServerCert(certDir, caCrt, caKey, []string{"127.0.0.1"})
	require.NoError(t, err)

	cfg, err := BuildServerTLSConfig(crt, key, caCrt, true)
	require.NoError(t, err)
	require.Equal(t, tls.NoClientCert, cfg.ClientAuth)
}

func TestBuildServerTLSConfig_MissingFilesError(t *testing.T) {
	_, err := BuildServerTLSConfig(
		filepath.Join(t.TempDir(), "nope.crt"),
		filepath.Join(t.TempDir(), "nope.key"), "", false)
	require.Error(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestServerTLSConfig_AllowsOptionalCertificate(t *testing.T) {
	cfg, err := ServerTLSConfig(&Config{})
	require.NoError(t, err)
	require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	require.Empty(t, cfg.Certificates)
	require.Equal(t, tls.NoClientCert, cfg.ClientAuth)
}

func TestServerTLSConfig_LoadsCertificateAndMutualTLS(t *testing.T) {
	caDir, certDir := t.TempDir(), t.TempDir()
	caCrt, caKey, err := devcert.EnsureDevCA(caDir)
	require.NoError(t, err)
	crt, key, err := devcert.EnsureServerCert(certDir, caCrt, caKey, []string{"127.0.0.1"})
	require.NoError(t, err)

	cfg, err := ServerTLSConfig(&Config{CertFile: crt, KeyFile: key, CAFile: caCrt})
	require.NoError(t, err)
	require.Len(t, cfg.Certificates, 1)
	require.Equal(t, tls.RequireAndVerifyClientCert, cfg.ClientAuth)
	require.NotNil(t, cfg.ClientCAs)

	cfg.InsecureSkipVerify = false
	require.Equal(t, tls.RequireAndVerifyClientCert, cfg.ClientAuth)
}

func TestServerTLSConfig_InsecureOverridesMutualTLS(t *testing.T) {
	caDir, certDir := t.TempDir(), t.TempDir()
	caCrt, caKey, err := devcert.EnsureDevCA(caDir)
	require.NoError(t, err)
	crt, key, err := devcert.EnsureServerCert(certDir, caCrt, caKey, []string{"127.0.0.1"})
	require.NoError(t, err)

	cfg, err := ServerTLSConfig(&Config{CertFile: crt, KeyFile: key, CAFile: caCrt, InsecureSkipVerify: true})
	require.NoError(t, err)
	require.Len(t, cfg.Certificates, 1)
	require.Equal(t, tls.NoClientCert, cfg.ClientAuth)
}

func TestServerTLSConfig_RejectsInvalidFiles(t *testing.T) {
	_, err := ServerTLSConfig(&Config{CertFile: "/nonexistent/server.crt", KeyFile: "/nonexistent/server.key"})
	require.ErrorIs(t, err, os.ErrNotExist)

	_, err = ServerTLSConfig(&Config{CAFile: "/nonexistent/ca.crt"})
	require.ErrorIs(t, err, os.ErrNotExist)

	badCA := filepath.Join(t.TempDir(), "bad-ca.crt")
	require.NoError(t, os.WriteFile(badCA, []byte("not a certificate"), 0o600))
	_, err = ServerTLSConfig(&Config{CAFile: badCA})
	require.EqualError(t, err, "append CA certificate")
}
