package minecraft

// slp.go 补测：Check 的默认超时兜底臂、ctx 期限优先臂、SetDeadline 失败臂、
// 握手地址端口非法臂与写握手失败臂（经 dialer 注入桩连接）、protocolVersion
// 两臂、dialSLP 的空地址臂、resolveSLPAddress 的非法地址臂、splitSLPAddress
// 的 resolve 失败透传臂、appendVarInt 多字节循环体、readVarInt 的 EOF 与超长
// 臂、readStatusResponse 的帧长/载荷截断/包 ID/串长各守卫臂、readConnVarInt
// 超长臂、parseStatus 的 version 形状错误与 players 容错臂。
//
// 诚实跳过：readStatusResponse 的最终 io.ReadFull（301）——串长守卫（296）
// 已保证 strLen ≤ bytes.Reader 剩余量，bytes.Reader 读满前不报错，该臂不可
// 达；splitSLPAddress 的 resolve 后再 SplitHostPort 防御臂（210）——
// resolveSLPAddress 返回值要么原样透传（已在 resolve 内验证可拆分），要么是
// JoinHostPort 输出（两类被容忍的 AddrError 输入经补括号后均可拆分），二次
// 拆分恒成功。

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stubConn 测试桩连接：从内置 reader 读、写直丢弃，可注入 deadline/write 错误。
type stubConn struct {
	io.Reader
	setDeadlineErr error
	writeErr       error
}

func (c *stubConn) Read(p []byte) (int, error) { return c.Reader.Read(p) }

func (c *stubConn) Write(p []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(p), nil
}

func (c *stubConn) Close() error                       { return nil }
func (c *stubConn) LocalAddr() net.Addr                { return stubAddr{} }
func (c *stubConn) RemoteAddr() net.Addr               { return stubAddr{} }
func (c *stubConn) SetDeadline(time.Time) error        { return c.setDeadlineErr }
func (c *stubConn) SetReadDeadline(t time.Time) error  { return c.setDeadlineErr }
func (c *stubConn) SetWriteDeadline(t time.Time) error { return c.setDeadlineErr }

type stubAddr struct{}

func (stubAddr) Network() string { return "tcp" }
func (stubAddr) String() string  { return "stub" }

func stubDialer(conn net.Conn) func(context.Context, string, string) (net.Conn, error) {
	return func(context.Context, string, string) (net.Conn, error) { return conn, nil }
}

func TestSLPCheckerDefaultTimeoutAndCtxDeadline(t *testing.T) {
	addr, stop := startSLPServer(t, statusFixture("vanilla", 47, 0, 0))
	defer stop()

	// Timeout<=0 → DefaultSLPTimeout（86）
	chk := NewSLPChecker(addr, 0)
	require.NoError(t, chk.Check(context.Background()))
	require.Equal(t, "done", chk.Details()["phase"])

	// ctx 期限早于超时上限 → 取 ctx 期限（97）
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	chk2 := NewSLPChecker(addr, time.Hour)
	require.NoError(t, chk2.Check(ctx))
}

func TestSLPCheckerDeadlineSetFailure(t *testing.T) {
	chk := &SLPChecker{Address: "127.0.0.1:25565", dialer: stubDialer(
		&stubConn{Reader: bytes.NewReader(nil), setDeadlineErr: errors.New("deadline refused")})}
	err := chk.Check(context.Background())
	require.ErrorContains(t, err, "set deadline")
	require.Equal(t, "handshake", chk.Details()["phase"])
}

func TestSLPCheckerHandshakeBadPortAndWriteFailure(t *testing.T) {
	// 端口非法：splitSLPAddress 拒绝（split 的 invalid port 守卫 + Check 的
	// handshake 阶段透传臂）
	badPort := &SLPChecker{Address: "host:abc", dialer: stubDialer(&stubConn{Reader: bytes.NewReader(nil)})}
	err := badPort.Check(context.Background())
	require.ErrorContains(t, err, "invalid port")
	require.Equal(t, "handshake", badPort.Details()["phase"])

	// 写握手失败
	chk := &SLPChecker{Address: "127.0.0.1:25565", dialer: stubDialer(
		&stubConn{Reader: bytes.NewReader(nil), writeErr: errors.New("pipe broken")})}
	err = chk.Check(context.Background())
	require.ErrorContains(t, err, "handshake: write")
	require.Equal(t, "handshake", chk.Details()["phase"])
}

