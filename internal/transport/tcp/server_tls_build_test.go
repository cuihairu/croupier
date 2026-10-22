package tcp

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/devcert"
	"github.com/stretchr/testify/require"
)

// Batch E (capability matrix #5): BuildServerTLSConfig is the shared server
// TLS semantics used by both the Server transport and the Agent local gateway.

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
