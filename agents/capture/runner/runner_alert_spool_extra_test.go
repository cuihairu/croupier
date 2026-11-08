package runner

// alert_spool.go 补测：legacy map 形态恢复、entries 空/重复键跳过、
// hitForKey 命中与未命中、writer 置空的入队回滚、writeFileAtomic 的
// MkdirAll / rename 失败注入。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/gate"
	"github.com/stretchr/testify/require"
)

func spoolHit(rule string) gate.Hit {
	return gate.Hit{
		Rule: rule, Table: "game.items", Key: "9001",
		Event: connector.ChangeEvent{
			Position: connector.Position{Kind: "gtid", Value: "g-" + rule},
			TsUnixMs: 1_700_000_000_000,
		},
	}
}

func TestAlertSpool_RecoversLegacyMapFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alerts.json")
	// 旧形态：map[string]gate.Hit；键序故意逆序，验证恢复后按 key 排序
	legacy := map[string]gate.Hit{
		"b-key": spoolHit("rule-b"),
		"a-key": spoolHit("rule-a"),
	}
	data, err := json.Marshal(legacy)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	s := newAlertSpool(path)
	got, ok := s.next()
	require.True(t, ok)
	require.Equal(t, "rule-a", got.Rule, "order must sort by key after legacy recovery")
	require.Equal(t, 2, s.spoolCountOn(t))

	// 恢复后再入队：落盘应翻新为 entries 形态且包含 legacy 条目
	require.NoError(t, s.enqueue(spoolHit("rule-c")))
	var file alertSpoolFile
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &file))
	require.Len(t, file.Entries, 3)
	require.Equal(t, "a-key", file.Entries[0].Key, "persisted order stays sorted")
}

func TestAlertSpool_SkipsEmptyAndDuplicateKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alerts.json")
	file := alertSpoolFile{Entries: []alertSpoolEntry{
		{Key: "", Hit: spoolHit("rule-empty")}, // 空键丢弃
		{Key: "k-b", Hit: spoolHit("rule-b1")}, // 首次保留
		{Key: "k-b", Hit: spoolHit("rule-b2")}, // 重复键跳过
		{Key: "k-a", Hit: spoolHit("rule-a1")}, // 追加在后
	}}
	data, err := json.Marshal(file)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	s := newAlertSpool(path)
	require.Equal(t, 2, s.spoolCountOn(t))
	require.Equal(t, []string{"k-b", "k-a"}, s.order)
	got, ok := s.next()
	require.True(t, ok)
	require.Equal(t, "rule-b1", got.Rule, "duplicate keeps first occurrence")
}

func TestAlertSpool_HitForKeyFoundAndMiss(t *testing.T) {
	s := newAlertSpool(filepath.Join(t.TempDir(), "alerts.json"))
	hit := spoolHit("rule-x")
	require.NoError(t, s.enqueue(hit))

	found := s.hitForKey(s.key(hit))
	require.Equal(t, "rule-x", found.Rule)
	require.Equal(t, "9001", found.Key)

	var miss = s.hitForKey("no-such-key")
	require.Empty(t, miss.Rule)
	require.Empty(t, miss.Key)
}

func (s *alertSpool) spoolCountOn(t *testing.T) int {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

func TestAlertSpool_NilWriterRollsBackEnqueue(t *testing.T) {
	s := newAlertSpool(filepath.Join(t.TempDir(), "alerts.json"))
	first := spoolHit("rule-1")
	require.NoError(t, s.enqueue(first))

	// 写盘句柄不可用：第二条入队报错且回滚，首条仍待发
	s.writeFile = nil
	err := s.enqueue(spoolHit("rule-2"))
	require.ErrorContains(t, err, "writer unavailable")
	require.Equal(t, 1, s.spoolCountOn(t))
	require.Equal(t, []string{s.key(first)}, s.order)
}

func TestWriteFileAtomic_MkdirAllFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))
	// 目录组件是普通文件 → MkdirAll 失败
	require.Error(t, writeFileAtomic(filepath.Join(blocker, "sub", "alerts.json"), []byte("{}")))
}

func TestWriteFileAtomic_CreateTempFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores dir permissions, cannot force CreateTemp failure")
	}
	ro := filepath.Join(t.TempDir(), "ro")
	require.NoError(t, os.Mkdir(ro, 0o500))
	require.Error(t, writeFileAtomic(filepath.Join(ro, "alerts.json"), []byte("{}")))
}

func TestWriteFileAtomic_RenameFailure(t *testing.T) {
	dir := t.TempDir()
	// 目标路径是已存在目录：rename 文件→目录失败，临时文件应清理
	target := filepath.Join(dir, "occupied")
	require.NoError(t, os.Mkdir(target, 0o755))
	require.Error(t, writeFileAtomic(target, []byte("{}")))
	leftovers, err := filepath.Glob(filepath.Join(dir, ".alerts-*"))
	require.NoError(t, err)
	require.Empty(t, leftovers, "temp file must be removed on rename failure")
}
