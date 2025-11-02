package objstore

import (
    "context"
    "time"
    "gocloud.dev/blob"
    _ "gocloud.dev/blob/s3blob"
)

type s3Store struct { bk *blob.Bucket; ttl time.Duration }

func openS3(ctx context.Context, c Config) (Store, error) {
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
    buf := make([]byte, 32*1024)
    for {
        n, er := r.Read(buf)
        if n > 0 { if _, ew := w.Write(buf[:n]); ew != nil { return ew } }
        if er != nil { if er.Error() == "EOF" { break } else { break } }
        if n == 0 { break }
    }
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

