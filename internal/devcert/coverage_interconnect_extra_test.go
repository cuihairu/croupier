package devcert

// devcert.go 补测（interconnect 面）：EnsureInterconnectCert 的文件已存在
// 短路臂、loadCA 全错误臂（CA 证书缺失 / 私钥缺失 / 非 PEM / 证书 DER 损坏
// / 私钥 DER 非法）及其经 interconnect 入口的传播、randomSerial 熵失败臂、
// rsa.GenerateKey 熵失败臂（GODEBUG=cryptocustomrand=1 + 计数熵源）、CA
// 证书与签名私钥不匹配导致的 CreateCertificate 错误臂、crt 写失败臂。
//
// 无诚实跳过：GenerateKey 与 CreateCertificate 错误臂分别复用本包既有的
// GODEBUG 熵注入与 CA 不匹配注入（coverage_entropy_test.go /
// coverage_v9_test.go 先例）。

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureInterconnectCertReuse(t *testing.T) {
	caDir := t.TempDir()
	caCrt, caKey, err := EnsureDevCA(caDir)
	require.NoError(t, err)

	dir := t.TempDir()
	c1, k1, err := EnsureInterconnectCert(dir, caCrt, caKey, "inst-1",
		[]string{"127.0.0.1", "node-1.local"})
	require.NoError(t, err)

	// 第二次调用：文件已存在 → 短路返回同路径
	c2, k2, err := EnsureInterconnectCert(dir, caCrt, caKey, "inst-1", []string{"127.0.0.1"})
	require.NoError(t, err)
	require.Equal(t, c1, c2)
	require.Equal(t, k1, k2)
}

func TestEnsureInterconnectCertLoadCAErrors(t *testing.T) {
	caDir := t.TempDir()
	caCrt, caKey, err := EnsureDevCA(caDir)
	require.NoError(t, err)

	// CA 证书缺失 / CA 私钥缺失
	_, _, err = EnsureInterconnectCert(t.TempDir(), filepath.Join(caDir, "nope.crt"), caKey, "i", nil)
	require.Error(t, err)
	_, _, err = EnsureInterconnectCert(t.TempDir(), caCrt, filepath.Join(caDir, "nope.key"), "i", nil)
	require.Error(t, err)

	// 非 PEM（cert 与 key 侧同判）
	badPem := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(badPem, []byte("not pem"), 0o600))
	_, _, err = EnsureInterconnectCert(t.TempDir(), badPem, badPem, "i", nil)
	require.ErrorContains(t, err, "invalid CA pem files")

	// PEM 合法但 DER 损坏（证书侧 / 私钥侧）
	tmp := t.TempDir()
	badCrt := filepath.Join(tmp, "bad.crt")
	badKey := filepath.Join(tmp, "bad.key")
	writeBadDERPEMV9(t, badCrt, "CERTIFICATE")
	writeBadDERPEMV9(t, badKey, "RSA PRIVATE KEY")
	_, _, err = EnsureInterconnectCert(t.TempDir(), badCrt, caKey, "i", nil)
	require.Error(t, err)
	_, _, err = EnsureInterconnectCert(t.TempDir(), caCrt, badKey, "i", nil)
	require.Error(t, err)
}

func TestEnsureInterconnectCertSerialEntropyFailure(t *testing.T) {
	caDir := t.TempDir()
	caCrt, caKey, err := EnsureDevCA(caDir)
	require.NoError(t, err)

	// 不设 GODEBUG：GenerateKey 用系统熵成功，但 randomSerial 直接读失败
	// reader → 序列号臂先报错
	swapRandReader(t, failingRandReader{})
	_, _, err = EnsureInterconnectCert(t.TempDir(), caCrt, caKey, "i", nil)
	require.Error(t, err, "randomSerial must fail when entropy reader fails")
}

// limitedRandReader 前 remain 次读取返回全零数据、其后失败：零数据令
// randomSerial 得到合法零序列号（[0, max) 恒接受，单次读取即返回），而
// GODEBUG 透传后的 rsa.GenerateKey 拿到全零素数候选继续读，耗尽后报错。
type limitedRandReader struct{ remain int }

func (r *limitedRandReader) Read(p []byte) (int, error) {
	if r.remain <= 0 {
		return 0, errors.New("entropy unavailable")
	}
	r.remain--
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestEnsureInterconnectCertKeygenEntropyFailure(t *testing.T) {
	caDir := t.TempDir()
	caCrt, caKey, err := EnsureDevCA(caDir)
	require.NoError(t, err)

	t.Setenv("GODEBUG", "cryptocustomrand=1")
	swapRandReader(t, &limitedRandReader{remain: 3})
	_, _, err = EnsureInterconnectCert(t.TempDir(), caCrt, caKey, "i", nil)
	require.Error(t, err, "keygen must fail once the limited entropy reader is exhausted")
}

func TestEnsureInterconnectCertCreateCertificateMismatch(t *testing.T) {
	// ca.crt 由 keyA 自签、ca.key 为 keyB：解析均合法但签发不匹配
	caCrt, caKey := writeMismatchedCAV9(t, t.TempDir())
	_, _, err := EnsureInterconnectCert(t.TempDir(), caCrt, caKey, "i", nil)
	require.Error(t, err, "mismatched CA cert/key must fail certificate creation")
}

func TestEnsureInterconnectCertWriteFailure(t *testing.T) {
	caDir := t.TempDir()
	caCrt, caKey, err := EnsureDevCA(caDir)
	require.NoError(t, err)

	// 目录位被文件占据 → MkdirAll/OpenFile 失败（crt 写失败臂）
	occupied := filepath.Join(t.TempDir(), "occupied")
	require.NoError(t, os.WriteFile(occupied, []byte("x"), 0o600))
	_, _, err = EnsureInterconnectCert(occupied, caCrt, caKey, "i", nil)
	require.Error(t, err)
}

func TestEnsureInterconnectCertKeyWriteFailure(t *testing.T) {
	caDir := t.TempDir()
	caCrt, caKey, err := EnsureDevCA(caDir)
	require.NoError(t, err)

	// key 路径被目录占据：crt 写成功后 key 写失败（OpenFile 目录 → EISDIR）
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "interconnect.key"), 0o755))
	_, _, err = EnsureInterconnectCert(dir, caCrt, caKey, "i", nil)
	require.Error(t, err)
}
