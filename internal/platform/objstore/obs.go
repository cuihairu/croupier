package objstore

import (
	"context"
	"fmt"
	obs "github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
	"time"
)

type obsStore struct {
	cli *obs.ObsClient
	bkt string
	ttl time.Duration
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
		cli: cli,
		bkt: c.Bucket,
		ttl: ttl,
	}, nil
}

func (s *obsStore) Put(ctx context.Context, key string, r ReadSeeker, size int64, contentType string) error {
	key = sanitizeKey(key)

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

	input := &obs.DeleteObjectInput{}
	input.Bucket = s.bkt
	input.Key = key

	_, err := s.cli.DeleteObject(input)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}
