package pg

// 进程内假 PG 复制协议服务（pgproto3.Backend）：把 connector.go 的网络
// 路径从「诚实跳过（需真实 PG，integration tag）」转成单元可覆盖——协议
// 处理走真实 pglogrepl/pgconn 客户端栈，服务端应答按脚本注入。
//
// 覆盖面：Open 成功臂、Discover 全流程与查询错误臂、slotConfirmedLSN
// 槽存在/自动建槽/空结果、Poll 启动序列的 abort 弃连（起点坏值/
// START_REPLICATION 拒绝）、pump 接收循环（keepalive/状态上报/XLogData
// 解码/白名单过滤/错误响应与 hint/各解析错误臂/对端断开）、Snapshot 成功
// 与失败臂、Close 的 pump-less repl 收尾与连接关闭。
//
// 诚实跳过：pump 的状态上报发送失败臂（698-700）与接收超时 continue 臂
// （711，需 10s 挂钟截止）、Snapshot 读连接建连失败臂（297-298，与 repl
// 同 DSN 拨号，repl 先失败）、Snapshot 的 rand.Read 失败（259-260）与生成
// slot 名标识校验失败臂（263-264，时间戳+随机后缀恒合法）、三处
// json.Marshal(string) 失败臂（connector 352/417/500、decoder 317——
// string 序列化恒成功）、address.go 默认端口臂（pgconn.ParseConfig 恒设
// 5432）——均结构性不可达或故障注入代价大于收益，不凑。

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgproto3"
)

// ---- 假服务器 harness ----

// handleConn 每个接受连接的处理器（idx 为连接序号：0=普通连接，1=复制
// 连接，2+=快照读连接等）。
type handleConn func(idx int, be *pgproto3.Backend, conn net.Conn)

type fakePG struct {
	t    *testing.T
	ln   net.Listener
	done chan struct{}
}

func newFakePG(t *testing.T, handle handleConn) *fakePG {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fake pg listen: %v", err)
	}
	f := &fakePG{t: t, ln: ln, done: make(chan struct{})}
	go func() {
		defer close(f.done)
		idx := 0
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // 监听关闭：接受循环退出
			}
			be := pgproto3.NewBackend(conn, conn)
			i := idx
			idx++
			go func() {
				defer conn.Close()
				if !f.handshake(be) {
					return
				}
				if handle != nil {
					handle(i, be, conn)
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		select {
		case <-f.done:
		case <-time.After(3 * time.Second):
			t.Log("fake pg accept loop did not stop")
		}
	})
	return f
}

func (f *fakePG) addr() string { return f.ln.Addr().String() }

// stopAccept 关闭监听：后续新连接被拒绝（ensureReplConn 拨号失败臂）。
// 已有连接不受影响；接受循环随之退出。
func (f *fakePG) stopAccept() { _ = f.ln.Close() }

// handshake 完成启动握手：AuthenticationOk + BackendKeyData + ReadyForQuery
// （sslmode=disable 客户端不发 SSLRequest）。
func (f *fakePG) handshake(be *pgproto3.Backend) bool {
	if _, err := be.ReceiveStartupMessage(); err != nil {
		return false
	}
	be.Send(&pgproto3.AuthenticationOk{})
	be.Send(&pgproto3.BackendKeyData{ProcessID: 42, SecretKey: []byte{0, 0, 0, 7}})
	be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	return be.Flush() == nil
}

// sendQueryResult 行集应答（names 为空时省略 RowDescription——纯
// CommandComplete 形态）。
func sendQueryResult(be *pgproto3.Backend, names []string, rows [][]string, tag string) error {
	if len(names) > 0 {
		fields := make([]pgproto3.FieldDescription, len(names))
		for i, n := range names {
			fields[i].Name = []byte(n)
		}
		be.Send(&pgproto3.RowDescription{Fields: fields})
	}
	for _, r := range rows {
		vals := make([][]byte, len(r))
		for i, v := range r {
			if v == "\x00" { // 哨兵：NULL 列
				vals[i] = nil
			} else {
				vals[i] = []byte(v)
			}
		}
		be.Send(&pgproto3.DataRow{Values: vals})
	}
	be.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
	be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	return be.Flush()
}

// ccOnly 仅 CommandComplete + ReadyForQuery（无行集）。
func ccOnly(be *pgproto3.Backend, tag string) error {
	be.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
	be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	return be.Flush()
}

func sendErr(be *pgproto3.Backend, msg string) error {
	be.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: msg})
	be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	return be.Flush()
}

// serveQueries 普通连接查询循环：route 按 SQL 分派，返回非 nil 即结束。
func serveQueries(be *pgproto3.Backend, route func(sql string) error) {
	for {
		msg, err := be.Receive()
		if err != nil {
			return
		}
		switch m := msg.(type) {
		case *pgproto3.Query:
			if route(m.String) != nil {
				return
			}
		case *pgproto3.Terminate:
			return
		}
	}
}

// ---- 复制流帧构造 ----

func pgEpochMicros() uint64 {
	ms := time.Now().UnixMilli() - pgEpochUnixMs
	return uint64(ms) * 1000
}

// keepaliveFrame 主 keepalive 帧（'k' + walEnd + 时间 + reply 位）。
func keepaliveFrame(walEnd uint64, reply bool) []byte {
	b := make([]byte, 0, 18)
	b = append(b, pglogrepl.PrimaryKeepaliveMessageByteID)
	b = binary.BigEndian.AppendUint64(b, walEnd)
	b = binary.BigEndian.AppendUint64(b, pgEpochMicros())
	if reply {
		b = append(b, 1)
	} else {
		b = append(b, 0)
	}
	return b
}

// xlogFrame XLogData 帧（'w' + walStart + serverWALEnd + 时间 + 消息负载）。
func xlogFrame(walStart uint64, data []byte) []byte {
	b := make([]byte, 0, 24+len(data))
	b = append(b, pglogrepl.XLogDataByteID)
	b = binary.BigEndian.AppendUint64(b, walStart)
	b = binary.BigEndian.AppendUint64(b, walStart+1)
	b = binary.BigEndian.AppendUint64(b, pgEpochMicros())
	return append(b, data...)
}

func sendCopy(be *pgproto3.Backend, data []byte) error {
	be.Send(&pgproto3.CopyData{Data: data})
	return be.Flush()
}

// drainCopy 排空客户端 COPY 方向的帧，直到连接断开/超时；记录收到过
// standby 状态上报（'r'）。
func drainCopy(conn net.Conn, be *pgproto3.Backend, gotStatus *atomic.Bool, wait time.Duration) {
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	for {
		msg, err := be.Receive()
		if err != nil {
			return
		}
		if cd, ok := msg.(*pgproto3.CopyData); ok && len(cd.Data) > 0 && cd.Data[0] == pglogrepl.StandbyStatusUpdateByteID {
			if gotStatus != nil {
				gotStatus.Store(true)
			}
		}
	}
}

