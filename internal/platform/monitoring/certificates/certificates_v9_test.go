// 覆盖目标：CheckCertificate 的 expired 分支（certificates.go:115-117）。
//
// 该分支只在「TLS 验证时证书仍有效、写入检查时已过期」的窗口内可达。构造方式：
//  1. 叶子证书 NotAfter 为确定的未来整秒（Go x509 序列化截断到整秒，见 PoC）；
//  2. 服务器证书链附带 80 张与签发 CA 同 Subject 的 RSA-8192「干扰」中间证书：
//     客户端链验证在叶子有效期检查之后、对每个候选中间执行一次完整 RSA
//     模幂验签（约 0.5ms/张，受 sigChecks=100 与 TLS 256KB 握手消息上限约束），
//     把「验证通过」到「Dial 返回」的时间拉长约 40-55ms（随 CPU 速度浮动）；
//  3. 在目标秒界前 15ms 相位对齐后发起 CheckCertificate：验证时刻仍 < 目标秒
//     （证书有效），检查时刻 > 目标秒（证书过期）→ status=expired。
//
// 干扰证书的模数为构造值（非素数乘积）：其自身签名从不被链验证，只需 Subject
// 匹配、SPKI 互异（避免 CertPool 去重）、IsCA+KeyUsageCertSign（避免快速拒绝）。
// 参数 2026-09 重校准（快速 CPU 验签提速）后 -count=10 实测稳定；仍保留至多
// 3 次尝试防御极端调度离群。
package certificates

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pemCert(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// certXRSACA 是 v9 的 RSA 签名 CA（由 v8InitCA 在根池初始化时追加进 SSL_CERT_FILE）。
// 叶子必须由 RSA CA 签发：干扰证书是 RSA 公钥，只有叶子签名算法为 RSA 时
// CheckSignatureFrom 才会执行真正的模幂运算（ECDSA 签名 + RSA 公钥会快速失败）。
var certXRSACA struct {
	once sync.Once
	key  *rsa.PrivateKey
	cert *x509.Certificate
	err  error
}

func certXAppendRSACA(caFile string) {
	certXRSACA.once.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			certXRSACA.err = err
			return
		}
		now := time.Now()
		tmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(9001),
			Subject:               pkix.Name{CommonName: "croupier-test-rsa-ca"},
			NotBefore:             now.Add(-time.Hour),
			NotAfter:              now.Add(24 * time.Hour),
			IsCA:                  true,
			KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
			BasicConstraintsValid: true,
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		if err != nil {
			certXRSACA.err = err
			return
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			certXRSACA.err = err
			return
		}
		f, err := os.OpenFile(caFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			certXRSACA.err = err
			return
		}
		defer f.Close()
		if _, err := f.WriteString(string(pemCert(der))); err != nil {
			certXRSACA.err = err
			return
		}
		certXRSACA.key = key
		certXRSACA.cert = cert
	})
}

// certXBuildDecays 生成 n 张干扰中间证书（Subject 同 caSubject，由工厂 CA 签发）。
func certXBuildDecays(caSubject pkix.Name, n int) ([][]byte, *ecdsa.PrivateKey, *x509.Certificate, error) {
	factoryKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	now := time.Now()
	factoryTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(9100),
		Subject:               pkix.Name{CommonName: "croupier-test-decay-factory"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	factoryDER, err := x509.CreateCertificate(rand.Reader, factoryTmpl, factoryTmpl, &factoryKey.PublicKey, factoryKey)
	if err != nil {
		return nil, nil, nil, err
	}
	factoryCert, err := x509.ParseCertificate(factoryDER)
	if err != nil {
		return nil, nil, nil, err
	}

	ders := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		// 任意 8192 位奇模数（2^8191 + 偏移）：位数决定验签耗时，数值本身无意义。
		mod := new(big.Int).Lsh(big.NewInt(1), 8191)
		mod.Add(mod, big.NewInt(int64(2*i+15677)))
		tmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(int64(9200 + i)),
			Subject:               caSubject,
			NotBefore:             now.Add(-time.Hour),
			NotAfter:              now.Add(24 * time.Hour),
			IsCA:                  true,
			KeyUsage:              x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, factoryCert, &rsa.PublicKey{N: mod, E: 65537}, factoryKey)
		if err != nil {
			return nil, nil, nil, err
		}
		ders = append(ders, der)
	}
	return ders, factoryKey, factoryCert, nil
}

