package svc

// coverage_gap_v11：继续补齐 openGorm / EnsureGameDatabase / seedBootstrap* 的
// 未覆盖分支：
//   - postgres 建库成功后重连失败（db.go L136-138）
//   - mysql DSN 无库名（L155-157）、建库后重连失败（L169-171）
//   - sqlserver「库不存在 → 建库 → 重连」全链路（L185-209、L541-550）：
//     通过最小 TDS 假服务器（prelogin + LOGIN7 + token 应答）驱动
//   - createSQLServerDatabase 的 DSN 解析失败（L458-460）
//   - seedBootstrapRoles / seedBootstrapPermissions 的 nil 条目跳过分支
//
// 所有 fake 均自包含（不依赖其他测试文件的辅助函数），仅监听 loopback。

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

// ============================================================================
// 假 PostgreSQL（按连接序号脚本化）：复用 simple query 协议思路
// ============================================================================

type gap11PGScriptServer struct {
	ln net.Listener

	mu sync.Mutex
	// startupErrs[i] 为第 i 个被接受连接在启动阶段返回的错误消息；
	// 空串表示正常完成握手。脚本之外的连接一律放行。
	startupErrs []string
	batchErr    bool // CREATE DATABASE 等查询返回错误
}

func startGap11PGScriptServer(t *testing.T, startupErrs []string, batchErr bool) (*gap11PGScriptServer, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &gap11PGScriptServer{ln: ln, startupErrs: startupErrs, batchErr: batchErr}
	go s.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return s, ln.Addr().String()
}

func (s *gap11PGScriptServer) acceptLoop() {
	connIdx := 0
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		idx := connIdx
		connIdx++
		go s.handle(conn, idx)
	}
}

func (s *gap11PGScriptServer) handle(conn net.Conn, idx int) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

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
		if binary.BigEndian.Uint32(body) == 80877103 { // SSLRequest
			_, _ = conn.Write([]byte{'N'})
			continue
		}
		break
	}

	s.mu.Lock()
	msg := ""
	if idx < len(s.startupErrs) {
		msg = s.startupErrs[idx]
	}
	batchErr := s.batchErr
	s.mu.Unlock()
	if msg != "" {
		gap11PGWriteError(conn, "3D000", msg)
		return
	}

	// AuthenticationOk + 必需 ParameterStatus + ReadyForQuery。
	gap11PGWriteMessage(conn, 'R', func(b *gap11PGBuf) { b.int32(0) })
	gap11PGWriteMessage(conn, 'S', func(b *gap11PGBuf) { b.str("server_version"); b.str("16.0") })
	gap11PGWriteMessage(conn, 'S', func(b *gap11PGBuf) { b.str("standard_conforming_strings"); b.str("on") })
	gap11PGWriteMessage(conn, 'S', func(b *gap11PGBuf) { b.str("client_encoding"); b.str("UTF8") })
	gap11PGWriteMessage(conn, 'Z', func(b *gap11PGBuf) { b.byte('I') })

	for {
		typ, payload, err := gap11PGReadMessage(conn)
		if err != nil {
			return
		}
		switch typ {
		case 'Q':
			gap11PGAnswerQuery(conn, string(payload), batchErr)
		case 'X':
			return
		}
	}
}

func gap11PGAnswerQuery(conn net.Conn, rawQuery string, batchErr bool) {
	query := trimNullSpace(rawQuery)
	if batchErr {
		gap11PGWriteError(conn, "3D000", "permission denied to create database")
		gap11PGWriteMessage(conn, 'Z', func(b *gap11PGBuf) { b.byte('I') })
		return
	}
	if startsWithIgnoreCase(query, "CREATE DATABASE") {
		gap11PGWriteMessage(conn, 'C', func(b *gap11PGBuf) { b.str("CREATE DATABASE") })
	} else {
		gap11PGWriteMessage(conn, 'C', func(b *gap11PGBuf) { b.str("SELECT 1") })
	}
	gap11PGWriteMessage(conn, 'Z', func(b *gap11PGBuf) { b.byte('I') })
}

func startsWithIgnoreCase(s, prefix string) bool {
	s = strings.TrimSpace(s)
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}