// ---- 普通连接路由构建 ----

// slotRoute 槽位点查询路由：confirmed 为空走创建路径，createRow 为 nil 时
// 创建返回空结果（empty-result 臂）。
func slotRoute(be *pgproto3.Backend, confirmed string, createRow *string) func(sql string) error {
	return func(sql string) error {
		switch {
		case strings.Contains(sql, "confirmed_flush_lsn"):
			if confirmed == "" {
				return ccOnly(be, "SELECT 0")
			}
			return sendQueryResult(be, []string{"confirmed_flush_lsn"},
				[][]string{{confirmed}}, "SELECT 1")
		case strings.Contains(sql, "pg_create_logical_replication_slot"):
			if createRow == nil {
				return ccOnly(be, "SELECT 0")
			}
			return sendQueryResult(be, []string{"lsn"}, [][]string{{*createRow}}, "SELECT 1")
		}
		return sendErr(be, "unexpected query: "+sql)
	}
}

// ---- 测试 ----

// Open + Poll + pump 全生命周期：槽存在续读 → keepalive（含 reply 触发
// 状态上报）→ 事务 XLogData 解码投递 → 白名单过滤 secret 表 → Close join。
func TestPollPumpLifecycleAgainstFake(t *testing.T) {
	var gotStatus atomic.Bool
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		if idx == 1 { // 复制连接：START_REPLICATION → CopyBoth → 帧流
			msg, err := be.Receive()
			if err != nil {
				return
			}
			q, ok := msg.(*pgproto3.Query)
			if !ok || !strings.Contains(q.String, "START_REPLICATION") {
				return
			}
			be.Send(&pgproto3.CopyBothResponse{})
			if be.Flush() != nil {
				return
			}
			time.Sleep(30 * time.Millisecond)
			_ = sendCopy(be, keepaliveFrame(0x1000, false))
			time.Sleep(20 * time.Millisecond)
			_ = sendCopy(be, keepaliveFrame(0x2000, true)) // reply 请求 → 立即状态上报
			time.Sleep(50 * time.Millisecond)
			// game.items 事务（OID itemsOID，2 列匹配下方 insert）
			_ = sendCopy(be, xlogFrame(0x3000, wireMessage('R', relPayload(itemsOID, "game", "items", 'd',
				relCol(FlagReplicaIdentity, "id", 23), relCol(0, "name", 25)))))
			_ = sendCopy(be, xlogFrame(0x3001, beginMsg(5)))
			_ = sendCopy(be, xlogFrame(0x3002, insertMsg(textDatum("1"), textDatum("sword"))))
			_ = sendCopy(be, xlogFrame(0x3003, commitMsg(0xABC)))
			// secret 表事务（OID 2）——白名单外，应被 pump 过滤
			_ = sendCopy(be, xlogFrame(0x4000, wireMessage('R', relPayload(2, "other", "secret", 'd',
				relCol(FlagReplicaIdentity, "id", 23)))))
			_ = sendCopy(be, xlogFrame(0x4001, beginMsg(6)))
			_ = sendCopy(be, xlogFrame(0x4002, wireMessage('I', append(append(u32v(2), 'N'), tupleData(textDatum("9"))...))))
			_ = sendCopy(be, xlogFrame(0x4003, commitMsg(0xDEF)))
			drainCopy(conn, be, &gotStatus, 5*time.Second)
			return
		}
		serveQueries(be, slotRoute(be, "0/16B0000", nil))
	}

	c, events := openPollConn(t, handle)
	defer c.Close(context.Background())

	ev := recvEvent(t, events)
	if ev.Table != "items" || ev.Database != "game" || ev.Operation != connector.OpInsert {
		t.Fatalf("event = %+v", ev)
	}
	if ev.SourceID != "cap-fake" || !ev.ColumnsMapped {
		t.Fatalf("event identity = %+v", ev)
	}
	if ev.Position.Kind != "lsn" || ev.Position.Value != (LSN(0xABC)).Decimal() {
		t.Fatalf("event position = %+v", ev.Position)
	}
	if !ev.Tx.End || ev.SchemaVersion != 1 {
		t.Fatalf("event tx/schema = %+v %d", ev.Tx, ev.SchemaVersion)
	}
	if got := string(ev.After["id"]); got != "1" {
		t.Fatalf("after.id = %s", got)
	}

	// 白名单外事件不得流出：通道在下一事件前应保持空（短暂观察后 Close）。
	time.Sleep(100 * time.Millisecond)
	select {
	case extra, ok := <-events:
		if ok {
			t.Fatalf("unexpected extra event: %+v", extra)
		}
	default:
	}

	// Close join：pump 收敛后 Close 返回，状态上报已被服务端收到。
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("close = %v", err)
	}
	if !gotStatus.Load() {
		t.Fatal("server never received standby status update")
	}
}

// 自动建槽：confirmed 查询空 → SQL 建槽取 consistent point → 记录日志。
func TestPollAutoCreatesSlot(t *testing.T) {
	createRow := "0/16B0000"
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		if idx == 1 {
			msg, err := be.Receive()
			if err != nil {
				return
			}
			if _, ok := msg.(*pgproto3.Query); !ok {
				return
			}
			be.Send(&pgproto3.CopyBothResponse{})
			_ = be.Flush()
			drainCopy(conn, be, nil, 3*time.Second)
			return
		}
		serveQueries(be, slotRoute(be, "", &createRow))
	}

	c, events := openPollConn(t, handle)
	// 建槽后流建立；无帧数据，pump 在 Close 时经 ctx 取消收敛。
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("close = %v", err)
	}
	waitClosed(t, events)
}

// 建槽空结果：create 查询返回零行 → 报错弃连（slotConfirmedLSN 672-674）。
func TestPollCreateSlotEmptyResult(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		if idx == 1 {
			// 不该到达复制连接：建槽失败在 slotConfirmedLSN 就收口
			drainCopy(conn, be, nil, 500*time.Millisecond)
			return
		}
		serveQueries(be, slotRoute(be, "", nil))
	}
	c := openConn(t, handle)
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	_, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}})
	if err == nil || !strings.Contains(err.Error(), "empty result") {
		t.Fatalf("poll empty create err = %v", err)
	}
	_ = c.Close(context.Background())
}

