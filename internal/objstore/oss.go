package objstore

import (
	"context"
	"fmt"
	oss "github.com/aliyun/aliyun-oss-go-sdk/oss"
	"time"
)

type ossStore struct {
	bk  *oss.Bucket
	ttl time.Duration
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
	return &ossStore{bk: bk, ttl: ttl}, nil
}

func (s *ossStore) Put(_ context.Context, key string, r ReadSeeker, _ int64, contentType string) error {
	key = sanitizeKey(key)
	opts := []oss.Option{}
	if contentType != "" {
		opts = append(opts, oss.ContentType(contentType))
	}
	return s.bk.PutObject(key, r, opts...)
}

func (s *ossStore) SignedURL(_ context.Context, key string, method string, expiry time.Duration) (string, error) {
	key = sanitizeKey(key)
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
	return s.bk.DeleteObject(key)
}
