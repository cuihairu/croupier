package source

// mysql_pump.go 补测：MariaDB GTID 事件的事务 ID 捕获、QueryEvent DDL 提交
// 边界（GSet 推进 + 暂存冲刷）、rowEvents 的 v1/v2 全事件类型翻译（写/删/
// 更新成对行像与畸形对跳过）、mustRowJSON 的不可序列化占位兜底、GetEvent
// 失败在主动停机下的静默收敛。
//
// 诚实跳过：无（binlog 事件载荷可内存构造）。

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/stretchr/testify/require"
)

func TestPumpCapturesMariadbGTID(t *testing.T) {
	gset, err := mysql.ParseGTIDSet("mariadb", "1-2-3")
	require.NoError(t, err)

	stream := &scriptedEventStream{events: []*replication.BinlogEvent{
		binlogEvent(replication.MARIADB_GTID_EVENT, &replication.MariadbGTIDEvent{
			GTID: mysql.MariadbGTID{DomainID: 1, ServerID: 2, SequenceNumber: 3},
		}),
		binlogEvent(replication.WRITE_ROWS_EVENTv1, rowsEvent([][]interface{}{{"a"}})),
		binlogEvent(replication.XID_EVENT, &replication.XIDEvent{GSet: gset}),
	}}
	s := newTestSource(nil)
	out := make(chan ChangeEvent, 4)
	go s.pump(context.Background(), stream, out)

	var events []ChangeEvent
	for event := range out {
		events = append(events, event)
	}
	require.Len(t, events, 1)
	require.Equal(t, "1-2-3", events[0].TxID, "mariadb GTID becomes the tx id")
	require.Equal(t, "1-2-3", events[0].GTID)
}

func TestPumpQueryEventCommitsPendingRows(t *testing.T) {
	// DDL Query 边界：GSet 推进 + 暂存行事件冲刷（XID 之外的提交路径）
	gset, err := mysql.ParseGTIDSet("mysql", "3e11fa47-71ca-11e1-9e33-c80aa9429562:1-7")
	require.NoError(t, err)

	stream := &scriptedEventStream{events: []*replication.BinlogEvent{
		binlogEvent(replication.WRITE_ROWS_EVENTv2, rowsEvent([][]interface{}{{"row-a"}})),
		binlogEvent(replication.QUERY_EVENT, &replication.QueryEvent{
			Query: []byte("CREATE TABLE game.items2 (id int)"), GSet: gset,
		}),
	}}
	s := newTestSource(nil)
	out := make(chan ChangeEvent, 4)
	go s.pump(context.Background(), stream, out)

	var events []ChangeEvent
	for event := range out {
		events = append(events, event)
	}
	require.Len(t, events, 1)
	require.Equal(t, gset.String(), events[0].GTID, "query boundary advances the bookmark")
	require.True(t, events[0].TxEnd)
}

func TestRowEventsTranslatesAllEventTypes(t *testing.T) {
	s := newTestSource(nil)
	rows := [][]interface{}{{int64(1), "x"}}

	write := func(v replication.EventType) []ChangeEvent {
		e := rowsEvent(rows)
		return s.rowEvents(binlogEvent(v, e), e)
	}
	for _, v := range []replication.EventType{
		replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2,
	} {
		evs := write(v)
		require.Len(t, evs, 1, "write v%d must emit insert", v)
		require.Equal(t, OpInsert, evs[0].Op)
	}
	for _, v := range []replication.EventType{
		replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2,
	} {
		evs := write(v)
		require.Len(t, evs, 1, "delete v%d must emit delete", v)
		require.Equal(t, OpDelete, evs[0].Op)
	}

	// 更新成对行像（v1/v2）：pair = e.Rows[i] = [before 像, after 像]
	for _, v := range []replication.EventType{
		replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2,
	} {
		pair := rowsEvent([][]interface{}{{[]interface{}{"old"}, []interface{}{"new"}}})
		evs := s.rowEvents(binlogEvent(v, pair), pair)
		require.Len(t, evs, 1, "update v%d must emit one paired update", v)
		require.Equal(t, OpUpdate, evs[0].Op)
		require.NotEmpty(t, evs[0].BeforeJSON)
		require.NotEmpty(t, evs[0].AfterJSON)
	}
}

func TestRowEventsSkipsMalformedUpdatePairs(t *testing.T) {
	s := newTestSource(nil)
	e := rowsEvent([][]interface{}{
		{"lonely"},                               // 奇数长度对 → 跳过
		{"not-slice", 42},                        // 非行像切片 → 跳过
		{[]interface{}{"o"}, []interface{}{"n"}}, // 合法对
	})
	evs := s.rowEvents(binlogEvent(replication.UPDATE_ROWS_EVENTv2, e), e)
	require.Len(t, evs, 1, "malformed pairs must be skipped, good pair kept")
	require.Equal(t, OpUpdate, evs[0].Op)

	// 非行事件类型 → 空翻译
	require.Empty(t, s.rowEvents(
		binlogEvent(replication.ROTATE_EVENT, &replication.RotateEvent{}),
		e))
}

func TestMustRowJSONErrorPlaceholder(t *testing.T) {
	// 不可序列化值 → 占位 JSON，不让单行毒性阻塞整条流
	raw := mustRowJSON([]interface{}{make(chan int)})
	require.True(t, strings.HasPrefix(string(raw), `{"rowJsonError":`), "got %s", raw)
}

func TestPumpSilentOnCancelledCtxError(t *testing.T) {
	// GetEvent 失败 + ctx 已取消 → 静默返回（主动停机不刷错误日志）
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := &blockedEventStream{ctx: ctx}
	s := newTestSource(nil)
	out := make(chan ChangeEvent, 1)
	done := make(chan struct{})
	go func() {
		s.pump(ctx, stream, out)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pump must return promptly when ctx is cancelled")
	}
}

// blockedEventStream 模拟被取消打断的复制流读取。
type blockedEventStream struct{ ctx context.Context }

func (b *blockedEventStream) GetEvent(ctx context.Context) (*replication.BinlogEvent, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