// 起点位点坏值：resolveStart 报错 → abort 弃连（复制连接关闭、缓存复位）。
func TestPollAbortOnBadStartPosition(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		if idx == 1 {
			drainCopy(conn, be, nil, 500*time.Millisecond)
			return
		}
		serveQueries(be, slotRoute(be, "0/16B0000", nil))
	}
	c := openConn(t, handle)
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	_, err := c.Poll(context.Background(), connector.PollRequest{
		Tables: []string{"game.items"},
		From:   connector.Position{Kind: "lsn", Value: "garbage"},
	})
	if err == nil || !strings.Contains(err.Error(), "parse checkpoint lsn") {
		t.Fatalf("poll bad start err = %v", err)
	}
	if c.repl != nil {
		t.Fatal("aborted poll must not keep replication connection")
	}
	_ = c.Close(context.Background())
}

// START_REPLICATION 拒绝：服务端回 ErrorResponse → abort 包装 + 弃连。
func TestPollStartReplicationRejected(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		if idx == 1 {
			msg, err := be.Receive()
			if err != nil {
				return
			}
			if _, ok := msg.(*pgproto3.Query); !ok {
				return
			}
			_ = sendErr(be, `slot "slot" is already active`)
			drainCopy(conn, be, nil, 500*time.Millisecond)
			return
		}
		serveQueries(be, slotRoute(be, "0/16B0000", nil))
	}
	c := openConn(t, handle)
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	_, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}})
	if err == nil || !strings.Contains(err.Error(), "start replication slot slot") {
		t.Fatalf("poll start rejected err = %v", err)
	}
	if c.repl != nil {
		t.Fatal("rejected start must drop replication connection")
	}
	_ = c.Close(context.Background())
}

// pump 错误臂逐项：服务端注入一条消息 → pump 日志报错并收敛（通道关闭）。
func TestPumpErrorArms(t *testing.T) {
	// inject 建 Poll 后注入单条服务端消息（repl 连接）。
	inject := func(emit func(be *pgproto3.Backend) error, gotStatus *atomic.Bool) (*PostgresConnector, <-chan connector.ChangeEvent) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			if idx == 1 {
				if _, err := be.Receive(); err != nil {
					return
				}
				be.Send(&pgproto3.CopyBothResponse{})
				if be.Flush() != nil {
					return
				}
				time.Sleep(20 * time.Millisecond)
				_ = emit(be)
				drainCopy(conn, be, gotStatus, 3*time.Second)
				return
			}
			serveQueries(be, slotRoute(be, "0/16B0000", nil))
		}
		return openPollConn(t, handle)
	}

	t.Run("错误响应带发布提示", func(t *testing.T) {
		c, ev := inject(func(be *pgproto3.Backend) error {
			be.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: `publication "pub" does not exist`})
			return be.Flush()
		}, nil)
		waitClosed(t, ev)
		_ = c.Close(context.Background())
	})

	t.Run("错误响应无提示", func(t *testing.T) {
		c, ev := inject(func(be *pgproto3.Backend) error {
			be.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: `slot "slot" is already active`})
			return be.Flush()
		}, nil)
		waitClosed(t, ev)
		_ = c.Close(context.Background())
	})

	t.Run("keepalive 解析失败", func(t *testing.T) {
		c, ev := inject(func(be *pgproto3.Backend) error {
			return sendCopy(be, []byte{pglogrepl.PrimaryKeepaliveMessageByteID, 0x01, 0x02})
		}, nil)
		waitClosed(t, ev)
		_ = c.Close(context.Background())
	})

	t.Run("xlog 解析失败", func(t *testing.T) {
		c, ev := inject(func(be *pgproto3.Backend) error {
			return sendCopy(be, []byte{pglogrepl.XLogDataByteID, 0x01})
		}, nil)
		waitClosed(t, ev)
		_ = c.Close(context.Background())
	})

	t.Run("解码失败", func(t *testing.T) {
		c, ev := inject(func(be *pgproto3.Backend) error {
			return sendCopy(be, xlogFrame(0x3000, []byte{'Z', 0x00})) // 未知 pgoutput 类型
		}, nil)
		waitClosed(t, ev)
		_ = c.Close(context.Background())
	})

	t.Run("未知 copydata 类型继续存活", func(t *testing.T) {
		var gotStatus atomic.Bool
		c, ev := inject(func(be *pgproto3.Backend) error {
			if err := sendCopy(be, []byte{'z', 0x00}); err != nil {
				return err
			}
			return sendCopy(be, keepaliveFrame(0x1000, true)) // reply → 状态上报证明存活
		}, &gotStatus)
		time.Sleep(300 * time.Millisecond)
		if !gotStatus.Load() {
			t.Fatal("pump did not survive unknown copydata type")
		}
		_ = c.Close(context.Background())
		waitClosed(t, ev)
	})

	t.Run("对端断开", func(t *testing.T) {
		c, ev := inject(func(be *pgproto3.Backend) error {
			return nil // 无注入：handler 直接返回 → 连接关闭 → pump EOF
		}, nil)
		waitClosed(t, ev)
		_ = c.Close(context.Background())
	})
}

// Discover 全流程：行集解析（含过滤/主键/malformed 行跳过）+ 无过滤全表 +
// 列查询与主键查询错误臂。
func TestDiscoverAgainstFake(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		serveQueries(be, func(sql string) error {
			switch {
			case strings.Contains(sql, "table_constraints"):
				// pk 行：game.items id；malformed 2 列行跳过（185-187）
				return sendQueryResult(be, []string{"table_schema", "table_name", "column_name"},
					[][]string{{"game", "items", "id"}, {"game", "items"}}, "SELECT 2")
			case strings.Contains(sql, "columns"):
				// 列行：game.items 2 列 + other.secret 1 列 + malformed 3 列行跳过（196-198）
				return sendQueryResult(be, []string{"table_schema", "table_name", "column_name", "data_type"},
					[][]string{
						{"game", "items", "id", "integer"},
						{"game", "items", "name", "text"},
						{"other", "secret", "code", "integer"},
						{"game", "items", "only3"},
					}, "SELECT 4")
			}
			return sendErr(be, "unexpected: "+sql)
		})
	}
	c := openConn(t, handle)

	res, err := c.Discover(context.Background(), connector.DiscoveryRequest{Tables: []string{"game.items"}})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(res.Tables) != 1 || res.Tables[0].Name != "game.items" {
		t.Fatalf("tables = %+v", res.Tables)
	}
	tab := res.Tables[0]
	if len(tab.Columns) != 2 || tab.Columns[0].Name != "id" || tab.Columns[1].Name != "name" {
		t.Fatalf("columns = %+v", tab.Columns)
	}
	if len(tab.PrimaryKey) != 1 || tab.PrimaryKey[0] != "id" {
		t.Fatalf("pk = %+v", tab.PrimaryKey)
	}
	if cap := PostgresCapabilities(); cap != res.Capabilities {
		t.Fatalf("capabilities = %+v, want %+v", res.Capabilities, cap)
	}

	// 无过滤：两表都在，顺序按首见（game.items 先）
	res2, err := c.Discover(context.Background(), connector.DiscoveryRequest{})
	if err != nil {
		t.Fatalf("discover all: %v", err)
	}
	if len(res2.Tables) != 2 || res2.Tables[0].Name != "game.items" || res2.Tables[1].Name != "other.secret" {
		t.Fatalf("all tables = %+v", res2.Tables)
	}
	_ = c.Close(context.Background())
}

