package objstore

import (
	"context"
	"fmt"
	oss "github.com/aliyun/aliyun-oss-go-sdk/oss"
	"strings"
	"time"
)

type ossStore struct {
	bk        *oss.Bucket
	ttl       time.Duration
	publicURL string // 公共访问 URL 前缀
}

func OpenOSS(_ context.Context, c Config) (Store, error) {
	cli, err := oss.New(c.Endpoint, c.AccessKey, c.SecretKey)
	if err != nil {
		return nil, err
	}
	bk, err := cli.Bucket(c.Bucket)
	if err != nil {
		return nil, err
	}
	ttl := c.SignedURLTTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	return &ossStore{
		bk:        bk,
		ttl:       ttl,
		publicURL: c.PublicURL,
	}, nil
}

func (s *ossStore) Put(_ context.Context, key string, r ReadSeeker, _ int64, contentType string) error {
	key = sanitizeKey(key)

	// 如果key包含路径，确保所有父目录都被创建（用于在List中正确显示）
	if strings.Contains(key, "/") {
		dir := key[:strings.LastIndex(key, "/")]
		if dir != "" {
			// 为每个目录级别创建标记对象
			parts := strings.Split(dir, "/")
			for i := range parts {
				prefix := strings.Join(parts[:i+1], "/") + "/"
				// 尝试创建目录标记（如果已存在会忽略错误）
				s.bk.PutObject(prefix, strings.NewReader(""))
			}
		}
	}

	opts := []oss.Option{}
	if contentType != "" {
		opts = append(opts, oss.ContentType(contentType))
	}
	return s.bk.PutObject(key, r, opts...)
}

func (s *ossStore) SignedURL(_ context.Context, key string, method string, expiry time.Duration) (string, error) {
	key = sanitizeKey(key)

	// 如果配置了 PublicURL,返回公共访问 URL (不签名)
	if s.publicURL != "" {
		return strings.TrimRight(s.publicURL, "/") + "/" + key, nil
	}

	// 否则返回签名 URL
	if expiry <= 0 {
		expiry = s.ttl
	}
	sec := int64(expiry / time.Second)
	var httpMethod oss.HTTPMethod
	switch method {
	case "PUT":
		httpMethod = oss.HTTPPut
	case "DELETE":
		httpMethod = oss.HTTPDelete
	case "GET", "":
		httpMethod = oss.HTTPGet
	default:
		return "", fmt.Errorf("unsupported method: %s", method)
	}
	return s.bk.SignURL(key, httpMethod, sec)
}

func (s *ossStore) Delete(_ context.Context, key string) error {
	key = sanitizeKey(key)

	// 如果是文件夹（以 / 结尾），需要递归删除所有对象
	if strings.HasSuffix(key, "/") {
		// 列出所有以该前缀开头的对象
		lor, err := s.bk.ListObjects(oss.Prefix(key), oss.MaxKeys(1000))
		if err != nil {
			return err
		}

		// 删除所有对象
		for _, obj := range lor.Objects {
			// 跳过文件夹标记本身
			if obj.Key == key {
				continue
			}

			if err := s.bk.DeleteObject(obj.Key); err != nil {
				return err
			}
		}
	}

	// 删除文件夹标记本身或单个文件
	return s.bk.DeleteObject(key)
}

func (s *ossStore) List(_ context.Context, prefix, marker, delimiter string, limit int) (ListResult, error) {
	result := ListResult{
		Objects:  make([]ObjectInfo, 0),
		Prefixes: make([]string, 0),
	}

	// 构建 List 选项
	opts := []oss.Option{
		oss.Prefix(prefix),
		oss.Marker(marker),
		oss.MaxKeys(limit),
	}
	if delimiter != "" {
		opts = append(opts, oss.Delimiter(delimiter))
	}

	// 调用 OSS ListObjects
	lor, err := s.bk.ListObjects(opts...)
	if err != nil {
		return ListResult{}, err
	}

	// 处理前缀（目录）
	for _, prefix := range lor.CommonPrefixes {
		result.Prefixes = append(result.Prefixes, prefix)
	}

	// 处理对象
	for _, obj := range lor.Objects {
		result.Objects = append(result.Objects, ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
		})
	}

	// 设置分页信息
	result.IsTruncated = lor.IsTruncated
	result.NextMarker = lor.NextMarker

	return result, nil
}

func (s *ossStore) CreatePrefix(_ context.Context, prefix string) error {
	prefix = sanitizeKey(prefix)
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	// OSS 通过创建一个以 / 结尾的空对象来表示目录
	return s.bk.PutObject(prefix, strings.NewReader(""))
}

func (s *ossStore) RenamePrefix(_ context.Context, oldPrefix, newPrefix string) error {
	oldPrefix = sanitizeKey(oldPrefix)
	newPrefix = sanitizeKey(newPrefix)

	if !strings.HasSuffix(oldPrefix, "/") {
		oldPrefix += "/"
	}
	if !strings.HasSuffix(newPrefix, "/") {
		newPrefix += "/"
	}

	// 列出所有需要重命名的对象
	result, err := s.List(context.Background(), oldPrefix, "", "", 0)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	// 复制所有对象到新前缀
	for _, obj := range result.Objects {
		oldKey := obj.Key
		newKey := strings.Replace(oldKey, oldPrefix, newPrefix, 1)

		// 使用 OSS 的 CopyObject 方法
		_, err := s.bk.CopyObject(oldKey, newKey)
		if err != nil {
			return fmt.Errorf("failed to copy object %s to %s: %w", oldKey, newKey, err)
		}

		// 删除旧对象
		if err := s.Delete(context.Background(), oldKey); err != nil {
			return fmt.Errorf("failed to delete old object %s: %w", oldKey, err)
		}
	}

	return nil
}
