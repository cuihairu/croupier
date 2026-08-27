// Package configsource adapts each project's existing config center for the
// online config explorer (read-first viewing + emergency edit). Croupier does
// not define a project's config workflow; it only reads from (and, for
// writable sources, writes back to) the project's own config center.
// See docs/research/config-workflows-analysis.md.
package configsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

// Entry is one node in the config directory tree.
type Entry struct {
	Name    string    `json:"name"` // 相对当前目录的名称
	Path    string    `json:"path"` // 完整路径（适配器内定位用）
	Dir     bool      `json:"dir"`  // 目录 or 文件
	Size    int64     `json:"size"` // 文件字节数（目录为 0）
	ModTime time.Time `json:"modTime,omitempty"`
}

// Source browses one config center as a directory tree.
type Source interface {
	// Type returns the source type (git/redis/nacos/db/croupier).
	Type() string
	// List returns direct children of dir ("" = root).
	List(ctx context.Context, dir string) ([]Entry, error)
	// Read returns the file content at path.
	Read(ctx context.Context, path string) ([]byte, error)
}

// WritableSource supports emergency edit: the write goes back to the
// project's own config center (平台不另存一份).
type WritableSource interface {
	Source
	Write(ctx context.Context, path string, content []byte, message string) error
}

// IsWritable reports whether the source supports emergency edit.
func IsWritable(s Source) bool {
	_, ok := s.(WritableSource)
	return ok
}

// New builds a Source from a binding. The binding's Config JSON schema
// depends on Type (see per-adapter config structs).
func New(binding *model.ConfigSourceBinding) (Source, error) {
	if binding == nil {
		return nil, errors.New("binding required")
	}
	cfg := map[string]interface{}{}
	if strings.TrimSpace(binding.Config) != "" {
		if err := json.Unmarshal([]byte(binding.Config), &cfg); err != nil {
			return nil, fmt.Errorf("invalid binding config json: %w", err)
		}
	}
	switch binding.Type {
	case model.ConfigSourceTypeGit:
		return newGitSource(cfg)
	case model.ConfigSourceTypeRedis:
		return newRedisSource(cfg)
	case model.ConfigSourceTypeNacos:
		return newNacosSource(cfg)
	case model.ConfigSourceTypeDB:
		return newDBSource(cfg)
	case model.ConfigSourceTypeCroupier:
		return newCroupierSource(cfg, binding.GameID, binding.Env)
	default:
		return nil, fmt.Errorf("unsupported config source type: %s", binding.Type)
	}
}

// cleanPath normalizes a user-supplied tree path and rejects traversal.
func cleanPath(p string) (string, error) {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return "", nil
	}
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		if part == ".." || strings.ContainsAny(part, "\x00\\") {
			return "", fmt.Errorf("invalid path segment: %q", part)
		}
		out = append(out, part)
	}
	return strings.Join(out, "/"), nil
}

// configString reads a string field from the raw config map.
func configString(cfg map[string]interface{}, key, def string) string {
	if v, ok := cfg[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

// configInt reads an int field from the raw config map.
func configInt(cfg map[string]interface{}, key string, def int) int {
	switch v := cfg[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n); err == nil {
			return n
		}
	}
	return def
}

// configStrings reads a []string field from the raw config map.
func configStrings(cfg map[string]interface{}, key string) []string {
	raw, ok := cfg[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// MaskSecrets returns a copy of the binding's Config JSON with credential
// fields redacted, for API responses.
func MaskSecrets(configJSON string) string {
	cfg := map[string]interface{}{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return "{}"
	}
	for _, key := range []string{"password", "token", "secret", "accessKey", "secretKey"} {
		if v, ok := cfg[key].(string); ok && v != "" {
			cfg[key] = "******"
		}
	}
	// DSN 内嵌密码（mysql user:pass@tcp(...)）整体脱敏
	if v, ok := cfg["dsn"].(string); ok && v != "" {
		cfg["dsn"] = maskDSN(v)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// maskDSN redacts the password portion of a MySQL DSN (user:pass@...).
func maskDSN(dsn string) string {
	if i := strings.Index(dsn, "@"); i > 0 {
		head := dsn[:i]
		if j := strings.Index(head, ":"); j >= 0 {
			return head[:j+1] + "******" + dsn[i:]
		}
	}
	return dsn
}