func TestDiscoverQueryErrorArms(t *testing.T) {
	t.Run("列查询失败", func(t *testing.T) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			serveQueries(be, func(sql string) error { return sendErr(be, "boom") })
		}
		c := openConn(t, handle)
		_, err := c.Discover(context.Background(), connector.DiscoveryRequest{})
		if err == nil || !strings.Contains(err.Error(), "discover columns") {
			t.Fatalf("cols err = %v", err)
		}
		_ = c.Close(context.Background())
	})

	t.Run("主键查询失败", func(t *testing.T) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			serveQueries(be, func(sql string) error {
				if strings.Contains(sql, "table_constraints") {
					return sendErr(be, "boom")
				}
				return sendQueryResult(be, []string{"table_schema", "table_name", "column_name", "data_type"},
					[][]string{{"game", "items", "id", "integer"}}, "SELECT 1")
			})
		}
		c := openConn(t, handle)
		_, err := c.Discover(context.Background(), connector.DiscoveryRequest{})
		if err == nil || !strings.Contains(err.Error(), "discover primary keys") {
			t.Fatalf("pk err = %v", err)
		}
		_ = c.Close(context.Background())
	})
}

// Snapshot 成功路径：临时槽 CREATE（一致点/快照名）→ 快照读连接元数据 +
// 行集 → 事件通道（位点统一一致点、末行 End、列名映射）。
func TestSnapshotAgainstFake(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		switch idx {
		case 1: // 复制连接：CREATE_REPLICATION_SLOT + DROP（Warn 臂）
			serveQueries(be, func(sql string) error {
				switch {
				case strings.Contains(sql, "CREATE_REPLICATION_SLOT"):
					return sendQueryResult(be, []string{"slot_name", "consistent_point", "snapshot_name", "output_plugin"},
						[][]string{{"snap", "0/16B0000", "snap-1", "pgoutput"}}, "SELECT 1")
				case strings.Contains(sql, "DROP_REPLICATION_SLOT"):
					return sendErr(be, "drop fail") // 覆盖 Warn 臂（282-283）
				}
				return sendErr(be, "unexpected repl: "+sql)
			})
		case 2: // 快照读连接
			serveQueries(be, snapshotReaderRoute(be))
		}
	}
	c := openConn(t, handle)
	events, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	ev1 := recvEvent(t, events)
	ev2 := recvEvent(t, events)
	waitClosed(t, events)

	wantPos, _ := ParseLSN("0/16B0000")
	for i, ev := range []connector.ChangeEvent{ev1, ev2} {
		if ev.Operation != connector.OpInsert || ev.Table != "items" || ev.Database != "game" {
			t.Fatalf("event %d = %+v", i, ev)
		}
		if ev.Position.Value != wantPos.Decimal() {
			t.Fatalf("event %d position = %+v, want %s", i, ev.Position, wantPos.Decimal())
		}
		if ev.Tx.Seq != int64(i+1) || ev.Tx.End != (i == 1) {
			t.Fatalf("event %d tx = %+v", i, ev.Tx)
		}
		if err := ev.Validate(); err != nil {
			t.Fatalf("event %d invalid: %v", i, err)
		}
	}
	if got := string(ev1.After["id"]); got != "1" {
		t.Fatalf("ev1 id = %s", got)
	}
	if got := string(ev2.After["name"]); got != `"shield"` {
		t.Fatalf("ev2 name = %s", got)
	}
	if got := string(ev2.After["id"]); got != "null" {
		t.Fatalf("ev2 id = %s, want explicit null", got)
	}
	_ = c.Close(context.Background())
}

// snapshotReaderRoute 快照读连接的查询分派（元数据 + 行集 + 快照事务）。
func snapshotReaderRoute(be *pgproto3.Backend) func(sql string) error {
	return func(sql string) error {
		switch {
		case strings.HasPrefix(sql, "BEGIN"):
			return ccOnly(be, "BEGIN")
		case strings.Contains(sql, "SET TRANSACTION SNAPSHOT"):
			return ccOnly(be, "SET")
		case strings.Contains(sql, "information_schema.columns"):
			return sendQueryResult(be, []string{"column_name", "data_type"},
				[][]string{{"id", "integer"}, {"name", "text"}}, "SELECT 2")
		case strings.Contains(sql, "table_constraints"):
			return sendQueryResult(be, []string{"column_name"}, [][]string{{"id"}}, "SELECT 1")
		case strings.HasPrefix(sql, "SELECT"):
			return sendQueryResult(be, []string{"id", "name"},
				[][]string{{"1", "sword"}, {"\x00", "shield"}}, "SELECT 2")
		case sql == "ROLLBACK":
			return ccOnly(be, "ROLLBACK")
		}
		return sendErr(be, "unexpected snapshot: "+sql)
	}
}

