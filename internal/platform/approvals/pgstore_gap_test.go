package approvals

import (
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakePGWireServer 起一个进程内最小化 PostgreSQL wire 协议服务端（仅
// 127.0.0.1 回环，无外部进程依赖）：完成 startup 握手（AuthenticationOk +
// ReadyForQuery），对简单协议查询（'Q'，gorm 自动 ping 的 "-- ping" 即此
// 路径）回 CommandComplete/ReadyForQuery。真实 SQL（扩展协议 Parse/Bind）
// 直接断开连接——NewPGStore 的 openDB 只需握手+ping 成功即可走到
// NewSQLStore 分支，AutoMigrate 随后失败是预期（桩不会说 SQL）。
func fakePGWireServer(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveFakePGConn(conn)
		}
	}()

	return "postgres://croupier:croupier@127.0.0.1:" + strconv.Itoa(ln.Addr().(*net.TCPAddr).Port) +
		"/croupier?sslmode=disable&connect_timeout=2"
}

func serveFakePGConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	// 首帧无类型前缀：int32 长度 + 载荷。可能是 SSLRequest 或 StartupMessage。
	startup, err := readPGUntypedFrame(conn)
	if err != nil {
		return
	}
	if len(startup) >= 4 && binary.BigEndian.Uint32(startup[:4]) == 80877103 {
		// SSLRequest → 拒绝升级（'N'），继续读真正的 StartupMessage。
		if _, err := conn.Write([]byte{'N'}); err != nil {
			return
		}
		_, err = readPGUntypedFrame(conn)
		if err != nil {
			return
		}
	}

	// AuthenticationOk。
	writePGMessage(conn, 'R', func(b []byte) []byte {
		return append(b, 0, 0, 0, 0)
	})
	// ParameterStatus（pgx 容忍缺失，补齐更接近真实服务端）。
	writePGParameterStatus(conn, "server_version", "16.2")
	writePGParameterStatus(conn, "client_encoding", "UTF8")
	// BackendKeyData。
	writePGMessage(conn, 'K', func(b []byte) []byte {
		return append(b, 0, 0, 0, 1, 0, 0, 0, 1)
	})
	// ReadyForQuery(idle)。
	writePGMessage(conn, 'Z', func(b []byte) []byte {
		return append(b, 'I')
	})

	for {
		msgType, _, err := readPGTypedFrame(conn)
		if err != nil {
			return
		}
		switch msgType {
		case 'Q': // 简单协议查询（含 gorm ping 的 "-- ping"）。
			// CommandComplete：tag 为空串，wire 体必须含结尾 NUL（解码端
			// 校验 IndexByte(src,0) == len(src)-1，缺 NUL 会判 bad connection）。
			writePGMessage(conn, 'C', func(b []byte) []byte { return append(b, 0) })
			writePGMessage(conn, 'Z', func(b []byte) []byte { return append(b, 'I') })
		case 'X': // Terminate。
			return
		default:
			// 扩展协议（Parse/Bind/...）说明调用方要跑真 SQL——断开让上层快速失败。
			return
		}
	}
}

// readPGUntypedFrame 读取 StartupMessage/SSLRequest 这类无类型字节帧。
func readPGUntypedFrame(conn net.Conn) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := readFull(conn, lenBuf[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n < 4 || n > 1<<20 {
		return nil, errFakePGProtocol
	}
	body := make([]byte, n-4)
	if _, err := readFull(conn, body); err != nil {
		return nil, err
	}
	return body, nil
}

// readPGTypedFrame 读取常规前端消息（1 字节类型 + int32 长度 + 载荷）。
func readPGTypedFrame(conn net.Conn) (byte, []byte, error) {
	var head [5]byte
	if _, err := readFull(conn, head[:]); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(head[1:5])
	if n < 4 || n > 1<<20 {
		return 0, nil, errFakePGProtocol
	}
	body := make([]byte, n-4)
	if _, err := readFull(conn, body); err != nil {
		return 0, nil, err
	}
	return head[0], body, nil
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if n > 0 {
			total += n
		}
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

type fakePGError struct{}

func (fakePGError) Error() string { return "fake pg protocol error" }

var errFakePGProtocol = fakePGError{}

// writePGMessage 写后端消息：类型字节 + int32 长度（含自身）+ 载荷。
func writePGMessage(conn net.Conn, msgType byte, build func([]byte) []byte) {
	body := build(nil)
	payload := make([]byte, 0, 5+len(body))
	payload = append(payload, msgType)
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(4+len(body)))
	payload = append(payload, lenBuf[:]...)
	payload = append(payload, body...)
	_, _ = conn.Write(payload)
}

func writePGParameterStatus(conn net.Conn, key, value string) {
	writePGMessage(conn, 'S', func(b []byte) []byte {
		b = append(b, key...)
		b = append(b, 0)
		b = append(b, value...)
		return append(b, 0)
	})
}

// TestNewPGStore_ReachesStoreConstruction：DSN 握手 + gorm 自动 ping 成功
// 后走到 NewSQLStore 构造分支并返回 Store（真实 PostgreSQL 的迁移不在
// 桩上验证——桩只回应握手与简单协议 ping）。
func TestNewPGStore_ReachesStoreConstruction(t *testing.T) {
	dsn := fakePGWireServer(t)

	store, err := NewPGStore(dsn)
	require.NoError(t, err, "handshake + ping against the stub server must succeed")
	require.NotNil(t, store)
}
