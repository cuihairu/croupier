// AUD-040 回归：agent 注册凭据与 game/env 授权门禁。
//
// 根因：register 只要求非空 agent_id/game_id/env，无 token/mTLS identity 校验，
// default config 为 Insecure:true——能访问控制端口的攻击者可注册伪 agent、
// 劫持 agent id、注册恶意函数承接控制面调用。修复为：RequireAuth 显式加固
// （监听面必须处于 mTLS，否则启动 fail-closed；注册要求对端已校验客户端证书
// 身份）+ AllowedGameEnvs 作用域白名单（game/env 授权）。两者缺省关闭/空 =
// 兼容存量部署（insecure 仍可启动，启动 WARN 提示未加固）。
package server

import (
	"context"
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

	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// writeAud040TestCert 生成自签证书并写出 cert/key/ca（复用 TLS 测试范式）。
func writeAud040TestCert(t *testing.T, dir string) (certPath, keyPath, caPath string) {
	t.Helper()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	caPath = filepath.Join(dir, "ca.pem")

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))
	require.NoError(t, os.WriteFile(caPath, certPEM, 0o600))
	return certPath, keyPath, caPath
}

// TestTCPListener_Aud040_RequireAuthStartupValidation 验证 RequireAuth 的启动
// fail-closed：监听面必须处于 mTLS（CA 配置 + 非明文），否则拒绝启动。
func TestTCPListener_Aud040_RequireAuthStartupValidation(t *testing.T) {
	certPath, keyPath, caPath := writeAud040TestCert(t, t.TempDir())

	t.Run("requireAuth with full mTLS starts", func(t *testing.T) {
		ln, err := NewTCPListener(&TCPListenerConfig{
			Address: "127.0.0.1:0", Insecure: false,
			CertFile: certPath, KeyFile: keyPath, CAFile: caPath,
			RequireAuth: true,
		}, nil, nil, nil)
		require.NoError(t, err)
		_ = ln.Close()
	})

	t.Run("requireAuth with insecure rejected", func(t *testing.T) {
		_, err := NewTCPListener(&TCPListenerConfig{
			Address: "127.0.0.1:0", Insecure: true, RequireAuth: true,
		}, nil, nil, nil)
		require.ErrorContains(t, err, "RequireAuth=true requires mTLS")
	})

	t.Run("requireAuth with server-cert-only rejected", func(t *testing.T) {
		// 服务端证书-only（无 CA，无客户端证书校验）不算 mTLS。
		_, err := NewTCPListener(&TCPListenerConfig{
			Address: "127.0.0.1:0", Insecure: false,
			CertFile: certPath, KeyFile: keyPath,
			RequireAuth: true,
		}, nil, nil, nil)
		require.ErrorContains(t, err, "RequireAuth=true requires mTLS")
	})
}

// TestAgentSessionHandler_Aud040_RegisterAuthorization 验证注册级门禁：
// RequireAuth 要求对端 mTLS 身份；AllowedGameEnvs 白名单限定可声明作用域。
func TestAgentSessionHandler_Aud040_RegisterAuthorization(t *testing.T) {
	certPath, keyPath, caPath := writeAud040TestCert(t, t.TempDir())

	// newAuthHandler 构造带指定 peerIdentity 的 handler；监听面配置为合法 mTLS
	//（RequireAuth 启动校验通过），peerIdentity 由测试直接注入模拟 serveConn。
	newAuthHandler := func(peerID string, allowed []GameEnvScope) (*agentSessionHandler, *TCPListener) {
		listener, err := NewTCPListener(&TCPListenerConfig{
			Address: "127.0.0.1:0", Insecure: false,
			CertFile: certPath, KeyFile: keyPath, CAFile: caPath,
			RequireAuth: true, AllowedGameEnvs: allowed,
		}, nil, nil, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = listener.Close() })
		return &agentSessionHandler{listener: listener, peerIdentity: peerID}, listener
	}
	registerReq := func(agentID, gameID, env string) []byte {
		data, err := proto.Marshal(&agentv1.RegisterRequest{
			AgentId: agentID, GameId: gameID, Env: env, Version: "1.0.0",
		})
		require.NoError(t, err)
		return data
	}

	t.Run("requireAuth without peer identity rejected", func(t *testing.T) {
		// serveConn 在 mTLS 下总会填充 peerIdentity，此处空值模拟握手未产出
		// 身份（注册级二次防线）。
		h, listener := newAuthHandler("", nil)
		_, err := h.handleRegister(context.Background(), registerReq("a1", "game-1", "prod"))
		require.ErrorContains(t, err, "mTLS client identity")
		_, ok := listener.sessionStore.Get("a1")
		require.False(t, ok, "未授权注册不得物化会话")
	})

	t.Run("requireAuth with peer identity allowed", func(t *testing.T) {
		h, _ := newAuthHandler("cn=agent-1", nil)
		resp, err := h.handleRegister(context.Background(), registerReq("a1", "game-1", "prod"))
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("allowlist allows matching scope", func(t *testing.T) {
		h, _ := newAuthHandler("cn=agent-1", []GameEnvScope{{GameID: "game-1", Env: "prod"}})
		_, err := h.handleRegister(context.Background(), registerReq("a1", "game-1", "prod"))
		require.NoError(t, err)
	})

	t.Run("allowlist rejects out-of-scope", func(t *testing.T) {
		h, listener := newAuthHandler("cn=agent-1", []GameEnvScope{{GameID: "game-1", Env: "prod"}})
		_, err := h.handleRegister(context.Background(), registerReq("a2", "game-2", "prod"))
		require.ErrorContains(t, err, "not authorized for game_id")
		_, ok := listener.sessionStore.Get("a2")
		require.False(t, ok, "越权作用域注册不得物化会话")
	})
}

// TestCertificateIdentity_Aud040 验证客户端证书身份推导：优先 CN，缺 CN 用
// DNS SAN，均无则空（交由 RequireAuth 拒绝）。
func TestCertificateIdentity_Aud040(t *testing.T) {
	assert.Equal(t, "agent-1", certificateIdentity(&x509.Certificate{
		Subject: pkix.Name{CommonName: "agent-1"},
	}))
	assert.Equal(t, "node-a.example.com", certificateIdentity(&x509.Certificate{
		DNSNames: []string{"node-a.example.com"},
	}))
	assert.Empty(t, certificateIdentity(&x509.Certificate{}))
	assert.Empty(t, certificateIdentity(nil))
}