// Snapshot 失败臂逐项（repl/读连接路由各自注入错误）。
func TestSnapshotErrorArms(t *testing.T) {
	t.Run("建槽失败", func(t *testing.T) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			if idx == 1 {
				serveQueries(be, func(sql string) error { return sendErr(be, "slot conflict") })
			}
		}
		c := openConn(t, handle)
		_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
		if err == nil || !strings.Contains(err.Error(), "create snapshot slot") {
			t.Fatalf("create err = %v", err)
		}
		_ = c.Close(context.Background())
	})

	t.Run("快照名缺失", func(t *testing.T) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			if idx == 1 {
				serveQueries(be, func(sql string) error {
					if strings.Contains(sql, "CREATE_REPLICATION_SLOT") {
						return sendQueryResult(be, []string{"slot_name", "consistent_point", "snapshot_name", "output_plugin"},
							[][]string{{"snap", "0/16B0000", "", "pgoutput"}}, "SELECT 1")
					}
					return sendErr(be, "unexpected: "+sql)
				})
			}
		}
		c := openConn(t, handle)
		_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
		if err == nil || !strings.Contains(err.Error(), "did not export snapshot") {
			t.Fatalf("snapshot-name err = %v", err)
		}
		_ = c.Close(context.Background())
	})

	t.Run("一致点坏值", func(t *testing.T) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			if idx == 1 {
				serveQueries(be, func(sql string) error {
					if strings.Contains(sql, "CREATE_REPLICATION_SLOT") {
						return sendQueryResult(be, []string{"slot_name", "consistent_point", "snapshot_name", "output_plugin"},
							[][]string{{"snap", "garbage", "snap-1", "pgoutput"}}, "SELECT 1")
					}
					return sendErr(be, "unexpected: "+sql)
				})
			}
		}
		c := openConn(t, handle)
		_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
		if err == nil || !strings.Contains(err.Error(), "parse snapshot consistent point") {
			t.Fatalf("consistent-point err = %v", err)
		}
		_ = c.Close(context.Background())
	})

	t.Run("快照事务 BEGIN 失败", func(t *testing.T) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			switch idx {
			case 1:
				serveQueries(be, createSlotRoute(be))
			case 2:
				serveQueries(be, func(sql string) error { return sendErr(be, "begin fail") })
			}
		}
		c := openConn(t, handle)
		_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
		if err == nil || !strings.Contains(err.Error(), "begin snapshot") {
			t.Fatalf("begin err = %v", err)
		}
		_ = c.Close(context.Background())
	})

	t.Run("快照导入失败", func(t *testing.T) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			switch idx {
			case 1:
				serveQueries(be, createSlotRoute(be))
			case 2:
				serveQueries(be, withSnapshotBegin(be, func(sql string) (bool, error) {
					if strings.Contains(sql, "SET TRANSACTION SNAPSHOT") {
						return true, sendErr(be, "import fail")
					}
					return false, nil
				}))
			}
		}
		c := openConn(t, handle)
		_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
		if err == nil || !strings.Contains(err.Error(), "import snapshot") {
			t.Fatalf("import err = %v", err)
		}
		_ = c.Close(context.Background())
	})

	t.Run("元数据列查询失败", func(t *testing.T) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			switch idx {
			case 1:
				serveQueries(be, createSlotRoute(be))
			case 2:
				serveQueries(be, withSnapshotBegin(be, func(sql string) (bool, error) {
					switch {
					case strings.Contains(sql, "SET TRANSACTION SNAPSHOT"):
						return true, ccOnly(be, "SET")
					case strings.Contains(sql, "information_schema.columns"):
						return true, sendErr(be, "cols fail")
					}
					return false, nil
				}))
			}
		}
		c := openConn(t, handle)
		_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
		if err == nil || !strings.Contains(err.Error(), "query table game.items columns") {
			t.Fatalf("metadata err = %v", err)
		}
		_ = c.Close(context.Background())
	})

	t.Run("行查询失败", func(t *testing.T) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			switch idx {
			case 1:
				serveQueries(be, createSlotRoute(be))
			case 2:
				serveQueries(be, withSnapshotBegin(be, func(sql string) (bool, error) {
					switch {
					case strings.Contains(sql, "SET TRANSACTION SNAPSHOT"):
						return true, ccOnly(be, "SET")
					case strings.Contains(sql, "information_schema.columns"):
						return true, sendQueryResult(be, []string{"column_name", "data_type"},
							[][]string{{"id", "integer"}}, "SELECT 1")
					case strings.Contains(sql, "table_constraints"):
						return true, sendQueryResult(be, []string{"column_name"}, [][]string{{"id"}}, "SELECT 1")
					case strings.HasPrefix(sql, "SELECT") && !strings.Contains(sql, "information_schema"):
						return true, sendErr(be, "select fail")
					}
					return false, nil
				}))
			}
		}
		c := openConn(t, handle)
		_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
		if err == nil || !strings.Contains(err.Error(), "snapshot table game.items") {
			t.Fatalf("select err = %v", err)
		}
		_ = c.Close(context.Background())
	})

	t.Run("行像列数不符", func(t *testing.T) {
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			switch idx {
			case 1:
				serveQueries(be, createSlotRoute(be))
			case 2:
				serveQueries(be, withSnapshotBegin(be, func(sql string) (bool, error) {
					switch {
					case strings.Contains(sql, "SET TRANSACTION SNAPSHOT"):
						return true, ccOnly(be, "SET")
					case strings.Contains(sql, "information_schema.columns"):
						return true, sendQueryResult(be, []string{"column_name", "data_type"},
							[][]string{{"id", "integer"}, {"name", "text"}}, "SELECT 2")
					case strings.Contains(sql, "table_constraints"):
						return true, sendQueryResult(be, []string{"column_name"}, [][]string{{"id"}}, "SELECT 1")
					case strings.HasPrefix(sql, "SELECT"):
						// 元数据 2 列但行集只 1 值 → rowImage 长度不符
						return true, sendQueryResult(be, []string{"id"}, [][]string{{"1"}}, "SELECT 1")
					}
					return false, nil
				}))
			}
		}
		c := openConn(t, handle)
		_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
		if err == nil || !strings.Contains(err.Error(), "row has 1 values") {
			t.Fatalf("rowimage err = %v", err)
		}
		_ = c.Close(context.Background())
	})
}

// withSnapshotBegin 读连接路由包装：BEGIN/ROLLBACK 正常应答，其余交 inner
// （handled=false 回错误应答）。
func withSnapshotBegin(be *pgproto3.Backend, inner func(sql string) (bool, error)) func(sql string) error {
	return func(sql string) error {
		if strings.HasPrefix(sql, "BEGIN") || sql == "ROLLBACK" {
			return ccOnly(be, "OK")
		}
		if handled, err := inner(sql); handled {
			return err
		}
		return sendErr(be, "unexpected: "+sql)
	}
}

// createSlotRoute CREATE_REPLICATION_SLOT 成功应答（DROP 也成功）。
func createSlotRoute(be *pgproto3.Backend) func(sql string) error {
	return func(sql string) error {
		switch {
		case strings.Contains(sql, "CREATE_REPLICATION_SLOT"):
			return sendQueryResult(be, []string{"slot_name", "consistent_point", "snapshot_name", "output_plugin"},
				[][]string{{"snap", "0/16B0000", "snap-1", "pgoutput"}}, "SELECT 1")
		case strings.Contains(sql, "DROP_REPLICATION_SLOT"):
			return ccOnly(be, "DROP REPLICATION SLOT")
		}
		return sendErr(be, "unexpected: "+sql)
	}
}

