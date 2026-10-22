package source

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	ch := make(chan ChangeEvent, 8)
	tbl := testTableMap()

	s.emit(tbl, [][]interface{}{{int64(42), "sword"}}, nil, OpInsert, 1700000000000, "1", "uuid:1-5", ch)
	ev := <-ch
	assert.Equal(t, "mysql", ev.SourceType)
	assert.Equal(t, "game", ev.Database)
	assert.Equal(t, "items", ev.Table)
	assert.Equal(t, OpInsert, ev.Op)
	vals := rowsJSON(t, ev.AfterJSON)
	require.Len(t, vals, 2)
	assert.Equal(t, "sword", vals[1])
	assert.Equal(t, "uuid:1-5", ev.GTID)
	assert.Equal(t, int64(1700000000000), ev.TsUnixMs)
	assert.Equal(t, "1", ev.ServerID)
	assert.Empty(t, ev.Account, "binlog carries no session account (C1 boundary)")

	// 白名单外：不出事件
	other := &replication.RowsEvent{Table: &replication.TableMapEvent{Schema: []byte("game"), Table: []byte("users")}}
	s.emit(other, [][]interface{}{{int64(1)}}, nil, OpInsert, 1, "1", "uuid:1-6", ch)
	select {
	case ev := <-ch:
		t.Fatalf("whitelisted-out table must be filtered, got %v", ev)
	default:
	}
}

func TestEmitUpdatePairsBeforeAfter(t *testing.T) {
	s := newTestSource(nil) // 全订
	ch := make(chan ChangeEvent, 8)
	tbl := testTableMap()

	s.emit(tbl, [][]interface{}{{int64(42), 99}}, [][]interface{}{{int64(42), 1}}, OpUpdate, 1, "1", "uuid:1-7", ch)
	ev := <-ch
	require.Equal(t, OpUpdate, ev.Op)
	before := rowsJSON(t, ev.BeforeJSON)
	after := rowsJSON(t, ev.AfterJSON)
	assert.Equal(t, float64(1), before[1])
	assert.Equal(t, float64(99), after[1])
}

func TestEmitDeleteKeepsBeforeOnly(t *testing.T) {
	s := newTestSource(nil)
	ch := make(chan ChangeEvent, 8)
	s.emit(testTableMap(), [][]interface{}{{int64(42)}}, nil, OpDelete, 1, "1", "uuid:1-8", ch)
	ev := <-ch
	assert.Equal(t, OpDelete, ev.Op)
	require.NotEmpty(t, ev.AfterJSON, "delete carries the last image as afterJson")
	assert.Empty(t, ev.BeforeJSON)
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
