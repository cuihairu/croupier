package objstore

import (
	"context"
	"fmt"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/s3blob"
	"io"
	"strings"
	"time"
)

type s3Store struct {
	bk        *blob.Bucket
	ttl       time.Duration
	publicURL string // 公共访问 URL 前缀
}

func OpenS3(ctx context.Context, c Config) (Store, error) {
	u := buildS3URL(c)
	bk, err := blob.OpenBucket(ctx, u)
	if err != nil {
		return nil, err
	}
	ttl := c.SignedURLTTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	return &s3Store{
		bk:        bk,
		ttl:       ttl,
		publicURL: c.PublicURL,
	}, nil
}

func (s *s3Store) Put(ctx context.Context, key string, r ReadSeeker, _ int64, contentType string) error {
	key = sanitizeKey(key)

	// 如果key包含路径，确保所有父目录都被创建（用于在List中正确显示）
	if strings.Contains(key, "/") {
		dir := key[:strings.LastIndex(key, "/")]
		if dir != "" {
			// 为每个目录级别创建标记对象
			parts := strings.Split(dir, "/")
			for i := range parts {
				prefix := strings.Join(parts[:i+1], "/") + "/"
				// 尝试创建目录标记，忽略错误（可能已存在）
				w, err := s.bk.NewWriter(ctx, prefix, &blob.WriterOptions{})
				if err == nil {
					w.Close()
				}
			}
		}
	}

	w, err := s.bk.NewWriter(ctx, key, &blob.WriterOptions{ContentType: contentType})
	if err != nil {
		return err
	}
	defer w.Close()
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return w.Close()
}

func (s *s3Store) SignedURL(ctx context.Context, key string, method string, expiry time.Duration) (string, error) {
	key = sanitizeKey(key)

	// 如果配置了 PublicURL,返回公共访问 URL (不签名)
	if s.publicURL != "" {
		return strings.TrimRight(s.publicURL, "/") + "/" + key, nil
	}

	// 否则返回签名 URL
	if expiry <= 0 {
		expiry = s.ttl
	}
	return s.bk.SignedURL(ctx, key, &blob.SignedURLOptions{Method: method, Expiry: expiry})
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	key = sanitizeKey(key)
	return s.bk.Delete(ctx, key)
}

func (s *s3Store) List(ctx context.Context, prefix, marker, delimiter string, limit int) (ListResult, error) {
	result := ListResult{
		Objects:  make([]ObjectInfo, 0),
		Prefixes: make([]string, 0),
	}

	// 构建 List 选项
	opts := &blob.ListOptions{
		Prefix:    prefix,
		Delimiter: delimiter,
	}

	// 使用 gocloud 的 List 方法
	iter := s.bk.List(opts)

	count := 0
	for {
		obj, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return ListResult{}, err
		}

		// 跳过标记之前的对象（用于分页）
		if marker != "" && obj.Key <= marker {
			continue
		}

		// 检查是否达到限制
		if limit > 0 && count >= limit {
			result.IsTruncated = true
			if len(result.Objects) > 0 {
				result.NextMarker = result.Objects[len(result.Objects)-1].Key
			}
			break
		}

		// 如果是前缀（目录），添加到前缀列表
		if obj.IsDir {
			result.Prefixes = append(result.Prefixes, obj.Key)
		} else {
			// 添加对象信息
			// 注意：gocloud blob.ListObject 没有 ETag 字段，我们使用 MD5 或留空
			result.Objects = append(result.Objects, ObjectInfo{
				Key:          obj.Key,
				Size:         obj.Size,
				LastModified: obj.ModTime,
				ETag:         "", // gocloud 不直接提供 ETag
			})
			count++
		}
	}

	return result, nil
}

func (s *s3Store) CreatePrefix(ctx context.Context, prefix string) error {
	prefix = sanitizeKey(prefix)
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	// S3 通过创建一个以 / 结尾的空对象来表示目录
	w, err := s.bk.NewWriter(ctx, prefix, &blob.WriterOptions{})
	if err != nil {
		return err
	}
	return w.Close()
}

func (s *s3Store) RenamePrefix(ctx context.Context, oldPrefix, newPrefix string) error {
	oldPrefix = sanitizeKey(oldPrefix)
	newPrefix = sanitizeKey(newPrefix)

	if !strings.HasSuffix(oldPrefix, "/") {
		oldPrefix += "/"
	}
	if !strings.HasSuffix(newPrefix, "/") {
		newPrefix += "/"
	}

	// 列出所有需要重命名的对象
	result, err := s.List(ctx, oldPrefix, "", "", 0)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	// 复制所有对象到新前缀
	for _, obj := range result.Objects {
		oldKey := obj.Key
		newKey := strings.Replace(oldKey, oldPrefix, newPrefix, 1)

		// 读取旧对象
		r, err := s.bk.NewReader(ctx, oldKey, nil)
		if err != nil {
			return fmt.Errorf("failed to read object %s: %w", oldKey, err)
		}

		// 写入新对象
		w, err := s.bk.NewWriter(ctx, newKey, &blob.WriterOptions{
			ContentType: r.ContentType(),
		})
		if err != nil {
			r.Close()
			return fmt.Errorf("failed to create writer for %s: %w", newKey, err)
		}

		_, err = io.Copy(w, r)
		r.Close()
		if err != nil {
			w.Close()
			return fmt.Errorf("failed to copy object %s to %s: %w", oldKey, newKey, err)
		}

		if err := w.Close(); err != nil {
			return fmt.Errorf("failed to write object %s: %w", newKey, err)
		}

		// 删除旧对象
		if err := s.Delete(ctx, oldKey); err != nil {
			return fmt.Errorf("failed to delete old object %s: %w", oldKey, err)
		}
	}

	return nil
}