// Snapshot 的 ctx 取消发送臂与事件自检失败臂（缓冲满后 ctx.Done 收敛）。
func TestSnapshotSendArms(t *testing.T) {
	t.Run("ctx 取消收敛", func(t *testing.T) {
		// 80 行 > 64 缓冲：不读时第 65 行阻塞在 out<-,ctx.Done 二选一 → 取消收敛。
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			switch idx {
			case 1:
				serveQueries(be, createSlotRoute(be))
			case 2:
				serveQueries(be, withSnapshotBegin(be, func(sql string) (bool, error) {
					switch {
					case strings.Contains(sql, "SET TRANSACTION SNAPSHOT"):
						return true, ccOnly(be, "SET")
					case strings.Contains(sql, "information_schema.columns"):
						return true, sendQueryResult(be, []string{"column_name", "data_type"},
							[][]string{{"id", "integer"}}, "SELECT 1")
					case strings.Contains(sql, "table_constraints"):
						return true, sendQueryResult(be, []string{"column_name"}, [][]string{{"id"}}, "SELECT 1")
					case strings.HasPrefix(sql, "SELECT"):
						rows := make([][]string, 0, 80)
						for i := 0; i < 80; i++ {
							rows = append(rows, []string{"1"})
						}
						return true, sendQueryResult(be, []string{"id"}, rows, "SELECT 80")
					}
					return false, nil
				}))
			}
		}
		c := openConn(t, handle)
		ctx, cancel := context.WithCancel(context.Background())
		events, err := c.Snapshot(ctx, connector.SnapshotRequest{Tables: []string{"game.items"}})
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		time.Sleep(150 * time.Millisecond) // 让 goroutine 填满缓冲后阻塞在发送
		cancel()
		// 收敛形态：排空缓冲内已排队事件后通道关闭（≤80 行）。
		n := 0
		for {
			select {
			case _, ok := <-events:
				if !ok {
					if n > 80 {
						t.Fatalf("drained %d events, exceeds row count", n)
					}
					return
				}
				n++
			case <-time.After(5 * time.Second):
				t.Fatalf("event stream did not close after cancel (drained %d)", n)
			}
		}
	})

	t.Run("事件自检失败", func(t *testing.T) {
		// 空 SourceID 的配置 → 快照事件过不了 Validate → goroutine 记录日志并返回。
		handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
			switch idx {
			case 1:
				serveQueries(be, createSlotRoute(be))
			case 2:
				serveQueries(be, snapshotReaderRoute(be))
			}
		}
		f := newFakePG(t, handle)
		t.Setenv(pgExtraDSNEnv, "postgres://u:p@"+f.addr()+"/croupier?sslmode=disable")
		c := NewPostgresConnector(connector.SourceConfig{}, nil)
		if err := c.Open(context.Background(), connector.SourceConfig{DSNEnv: pgExtraDSNEnv}); err != nil {
			t.Fatalf("open: %v", err)
		}
		events, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		waitClosed(t, events) // 无事件流出（Validate 失败即断流）
		_ = c.Close(context.Background())
	})
}

// Close 的 pump-less repl 收尾（864-869）与连接关闭（871-875）。
func TestCloseWithOpenConnAndRepl(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		serveQueries(be, func(sql string) error { return ccOnly(be, "SELECT 0") })
	}
	c := openConn(t, handle)
	repl1, err := c.ensureReplConn()
	if err != nil {
		t.Fatalf("ensure repl: %v", err)
	}
	repl2, err := c.ensureReplConn() // 缓存命中：同一连接
	if err != nil {
		t.Fatalf("ensure repl again: %v", err)
	}
	if repl1 != repl2 {
		t.Fatal("expected cached repl connection")
	}
	if c.repl == nil || c.conn == nil {
		t.Fatal("expected open connections")
	}
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("close = %v", err)
	}
	if c.repl != nil || c.conn != nil {
		t.Fatalf("close leftovers: repl=%v conn=%v", c.repl, c.conn)
	}
}

// recvEvent 从事件通道取一条（5s 超时）。
func recvEvent(t *testing.T, events <-chan connector.ChangeEvent) connector.ChangeEvent {
	t.Helper()
	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("event stream closed before deliver")
		}
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event")
		return connector.ChangeEvent{}
	}
}

// waitClosed 等事件通道关闭（5s 超时）。
func waitClosed(t *testing.T, events <-chan connector.ChangeEvent) {
	t.Helper()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("event stream delivered after close expectation")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("event stream did not close")
	}
}

// openConn 建假服务器并 Open（不 Poll）。
func openConn(t *testing.T, handle handleConn) *PostgresConnector {
	t.Helper()
	f := newFakePG(t, handle)
	t.Setenv(pgExtraDSNEnv, "postgres://u:p@"+f.addr()+"/croupier?sslmode=disable")
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	if err := c.Open(context.Background(), connector.SourceConfig{
		DSNEnv: pgExtraDSNEnv, SourceID: "cap-fake",
	}); err != nil {
		t.Fatalf("open: %v", err)
	}
	return c
}

// openPollConn 建假服务器 + Open + Poll（槽/发布名用默认 slot/pub）。
func openPollConn(t *testing.T, handle handleConn) (*PostgresConnector, <-chan connector.ChangeEvent) {
	t.Helper()
	c := openConn(t, handle)
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	events, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	return c, events
}

// Snapshot 的 ensureReplConn 拨号失败臂（263-264/631-632）：Open 后关闭
// 监听，快照的复制连接拨号被拒 → 错误返回。
func TestSnapshotReplConnDialFail(t *testing.T) {
	f := newFakePG(t, func(idx int, be *pgproto3.Backend, conn net.Conn) {
		serveQueries(be, func(sql string) error { return ccOnly(be, "SELECT 0") })
	})
	t.Setenv(pgExtraDSNEnv, "postgres://u:p@"+f.addr()+"/croupier?sslmode=disable")
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	if err := c.Open(context.Background(), connector.SourceConfig{
		DSNEnv: pgExtraDSNEnv, SourceID: "cap-fake",
	}); err != nil {
		t.Fatalf("open: %v", err)
	}
	f.stopAccept()
	_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
	if err == nil || !strings.Contains(err.Error(), "connect replication connection") {
		t.Fatalf("repl dial err = %v", err)
	}
	_ = c.Close(context.Background())
}

// snapshotTable 的 ROLLBACK 失败臂（297-298）：行集读取成功后回滚失败。
func TestSnapshotRollbackFail(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		switch idx {
		case 1:
			serveQueries(be, createSlotRoute(be))
		case 2:
			serveQueries(be, func(sql string) error {
				switch {
				case strings.HasPrefix(sql, "BEGIN"), strings.Contains(sql, "SET TRANSACTION SNAPSHOT"):
					return ccOnly(be, "OK")
				case strings.Contains(sql, "information_schema.columns"):
					return sendQueryResult(be, []string{"column_name", "data_type"},
						[][]string{{"id", "integer"}}, "SELECT 1")
				case strings.Contains(sql, "table_constraints"):
					return sendQueryResult(be, []string{"column_name"}, [][]string{{"id"}}, "SELECT 1")
				case strings.HasPrefix(sql, "SELECT"):
					return sendQueryResult(be, []string{"id"}, [][]string{{"1"}}, "SELECT 1")
				case sql == "ROLLBACK":
					return sendErr(be, "rollback fail")
				}
				return sendErr(be, "unexpected: "+sql)
			})
		}
	}
	c := openConn(t, handle)
	_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
	if err == nil || !strings.Contains(err.Error(), "snapshot table game.items rollback") {
		t.Fatalf("rollback err = %v", err)
	}
	_ = c.Close(context.Background())
}

