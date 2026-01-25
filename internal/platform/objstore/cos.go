package objstore

import (
	"context"
	"fmt"
	cos "github.com/tencentyun/cos-go-sdk-v5"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type cosStore struct {
	cli       *cos.Client
	ttl       time.Duration
	sid       string
	sk        string
	publicURL string // 公共访问 URL 前缀
}

func OpenCOS(_ context.Context, c Config) (Store, error) {
	// build bucket URL
	var bucketURL *url.URL
	if c.Endpoint != "" {
		u, err := url.Parse(c.Endpoint)
		if err != nil {
			return nil, err
		}
		// if host not contains bucket, use path-style
		if !strings.Contains(u.Host, c.Bucket) {
			if !strings.HasSuffix(u.Path, "/"+c.Bucket) {
				u.Path = "/" + c.Bucket
			}
		}
		bucketURL = u
	} else {
		if c.Region == "" {
			return nil, fmt.Errorf("region required for cos when endpoint empty")
		}
		u, _ := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", c.Bucket, c.Region))
		bucketURL = u
	}
	b := &cos.BaseURL{BucketURL: bucketURL}
	cli := cos.NewClient(b, &http.Client{Transport: &cos.AuthorizationTransport{SecretID: c.AccessKey, SecretKey: c.SecretKey}})
	ttl := c.SignedURLTTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	return &cosStore{
		cli:       cli,
		ttl:       ttl,
		sid:       c.AccessKey,
		sk:        c.SecretKey,
		publicURL: c.PublicURL,
	}, nil
}

func (s *cosStore) Put(ctx context.Context, key string, r ReadSeeker, _ int64, contentType string) error {
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
				s.cli.Object.Put(ctx, prefix, strings.NewReader(""), nil)
			}
		}
	}

	opt := &cos.ObjectPutOptions{}
	if contentType != "" {
		opt.ObjectPutHeaderOptions = &cos.ObjectPutHeaderOptions{ContentType: contentType}
	}
	_, err := s.cli.Object.Put(ctx, key, r, opt)
	return err
}

func (s *cosStore) SignedURL(ctx context.Context, key string, method string, expiry time.Duration) (string, error) {
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
	m := http.MethodGet
	switch strings.ToUpper(method) {
	case http.MethodPut:
		m = http.MethodPut
	case http.MethodDelete:
		m = http.MethodDelete
	case http.MethodGet:
		fallthrough
	default:
		m = http.MethodGet
	}
	u, err := s.cli.Object.GetPresignedURL(ctx, m, key, s.sid, s.sk, time.Duration(sec)*time.Second, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *cosStore) Delete(ctx context.Context, key string) error {
	key = sanitizeKey(key)

	// 如果是文件夹（以 / 结尾），需要递归删除所有对象
	if strings.HasSuffix(key, "/") {
		// 列出所有以该前缀开头的对象
		resp, _, err := s.cli.Bucket.Get(ctx, &cos.BucketGetOptions{
			Prefix:  key,
			MaxKeys: 1000,
		})
		if err != nil {
			return err
		}

		// 删除所有对象
		for _, obj := range resp.Contents {
			// 跳过文件夹标记本身
			if obj.Key == key {
				continue
			}

			if _, err := s.cli.Object.Delete(ctx, obj.Key); err != nil {
				return err
			}
		}
	}

	// 删除文件夹标记本身或单个文件
	_, err := s.cli.Object.Delete(ctx, key)
	return err
}

func (s *cosStore) List(ctx context.Context, prefix, marker, delimiter string, limit int) (ListResult, error) {
	result := ListResult{
		Objects:  make([]ObjectInfo, 0),
		Prefixes: make([]string, 0),
	}

	// 构建 List 选项
	opts := &cos.BucketGetOptions{
		Prefix:    prefix,
		Marker:    marker,
		MaxKeys:   limit,
		Delimiter: delimiter,
	}

	// 调用 COS GetBucket
	resp, _, err := s.cli.Bucket.Get(ctx, opts)
	if err != nil {
		return ListResult{}, err
	}

	// 处理前缀（目录）
	for _, commonPrefix := range resp.CommonPrefixes {
		result.Prefixes = append(result.Prefixes, commonPrefix)
	}

	// 处理对象
	for _, obj := range resp.Contents {
		// 解析时间
		modTime, err := time.Parse(time.RFC3339, obj.LastModified)
		if err != nil {
			continue // 跳过时间解析失败的对象
		}

		result.Objects = append(result.Objects, ObjectInfo{
			Key:          obj.Key,
			Size:         int64(obj.Size), // COS 返回的大小是 int64
			LastModified: modTime,
			ETag:         strings.Trim(obj.ETag, "\""), // COS ETag 包含引号，需要去除
		})
	}

	// 设置分页信息
	result.IsTruncated = resp.IsTruncated
	result.NextMarker = resp.NextMarker

	return result, nil
}

func (s *cosStore) CreatePrefix(ctx context.Context, prefix string) error {
	prefix = sanitizeKey(prefix)
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	// COS 通过创建一个以 / 结尾的空对象来表示目录
	_, err := s.cli.Object.Put(ctx, prefix, strings.NewReader(""), nil)
	return err
}

func (s *cosStore) RenamePrefix(_ context.Context, oldPrefix, newPrefix string) error {
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
	ctx := context.Background()
	for _, obj := range result.Objects {
		oldKey := obj.Key
		newKey := strings.Replace(oldKey, oldPrefix, newPrefix, 1)

		// 使用 COS 的 Copy 方法
		// sourceURL 格式: <bucket-name>.cos.<region>.myqcloud.com/<object-key>
		_, _, err := s.cli.Object.Copy(ctx, oldKey, newKey, nil)
		if err != nil {
			return fmt.Errorf("failed to copy object %s to %s: %w", oldKey, newKey, err)
		}

		// 删除旧对象
		if err := s.Delete(ctx, oldKey); err != nil {
			return fmt.Errorf("failed to delete old object %s: %w", oldKey, err)
		}
	}

	return nil
}
