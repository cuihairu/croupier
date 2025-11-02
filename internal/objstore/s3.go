package objstore

import (
    "context"
    "time"
    "io"
    "gocloud.dev/blob"
    _ "gocloud.dev/blob/s3blob"
)

type s3Store struct { bk *blob.Bucket; ttl time.Duration }

func OpenS3(ctx context.Context, c Config) (Store, error) {
    u := buildS3URL(c)
    bk, err := blob.OpenBucket(ctx, u)
    if err != nil { return nil, err }
    ttl := c.SignedURLTTL
    if ttl == 0 { ttl = 15 * time.Minute }
    return &s3Store{bk: bk, ttl: ttl}, nil
}

func (s *s3Store) Put(ctx context.Context, key string, r ReadSeeker, _ int64, contentType string) error {
    key = sanitizeKey(key)
    w, err := s.bk.NewWriter(ctx, key, &blob.WriterOptions{ContentType: contentType})
    if err != nil { return err }
    defer w.Close()
    if _, err := io.Copy(w, r); err != nil { return err }
    return w.Close()
}

func (s *s3Store) SignedURL(ctx context.Context, key string, method string, expiry time.Duration) (string, error) {
    key = sanitizeKey(key)
    if expiry <= 0 { expiry = s.ttl }
    return s.bk.SignedURL(ctx, key, &blob.SignedURLOptions{Method: method, Expiry: expiry})
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
    key = sanitizeKey(key)
    return s.bk.Delete(ctx, key)
}