// tableMetadata 各错误臂：键查询失败、列/键行数不符、列集为空。
func TestSnapshotMetadataArms(t *testing.T) {
	cases := []struct {
		name, want string
		colRows    [][]string
		keyRows    [][]string
		colsErr    bool
		keysErr    bool
	}{
		{name: "主键查询失败", want: "query table game.items keys", keysErr: true},
		{name: "列行数不符", want: "returned invalid column row",
			colRows: [][]string{{"id", "integer", "x"}}, keyRows: [][]string{{"id"}}},
		{name: "键行数不符", want: "returned invalid key row",
			colRows: [][]string{{"id", "integer"}}, keyRows: [][]string{{"id", "x"}}},
		{name: "列集为空", want: "not found",
			colRows: [][]string{}, keyRows: [][]string{{"id"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
				switch idx {
				case 1:
					serveQueries(be, createSlotRoute(be))
				case 2:
					serveQueries(be, withSnapshotBegin(be, func(sql string) (bool, error) {
						switch {
						case strings.Contains(sql, "SET TRANSACTION SNAPSHOT"):
							return true, ccOnly(be, "SET")
						case strings.Contains(sql, "information_schema.columns"):
							if tc.colsErr {
								return true, sendErr(be, "cols fail")
							}
							return true, sendQueryResult(be, []string{"column_name", "data_type"}, tc.colRows, "SELECT")
						case strings.Contains(sql, "table_constraints"):
							if tc.keysErr {
								return true, sendErr(be, "keys fail")
							}
							return true, sendQueryResult(be, []string{"column_name"}, tc.keyRows, "SELECT")
						}
						return false, nil
					}))
				}
			}
			c := openConn(t, handle)
			_, err := c.Snapshot(context.Background(), connector.SnapshotRequest{Tables: []string{"game.items"}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s err = %v, want %q", tc.name, err, tc.want)
			}
			_ = c.Close(context.Background())
		})
	}
}

// Poll 前置守卫：连接未打开（521-522）。
func TestPollNotOpen(t *testing.T) {
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	_, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}})
	if err == nil || !strings.Contains(err.Error(), "connector not open") {
		t.Fatalf("poll not-open err = %v", err)
	}
}

// replicationOptions 校验臂：publication 缺失（651-652）、槽/发布名非法
// （662-663/670-671）。
func TestPollReplicationOptionsArms(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		serveQueries(be, slotRoute(be, "0/16B0000", nil))
	}
	c := openConn(t, handle)

	// publication 缺失
	c.cfg.Options = map[string]string{"slotName": "slot"}
	if _, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}}); err == nil ||
		!strings.Contains(err.Error(), "options.publication required") {
		t.Fatalf("pub missing err = %v", err)
	}
	// 槽名非法（大写）
	c.cfg.Options = map[string]string{"slotName": "BAD", "publication": "pub"}
	if _, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}}); err == nil ||
		!strings.Contains(err.Error(), "slot name") {
		t.Fatalf("slot ident err = %v", err)
	}
	// 发布名非法
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "bad name!"}
	if _, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}}); err == nil ||
		!strings.Contains(err.Error(), "publication name") {
		t.Fatalf("pub ident err = %v", err)
	}
	_ = c.Close(context.Background())
}

// slotConfirmedLSN 的槽查询失败臂（698-700）：槽查询走已打开的业务连接
// （conn0 = 连接序号 0），复制连接（序号 1）不会被用到。
func TestPollSlotQueryError(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		if idx == 0 {
			serveQueries(be, func(sql string) error { return sendErr(be, "slot gone") })
			return
		}
		serveQueries(be, func(sql string) error { return ccOnly(be, "SELECT 0") })
	}
	c := openConn(t, handle)
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	_, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}})
	if err == nil || !strings.Contains(err.Error(), "pg: query slot slot") {
		t.Fatalf("slot query err = %v", err)
	}
	_ = c.Close(context.Background())
}

// Poll 的 ctx 取消收敛臂（711）：接收中取消 → ReceiveMessage 报 ctx 取消
// → pump 正常收敛返回（不计错误）并关闭事件通道。
func TestPollCtxCancel(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		if idx == 1 {
			if _, err := be.Receive(); err != nil {
				return
			}
			be.Send(&pgproto3.CopyBothResponse{})
			_ = be.Flush()
			drainCopy(conn, be, nil, 3*time.Second)
			return
		}
		serveQueries(be, slotRoute(be, "0/16B0000", nil))
	}
	c := openConn(t, handle)
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	ctx, cancel := context.WithCancel(context.Background())
	events, err := c.Poll(ctx, connector.PollRequest{Tables: []string{"game.items"}})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()
	waitClosed(t, events)
	_ = c.Close(context.Background())
}

// Poll 的 pump 非 CopyData 消息继续臂（728）：CopyBothResponse 后发一个
// 非 CopyData 后端消息，pump 跳过继续，随后 keepalive 照常处理。
func TestPollPumpNonCopyDataContinue(t *testing.T) {
	var gotStatus atomic.Bool
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		if idx == 1 {
			if _, err := be.Receive(); err != nil {
				return
			}
			be.Send(&pgproto3.CopyBothResponse{})
			_ = be.Flush()
			time.Sleep(20 * time.Millisecond)
			be.Send(&pgproto3.CopyDone{}) // 非 CopyData 后端消息 → continue
			_ = be.Flush()
			time.Sleep(20 * time.Millisecond)
			_ = sendCopy(be, keepaliveFrame(0x1000, true))
			drainCopy(conn, be, &gotStatus, 3*time.Second)
			return
		}
		serveQueries(be, slotRoute(be, "0/16B0000", nil))
	}
	c, ev := openPollConn(t, handle)
	time.Sleep(300 * time.Millisecond)
	if !gotStatus.Load() {
		t.Fatal("pump did not survive non-CopyData message")
	}
	_ = c.Close(context.Background())
	waitClosed(t, ev)
}

