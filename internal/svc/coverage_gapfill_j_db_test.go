package svc

// 假 PostgreSQL / MySQL wire 服务器：覆盖 openGorm / EnsureGameDatabase 中
// 「数据库不存在 → 自动建库 → 重连」链路的错误与成功分支。
// PostgreSQL 侧走 simple query 协议（DSN 强制 default_query_exec_mode=
// simple_protocol）；MySQL 侧实现最小握手 + ERR/OK 脚本。

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// fake PostgreSQL
// ---------------------------------------------------------------------------

type fakePGServer struct {
	ln net.Listener

	mu          sync.Mutex
	closed      bool
	pingCount   int
	failPing    int // 前 N 个非 CREATE/probe 查询（即 Ping）回 does-not-exist 错误
	failCreate  bool
	probeExists bool
}

func startFakePGServer(t *testing.T) (*fakePGServer, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &fakePGServer{ln: ln}
	go s.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return s, ln.Addr().String()
}

func (s *fakePGServer) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakePGServer) handle(conn net.Conn) {
	defer conn.Close()
	// StartupMessage / SSLRequest：int32 长度前缀帧。
	head := make([]byte, 4)
	for {
		if _, err := io.ReadFull(conn, head); err != nil {
			return
		}
		frameLen := int(binary.BigEndian.Uint32(head))
		if frameLen < 8 || frameLen > 10000 {
			return
		}
		body := make([]byte, frameLen-4)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		code := binary.BigEndian.Uint32(body)
		if code == 80877103 { // SSLRequest
			_, _ = conn.Write([]byte{'N'})
			continue
		}
		break // StartupMessage
	}
	// AuthenticationOk + ReadyForQuery
	writePGMessage(conn, 'R', func(b *pgBuf) { b.int32(0) })
	writePGMessage(conn, 'Z', func(b *pgBuf) { b.byte('I') })

	for {
		typ, payload, err := readPGMessage(conn)
		if err != nil {
			return
		}
		switch typ {
		case 'Q':
			query := strings.TrimSpace(strings.TrimRight(string(payload), "\x00"))
			s.answerQuery(conn, query)
		case 'X': // Terminate
			return
		default:
			// 忽略其他 frontend 消息。
		}
	}
}

func (s *fakePGServer) answerQuery(conn net.Conn, query string) {
	s.mu.Lock()
	failPing := s.pingCount < s.failPing
	if !strings.HasPrefix(strings.ToUpper(query), "CREATE DATABASE") &&
		!strings.HasPrefix(strings.ToUpper(query), "SELECT EXISTS") {
		s.pingCount++
	}
	failCreate := s.failCreate
	probeExists := s.probeExists
	s.mu.Unlock()

	upper := strings.ToUpper(query)
	switch {
	case strings.HasPrefix(upper, "CREATE DATABASE"):
		if failCreate {
			writePGError(conn, "3D000", fmt.Sprintf("database %q already exists", "x"))
			writePGMessage(conn, 'Z', func(b *pgBuf) { b.byte('I') })
			return
		}
		writePGMessage(conn, 'C', func(b *pgBuf) { b.str("CREATE DATABASE") })
	case strings.HasPrefix(upper, "SELECT EXISTS"):
		if !probeExists {
			writePGError(conn, "3D000", "probe failed")
			writePGMessage(conn, 'Z', func(b *pgBuf) { b.byte('I') })
			return
		}
		writePGMessage(conn, 'T', func(b *pgBuf) {
			b.int16(1)
			b.str("exists")
			b.int32(0)  // table oid
			b.int16(0)  // column attr
			b.int32(16) // bool type oid
			b.int16(1)
			b.int32(-1)
			b.int16(0)
		})
		writePGMessage(conn, 'D', func(b *pgBuf) {
			b.int16(1)
			b.int32(1)
			b.byte('t')
		})
		writePGMessage(conn, 'C', func(b *pgBuf) { b.str("SELECT 1") })
	case failPing:
		writePGError(conn, "3D000", fmt.Sprintf("database %q does not exist", "game_x"))
		writePGMessage(conn, 'Z', func(b *pgBuf) { b.byte('I') })
		return
	default:
		writePGMessage(conn, 'C', func(b *pgBuf) { b.str("SELECT 1") })
	}
	writePGMessage(conn, 'Z', func(b *pgBuf) { b.byte('I') })
}

