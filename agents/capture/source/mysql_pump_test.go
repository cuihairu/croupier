package source

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scriptedEventStream struct {
	events []*replication.BinlogEvent
	index  int
}

func (s *scriptedEventStream) GetEvent(context.Context) (*replication.BinlogEvent, error) {
	if s.index >= len(s.events) {
		return nil, fmt.Errorf("scripted stream exhausted")
	}
	event := s.events[s.index]
	s.index++
	return event, nil
}

func newTestSource(tables map[string]bool) *MySQLSource {
	s := NewMySQLSource(MySQLConfig{Host: "h", Port: 3306, User: "u", ServerID: 77, Tables: tables}, nil)
	return s
}

func testTableMap() *replication.RowsEvent {
	return &replication.RowsEvent{Table: &replication.TableMapEvent{Schema: []byte("game"), Table: []byte("items")}}
}

func rowsJSON(t *testing.T, raw json.RawMessage) []interface{} {
	t.Helper()
	var out []interface{}
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func TestEmitInsertFiltersWhitelist(t *testing.T) {
	s := newTestSource(map[string]bool{"game.items": true})
	tbl := testTableMap()

	evs := s.emitRows(tbl, [][]interface{}{{int64(42), "sword"}}, nil, OpInsert, 1700000000000, "1")
	require.Len(t, evs, 1)
	ev := evs[0]
	assert.Equal(t, "mysql", ev.SourceType)
	assert.Equal(t, "game", ev.Database)
	assert.Equal(t, "items", ev.Table)
	assert.Equal(t, OpInsert, ev.Op)
	vals := rowsJSON(t, ev.AfterJSON)
	require.Len(t, vals, 2)
	assert.Equal(t, "sword", vals[1])
	assert.Equal(t, int64(1700000000000), ev.TsUnixMs)
	assert.Equal(t, "1", ev.ServerID)
	assert.Empty(t, ev.Account, "binlog carries no session account (C1 boundary)")
	// 事务元数据由 pump 在提交边界（annotateTx）统一补齐，emitRows 本身不填。
	assert.Empty(t, ev.GTID)

	// 白名单外：不出事件
	other := &replication.RowsEvent{Table: &replication.TableMapEvent{Schema: []byte("game"), Table: []byte("users")}}
	assert.Empty(t, s.emitRows(other, [][]interface{}{{int64(1)}}, nil, OpInsert, 1, "1"))
}

func TestEmitUpdatePairsBeforeAfter(t *testing.T) {
	s := newTestSource(nil) // 全订
	tbl := testTableMap()

	evs := s.emitRows(tbl, [][]interface{}{{int64(42), 99}}, [][]interface{}{{int64(42), 1}}, OpUpdate, 1, "1")
	require.Len(t, evs, 1)
	ev := evs[0]
	require.Equal(t, OpUpdate, ev.Op)
	before := rowsJSON(t, ev.BeforeJSON)
	after := rowsJSON(t, ev.AfterJSON)
	assert.Equal(t, float64(1), before[1])
	assert.Equal(t, float64(99), after[1])
}

func TestEmitDeleteKeepsBeforeOnly(t *testing.T) {
	s := newTestSource(nil)
	evs := s.emitRows(testTableMap(), [][]interface{}{{int64(42)}}, nil, OpDelete, 1, "1")
	require.Len(t, evs, 1)
	ev := evs[0]
	assert.Equal(t, OpDelete, ev.Op)
	require.NotEmpty(t, ev.AfterJSON, "delete carries the last image as afterJson")
	assert.Empty(t, ev.BeforeJSON)
}

func TestAnnotateTxMetadata(t *testing.T) {
	// 事务元数据由 pump 在提交边界统一补齐：同事务 ID、连续序号、末事件标记。
	pending := []ChangeEvent{{Table: "a"}, {Table: "b"}, {Table: "c"}}
	annotateTx(pending, "uuid:1-5")
	assert.Equal(t, "uuid:1-5", pending[0].TxID)
	assert.Equal(t, int64(1), pending[0].TxSeq)
	assert.False(t, pending[0].TxEnd)
	assert.Equal(t, "uuid:1-5", pending[1].TxID)
	assert.Equal(t, int64(2), pending[1].TxSeq)
	assert.False(t, pending[1].TxEnd)
	assert.Equal(t, "uuid:1-5", pending[2].TxID)
	assert.Equal(t, int64(3), pending[2].TxSeq)
	assert.True(t, pending[2].TxEnd)

	// 无待放行事件时幂等，不改状态。
	assert.Empty(t, annotateTx(nil, ""))
}

func TestPumpAnnotatesCommittedTransactionAndPosition(t *testing.T) {
	const gtidText = "3e11fa47-71ca-11e1-9e33-c80aa9429562:1-5"
	gset, err := mysql.ParseGTIDSet("mysql", gtidText)
	require.NoError(t, err)
	sid, err := uuid.Parse("3E11FA47-71CA-11E1-9E33-C80AA9429562")
	require.NoError(t, err)

	stream := &scriptedEventStream{events: []*replication.BinlogEvent{
		binlogEvent(replication.GTID_EVENT, &replication.GTIDEvent{SID: sid[:], GNO: 5}),
		binlogEvent(replication.QUERY_EVENT, &replication.QueryEvent{Query: []byte("BEGIN"), GSet: gset}),
		binlogEvent(replication.WRITE_ROWS_EVENTv2, rowsEvent([][]interface{}{{int64(42), "sword"}})),
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
	assert.Equal(t, gset.String(), events[0].GTID)
	assert.Equal(t, "3e11fa47-71ca-11e1-9e33-c80aa9429562:5", events[0].TxID)
	assert.Equal(t, int64(1), events[0].TxSeq)
	assert.True(t, events[0].TxEnd)
	assert.Equal(t, gset.String(), s.Bookmark().Position)
}

func TestPumpDoesNotAdvanceBookmarkOnBegin(t *testing.T) {
	const gtidText = "3e11fa47-71ca-11e1-9e33-c80aa9429562:1-5"
	gset, err := mysql.ParseGTIDSet("mysql", gtidText)
	require.NoError(t, err)
	sid, err := uuid.Parse("3E11FA47-71CA-11E1-9E33-C80AA9429562")
	require.NoError(t, err)

	stream := &scriptedEventStream{events: []*replication.BinlogEvent{
		binlogEvent(replication.GTID_EVENT, &replication.GTIDEvent{SID: sid[:], GNO: 5}),
		binlogEvent(replication.QUERY_EVENT, &replication.QueryEvent{Query: []byte("BEGIN"), GSet: gset}),
		binlogEvent(replication.WRITE_ROWS_EVENTv2, rowsEvent([][]interface{}{{int64(42), "sword"}})),
	}}
	s := newTestSource(nil)
	out := make(chan ChangeEvent, 4)
	go s.pump(context.Background(), stream, out)

	var events []ChangeEvent
	for event := range out {
		events = append(events, event)
	}
	assert.Empty(t, events, "uncommitted rows must not be emitted")
	assert.Empty(t, s.Bookmark().Position, "BEGIN must not advance the recovery position")
}

func TestIsTransactionBegin(t *testing.T) {
	for _, query := range []string{"BEGIN", " begin ", "START TRANSACTION", "start transaction /* comment */"} {
		assert.True(t, isTransactionBegin([]byte(query)), query)
	}
	for _, query := range []string{"COMMIT", "CREATE TABLE t (id int)", "SELECT 1"} {
		assert.False(t, isTransactionBegin([]byte(query)), query)
	}
}

func binlogEvent(eventType replication.EventType, event replication.Event) *replication.BinlogEvent {
	return &replication.BinlogEvent{
		Header: &replication.EventHeader{EventType: eventType, Timestamp: 1700000000, ServerID: 1},
		Event:  event,
	}
}

func rowsEvent(rows [][]interface{}) *replication.RowsEvent {
	return &replication.RowsEvent{
		Table: &replication.TableMapEvent{Schema: []byte("game"), Table: []byte("items")},
		Rows:  rows,
	}
}

func TestBookmarkSnapshotAdvances(t *testing.T) {
	s := newTestSource(nil)
	assert.Equal(t, "", s.Bookmark().Position)
	s.setGTID("uuid:1-5")
	assert.Equal(t, "uuid:1-5", s.Bookmark().Position)
	s.setGTID("uuid:1-9")
	assert.Equal(t, "uuid:1-9", s.Bookmark().Position)
	assert.True(t, s.Bookmark().Valid())
}

func TestOpenRejectsBadBookmarkAndUnprobedStart(t *testing.T) {
	s := newTestSource(nil)
	s.probe = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	_, err := s.Open(context.Background(), Bookmark{Position: "not a gtid"})
	require.Error(t, err, "bad bookmark must fail fast")

	s2 := newTestSource(nil)
	s2.probe = func(ctx context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	_, err = s2.Open(context.Background(), Bookmark{})
	require.Error(t, err, "no bookmark + failing probe = no blind rescan")

	// probe 返回坏串也报错
	s3 := newTestSource(nil)
	s3.probe = func(ctx context.Context) (string, error) { return "garbage", nil }
	_, err = s3.Open(context.Background(), Bookmark{})
	require.Error(t, err)
}

func TestOpenWithFakeProbeConnectFailsFast(t *testing.T) {
	// probe 可用但源不可达：StartSyncGTID 报错，Open 收口为 error（不挂死）。
	s := newTestSource(nil)
	s.probe = func(ctx context.Context) (string, error) { return "3E11FA47-71CA-11E1-9E33-C80AA9429562:1-5", nil }
	_, err := s.Open(context.Background(), Bookmark{})
	require.Error(t, err)
	assert.Nil(t, s.syncer, "failed open must not leak syncer")
}