// delete 的 appendRow 失败臂（decoder 171-172）：合法 delete 消息但关系
// 未注册 → 包装错误。
func TestDecoderDeleteUnknownRelation(t *testing.T) {
	d := NewDecoder("s")
	if _, err := d.Feed(beginMsg(1), "100"); err != nil {
		t.Fatalf("begin: %v", err)
	}
	payload := append(u32v(999), 'K')
	payload = append(payload, tupleData(textDatum("1"))...)
	_, err := d.Feed(wireMessage('D', payload), "100")
	if err == nil || !strings.Contains(err.Error(), "pg: delete:") ||
		!strings.Contains(err.Error(), "relation 999") {
		t.Fatalf("delete unknown relation err = %v", err)
	}
}

// Poll 的 ensureReplConn 拨号失败臂（521-522）：Open 后关闭监听，复制连接
// 被拒 → Poll 错误返回。
func TestPollReplConnDialFail(t *testing.T) {
	f := newFakePG(t, func(idx int, be *pgproto3.Backend, conn net.Conn) {
		serveQueries(be, func(sql string) error { return ccOnly(be, "SELECT 0") })
	})
	t.Setenv(pgExtraDSNEnv, "postgres://u:p@"+f.addr()+"/croupier?sslmode=disable")
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	if err := c.Open(context.Background(), connector.SourceConfig{
		DSNEnv: pgExtraDSNEnv, SourceID: "cap-fake",
	}); err != nil {
		t.Fatalf("open: %v", err)
	}
	f.stopAccept()
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	_, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}})
	if err == nil || !strings.Contains(err.Error(), "connect replication connection") {
		t.Fatalf("poll repl dial err = %v", err)
	}
	_ = c.Close(context.Background())
}

// ensureReplConn 的 RuntimeParams 复制循环体（608-609）：DSN 带
// application_name（进 RuntimeParams）→ 克隆时逐项复制。
func TestEnsureReplConnCopiesRuntimeParams(t *testing.T) {
	f := newFakePG(t, func(idx int, be *pgproto3.Backend, conn net.Conn) {
		serveQueries(be, func(sql string) error { return ccOnly(be, "SELECT 0") })
	})
	t.Setenv(pgExtraDSNEnv, "postgres://u:p@"+f.addr()+"/croupier?sslmode=disable&application_name=pgtest")
	c := NewPostgresConnector(connector.SourceConfig{}, nil)
	if err := c.Open(context.Background(), connector.SourceConfig{
		DSNEnv: pgExtraDSNEnv, SourceID: "cap-fake",
	}); err != nil {
		t.Fatalf("open: %v", err)
	}
	repl, err := c.ensureReplConn()
	if err != nil {
		t.Fatalf("ensure repl: %v", err)
	}
	if repl == nil {
		t.Fatal("nil repl conn")
	}
	_ = c.Close(context.Background())
}

// resolveStart 的 From 位点解析失败臂（651-652）：garbage lsn From。
func TestPollResolveStartFromGarbage(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		serveQueries(be, slotRoute(be, "0/16B0000", nil))
	}
	c := openConn(t, handle)
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	_, err := c.Poll(context.Background(), connector.PollRequest{
		Tables: []string{"game.items"},
		From:   connector.Position{Kind: "lsn", Value: "garbage"},
	})
	if err == nil || !strings.Contains(err.Error(), "parse checkpoint lsn") {
		t.Fatalf("resolve garbage err = %v", err)
	}
	_ = c.Close(context.Background())
}

// slotConfirmedLSN 的建槽失败臂（670-671）：槽查询 0 行 → 建槽报错。
func TestPollCreateSlotFail(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		serveQueries(be, func(sql string) error {
			if strings.Contains(sql, "pg_create_logical_replication_slot") {
				return sendErr(be, "create fail")
			}
			// pg_replication_slots 查询：0 行（槽不存在）
			return sendQueryResult(be, []string{"confirmed_flush_lsn"}, [][]string{}, "SELECT 0")
		})
	}
	c := openConn(t, handle)
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	_, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}})
	if err == nil || !strings.Contains(err.Error(), "pg: create slot slot") {
		t.Fatalf("create slot err = %v", err)
	}
	_ = c.Close(context.Background())
}

// pump 发送选择臂的 ctx 取消（764）：单帧 70 事件 > 64 缓冲，不读时第 65
// 事件阻塞 → 取消收敛。
func TestPollPumpSendCancel(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		if idx == 1 {
			if _, err := be.Receive(); err != nil {
				return
			}
			be.Send(&pgproto3.CopyBothResponse{})
			_ = be.Flush()
			time.Sleep(20 * time.Millisecond)
			// 2 列关系 + begin + 70 insert + commit（每条消息独立 XLogData 帧：
			// Feed 单消息解析，帧内不带后续消息）
			_ = sendCopy(be, xlogFrame(0x3000, wireMessage('R', relPayload(itemsOID, "game", "items", 'd',
				relCol(FlagReplicaIdentity, "id", 23), relCol(0, "name", 25)))))
			_ = sendCopy(be, xlogFrame(0x3001, beginMsg(1)))
			for i := 0; i < 70; i++ {
				_ = sendCopy(be, xlogFrame(0x3002+uint64(i),
					insertMsg(textDatum("1"), textDatum("v"))))
			}
			_ = sendCopy(be, xlogFrame(0x4000, commitMsg(10)))
			drainCopy(conn, be, nil, 3*time.Second)
			return
		}
		serveQueries(be, slotRoute(be, "0/16B0000", nil))
	}
	c := openConn(t, handle)
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	ctx, cancel := context.WithCancel(context.Background())
	events, err := c.Poll(ctx, connector.PollRequest{Tables: []string{"game.items"}})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	time.Sleep(150 * time.Millisecond) // 让 pump 填满缓冲后阻塞在发送
	cancel()
	n := 0
	for {
		select {
		case _, ok := <-events:
			if !ok {
				if n > 70 || n == 0 {
					t.Fatalf("drained %d events, want 1..70", n)
				}
				return
			}
			n++
		case <-time.After(5 * time.Second):
			t.Fatalf("event stream did not close after cancel (drained %d)", n)
		}
	}
}

// resolveStart 的 confirmed 位点解析失败臂（651-652）：槽查询返回垃圾
// confirmed_flush_lsn → resolveStartLSN 解析失败。
func TestPollResolveConfirmedGarbage(t *testing.T) {
	handle := func(idx int, be *pgproto3.Backend, conn net.Conn) {
		serveQueries(be, slotRoute(be, "not-a-lsn", nil))
	}
	c := openConn(t, handle)
	c.cfg.Options = map[string]string{"slotName": "slot", "publication": "pub"}
	_, err := c.Poll(context.Background(), connector.PollRequest{Tables: []string{"game.items"}})
	if err == nil || !strings.Contains(err.Error(), "parse slot confirmed position") {
		t.Fatalf("confirmed garbage err = %v", err)
	}
	_ = c.Close(context.Background())
}