type pgBuf struct{ b []byte }

func (p *pgBuf) byte(v byte) { p.b = append(p.b, v) }
func (p *pgBuf) int16(v int16) {
	p.b = binary.BigEndian.AppendUint16(p.b, uint16(v))
}
func (p *pgBuf) int32(v int32) {
	p.b = binary.BigEndian.AppendUint32(p.b, uint32(v))
}
func (p *pgBuf) str(v string) { p.b = append(p.b, v...); p.b = append(p.b, 0) }

func writePGMessage(conn net.Conn, typ byte, build func(*pgBuf)) {
	var body pgBuf
	build(&body)
	head := make([]byte, 5)
	// PG 协议：长度字段 = 4（自身）+ payload，不含类型字节。
	binary.BigEndian.PutUint32(head, uint32(len(body.b)+4))
	head[4] = typ
	_, _ = conn.Write(head)
	_, _ = conn.Write(body.b)
}

func writePGError(conn net.Conn, code, message string) {
	writePGMessage(conn, 'E', func(b *pgBuf) {
		b.byte('S')
		b.str("ERROR")
		b.byte('V')
		b.str("ERROR")
		b.byte('C')
		b.str(code)
		b.byte('M')
		b.str(message)
		b.byte(0)
	})
}

func readPGMessage(conn net.Conn) (byte, []byte, error) {
	head := make([]byte, 5)
	if _, err := io.ReadFull(conn, head); err != nil {
		return 0, nil, err
	}
	frameLen := int(binary.BigEndian.Uint32(head[:4])) - 4
	if frameLen < 0 || frameLen > 1<<20 {
		return 0, nil, fmt.Errorf("bad frame length")
	}
	payload := make([]byte, frameLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return 0, nil, err
	}
	return head[4], payload, nil
}

func fakePGDSN(addr, dbName string) string {
	host, port, _ := net.SplitHostPort(addr)
	return fmt.Sprintf(
		"host=%s port=%s user=testuser dbname=%s sslmode=disable default_query_exec_mode=simple_protocol",
		host, port, dbName)
}

// L119-143: openGorm postgres 的「数据库不存在 → 建库 → 重连成功」链路。
func TestOpenGorm_PostgresAutoCreateSuccess(t *testing.T) {
	s, addr := startFakePGServer(t)
	s.mu.Lock()
	s.failPing = 1 // 第一次 Ping（game_x 库）报 does not exist；之后正常
	s.mu.Unlock()

	db, err := openGorm("postgres", fakePGDSN(addr, "game_x"))
	require.NoError(t, err)
	require.NotNil(t, db)
}