func trimNullSpace(s string) string {
	for len(s) > 0 && (s[len(s)-1] == 0 || s[len(s)-1] == ' ' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}

type gap11PGBuf struct{ b []byte }

func (p *gap11PGBuf) byte(v byte)   { p.b = append(p.b, v) }
func (p *gap11PGBuf) int32(v int32) { p.b = binary.BigEndian.AppendUint32(p.b, uint32(v)) }
func (p *gap11PGBuf) str(v string)  { p.b = append(p.b, v...); p.b = append(p.b, 0) }
func (p *gap11PGBuf) out() []byte   { return p.b }

func gap11PGWriteMessage(conn net.Conn, typ byte, build func(*gap11PGBuf)) {
	var body gap11PGBuf
	build(&body)
	head := make([]byte, 5)
	head[0] = typ
	binary.BigEndian.PutUint32(head[1:], uint32(len(body.b)+4))
	_, _ = conn.Write(head)
	_, _ = conn.Write(body.b)
}

func gap11PGWriteError(conn net.Conn, code, message string) {
	gap11PGWriteMessage(conn, 'E', func(b *gap11PGBuf) {
		b.byte('S')
		b.str("FATAL")
		b.byte('V')
		b.str("FATAL")
		b.byte('C')
		b.str(code)
		b.byte('M')
		b.str(message)
		b.byte(0)
	})
}

func gap11PGReadMessage(conn net.Conn) (byte, []byte, error) {
	head := make([]byte, 5)
	if _, err := io.ReadFull(conn, head); err != nil {
		return 0, nil, err
	}
	frameLen := int(binary.BigEndian.Uint32(head[1:5])) - 4
	if frameLen < 0 || frameLen > 1<<20 {
		return 0, nil, fmt.Errorf("bad frame length")
	}
	payload := make([]byte, frameLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return 0, nil, err
	}
	return head[0], payload, nil
}

func gap11PGDSN(addr, dbName string) string {
	host, port, _ := net.SplitHostPort(addr)
	return fmt.Sprintf(
		"host=%s port=%s user=testuser dbname=%s sslmode=disable default_query_exec_mode=simple_protocol",
		host, port, dbName)
}

// L136-138: postgres 建库成功后重连失败。
// 连接脚本：#1 报 database does not exist；#2（建库）正常；#3（重连）认证失败。
func TestOpenGorm_PostgresReconnectAfterCreateFails(t *testing.T) {
	_, addr := startGap11PGScriptServer(t, []string{
		`database "game_x" does not exist`,
		"",
		`password authentication failed for user "testuser"`,
	}, false)

	_, err := openGorm("postgres", gap11PGDSN(addr, "game_x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect after creating database")
}

// L410-414 之外的另一条 create 失败路径：CREATE DATABASE 查询被拒（probe 也失败）。
func TestOpenGorm_PostgresCreateQueryDenied(t *testing.T) {
	_, addr := startGap11PGScriptServer(t, []string{
		`database "game_x" does not exist`,
		"",
	}, true)

	_, err := openGorm("postgres", gap11PGDSN(addr, "game_x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create database")
}

// ============================================================================
// 假 MySQL（按连接序号脚本化）
// ============================================================================

type gap11MySQLScriptConn struct {
	authErrCode int // >0：握手后回 ERR
	authErrMsg  string
	batchErr    bool // CREATE DATABASE 查询回 ERR
}

type gap11MySQLScriptServer struct {
	ln    net.Listener
	mu    sync.Mutex
	conns []gap11MySQLScriptConn
}

func startGap11MySQLScriptServer(t *testing.T, conns []gap11MySQLScriptConn) (*gap11MySQLScriptServer, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &gap11MySQLScriptServer{ln: ln, conns: conns}
	go s.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return s, ln.Addr().String()
}

func (s *gap11MySQLScriptServer) acceptLoop() {
	connIdx := 0
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		idx := connIdx
		connIdx++
		go s.handle(conn, idx)
	}
}

func (s *gap11MySQLScriptServer) handle(conn net.Conn, idx int) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	s.mu.Lock()
	var script gap11MySQLScriptConn
	if idx < len(s.conns) {
		script = s.conns[idx]
	}
	s.mu.Unlock()

	if err := gap11MySQLWriteHandshake(conn, uint32(idx+1)); err != nil {
		return
	}
	// HandshakeResponse41（一帧，忽略内容）。
	if _, err := gap11MySQLReadPacket(conn); err != nil {
		return
	}
	if script.authErrCode > 0 {
		gap11MySQLWritePacket(conn, 2, gap11MySQLErrBody(script.authErrCode, script.authErrMsg))
		return
	}
	gap11MySQLWritePacket(conn, 2, gap11MySQLOKBody())

	for {
		payload, err := gap11MySQLReadPacket(conn)
		if err != nil || len(payload) == 0 {
			return
		}
		switch payload[0] {
		case 0x01: // COM_QUIT
			return
		case 0x03: // COM_QUERY
			if script.batchErr && startsWithIgnoreCase(trimNullSpace(string(payload[1:])), "CREATE DATABASE") {
				gap11MySQLWritePacket(conn, 1, gap11MySQLErrBody(1007, "Can't create database"))
				continue
			}
			if startsWithIgnoreCase(trimNullSpace(string(payload[1:])), "SELECT VERSION()") {
				gap11MySQLWriteVersionResultSet(conn)
				continue
			}
			gap11MySQLWritePacket(conn, 1, gap11MySQLOKBody())
		default:
			gap11MySQLWritePacket(conn, 1, gap11MySQLOKBody())
		}
	}
}

func gap11MySQLWriteHandshake(conn net.Conn, threadID uint32) error {
	var p []byte
	p = append(p, 0x0a)
	p = append(p, "8.0.36\x00"...)
	p = binary.LittleEndian.AppendUint32(p, threadID)
	p = append(p, []byte("12345678")...)
	p = append(p, 0x00)
	p = binary.LittleEndian.AppendUint16(p, 0x0200|0x8000|0x0004)
	p = append(p, 33)
	p = binary.LittleEndian.AppendUint16(p, 0x0002)
	p = binary.LittleEndian.AppendUint16(p, 0x0008)
	p = append(p, 21)
	p = append(p, make([]byte, 10)...)
	p = append(p, []byte("123456789012\x00")...)
	p = append(p, "mysql_native_password\x00"...)
	return gap11MySQLWritePacket(conn, 0, p)
}

func gap11MySQLWritePacket(conn net.Conn, seq byte, payload []byte) error {
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

func gap11MySQLReadPacket(conn net.Conn) ([]byte, error) {
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

func gap11MySQLOKBody() []byte {
	var p []byte
	p = append(p, 0x00)
	p = append(p, 0, 0, 0)
	p = append(p, 0, 0, 0)
	p = binary.LittleEndian.AppendUint16(p, 0x0002)
	p = binary.LittleEndian.AppendUint16(p, 0x0000)
	return p
}

func gap11MySQLErrBody(code int, message string) []byte {
	var p []byte
	p = append(p, 0xff)
	p = binary.LittleEndian.AppendUint16(p, uint16(code))
	p = append(p, '#')
	p = append(p, "42000"...)
	p = append(p, message...)
	return p
}

// 回应 SELECT VERSION()：单列单行 "8.0.36"（gorm mysql dialector 初始化需要）。
func gap11MySQLWriteVersionResultSet(conn net.Conn) {
	_ = gap11MySQLWritePacket(conn, 1, []byte{0x01})
	var col []byte
	for _, s := range []string{"def", "", "", "", "VERSION()", ""} {
		col = append(col, byte(len(s)))
		col = append(col, s...)
	}
	col = append(col, 0x0c)
	col = binary.LittleEndian.AppendUint16(col, 33)
	col = binary.LittleEndian.AppendUint32(col, 32)
	col = append(col, 0xfd)
	col = binary.LittleEndian.AppendUint16(col, 0)
	col = append(col, 0)
	col = append(col, 0, 0)
	_ = gap11MySQLWritePacket(conn, 2, col)
	_ = gap11MySQLWritePacket(conn, 3, []byte{0xfe, 0x00, 0x00, 0x02, 0x00})
	row := []byte{byte(len("8.0.36"))}
	row = append(row, "8.0.36"...)
	_ = gap11MySQLWritePacket(conn, 4, row)
	_ = gap11MySQLWritePacket(conn, 5, []byte{0xfe, 0x00, 0x00, 0x02, 0x00})
}

func gap11MySQLDSN(addr string) string {
	return fmt.Sprintf("testuser:testpass@tcp(%s)/?allowNativePasswords=true", addr)
}

func gap11MySQLDSNWithDB(addr, dbName string) string {
	return fmt.Sprintf("testuser:testpass@tcp(%s)/%s?allowNativePasswords=true", addr, dbName)
}

// L155-157: mysql 连接报 Unknown database 但 DSN 中解析不出库名。
func TestOpenGorm_MySQLNoDatabaseNameInDSN(t *testing.T) {
	_, addr := startGap11MySQLScriptServer(t, []gap11MySQLScriptConn{
		{authErrCode: 1049, authErrMsg: "Unknown database ''"},
	})

	_, err := openGorm("mysql", gap11MySQLDSN(addr))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to extract database name")
}

// L169-171: mysql 建库成功后重连失败。
// 连接脚本：#1 Unknown database；#2 正常（VERSION + CREATE OK）；#3 Access denied。
func TestOpenGorm_MySQLReconnectAfterCreateFails(t *testing.T) {
	_, addr := startGap11MySQLScriptServer(t, []gap11MySQLScriptConn{
		{authErrCode: 1049, authErrMsg: "Unknown database 'game_x'"},
		{},
		{authErrCode: 1045, authErrMsg: "Access denied for user 'testuser'"},
	})

	_, err := openGorm("mysql", gap11MySQLDSNWithDB(addr, "game_x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect after creating database")
}

// ============================================================================
// 假 SQL Server（最小 TDS 协议：prelogin + LOGIN7 + ERROR/DONE token）
// ============================================================================

type gap11TDSScriptConn struct {
	loginErr string // 非空：登录阶段回 ERROR token（消息需含分支匹配关键字）
	batchErr bool   // SQL batch（CREATE DATABASE）回 ERROR
}

type gap11TDSServer struct {
	ln    net.Listener
	mu    sync.Mutex
	conns []gap11TDSScriptConn
}

func startGap11TDSServer(t *testing.T, conns []gap11TDSScriptConn) (*gap11TDSServer, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &gap11TDSServer{ln: ln, conns: conns}
	go s.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return s, ln.Addr().String()
}

func (s *gap11TDSServer) acceptLoop() {
	connIdx := 0
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		idx := connIdx
		connIdx++
		go s.handle(conn, idx)
	}
}

func (s *gap11TDSServer) handle(conn net.Conn, idx int) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	s.mu.Lock()
	var script gap11TDSScriptConn
	if idx < len(s.conns) {
		script = s.conns[idx]
	}
	s.mu.Unlock()

	// 1) PRELOGIN（包类型 0x12）：回 VERSION + ENCRYPTION=not supported（全程明文）。
	typ, _, _, err := gap11TDSReadPacket(conn)
	if err != nil || typ != 0x12 {
		return
	}
	if err := gap11TDSWritePacket(conn, 4, gap11TDSPreloginResponse()); err != nil {
		return
	}

	// 2) LOGIN7（包类型 0x10）：读到 EOM 为止（内容不解析）。
	for {
		typ, status, _, err := gap11TDSReadPacket(conn)
		if err != nil || typ != 0x10 {
			return
		}
		if status&0x01 != 0 {
			break
		}
	}

	// 3) 登录结果（ERROR 必须跟 DONE token，登录循环靠 DONE 携带错误并终止）。
	if script.loginErr != "" {
		_ = gap11TDSWritePacket(conn, 4, append(gap11TDSErrorToken(script.loginErr), gap11TDSDoneToken()...))
		return
	}
	if err := gap11TDSWritePacket(conn, 4, append(gap11TDSLoginAckToken(), gap11TDSDoneToken()...)); err != nil {
		return
	}

	// 4) 后续请求（SQL batch 等）。
	for {
		typ, _, _, err := gap11TDSReadPacket(conn)
		if err != nil {
			return
		}
		switch typ {
		case 0x12:
			_ = gap11TDSWritePacket(conn, 4, gap11TDSPreloginResponse())
		case 0x01: // SQL batch
			if script.batchErr {
				_ = gap11TDSWritePacket(conn, 4, append(gap11TDSErrorToken("CREATE DATABASE failed"), gap11TDSDoneToken()...))
				continue
			}
			_ = gap11TDSWritePacket(conn, 4, gap11TDSDoneToken())
		default:
			_ = gap11TDSWritePacket(conn, 4, gap11TDSDoneToken())
		}
	}
}

func gap11TDSWritePacket(conn net.Conn, typ byte, payload []byte) error {
	total := 8 + len(payload)
	head := []byte{typ, 0x01, byte(total >> 8), byte(total & 0xff), 0, 0, 1, 0}
	if _, err := conn.Write(head); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}

func gap11TDSReadPacket(conn net.Conn) (typ byte, status byte, payload []byte, err error) {
	head := make([]byte, 8)
	if _, err = io.ReadFull(conn, head); err != nil {
		return
	}
	typ = head[0]
	status = head[1]
	size := int(binary.BigEndian.Uint16(head[2:4]))
	if size < 8 {
		err = fmt.Errorf("bad tds packet size")
		return
	}
	payload = make([]byte, size-8)
	_, err = io.ReadFull(conn, payload)
	return
}

// PRELOGIN 响应：选项表（5 字节/项 + 0xFF 终止）+ 数据区。
// VERSION(0x00) 6 字节；ENCRYPTION(0x01)=0x02（不支持加密 → 客户端全程明文）。
func gap11TDSPreloginResponse() []byte {
	p := []byte{
		0x00, 0x00, 0x0b, 0x00, 0x06, // VERSION @11 len6
		0x01, 0x00, 0x11, 0x00, 0x01, // ENCRYPTION @17 len1
		0xff,
	}
	p = append(p, 0x0f, 0x00, 0x00, 0x9f, 0x00, 0x00)
	p = append(p, 0x02)
	return p
}

func gap11Ucs2(s string) []byte {
	out := make([]byte, 0, len(s)*2)
	for _, u := range utf16.Encode([]rune(s)) {
		out = binary.LittleEndian.AppendUint16(out, u)
	}
	return out
}

// ERROR token (0xAA)：长度/编号/状态/类别/消息(UsVarChar)/服务器/过程/行号。
// 注意：go-mssqldb 解析时 ServerName/ProcName 也按 UCS2 处理（长度=字符数）。
func gap11TDSErrorToken(msg string) []byte {
	var body []byte
	body = binary.LittleEndian.AppendUint32(body, 4060)
	body = append(body, 1, 16)
	m := gap11Ucs2(msg)
	body = binary.LittleEndian.AppendUint16(body, uint16(len(m)/2))
	body = append(body, m...)
	srv := gap11Ucs2("localhost")
	body = append(body, uint8(len(srv)/2))
	body = append(body, srv...)
	body = append(body, 0) // ProcName: 空串
	body = binary.LittleEndian.AppendUint32(body, 1)
	tok := []byte{0xAA}
	tok = binary.LittleEndian.AppendUint16(tok, uint16(len(body)))
	return append(tok, body...)
}

// LOGINACK token (0xAD)。
func gap11TDSLoginAckToken() []byte {
	var body []byte
	body = append(body, 0x01)
	body = binary.BigEndian.AppendUint32(body, 0x74000004)
	prog := gap11Ucs2("Microsoft SQL Server")
	body = binary.LittleEndian.AppendUint16(body, uint16(len(prog)/2))
	body = append(body, prog...)
	body = binary.BigEndian.AppendUint32(body, 0x0f000000)
	tok := []byte{0xAD}
	tok = binary.LittleEndian.AppendUint16(tok, uint16(len(body)))
	return append(tok, body...)
}

// DONE token (0xFD)：status=final。
func gap11TDSDoneToken() []byte {
	b := []byte{0xFD}
	b = binary.LittleEndian.AppendUint16(b, 0)
	b = binary.LittleEndian.AppendUint16(b, 0)
	b = binary.LittleEndian.AppendUint64(b, 0)
	return b
}

func gap11SQLServerDSN(addr string) string {
	return fmt.Sprintf("sqlserver://sa:Passw0rd@%s?encryption=disable", addr)
}

func gap11SQLServerDSNWithDB(addr, dbName string) string {
	return fmt.Sprintf("sqlserver://sa:Passw0rd@%s?database=%s&encryption=disable", addr, dbName)
}

// L185-190: sqlserver 登录报「库不存在」但 DSN 无 database= 参数，提取不出库名。
func TestOpenGorm_SQLServerNoDatabaseName(t *testing.T) {
	_, addr := startGap11TDSServer(t, []gap11TDSScriptConn{
		{loginErr: "Could not locate database 'game_x' in sysdatabases"},
	})

	_, err := openGorm("sqlserver", gap11SQLServerDSN(addr))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to extract database name")
}

// L197-199: sqlserver 库不存在 → 建库失败。
func TestOpenGorm_SQLServerCreateFails(t *testing.T) {
	_, addr := startGap11TDSServer(t, []gap11TDSScriptConn{
		{loginErr: "Could not locate database 'game_x' in sysdatabases"},
		{batchErr: true},
	})

	_, err := openGorm("sqlserver", gap11SQLServerDSNWithDB(addr, "game_x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create database")
}

// L201-204: sqlserver 建库成功后重连失败。
func TestOpenGorm_SQLServerReconnectAfterCreateFails(t *testing.T) {
	_, addr := startGap11TDSServer(t, []gap11TDSScriptConn{
		{loginErr: "Could not locate database 'game_x' in sysdatabases"},
		{},
		{loginErr: "Login failed for user 'sa'"},
	})

	_, err := openGorm("sqlserver", gap11SQLServerDSNWithDB(addr, "game_x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect after creating database")
}

// L209: sqlserver 库不存在 → 建库 → 重连成功。
func TestOpenGorm_SQLServerAutoCreateSuccess(t *testing.T) {
	_, addr := startGap11TDSServer(t, []gap11TDSScriptConn{
		{loginErr: "Could not locate database 'game_x' in sysdatabases"},
		{},
		{},
	})

	db, err := openGorm("sqlserver", gap11SQLServerDSNWithDB(addr, "game_x"))
	require.NoError(t, err)
	require.NotNil(t, db)
}

// L541-545: EnsureGameDatabase sqlserver 建库失败。
func TestEnsureGameDatabase_SQLServerCreateFails(t *testing.T) {
	_, addr := startGap11TDSServer(t, []gap11TDSScriptConn{
		{loginErr: "Could not locate database 'game_demo_prod' in sysdatabases"},
		{batchErr: true},
	})

	_, err := EnsureGameDatabase("sqlserver", gap11SQLServerDSNWithDB(addr, "croupier_meta"), "game_demo_prod")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create sqlserver database")
}

// L550: EnsureGameDatabase sqlserver 建库成功 → 返回 game DSN。
func TestEnsureGameDatabase_SQLServerSuccess(t *testing.T) {
	_, addr := startGap11TDSServer(t, []gap11TDSScriptConn{
		{loginErr: "Could not locate database 'game_demo_prod' in sysdatabases"},
		{},
	})

	gameDSN, err := EnsureGameDatabase("sqlserver", gap11SQLServerDSNWithDB(addr, "croupier_meta"), "game_demo_prod")
	require.NoError(t, err)
	assert.Contains(t, gameDSN, "game_demo_prod")
}

// L458-460: createSQLServerDatabase 的 DSN 解析失败（sqlserver DriverContext 急切解析）。
func TestCreateSQLServerDatabase_InvalidDialTimeoutDSN(t *testing.T) {
	err := createSQLServerDatabase("sqlserver://u:p@h?dial timeout=abc", "db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid dial timeout")
}

// ============================================================================
// seedBootstrapRoles / seedBootstrapPermissions 的 nil 条目分支
// ============================================================================

func newGap11SeedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

// L868-869: seedBootstrapRoles 跳过 nil 角色条目。
func TestSeedBootstrapRoles_SkipsNilRoleEntry(t *testing.T) {
	db := newGap11SeedDB(t)
	am := NewAdminManager(t.TempDir())
	am.roles["ghost"] = nil

	ctx := &ServiceContext{
		DB:           db,
		AdminManager: am,
		RoleModel:    model.NewRoleModel(db),
	}
	require.NoError(t, seedBootstrapRoles(ctx))
}

// L950-951: seedBootstrapPermissions 跳过 nil 权限条目。
func TestSeedBootstrapPermissions_SkipsNilPermEntry(t *testing.T) {
	db := newGap11SeedDB(t)
	am := NewAdminManager(t.TempDir())
	am.permissions["ghost"] = nil

	ctx := &ServiceContext{
		DB:              db,
		AdminManager:    am,
		PermissionModel: model.NewPermissionModel(db),
	}
	require.NoError(t, seedBootstrapPermissions(ctx))
}
