package minecraft

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	healthprobe "github.com/cuihairu/croupier/core/healthprobe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- 测试侧 SLP 服务器（按协议自写，非第三方代码）----

func testVarInt(b []byte, v uint32) []byte {
	for {
		if v&^0x7F == 0 {
			return append(b, byte(v))
		}
		b = append(b, byte(v&0x7F)|0x80)
		v >>= 7
	}
}

type testVarIntReader struct{ *bytes.Reader }

func (r testVarIntReader) readVarInt() (uint32, error) {
	var result uint32
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= uint32(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			return result, nil
		}
	}
	return 0, fmt.Errorf("varint too long")
}

// readTestFrame 读一帧：长度前缀 + 载荷。
func readTestFrame(conn net.Conn) ([]byte, error) {
	one := make([]byte, 1)
	var length uint32
	for i := 0; i < 5; i++ {
		if _, err := ioReadFull(conn, one); err != nil {
			return nil, err
		}
		length |= uint32(one[0]&0x7F) << (7 * i)
		if one[0]&0x80 == 0 {
			break
		}
	}
	payload := make([]byte, length)
	if _, err := ioReadFull(conn, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func ioReadFull(conn net.Conn, buf []byte) (int, error) {
	return ioReadFullReader(conn, buf)
}

func ioReadFullReader(r interface{ Read([]byte) (int, error) }, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// startSLPServer 起 mock SLP 服务器：解析握手（校验 nextState=1）、读 Status
// Request、回 statusJSON。statusJSON 为 nil 时回写完握手即断开（badServer 场景）。
func startSLPServer(t *testing.T, statusJSON []byte) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				hs, err := readTestFrame(conn)
				if err != nil {
					return
				}
				r := testVarIntReader{bytes.NewReader(hs)}
				id, err := r.readVarInt()
				if err != nil || id != 0 {
					return
				}
				proto, err := r.readVarInt() // 协议版本
				if err != nil {
					return
				}
				hostLen, err := r.readVarInt()
				if err != nil || hostLen > 1024 {
					return
				}
				host := make([]byte, hostLen)
				if _, err := ioReadFullReader(r, host); err != nil { // 帧内读，不动连接流
					return
				}
				portBytes := make([]byte, 2)
				if _, err := ioReadFullReader(r, portBytes); err != nil {
					return
				}
				nextState, err := r.readVarInt()
				if err != nil || nextState != 1 {
					return // 非 status 握手，不回应
				}
				_ = proto
				_ = string(host)

				if _, err := readTestFrame(conn); err != nil {
					return // Status Request
				}
				if statusJSON == nil {
					return
				}
				// 帧内载荷 = VarInt(包 ID) + VarInt(串长) + JSON。
				payload := append(testVarInt(nil, uint32(slpPacketResponse)), testVarInt(nil, uint32(len(statusJSON)))...)
				payload = append(payload, statusJSON...)
				resp := append(testVarInt(nil, uint32(len(payload))), payload...)
				_, _ = conn.Write(resp)
			}()
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

func statusFixture(name string, protocol int, online, max int) []byte {
	body := map[string]interface{}{
		"version": map[string]interface{}{"name": name, "protocol": protocol},
		"players": map[string]interface{}{"online": online, "max": max},
	}
	b, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return b
}

// ---- 用例 ----

func TestSLPCheckerSuccess(t *testing.T) {
	addr, stop := startSLPServer(t, statusFixture("Paper 1.21.4", 769, 3, 20))
	defer stop()

	chk := NewSLPChecker(addr, time.Second)
	require.NoError(t, chk.Check(context.Background()))

	d := chk.Details()
	assert.Equal(t, "minecraft", d["probeType"])
	assert.Equal(t, "done", d["phase"])
	assert.Equal(t, "Paper 1.21.4", d["version"])
	assert.Equal(t, "769", d["serverProtocol"])
	assert.Equal(t, "3/20", d["players"])
}

func TestSLPCheckerDialFailure(t *testing.T) {
	chk := NewSLPChecker("127.0.0.1:1", time.Second)
	err := chk.Check(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minecraft slp: dial")
	assert.NotContains(t, err.Error(), "protocol failed")
	assert.Equal(t, "dial", chk.Details()["phase"])
}

func TestSLPCheckerHandshakeClosed(t *testing.T) {
	// 接受连接后立即断开：写握手阶段失败（或读响应 EOF 前）→ handshake/status
	// 阶段可辨，绝非 dial。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { _ = conn.Close() }()
		}
	}()

	chk := NewSLPChecker(ln.Addr().String(), time.Second)
	err = chk.Check(context.Background())
	require.Error(t, err)
	d := chk.Details()
	assert.True(t, d["phase"] == "handshake" || d["phase"] == "status",
		"want handshake|status, got %q (err=%v)", d["phase"], err)
}

func TestSLPCheckerGarbageStatus(t *testing.T) {
	// 回一帧非协议字节（长度对了但包 ID 乱）：status 阶段失败。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				if _, err := readTestFrame(conn); err != nil { // 握手
					return
				}
				if _, err := readTestFrame(conn); err != nil { // Status Request
					return
				}
				garbage := append(testVarInt(nil, 3), 0x7f, 0x01, 0x02)
				_, _ = conn.Write(garbage)
			}()
		}
	}()

	chk := NewSLPChecker(ln.Addr().String(), time.Second)
	err = chk.Check(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minecraft slp: status")
	assert.Equal(t, "status", chk.Details()["phase"])
}

