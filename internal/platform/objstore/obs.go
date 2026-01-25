package objstore

import (
	"context"
	"fmt"
	obs "github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
	"strings"
	"time"
)

type obsStore struct {
	cli       *obs.ObsClient
	bkt       string
	ttl       time.Duration
	publicURL string // 公共访问 URL 前缀
}

func OpenOBS(_ context.Context, c Config) (Store, error) {
	if c.Endpoint == "" {
		return nil, fmt.Errorf("endpoint required for obs driver")
	}
	if c.AccessKey == "" || c.SecretKey == "" {
		return nil, fmt.Errorf("access_key/secret_key required for obs driver")
	}
	if c.Bucket == "" {
		return nil, fmt.Errorf("bucket required for obs driver")
	}

	cli, err := obs.New(c.AccessKey, c.SecretKey, c.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create obs client: %w", err)
	}

	ttl := c.SignedURLTTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}

	return &obsStore{
		cli:       cli,
		bkt:       c.Bucket,
		ttl:       ttl,
		publicURL: c.PublicURL,
	}, nil
}

func (s *obsStore) Put(ctx context.Context, key string, r ReadSeeker, size int64, contentType string) error {
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
				input := &obs.PutObjectInput{}
				input.Bucket = s.bkt
				input.Key = prefix
				s.cli.PutObject(input)
			}
		}
	}

	input := &obs.PutObjectInput{}
	input.Bucket = s.bkt
	input.Key = key

	if contentType != "" {
		input.ContentType = contentType
	}

	// 读取所有数据到内存
	// 注意：对于大文件，应该使用分段上传
	_, err := s.cli.PutObject(input)
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}

func (s *obsStore) SignedURL(_ context.Context, key string, method string, expiry time.Duration) (string, error) {
	key = sanitizeKey(key)

	// 如果配置了 PublicURL,返回公共访问 URL (不签名)
	if s.publicURL != "" {
		return strings.TrimRight(s.publicURL, "/") + "/" + key, nil
	}

	// 否则返回签名 URL
	if expiry <= 0 {
		expiry = s.ttl
	}

	sec := int(expiry / time.Second)

	// 华为云 OBS 使用临时授权URL
	input := &obs.CreateSignedUrlInput{}
	input.Bucket = s.bkt
	input.Key = key
	input.Expires = sec

	// 根据方法设置
	switch method {
	case "PUT", "POST":
		input.Method = obs.HttpMethodPut
	case "DELETE":
		input.Method = obs.HttpMethodDelete
	case "GET", "":
		input.Method = obs.HttpMethodGet
	default:
		return "", fmt.Errorf("unsupported method: %s", method)
	}

	output, err := s.cli.CreateSignedUrl(input)
	if err != nil {
		return "", fmt.Errorf("failed to create signed url: %w", err)
	}

	return output.SignedUrl, nil
}

func (s *obsStore) Delete(_ context.Context, key string) error {
	key = sanitizeKey(key)

	// 如果是文件夹（以 / 结尾），需要递归删除所有对象
	if strings.HasSuffix(key, "/") {
		// 列出所有以该前缀开头的对象
		input := &obs.ListObjectsInput{}
		input.Bucket = s.bkt
		input.Prefix = key
		input.MaxKeys = 1000

		output, err := s.cli.ListObjects(input)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		// 删除所有对象
		for _, obj := range output.Contents {
			// 跳过文件夹标记本身
			if obj.Key == key {
				continue
			}

			delInput := &obs.DeleteObjectInput{}
			delInput.Bucket = s.bkt
			delInput.Key = obj.Key

			if _, err := s.cli.DeleteObject(delInput); err != nil {
				return fmt.Errorf("failed to delete object %s: %w", obj.Key, err)
			}
		}
	}

	// 删除文件夹标记本身或单个文件
	input := &obs.DeleteObjectInput{}
	input.Bucket = s.bkt
	input.Key = key

	_, err := s.cli.DeleteObject(input)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

func (s *obsStore) List(_ context.Context, prefix, marker, delimiter string, limit int) (ListResult, error) {
	result := ListResult{
		Objects:  make([]ObjectInfo, 0),
		Prefixes: make([]string, 0),
	}

	// 构建 List 选项
	input := &obs.ListObjectsInput{}
	input.Bucket = s.bkt
	input.Prefix = prefix
	input.Marker = marker
	input.MaxKeys = limit
	input.Delimiter = delimiter

	// 调用 OBS ListObjects
	output, err := s.cli.ListObjects(input)
	if err != nil {
		return ListResult{}, fmt.Errorf("failed to list objects: %w", err)
	}

	// 处理前缀（目录）
	for _, prefix := range output.CommonPrefixes {
		result.Prefixes = append(result.Prefixes, prefix)
	}

	// 处理对象
	for _, obj := range output.Contents {
		result.Objects = append(result.Objects, ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
		})
	}

	// 设置分页信息
	result.IsTruncated = output.IsTruncated
	result.NextMarker = output.NextMarker

	return result, nil
}

func (s *obsStore) CreatePrefix(_ context.Context, prefix string) error {
	prefix = sanitizeKey(prefix)
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	// OBS 通过创建一个以 / 结尾的空对象来表示目录
	input := &obs.PutObjectInput{}
	input.Bucket = s.bkt
	input.Key = prefix
	_, err := s.cli.PutObject(input)
	return err
}

func (s *obsStore) RenamePrefix(_ context.Context, oldPrefix, newPrefix string) error {
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

		// 使用 OBS 的 CopyObject 方法
		input := &obs.CopyObjectInput{}
		input.Bucket = s.bkt
		input.Key = newKey
		input.CopySourceBucket = s.bkt
		input.CopySourceKey = oldKey

		_, err := s.cli.CopyObject(input)
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