func TestProtocolVersionAndDialAddressArms(t *testing.T) {
	require.Equal(t, 47, (&SLPChecker{ProtocolVersion: 47}).protocolVersion())
	require.Equal(t, DefaultProtocolVersion, (&SLPChecker{}).protocolVersion())

	// 空地址：dialSLP 的 resolve 失败透传
	_, err := (&SLPChecker{}).dialSLP(context.Background(), time.Second)
	require.ErrorContains(t, err, "address required")
}

func TestResolveAndSplitAddressArms(t *testing.T) {
	// 裸 IPv6 之外的非法形态（缺 ']'）：非被容忍的 AddrError → invalid address
	_, err := resolveSLPAddress("[::1")
	require.ErrorContains(t, err, "invalid address")

	_, _, err = splitSLPAddress("")
	require.ErrorContains(t, err, "address required")
}

func TestAppendVarIntMultiByte(t *testing.T) {
	// 300 = 0b100101100 → 低 7 位 |0x80 后接终字节
	require.Equal(t, []byte{0xAC, 0x02}, appendVarInt(nil, 300))
}

func TestReadVarIntErrors(t *testing.T) {
	_, err := readVarInt(bytes.NewReader(nil))
	require.Error(t, err)

	_, err = readVarInt(bytes.NewReader([]byte{0x80, 0x80, 0x80, 0x80, 0x80}))
	require.ErrorContains(t, err, "varint too long")
}

func TestReadConnVarIntTooLong(t *testing.T) {
	five := []byte{0x80, 0x80, 0x80, 0x80, 0x80}
	_, err := readConnVarInt(&stubConn{Reader: bytes.NewReader(five)})
	require.ErrorContains(t, err, "varint too long")
}

func TestReadStatusResponseGuardArms(t *testing.T) {
	// 帧长为 0
	_, err := readStatusResponse(&stubConn{Reader: bytes.NewReader(testVarInt(nil, 0))})
	require.ErrorContains(t, err, "invalid frame length")

	// 载荷截断
	_, err = readStatusResponse(&stubConn{
		Reader: bytes.NewReader(append(testVarInt(nil, 8), 0x01))})
	require.ErrorContains(t, err, "read payload")

	// 包 ID 变长字节不完整
	_, err = readStatusResponse(&stubConn{
		Reader: bytes.NewReader(append(testVarInt(nil, 1), 0x80))})
	require.ErrorContains(t, err, "read packet id")

	// 串长变长字节不完整
	payload := append([]byte{slpPacketResponse}, 0x80)
	_, err = readStatusResponse(&stubConn{Reader: bytes.NewReader(
		append(testVarInt(nil, uint32(len(payload))), payload...))})
	require.ErrorContains(t, err, "read string length")

	// 串长超出剩余载荷
	payload = append([]byte{slpPacketResponse}, testVarInt(nil, 16)...)
	_, err = readStatusResponse(&stubConn{Reader: bytes.NewReader(
		append(testVarInt(nil, uint32(len(payload))), payload...))})
	require.ErrorContains(t, err, "exceeds payload")
}

func TestParseStatusArms(t *testing.T) {
	// version 键存在但形状错 → malformed version
	_, _, err := parseStatus([]byte(`{"version":5}`))
	require.ErrorContains(t, err, "malformed version")

	// players 解析失败被容忍：以 version 为准返回成功、players 归零
	st, pl, err := parseStatus([]byte(`{"version":{"name":"v","protocol":47},"players":"oops"}`))
	require.NoError(t, err)
	require.Equal(t, "v", st.Name)
	require.Equal(t, 0, pl.Online)
}
