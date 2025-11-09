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
	cli *cos.Client
	ttl time.Duration
	sid string
	sk  string
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
	return &cosStore{cli: cli, ttl: ttl, sid: c.AccessKey, sk: c.SecretKey}, nil
}

func (s *cosStore) Put(ctx context.Context, key string, r ReadSeeker, _ int64, contentType string) error {
	key = sanitizeKey(key)
	opt := &cos.ObjectPutOptions{}
	if contentType != "" {
		opt.ObjectPutHeaderOptions = &cos.ObjectPutHeaderOptions{ContentType: contentType}
	}
	_, err := s.cli.Object.Put(ctx, key, r, opt)
	return err
}

func (s *cosStore) SignedURL(ctx context.Context, key string, method string, expiry time.Duration) (string, error) {
	key = sanitizeKey(key)
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
	_, err := s.cli.Object.Delete(ctx, key)
	return err
}
