package session

// sshtarget.go 补测：NewSSHChannelTarget 装配守卫闭集（缺用户/缺凭据/坏密钥/
// 空口令文件/缺 host key 策略/坏 known_hosts）、真实 SSH 会话的 ID/Stderr/
// Wait 收尾、Resize 非法窗口与已断会话分支。

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSSHChannelTarget_Guards(t *testing.T) {
	_, err := NewSSHChannelTarget(SSHConfig{User: "  "})
	require.ErrorContains(t, err, "user is required")

	// 无任何凭据
	_, err = NewSSHChannelTarget(SSHConfig{User: "ops"})
	require.ErrorContains(t, err, "requires private key or password")

	// 私钥文件缺失 / 内容非 PEM
	_, err = NewSSHChannelTarget(SSHConfig{
		User: "ops", PrivateKeyFile: filepath.Join(t.TempDir(), "missing.pem")})
	require.ErrorContains(t, err, "read ssh private key")
	_, err = NewSSHChannelTarget(SSHConfig{
		User: "ops", PrivateKeyFile: writeTestFile(t, "bad.pem", []byte("not a key"))})
	require.ErrorContains(t, err, "parse ssh private key")

	// 口令文件缺失 / 空文件
	_, err = NewSSHChannelTarget(SSHConfig{
		User: "ops", PasswordFile: filepath.Join(t.TempDir(), "missing.txt")})
	require.ErrorContains(t, err, "read ssh password file")
	_, err = NewSSHChannelTarget(SSHConfig{
		User: "ops", PasswordFile: writeTestFile(t, "empty.txt", []byte(""))})
	require.ErrorContains(t, err, "password file is empty")

	// 有凭据但缺 host key 策略；known_hosts 文件打不开
	_, err = NewSSHChannelTarget(SSHConfig{
		User: "ops", PasswordFile: writeTestFile(t, "p.txt", []byte("pw\n"))})
	require.ErrorContains(t, err, "host key policy required")
	_, err = NewSSHChannelTarget(SSHConfig{
		User: "ops", PasswordFile: writeTestFile(t, "p2.txt", []byte("pw\n")),
		KnownHostsFile: filepath.Join(t.TempDir(), "no-known-hosts")})
	require.ErrorContains(t, err, "load known_hosts")
}

func TestSSHTerminalLifecycleStderrAndWait(t *testing.T) {
	srv := newEchoSSHServer(t, "ops", "secret")
	tgt := newTestTarget(t, "ops", "secret")
	require.Equal(t, "ssh", tgt.Name())

	host, port := srv.HostPort()
	sess, err := tgt.Open(context.Background(), SessionRequest{
		SessionID: "sess-x", Protocol: ProtocolSSH, Host: host, Port: port,
	})
	require.NoError(t, err)
	term, ok := sess.(Terminal)
	require.True(t, ok)
	require.Equal(t, "sess-x", sess.ID())
	require.NotNil(t, term.Stderr())

	// 非法窗口尺寸被闸在本地
	require.Error(t, term.Resize(0, 0))

	// 断开后 Resize：远端链已拆 → 报错
	require.NoError(t, sess.Close())
	require.Error(t, term.Resize(120, 40))

	// Wait 收尾（幂等 once）：已断会话立即返回
	exit, err := term.Wait()
	_ = exit
	require.Error(t, err, "closed remote session must surface a wait error")
	_, err = term.Wait()
	require.Error(t, err, "second wait must reuse cached result")
}
