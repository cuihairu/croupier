package objstore

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type fileStore struct {
	base         string
	publicPrefix string
	ttl          time.Duration
}

func OpenFile(_ context.Context, c Config) (Store, error) {
	if c.BaseDir == "" {
		return nil, fmt.Errorf("base_dir required for file driver")
	}
	if err := os.MkdirAll(c.BaseDir, 0o755); err != nil {
		return nil, err
	}
	ttl := c.SignedURLTTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	return &fileStore{base: c.BaseDir, publicPrefix: "/uploads/", ttl: ttl}, nil
}

func (s *fileStore) Put(_ context.Context, key string, r ReadSeeker, _ int64, _ string) error {
	key = sanitizeKey(key)
	path := filepath.Join(s.base, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return nil
}

func (s *fileStore) SignedURL(_ context.Context, key string, method string, _ time.Duration) (string, error) {
	if method == "DELETE" {
		return "", fmt.Errorf("not supported")
	}
	key = sanitizeKey(key)
	// Return relative URL served by HTTP server under /uploads/
	u := url.URL{Path: s.publicPrefix + key}
	return u.String(), nil
}

func (s *fileStore) Delete(_ context.Context, key string) error {
	key = sanitizeKey(key)
	path := filepath.Join(s.base, filepath.FromSlash(key))
	return os.Remove(path)
}