// certXSpinUntil 忙等到指定墙钟时刻（相位对齐，精度微秒级）。
func certXSpinUntil(t time.Time) {
	for {
		d := time.Until(t)
		if d <= 0 {
			return
		}
		if d > 2*time.Millisecond {
			time.Sleep(d - 2*time.Millisecond)
		}
	}
}

// TestStore_CheckCertificate_ExpiredBranch_V9 覆盖 certificates.go:115-117。
func TestStore_CheckCertificate_ExpiredBranch_V9(t *testing.T) {
	v8RequireCA(t)
	require.NoError(t, certXRSACA.err)
	require.NotNil(t, certXRSACA.cert)

	const (
		// 发射提前量必须落在 (发射→Verify 取 now 的延迟, 干扰链验签总耗时)
		// 区间内：太小则 Verify 起步已过秒界（握手失败走 error 分支），太大
		// 则验证在秒界前完成（判定未过期走 expiring 分支）。2026-09 实测快速
		// CPU 上 80 张 8192 位模幂验签总耗时约 40ms（原 55ms 提前量因此闭合，
		// 判定未过期）、本地发射→Verify 取时延迟 1-3ms；取 15ms 两端各留
		// 5 倍以上余量。张数不可加大：受 x509 sigChecks=100 与 TLS 256KB
		// 握手消息上限双重约束。
		certXDecayCount = 80
		certXLaunch     = 15 * time.Millisecond // 目标秒界前的发射提前量
	)

	decayDERs, _, _, err := certXBuildDecays(certXRSACA.cert.Subject, certXDecayCount)
	require.NoError(t, err)

	var current atomic.Value // tls.Certificate
	tlsCfg := &tls.Config{GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		c := current.Load().(tls.Certificate)
		return &c, nil
	}}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), TLSConfig: tlsCfg}
	go srv.ServeTLS(ln, "", "")
	t.Cleanup(func() { _ = srv.Close() })

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	port, err := net.LookupPort("tcp", portStr)
	require.NoError(t, err)

	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	signLeaf := func(notAfter time.Time, serial int64) []byte {
		now := time.Now()
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: "localhost"},
			NotBefore:    now.Add(-time.Hour),
			NotAfter:     notAfter,
			KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
			DNSNames:     []string{"localhost"},
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, certXRSACA.cert, &leafKey.PublicKey, certXRSACA.key)
		require.NoError(t, err)
		return der
	}

	// 预热：两次长寿命证书的完整握手，稳定代码路径与调度。
	for w := 0; w < 2; w++ {
		warm := signLeaf(time.Now().Add(24*time.Hour), int64(9300+w))
		current.Store(tls.Certificate{Certificate: append([][]byte{warm}, decayDERs...), PrivateKey: leafKey})
		c, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp",
			fmt.Sprintf("127.0.0.1:%d", port), &tls.Config{ServerName: "127.0.0.1"})
		if err == nil {
			_ = c.Close()
		}
	}

	row := &Certificate{Domain: "127.0.0.1", Port: port, Enabled: true, Status: "pending", AlertDays: 30}
	require.NoError(t, db.Create(row).Error)

	for attempt := 0; attempt < 3; attempt++ {
		target := time.Now().Add(2 * time.Second).Truncate(time.Second)
		leaf := signLeaf(target, int64(9400+attempt))
		current.Store(tls.Certificate{Certificate: append([][]byte{leaf}, decayDERs...), PrivateKey: leafKey})

		certXSpinUntil(target.Add(-certXLaunch))
		require.NoError(t, store.CheckCertificate(row.ID))

		var updated Certificate
		require.NoError(t, db.First(&updated, row.ID).Error)
		if updated.Status == "expired" {
			assert.Empty(t, updated.ErrorMsg)
			return
		}
		// 未命中（极端调度离群导致验证晚于目标秒，走了 error 分支）则重试下一整秒。
	}
	t.Fatal("expired branch not reached after 3 attempts")
}

var _ = context.Background
