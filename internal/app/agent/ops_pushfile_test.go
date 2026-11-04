package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPushServer(base string) *OpsServer {
	cfg := DefaultOpsConfig()
	cfg.Enabled = true
	cfg.PushBaseDir = base
	return NewOpsServer(cfg, "agent-1", "test", nil)
}

func TestPushFile_WritesUnderBase(t *testing.T) {
	base := t.TempDir()
	s := newPushServer(base)
	content := []byte("hello config\n")
	resp, err := s.PushFile(context.Background(), &opsv1.PushFileRequest{
		Path: filepath.Join(base, "etc", "app.yaml"), Content: content, Mode: 0o644,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, int64(len(content)), resp.BytesWritten)
	data, err := os.ReadFile(filepath.Join(base, "etc", "app.yaml"))
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestPushFile_RejectsEscape(t *testing.T) {
	base := t.TempDir()
	s := newPushServer(base)
	_, err := s.PushFile(context.Background(), &opsv1.PushFileRequest{
		Path: filepath.Join(base, "..", "outside"), Content: []byte("x"),
	})
	require.Error(t, err)
}

func TestPushFile_DisabledWithoutBase(t *testing.T) {
	cfg := DefaultOpsConfig()
	cfg.Enabled = true
	s := NewOpsServer(cfg, "a", "v", nil)
	_, err := s.PushFile(context.Background(), &opsv1.PushFileRequest{Path: "/tmp/x", Content: []byte("y")})
	require.Error(t, err)
}

func TestPushFile_HashPinMismatch(t *testing.T) {
	base := t.TempDir()
	s := newPushServer(base)
	_, err := s.PushFile(context.Background(), &opsv1.PushFileRequest{
		Path: filepath.Join(base, "x"), Content: []byte("abc"), ExpectedSha256: "deadbeef",
	})
	require.Error(t, err)
}

func TestPushFile_NoOverwriteWithoutForce(t *testing.T) {
	base := t.TempDir()
	s := newPushServer(base)
	p := filepath.Join(base, "x")
	require.NoError(t, os.WriteFile(p, []byte("existing"), 0o644))
	_, err := s.PushFile(context.Background(), &opsv1.PushFileRequest{Path: p, Content: []byte("new")})
	require.Error(t, err)
	// force overwrites
	resp, err := s.PushFile(context.Background(), &opsv1.PushFileRequest{Path: p, Content: []byte("new"), Force: true})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	data, _ := os.ReadFile(p)
	assert.Equal(t, "new", string(data))
}

func TestPushFile_RejectsTargetSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "escaped.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside"), 0o644))

	// Pre-plant a symlink at the target path; it points outside the base.
	target := filepath.Join(base, "linked.yaml")
	require.NoError(t, os.Symlink(outsideFile, target))

	s := newPushServer(base)
	_, err := s.PushFile(context.Background(), &opsv1.PushFileRequest{
		Path: target, Content: []byte("evil"), Force: true, Mode: 0o644,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside")

	// The outside file must not be overwritten.
	data, err := os.ReadFile(outsideFile)
	require.NoError(t, err)
	assert.Equal(t, "outside", string(data))
}