// L119-124: 连接错误提到数据库不存在，但 DSN 无法解析出库名。
func TestOpenGorm_PostgresAutoCreateNoDatabaseName(t *testing.T) {
	s, addr := startFakePGServer(t)
	s.mu.Lock()
	s.failPing = 1
	s.mu.Unlock()

	dsn := fmt.Sprintf("host=%s port=%s user=testuser sslmode=disable default_query_exec_mode=simple_protocol",
		strings.Split(addr, ":")[0], strings.Split(addr, ":")[1])
	_, err := openGorm("postgres", dsn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to extract database name")
}

// L131-133 + L411-413: CREATE 失败但并发建库已存在（probe 命中）→ 视为成功。
func TestOpenGorm_PostgresCreateRaceProbeExists(t *testing.T) {
	s, addr := startFakePGServer(t)
	s.mu.Lock()
	s.failPing = 1
	s.failCreate = true
	s.probeExists = true
	s.mu.Unlock()

	db, err := openGorm("postgres", fakePGDSN(addr, "game_x"))
	require.NoError(t, err)
	require.NotNil(t, db)
}

// L131-133 + L414: CREATE 失败且 probe 也失败。
func TestOpenGorm_PostgresCreateFails(t *testing.T) {
	s, addr := startFakePGServer(t)
	s.mu.Lock()
	s.failPing = 1
	s.failCreate = true
	s.probeExists = false
	s.mu.Unlock()

	_, err := openGorm("postgres", fakePGDSN(addr, "game_x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create database")
}

// L517-526: EnsureGameDatabase postgres：目标库不存在 → 建库成功 → 返回 DSN。
func TestEnsureGameDatabase_PostgresAutoCreate(t *testing.T) {
	s, addr := startFakePGServer(t)
	s.mu.Lock()
	s.failPing = 1
	s.mu.Unlock()

	metaDSN := fakePGDSN(addr, "croupier_meta")
	gameDSN, err := EnsureGameDatabase("postgres", metaDSN, "game_demo_prod")
	require.NoError(t, err)
	assert.Contains(t, gameDSN, "game_demo_prod")
}

// L519-521: EnsureGameDatabase postgres 建库失败。
func TestEnsureGameDatabase_PostgresCreateFails(t *testing.T) {
	s, addr := startFakePGServer(t)
	s.mu.Lock()
	s.failPing = 1
	s.failCreate = true
	s.probeExists = false
	s.mu.Unlock()

	_, err := EnsureGameDatabase("postgres", fakePGDSN(addr, "croupier_meta"), "game_demo_prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create postgres database")
}

// ---------------------------------------------------------------------------
// fake MySQL
// ---------------------------------------------------------------------------

type fakeMySQLServer struct {
	ln net.Listener

	mu           sync.Mutex
	firstConnErr bool // 第一个连接的握手后回 ERR 1049
	createErr    bool
}

func startFakeMySQLServer(t *testing.T) (*fakeMySQLServer, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &fakeMySQLServer{ln: ln}
	go s.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return s, ln.Addr().String()
}

func (s *fakeMySQLServer) acceptLoop() {
	connID := 0
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		connID++
		go s.handle(conn, connID)
	}
}

func (s *fakeMySQLServer) handle(conn net.Conn, connID int) {
	defer conn.Close()
	s.mu.Lock()
	firstErr := s.firstConnErr && connID == 1
	createErr := s.createErr
	s.mu.Unlock()

	if err := writeMySQLHandshake(conn, uint32(connID)); err != nil {
		return
	}
	// 读客户端 HandshakeResponse41（一帧，忽略内容）。
	if _, err := readMySQLPacket(conn); err != nil {
		return
	}
	if firstErr {
		writeMySQLERR(conn, 1049, "Unknown database 'game_x'")
		return
	}
	writeMySQLOK(conn)

	for {
		payload, err := readMySQLPacket(conn)
		if err != nil || len(payload) == 0 {
			return
		}
		switch payload[0] {
		case 0x01: // COM_QUIT
			return
		case 0x03: // COM_QUERY
			if createErr && strings.HasPrefix(strings.ToUpper(string(payload[1:])), "CREATE DATABASE") {
				writeMySQLERR(conn, 1007, "Can't create database")
				continue
			}
			writeMySQLOK(conn)
		case 0x0e: // COM_PING
			writeMySQLOK(conn)
		default:
			writeMySQLOK(conn)
		}
	}
}

func writeMySQLHandshake(conn net.Conn, threadID uint32) error {
	var p []byte
	p = append(p, 0x0a)
	p = append(p, "8.0.36\x00"...)
	p = binary.LittleEndian.AppendUint32(p, threadID)
	p = append(p, []byte("12345678")...) // auth-plugin-data-part-1
	p = append(p, 0x00)
	p = binary.LittleEndian.AppendUint16(p, 0x0200|0x8000|0x0004) // PROTOCOL_41|SECURE_CONN|CONNECT_WITH_DB
	p = append(p, 33)                                             // charset
	p = binary.LittleEndian.AppendUint16(p, 0x0002)               // status autocommit
	p = binary.LittleEndian.AppendUint16(p, 0x0008)               // PLUGIN_AUTH
	p = append(p, 21)                                             // auth data len
	p = append(p, make([]byte, 10)...)
	p = append(p, []byte("12345678901234\x00")...) // auth-plugin-data-part-2
	p = append(p, "mysql_native_password\x00"...)
	return writeMySQLPacket(conn, 0, p)
}

func writeMySQLPacket(conn net.Conn, seq byte, payload []byte) error {
	head := make([]byte, 4)
	n := len(payload)
	head[0] = byte(n)
	head[1] = byte(n >> 8)
	head[2] = byte(n >> 16)
	head[3] = seq
	if _, err := conn.Write(head); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}

func readMySQLPacket(conn net.Conn) ([]byte, error) {
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return nil, err
	}
	n := int(head[0]) | int(head[1])<<8 | int(head[2])<<16
	if n > 1<<20 {
		return nil, fmt.Errorf("packet too large")
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func writeMySQLOK(conn net.Conn) {
	var p []byte
	p = append(p, 0x00)
	p = append(p, 0, 0, 0) // affected rows
	p = append(p, 0, 0, 0) // last insert id
	p = binary.LittleEndian.AppendUint16(p, 0x0002)
	p = binary.LittleEndian.AppendUint16(p, 0x0000)
	_ = writeMySQLPacket(conn, 1, p)
}

func writeMySQLERR(conn net.Conn, code int, message string) {
	var p []byte
	p = append(p, 0xff)
	p = binary.LittleEndian.AppendUint16(p, uint16(code))
	p = append(p, '#')
	p = append(p, "42000"...)
	p = append(p, message...)
	_ = writeMySQLPacket(conn, 1, p)
}

func fakeMySQLDSN(addr, dbName string) string {
	return fmt.Sprintf("testuser:testpass@tcp(%s)/%s?allowNativePasswords=true", addr, dbName)
}

// L152-176: openGorm mysql 的 Unknown database → 建库 → 重连链路。
func TestOpenGorm_MySQLAutoCreateSuccess(t *testing.T) {
	s, addr := startFakeMySQLServer(t)
	s.mu.Lock()
	s.firstConnErr = true
	s.mu.Unlock()

	db, err := openGorm("mysql", fakeMySQLDSN(addr, "game_x"))
	require.NoError(t, err)
	require.NotNil(t, db)
}

// L164-166: 建库失败。
func TestOpenGorm_MySQLCreateFails(t *testing.T) {
	s, addr := startFakeMySQLServer(t)
	s.mu.Lock()
	s.firstConnErr = true
	s.createErr = true
	s.mu.Unlock()

	_, err := openGorm("mysql", fakeMySQLDSN(addr, "game_x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create database")
}

// L529-538: EnsureGameDatabase mysql：Unknown database → 建库 → 返回 DSN。
func TestEnsureGameDatabase_MySQLAutoCreate(t *testing.T) {
	s, addr := startFakeMySQLServer(t)
	s.mu.Lock()
	s.firstConnErr = true
	s.mu.Unlock()

	gameDSN, err := EnsureGameDatabase("mysql", fakeMySQLDSN(addr, "croupier_meta"), "game_demo_prod")
	require.NoError(t, err)
	assert.Contains(t, gameDSN, "game_demo_prod")
}

// L531-533: EnsureGameDatabase mysql 建库失败。
func TestEnsureGameDatabase_MySQLCreateFails(t *testing.T) {
	s, addr := startFakeMySQLServer(t)
	s.mu.Lock()
	s.firstConnErr = true
	s.createErr = true
	s.mu.Unlock()

	_, err := EnsureGameDatabase("mysql", fakeMySQLDSN(addr, "croupier_meta"), "game_demo_prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create mysql database")
}
