// 覆盖目标：CheckCertificate 的 expiring 状态分支（本地 TLS 服务器 + SSL_CERT_FILE
// 注入测试 CA）、CheckAllCertificates 的查询失败与单项检查失败 continue、
// ListCertificates 的 Count 失败。
package certificates

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// v8CA 单次生成测试 CA（进程内 roots 加载只发生一次，所有测试必须共用同一 CA）。
var v8CA struct {
	once     sync.Once
	key      *ecdsa.PrivateKey
	cert     *x509.Certificate
	der      []byte
	caFile   string
	setUpErr error
}

// TestMain 在所有测试运行前生成测试 CA 并通过 SSL_CERT_FILE 注入进程信任根。
// crypto/x509 的系统根池按进程仅加载一次（root.go once），若不提前注入，
// 先行触发的 TLS 验证会把真实系统根缓存，导致后续自签 CA 无法被信任。
func TestMain(m *testing.M) {
	v8CA.once.Do(v8InitCA)
	os.Exit(m.Run())
}

func v8InitCA() {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		v8CA.setUpErr = err
		return
	}
	now := time.Now()
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "croupier-test-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		v8CA.setUpErr = err
		return
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		v8CA.setUpErr = err
		return
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	dir, err := os.MkdirTemp("", "croupier-cert-test-")
	if err != nil {
		v8CA.setUpErr = err
		return
	}
	caFile := filepath.Join(dir, "test-ca.pem")
	if err := os.WriteFile(caFile, caPEM, 0o600); err != nil {
		v8CA.setUpErr = err
		return
	}
	v8CA.key = caKey
	v8CA.cert = caCert
	v8CA.der = caDER
	v8CA.caFile = caFile
	if err := os.Setenv("SSL_CERT_FILE", caFile); err != nil {
		v8CA.setUpErr = err
		return
	}
}

func v8RequireCA(t *testing.T) {
	t.Helper()
	require.NoError(t, v8CA.setUpErr)
	require.NotNil(t, v8CA.cert)
}

// startTestTLSServer 启动一张由测试 CA 签发、有效期可控的本地 TLS 服务。
func startTestTLSServer(t *testing.T, notAfter time.Time) (port int) {
	t.Helper()
	v8RequireCA(t)

	now := time.Now()
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:     []string{"localhost"},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, v8CA.cert, &leafKey.PublicKey, v8CA.key)
	require.NoError(t, err)

	serverCert := tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	_, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, err)
	port, err = net.LookupPort("tcp", portStr)
	require.NoError(t, err)
	return port
}

// CheckCertificate：证书剩余天数 <= AlertDays → status=expiring。
func TestStore_CheckCertificate_Expiring_V8(t *testing.T) {
	port := startTestTLSServer(t, time.Now().Add(5*24*time.Hour))

	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	cert := &Certificate{
		Domain:    "127.0.0.1",
		Port:      port,
		Enabled:   true,
		Status:    "pending",
		AlertDays: 30,
	}
	require.NoError(t, db.Create(cert).Error)

	require.NoError(t, store.CheckCertificate(cert.ID))

	var updated Certificate
	require.NoError(t, db.First(&updated, cert.ID).Error)
	assert.Equal(t, "expiring", updated.Status)
	assert.Empty(t, updated.ErrorMsg)
	assert.Equal(t, 4, updated.DaysLeft)
	assert.Equal(t, "croupier-test-ca", updated.Issuer)
	assert.Equal(t, "localhost", updated.Subject)
	assert.Contains(t, updated.KeyUsage, "Digital Signature")
	assert.Contains(t, updated.KeyUsage, "Server Authentication")
}

// CheckCertificate：剩余天数充足 → status=valid。
func TestStore_CheckCertificate_Valid_V8(t *testing.T) {
	port := startTestTLSServer(t, time.Now().Add(200*24*time.Hour))

	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	cert := &Certificate{
		Domain:    "127.0.0.1",
		Port:      port,
		Enabled:   true,
		Status:    "pending",
		AlertDays: 30,
	}
	require.NoError(t, db.Create(cert).Error)

	require.NoError(t, store.CheckCertificate(cert.ID))

	var updated Certificate
	require.NoError(t, db.First(&updated, cert.ID).Error)
	assert.Equal(t, "valid", updated.Status)
}

// CheckAllCertificates：查询失败 → 直接返回错误。
func TestStore_CheckAllCertificates_FindError_V8(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("v8:query_err", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "certificates" {
			_ = tx.AddError(errors.New("injected query failure"))
		}
	}))

	err := store.CheckAllCertificates()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected query failure")
}

// CheckAllCertificates：单项检查失败（fetch 失败且 Save 注入错误）→ continue。
func TestStore_CheckAllCertificates_SingleCheckErrorContinues_V8(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	cert := &Certificate{
		Domain:    "nonexistent.invalid",
		Port:      443,
		Enabled:   true,
		Status:    "pending",
		AlertDays: 30,
	}
	require.NoError(t, db.Create(cert).Error)

	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("v8:update_err", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "certificates" {
			_ = tx.AddError(errors.New("injected save failure"))
		}
	}))

	// 单项失败被吞掉，整体仍成功。
	require.NoError(t, store.CheckAllCertificates())
}

// ListCertificates：Count 失败 → 返回错误。
func TestStore_ListCertificates_CountError_V8(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("v8:count_err", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "certificates" {
			_ = tx.AddError(errors.New("injected count failure"))
		}
	}))

	_, _, err := store.ListCertificates(1, 10, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected count failure")
}

var _ = context.Background
