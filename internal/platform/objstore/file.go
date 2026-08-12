package objstore

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fileStore struct {
	base         string
	publicPrefix string
	ttl          time.Duration
	publicURL    string // 公共访问 URL 前缀
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
	return &fileStore{
		base:         c.BaseDir,
		publicPrefix: "/uploads/",
		ttl:          ttl,
		publicURL:    c.PublicURL,
	}, nil
}

func (s *fileStore) Put(_ context.Context, key string, r ReadSeeker, _ int64, _ string) error {
	key = sanitizeKey(key)
	rawPath := filepath.Join(s.base, filepath.FromSlash(key))

	// 确保路径在 base 目录内，防止路径遍历攻击
	safePath, err := s.validateAndCleanPath(rawPath)
	if err != nil {
		return err
	}

	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(safePath), 0o755); err != nil { // lgtm[path-injection] — safePath validated by validateAndCleanPath
		return err
	}

	f, err := os.Create(safePath) // lgtm[path-injection] — safePath validated by validateAndCleanPath
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

	// 如果配置了 PublicURL,使用 PublicURL
	if s.publicURL != "" {
		return strings.TrimRight(s.publicURL, "/") + "/" + key, nil
	}

	// 否则返回相对路径,由 HTTP 服务器的 /uploads/ 路径提供
	u := url.URL{Path: s.publicPrefix + key}
	return u.String(), nil
}

func (s *fileStore) Delete(_ context.Context, key string) error {
	key = sanitizeKey(key)

	// 如果是文件夹（以 / 结尾），需要递归删除
	if strings.HasSuffix(key, "/") {
		rawPath := filepath.Join(s.base, filepath.FromSlash(key))
		// 确保路径在 base 目录内，防止路径遍历攻击
		safePath, err := s.validateAndCleanPath(rawPath)
		if err != nil {
			return err
		}
		return os.RemoveAll(safePath) // lgtm[path-injection] — safePath validated by validateAndCleanPath
	}

	// 否则删除单个文件
	rawPath := filepath.Join(s.base, filepath.FromSlash(key))
	// 确保路径在 base 目录内，防止路径遍历攻击
	safePath, err := s.validateAndCleanPath(rawPath)
	if err != nil {
		return err
	}
	return os.Remove(safePath) // lgtm[path-injection] — safePath validated by validateAndCleanPath
}

func (s *fileStore) List(_ context.Context, prefix, marker, delimiter string, limit int) (ListResult, error) {
	result := ListResult{
		Objects:  make([]ObjectInfo, 0),
		Prefixes: make([]string, 0),
	}

	// Normalize both values before touching the filesystem. List used to join
	// the raw prefix directly, which let "../" escape the configured base.
	prefix = sanitizeKey(prefix)
	marker = sanitizeKey(marker)
	searchPath, err := s.validateAndCleanPath(filepath.Join(s.base, filepath.FromSlash(prefix)))
	if err != nil {
		return ListResult{}, err
	}

	// 遍历目录
	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过根目录本身
		if path == searchPath {
			return nil
		}

		// 跳过标记之前的文件（用于分页）
		if marker != "" {
			relPath, _ := filepath.Rel(s.base, path)
			if relPath <= marker {
				return nil
			}
		}

		// 检查是否达到限制
		if limit > 0 && len(result.Objects) >= limit {
			result.IsTruncated = true
			if len(result.Objects) > 0 {
				lastObj := result.Objects[len(result.Objects)-1]
				result.NextMarker = lastObj.Key
			}
			return fmt.Errorf("limit reached")
		}

		// 如果是目录且使用了分隔符，添加到前缀列表
		if info.IsDir() && delimiter != "" {
			relPath, _ := filepath.Rel(s.base, path)
			result.Prefixes = append(result.Prefixes, filepath.ToSlash(relPath)+delimiter)
			return filepath.SkipDir
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 获取相对路径作为键
		relPath, err := filepath.Rel(s.base, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(relPath)

		// 添加文件信息
		result.Objects = append(result.Objects, ObjectInfo{
			Key:          key,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			ETag:         info.ModTime().String(), // 文件系统没有 ETag，使用修改时间
		})

		return nil
	})

	if err != nil && err.Error() != "limit reached" {
		return ListResult{}, err
	}

	return result, nil
}

func (s *fileStore) CreatePrefix(_ context.Context, prefix string) error {
	prefix = sanitizeKey(prefix)
	// 确保以 / 结尾
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	rawPath := filepath.Join(s.base, filepath.FromSlash(prefix))
	// 确保路径在 base 目录内，防止路径遍历攻击
	safePath, err := s.validateAndCleanPath(rawPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(safePath, 0o755) // lgtm[path-injection] — safePath validated by validateAndCleanPath
}

func (s *fileStore) RenamePrefix(_ context.Context, oldPrefix, newPrefix string) error {
	oldPrefix = sanitizeKey(oldPrefix)
	newPrefix = sanitizeKey(newPrefix)

	if !strings.HasSuffix(oldPrefix, "/") {
		oldPrefix += "/"
	}
	if !strings.HasSuffix(newPrefix, "/") {
		newPrefix += "/"
	}

	oldRawPath := filepath.Join(s.base, filepath.FromSlash(oldPrefix))
	newRawPath := filepath.Join(s.base, filepath.FromSlash(newPrefix))

	// 确保路径在 base 目录内，防止路径遍历攻击
	oldSafePath, err := s.validateAndCleanPath(oldRawPath)
	if err != nil {
		return err
	}
	newSafePath, err := s.validateAndCleanPath(newRawPath)
	if err != nil {
		return err
	}

	// 使用系统 rename 命令移动目录
	return os.Rename(oldSafePath, newSafePath) // lgtm[path-injection] — both paths validated by validateAndCleanPath
}

// validateAndCleanPath ensures the path stays within the base directory and returns the cleaned path.
func (s *fileStore) validateAndCleanPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	absBase, err := filepath.Abs(s.base)
	if err != nil {
		return "", fmt.Errorf("invalid base path: %w", err)
	}
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return "", fmt.Errorf("path traversal detected: %s is outside base directory", path)
	}
	return absPath, nil
}

// validatePath ensures the path stays within the base directory to prevent path traversal attacks.
// Deprecated: Use validateAndCleanPath instead for CodeQL compatibility.
func (s *fileStore) validatePath(path string) error {
	_, err := s.validateAndCleanPath(path)
	return err
}
