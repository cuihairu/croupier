// 覆盖目标：CheckCertificate 的 expired 分支（certificates.go:126-128）。
//
// 历史演变：v9 曾用「80 张 RSA-8192 干扰中间证书拖慢链验签 + 秒界前 15ms
// 相位对齐发射」的物理时序竞速构造「TLS 验证时证书仍有效、写入检查时已过期」
// 窗口；该窗口随 CPU 速度与调度浮动，换机即失稳（2026-09-14 三连败后退役）。
// 现改为注入 fetchCert：真实 TLS 验证会先拒绝已过期证书（bad certificate 走
// error 分支），expired 分支的语义本就是「拿到证书信息后发现已过期」，注入
// 一个 NotAfter 在过去的证书即可确定性覆盖，零时序依赖。
package certificates

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCert 构造仅含时间字段的证书（CheckCertificate 只消费时间/主体/算法字段）。
func fakeCert(notAfter time.Time) *x509.Certificate {
	return &x509.Certificate{
		Subject:      pkix.Name{CommonName: "expired.example.com"},
		Issuer:       pkix.Name{CommonName: "croupier-test-ca"},
		SerialNumber: big.NewInt(1),
		NotBefore:    notAfter.Add(-90 * 24 * time.Hour),
		NotAfter:     notAfter,
	}
}

// CheckCertificate：证书 NotAfter 在过去 → status=expired（certificates.go:127-129）。
func TestStore_CheckCertificate_ExpiredBranch(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	expiredAt := time.Now().Add(-48 * time.Hour)
	store.fetchCert = func(domain string, port int) (*x509.Certificate, error) {
		assert.Equal(t, "expired.example.com", domain)
		return fakeCert(expiredAt), nil
	}

	cert := &Certificate{
		Domain:    "expired.example.com",
		Port:      443,
		Enabled:   true,
		Status:    "pending",
		AlertDays: 30,
	}
	require.NoError(t, db.Create(cert).Error)

	require.NoError(t, store.CheckCertificate(cert.ID))

	var updated Certificate
	require.NoError(t, db.First(&updated, cert.ID).Error)
	assert.Equal(t, "expired", updated.Status)
	assert.Empty(t, updated.ErrorMsg)
	assert.Equal(t, "expired.example.com", updated.Subject)
	assert.Equal(t, "croupier-test-ca", updated.Issuer)
	assert.WithinDuration(t, expiredAt, updated.ValidTo, time.Second)
	// 过期整 2 天 → int 截断后 -2
	assert.Equal(t, -2, updated.DaysLeft)
	assert.False(t, updated.LastChecked.IsZero())
}

// CheckCertificate：fetchCert 返回错误 → status=error 且透出错误消息。
// （v9 之前该分支仅由 nonexistent.invalid 的 DNS 失败覆盖，此处改由注入点
// 确定性覆盖同一逻辑路径。）
func TestStore_CheckCertificate_FetchErrorBranch(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	store.fetchCert = func(domain string, port int) (*x509.Certificate, error) {
		return nil, errors.New("boom: injected fetch failure")
	}

	cert := &Certificate{Domain: "down.example.com", Port: 443, Enabled: true, Status: "pending", AlertDays: 30}
	require.NoError(t, db.Create(cert).Error)

	require.NoError(t, store.CheckCertificate(cert.ID))

	var updated Certificate
	require.NoError(t, db.First(&updated, cert.ID).Error)
	assert.Equal(t, "error", updated.Status)
	assert.Contains(t, updated.ErrorMsg, "boom")
	assert.False(t, updated.LastChecked.IsZero())
}
