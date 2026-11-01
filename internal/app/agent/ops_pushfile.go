package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// PushFile writes a configuration file delivered by the OPS-006 batch config
// push. Fail-closed on every boundary:
//   - PushBaseDir empty → disabled.
//   - Target path must be absolute and inside PushBaseDir (symlink/.. traversal
//     rejected by path containment after EvalSymlinks on the base dir).
//   - Content size must be ≤ MaxPushFileBytes.
//   - expected_sha256, when present, must match the content (content-pinned).
//   - Overwriting an existing file requires force=true.
func (s *OpsServer) PushFile(ctx context.Context, req *opsv1.PushFileRequest) (*opsv1.PushFileResponse, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("ops is not enabled")
	}
	base := strings.TrimSpace(s.config.PushBaseDir)
	if base == "" {
		return nil, fmt.Errorf("ops config push is not enabled (pushBaseDir empty)")
	}
	path := strings.TrimSpace(req.GetPath())
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("push target path must be absolute")
	}
	if len(req.GetContent()) == 0 {
		return nil, fmt.Errorf("push content is empty")
	}
	maxBytes := s.config.MaxPushFileBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	if int64(len(req.GetContent())) > maxBytes {
		return nil, fmt.Errorf("push content exceeds %d bytes", maxBytes)
	}

	// Resolve base dir (may not exist yet — create it 0700).
	absBase, err := filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("resolve push base dir: %w", err)
	}
	if err := os.MkdirAll(absBase, 0o700); err != nil {
		return nil, fmt.Errorf("create push base dir: %w", err)
	}
	realBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		realBase = absBase
	}
	// Target must be inside the base dir (lexical + symlink-resolved check).
	absTarget, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve target path: %w", err)
	}
	rel, err := filepath.Rel(realBase, absTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("push target escapes allowed base dir")
	}
	// Existing parent dir may be a symlink; verify resolved target stays inside.
	if realParent, err := filepath.EvalSymlinks(filepath.Dir(absTarget)); err == nil {
		rel2, err2 := filepath.Rel(realBase, realParent)
		if err2 != nil || rel2 == ".." || strings.HasPrefix(rel2, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("push target parent escapes allowed base dir")
		}
	}

	if !req.GetForce() {
		if _, err := os.Stat(absTarget); err == nil {
			return nil, fmt.Errorf("push target exists (force=false): %s", absTarget)
		}
	}

	// Content hash (content-pin check when expected provided).
	sum := sha256.Sum256(req.GetContent())
	gotHex := hex.EncodeToString(sum[:])
	if exp := strings.ToLower(strings.TrimSpace(req.GetExpectedSha256())); exp != "" {
		if exp != gotHex {
			return nil, fmt.Errorf("push content sha256 mismatch: expected %s, got %s", exp, gotHex)
		}
	}

	mode := os.FileMode(req.GetMode())
	if mode == 0 {
		mode = 0o644
	}
	if err := os.MkdirAll(filepath.Dir(absTarget), 0o755); err != nil {
		return nil, fmt.Errorf("create target dir: %w", err)
	}
	if err := os.WriteFile(absTarget, req.GetContent(), mode); err != nil {
		return nil, fmt.Errorf("write push target: %w", err)
	}

	return &opsv1.PushFileResponse{
		Success:      true,
		Message:      "pushed",
		BytesWritten: int64(len(req.GetContent())),
		Sha256:       gotHex,
	}, nil
}
