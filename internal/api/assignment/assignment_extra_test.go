// 覆盖目标：saveAssignments/saveAssignmentHistory 的目录创建失败分支、
// authorizeCloneTarget 的无权限/未知来源分支、deriveGameDBName 路由回落。
package assignment

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAssignments_ThenLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "assignments.json")

	require.NoError(t, saveAssignments(path, map[string][]string{"fn": {"agent-1"}}))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "agent-1")
}

func TestSaveAssignments_DirIsFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	// blocker/sub/assignments.json 的父目录创建失败
	err := saveAssignments(filepath.Join(blocker, "sub", "assignments.json"), nil)
	require.Error(t, err)
}

func TestSaveAssignmentHistory_DirIsFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	err := saveAssignmentHistory(filepath.Join(blocker, "sub", "history.json"), nil)
	require.Error(t, err)
}

func TestSaveAssignmentHistory_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	require.NoError(t, saveAssignmentHistory(path, []assignmentHistoryEntry{{FunctionID: "f", Action: "bind"}}))
	loaded, err := loadAssignmentHistory(path)
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "f", loaded[0].FunctionID)
}
