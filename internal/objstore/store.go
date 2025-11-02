package objstore

import (
    "context"
    "errors"
    "fmt"
    "net/url"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

type Store interface {
    Put(ctx context.Context, key string, r ReadSeeker, size int64, contentType string) error
    SignedURL(ctx context.Context, key string, method string, expiry time.Duration) (string, error)
    Delete(ctx context.Context, key string) error
}

// ReadSeeker abstracts the subset we need to stream into drivers.
type ReadSeeker interface {
    Read(p []byte) (int, error)
    Seek(offset int64, whence int) (int64, error)
}

type Config struct {
    Driver         string
    Bucket         string
    Region         string
    Endpoint       string
    AccessKey      string
    SecretKey      string
    ForcePathStyle bool
    BaseDir        string
    SignedURLTTL   time.Duration
}

func FromEnv() Config {
    c := Config{
        Driver:   os.Getenv("STORAGE_DRIVER"),
        Bucket:   os.Getenv("STORAGE_BUCKET"),
        Region:   os.Getenv("STORAGE_REGION"),
        Endpoint: os.Getenv("STORAGE_ENDPOINT"),
        AccessKey: os.Getenv("STORAGE_ACCESS_KEY"),
        SecretKey: os.Getenv("STORAGE_SECRET_KEY"),
        BaseDir:  os.Getenv("STORAGE_BASE_DIR"),
    }
    if v := strings.ToLower(os.Getenv("STORAGE_FORCE_PATH_STYLE")); v == "true" || v == "1" || v == "yes" { c.ForcePathStyle = true }
    if v := os.Getenv("STORAGE_SIGNED_URL_TTL"); v != "" {
        if d, err := time.ParseDuration(v); err == nil { c.SignedURLTTL = d }
    }
    return c
}

func Validate(c Config) error {
    switch strings.ToLower(c.Driver) {
    case "s3":
        if c.Bucket == "" { return errors.New("bucket required for s3 driver") }
        // credentials via env (AWS_ACCESS_KEY_ID/SECRET) or IAM; we don't enforce here
    case "oss":
        if c.Bucket == "" { return errors.New("bucket required for oss driver") }
        if c.Endpoint == "" { return errors.New("endpoint required for oss driver") }
        if c.AccessKey == "" || c.SecretKey == "" { return errors.New("access_key/secret_key required for oss driver") }
    case "cos":
        if c.Bucket == "" { return errors.New("bucket required for cos driver") }
        if c.Region == "" && c.Endpoint == "" { return errors.New("region or endpoint required for cos driver") }
        if c.AccessKey == "" || c.SecretKey == "" { return errors.New("access_key/secret_key required for cos driver") }
    case "file":
        if c.BaseDir == "" { return errors.New("base_dir required for file driver") }
        if err := os.MkdirAll(c.BaseDir, 0o755); err != nil { return fmt.Errorf("ensure base_dir: %w", err) }
    case "":
        return errors.New("STORAGE_DRIVER not set")
    default:
        return fmt.Errorf("unknown storage driver: %s", c.Driver)
    }
    return nil
}

// sanitizeKey prevents path traversal.
func sanitizeKey(key string) string {
    key = filepath.ToSlash(key)
    key = strings.TrimLeft(key, "/")
    parts := strings.Split(key, "/")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        if p == "" || p == "." || p == ".." { continue }
        out = append(out, p)
    }
    return strings.Join(out, "/")
}

// buildS3URL constructs a gocloud s3 URL with query params.
func buildS3URL(c Config) string {
    u := url.URL{Scheme: "s3", Host: c.Bucket}
    q := url.Values{}
    if c.Region != "" { q.Set("region", c.Region) }
    if c.Endpoint != "" { q.Set("endpoint", c.Endpoint) }
    if c.ForcePathStyle { q.Set("s3ForcePathStyle", "true") }
    u.RawQuery = q.Encode()
    return u.String()
}