func TestSLPCheckerMissingVersion(t *testing.T) {
	// JSON 合法但没有 version 键：SLP 未验证（协议半途而非连通性问题）。
	addr, stop := startSLPServer(t, []byte(`{"players":{"online":1,"max":2}}`))
	defer stop()
	chk := NewSLPChecker(addr, time.Second)
	err := chk.Check(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response missing version")
	assert.Equal(t, "status", chk.Details()["phase"])
	assert.NotContains(t, chk.Details(), "players") // 失败不记摘要
}

func TestSLPCheckerInvalidJSON(t *testing.T) {
	addr, stop := startSLPServer(t, []byte(`not-json`))
	defer stop()
	chk := NewSLPChecker(addr, time.Second)
	err := chk.Check(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestSLPCheckerLargeFavicon(t *testing.T) {
	// favicon 大响应（~64KB）不破帧解析。
	body := statusFixture("Paper 1.21.4", 769, 0, 20)
	favicon := fmt.Sprintf(`,"favicon":"data:image/png;base64,%s"}`, strings.Repeat("A", 64*1024))
	withFavicon := append(bytes.TrimSuffix(body, []byte("}")), []byte(favicon)...)

	addr, stop := startSLPServer(t, withFavicon)
	defer stop()
	chk := NewSLPChecker(addr, time.Second)
	require.NoError(t, chk.Check(context.Background()))
	assert.Equal(t, "Paper 1.21.4", chk.Details()["version"])
}

// TestSLPCheckerHandshakeShape 校验握手帧内容（协议正确性而非仅成功）：
// 服务器侧收到的 host/port/nextState 断言。
func TestSLPCheckerHandshakeShape(t *testing.T) {
	var gotProto, gotHost, gotPort, gotNext atomic.Value
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				hs, err := readTestFrame(conn)
				if err != nil {
					return
				}
				r := testVarIntReader{bytes.NewReader(hs)}
				id, err := r.readVarInt()
				if err != nil || id != 0 {
					return
				}
				proto, _ := r.readVarInt()
				hostLen, _ := r.readVarInt()
				host := make([]byte, hostLen)
				if _, err := ioReadFullReader(r, host); err != nil { // 帧内读，不动连接流
					return
				}
				portBytes := make([]byte, 2)
				if _, err := ioReadFullReader(r, portBytes); err != nil {
					return
				}
				port := int(portBytes[0])<<8 | int(portBytes[1])
				next, _ := r.readVarInt()
				gotProto.Store(int(proto))
				gotHost.Store(string(host))
				gotPort.Store(port)
				gotNext.Store(int(next))

				if _, err := readTestFrame(conn); err != nil {
					return
				}
				sf := statusFixture("v", 1, 0, 0)
				payload := append(testVarInt(nil, uint32(slpPacketResponse)), testVarInt(nil, uint32(len(sf)))...)
				payload = append(payload, sf...)
				resp := append(testVarInt(nil, uint32(len(payload))), payload...)
				_, _ = conn.Write(resp)
			}()
		}
	}()

	// 地址显式端口：握手 host 不带端口、端口独立字段、nextState=1。
	chk := NewSLPChecker(ln.Addr().String(), time.Second)
	require.NoError(t, chk.Check(context.Background()))
	assert.EqualValues(t, DefaultProtocolVersion, gotProto.Load())
	assert.Equal(t, "127.0.0.1", gotHost.Load())
	port := gotPort.Load().(int)
	assert.Equal(t, ln.Addr().(*net.TCPAddr).Port, port)
	assert.EqualValues(t, 1, gotNext.Load())
}

func TestResolveSLPAddress(t *testing.T) {
	got, err := resolveSLPAddress("127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:25565", got) // 缺端口补默认

	got, err = resolveSLPAddress("mc.example.com:25566")
	require.NoError(t, err)
	assert.Equal(t, "mc.example.com:25566", got)

	got, err = resolveSLPAddress("::1")
	require.NoError(t, err)
	assert.Equal(t, "[::1]:25565", got) // 裸 IPv6 补方括号

	_, err = resolveSLPAddress("")
	assert.ErrorContains(t, err, "address required")
}

// TestSLPCheckerViaNewChecker 工厂接线：protocol=minecraft 解析到本包 Checker。
func TestSLPCheckerViaNewChecker(t *testing.T) {
	chk, err := healthprobe.NewChecker(healthprobe.TargetConfig{
		Name: "mc", Category: healthprobe.TargetCategoryGame, Type: healthprobe.TargetTypeProtocol, Address: "127.0.0.1:25565", Protocol: "minecraft",
	})
	require.NoError(t, err)
	slp, ok := chk.(*SLPChecker)
	require.True(t, ok)
	assert.Equal(t, "127.0.0.1:25565", slp.Address)
	assert.Contains(t, healthprobe.RegisteredGameProtocols(), "minecraft")
}
