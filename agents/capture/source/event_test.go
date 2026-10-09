package source

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleEvent() ChangeEvent {
	return ChangeEvent{
		SourceType: "mysql",
		Database:   "game",
		Table:      "items",
		Op:         OpUpdate,
		BeforeJSON: json.RawMessage(`{"id":42,"count":1}`),
		AfterJSON:  json.RawMessage(`{"id":42,"count":99}`),
		GTID:       "3E11FA47-71CA-11E1-9E33-C80AA9429562:1-5",
		Account:    "game_app",
		ClientAddr: "10.0.0.8:41230",
		TsUnixMs:   1700000000000,
		ServerID:   "mysql-1",
	}
}

func TestChangeEventTypeAndFields(t *testing.T) {
	e := sampleEvent()
	data, err := json.Marshal(e)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	for _, key := range []string{"sourceType", "database", "table", "op", "beforeJson", "afterJson", "gtid", "account", "tsUnixMs", "serverId"} {
		assert.Contains(t, m, key)
	}

	// insert 无前像：beforeJson 省略
	e.Op = OpInsert
	e.BeforeJSON = nil
	data, _ = json.Marshal(e)
	assert.NotContains(t, string(data), "beforeJson")
}

func TestIDempotencyKey(t *testing.T) {
	e := sampleEvent()
	k1 := e.IDempotencyKey()
	assert.Contains(t, k1, e.GTID)
	assert.Contains(t, k1, "game.items")
	assert.Contains(t, k1, string(OpUpdate))
	assert.Equal(t, k1, e.IDempotencyKey(), "same event = same key")

	// 同一行同 op 前后像相同 → 同键（at-least-once 去重锚点）
	dup := e
	assert.Equal(t, k1, dup.IDempotencyKey())

	// 前像不同（行内容变）→ 键变
	other := e
	other.AfterJSON = json.RawMessage(`{"id":42,"count":100}`)
	assert.NotEqual(t, k1, other.IDempotencyKey())

	// delete 只用前像
	del := e
	del.Op = OpDelete
	del.AfterJSON = nil
	assert.NotEqual(t, k1, del.IDempotencyKey())
	assert.Equal(t, del.IDempotencyKey(), del.IDempotencyKey())
}

func TestMatchesTable(t *testing.T) {
	e := sampleEvent()
	whitelist := map[string]bool{"game.items": true, "game.currency": true}
	assert.True(t, e.MatchesTable(whitelist))

	e.Table = "users"
	assert.False(t, e.MatchesTable(whitelist), "not in subscription whitelist")

	// 空白名单 = 全订（仅测试语义；生产配置必须显式列出）
	assert.True(t, e.MatchesTable(nil))
}

func TestBookmarkStoreRoundTripAndAtomic(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "pos", "bookmark.json")) // 父目录不存在也应可写

	b, err := s.Load()
	require.NoError(t, err)
	assert.False(t, b.Valid(), "missing file = empty bookmark (fresh rescan)")

	b = Bookmark{Position: "3E11FA47:1-5", UpdatedAtUnixMs: time.Now().UnixMilli()}
	require.NoError(t, s.Save(b))

	got, err := s.Load()
	require.NoError(t, err)
	assert.Equal(t, b, got)
	assert.True(t, got.Valid())

	// 覆盖推进
	b.Position = "3E11FA47:1-9"
	require.NoError(t, s.Save(b))
	got, _ = s.Load()
	assert.Equal(t, "3E11FA47:1-9", got.Position)

	// 临时文件不残留
	entries, _ := os.ReadDir(filepath.Join(dir, "pos"))
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bookmark-", "temp file must be renamed away")
	}
}

func TestBookmarkStoreCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bookmark.json")
	require.NoError(t, os.WriteFile(path, []byte("{broken"), 0o644))

	_, err := NewStore(path).Load()
	require.Error(t, err, "corrupt bookmark must surface, not silently rescan")
}

// compile-time 接口面自检：ctx 形态可用。
var _ = context.Background
