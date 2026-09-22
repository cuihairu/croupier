package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.yaml")
	require.NoError(t, os.WriteFile(path, []byte("events:\n  - id: a\n"), 0o644))
	out, err := readYAML(path)
	require.NoError(t, err)
	require.Contains(t, out, "events")

	// 缺文件
	_, err = readYAML(filepath.Join(dir, "missing.yaml"))
	assert.Error(t, err)

	// 非法 yaml
	bad := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(bad, []byte("{unclosed"), 0o644))
	_, err = readYAML(bad)
	assert.Error(t, err)
}

func TestEnsureDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")
	require.NoError(t, ensureDir(nested))
	info, err := os.Stat(nested)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestIndexById(t *testing.T) {
	// 正常形态：只收 id 为字符串的映射条目
	root := map[string]Any{
		"events": []any{
			map[string]Any{"id": "a", "x": 1},
			map[string]Any{"no-id": true}, // 缺 id → 跳过
			"scalar",                      // 非映射 → 跳过
			map[string]Any{"id": 42},      // id 非字符串 → 跳过
			map[string]Any{"id": "b"},
		},
	}
	m := indexById(root, "events")
	require.Len(t, m, 2)
	require.Contains(t, m, "a")
	require.Contains(t, m, "b")

	// section 缺失 / 非数组 → nil（调用方跳过派生键）
	assert.Nil(t, indexById(root, "missing"))
	assert.Nil(t, indexById(map[string]Any{"events": "not-a-list"}, "events"))
	assert.Nil(t, indexById(nil, "events"))

	// 空数组 → 非 nil 空索引（原语义：仍写入派生键）
	empty := indexById(map[string]Any{"events": []any{}}, "events")
	require.NotNil(t, empty)
	assert.Empty(t, empty)
}

// writeConfigs 播种一份最小 analytics 配置目录。
func writeConfigs(t *testing.T, dir string) {
	t.Helper()
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	write("events.yaml", "events:\n  - id: evt_login\n    label: 登录\n  - junk: true\n")
	write("metrics.yaml", "metrics:\n  - id: m_dau\n")
	write("game_types.yaml", "game_types:\n  - id: slingo\n")
	write("taxonomy.yaml", "taxonomy:\n  root: events\n")
}

func TestRunExport(t *testing.T) {
	cfgDir := t.TempDir()
	writeConfigs(t, cfgDir)

	outPath := filepath.Join(cfgDir, "deep", "nested", "spec.json")
	require.NoError(t, runExport(cfgDir, outPath))

	raw, err := os.ReadFile(outPath)
	require.NoError(t, err)
	var spec Spec
	require.NoError(t, json.Unmarshal(raw, &spec))
	assert.Equal(t, "1", spec.Version)
	require.Contains(t, spec.Derived, "eventsById")
	require.Contains(t, spec.Derived, "metricsById")
	require.Contains(t, spec.Derived, "gameTypesById")
	assert.Equal(t, "登录", spec.Events["events"].([]any)[0].(map[string]Any)["label"])

	// taxonomy 缺省可容忍
	require.NoError(t, os.Remove(filepath.Join(cfgDir, "taxonomy.yaml")))
	out2 := filepath.Join(t.TempDir(), "spec.json")
	require.NoError(t, runExport(cfgDir, out2))
}

func TestRunExportErrors(t *testing.T) {
	cfgDir := t.TempDir()
	outPath := filepath.Join(t.TempDir(), "spec.json")

	// 三份必需配置逐个缺失时的报错路径
	assert.ErrorContains(t, runExport(cfgDir, outPath), "read events")
	writeConfigs(t, cfgDir)
	require.NoError(t, os.Remove(filepath.Join(cfgDir, "metrics.yaml")))
	assert.ErrorContains(t, runExport(cfgDir, outPath), "read metrics")
	writeConfigs(t, cfgDir)
	require.NoError(t, os.Remove(filepath.Join(cfgDir, "game_types.yaml")))
	assert.ErrorContains(t, runExport(cfgDir, outPath), "read game_types")

	// out 落点已是目录 → WriteFile 失败
	writeConfigs(t, cfgDir)
	outIsDir := filepath.Join(t.TempDir(), "occupied")
	require.NoError(t, os.MkdirAll(outIsDir, 0o755))
	assert.Error(t, runExport(cfgDir, outIsDir))

	// yaml 里的 NaN 无法 JSON 序列化 → MarshalIndent 失败
	writeConfigs(t, cfgDir)
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "events.yaml"),
		[]byte("events:\n  - id: a\n    score: .nan\n"), 0o644))
	assert.ErrorContains(t, runExport(cfgDir, outPath), "unsupported value")

	// out 的父路径组件已是文件 → ensureDir(MkdirAll) 失败
	writeConfigs(t, cfgDir)
	barrier := filepath.Join(t.TempDir(), "barrier")
	require.NoError(t, os.WriteFile(barrier, []byte("x"), 0o644))
	assert.Error(t, runExport(cfgDir, filepath.Join(barrier, "sub", "spec.json")))
}
