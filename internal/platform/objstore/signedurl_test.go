package objstore

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestS3Store_SignedURL_PublicURLPath tests S3 SignedURL with public URL
func TestS3Store_SignedURL_PublicURLPath(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		key       string
		expected  string
	}{
		{
			name:      "basic public URL",
			publicURL: "https://cdn.example.com",
			key:       "path/to/file.txt",
			expected:  "https://cdn.example.com/path/to/file.txt",
		},
		{
			name:      "public URL with trailing slash",
			publicURL: "https://cdn.example.com/",
			key:       "path/to/file.txt",
			expected:  "https://cdn.example.com/path/to/file.txt",
		},
		{
			name:      "public URL with base path",
			publicURL: "https://cdn.example.com/files",
			key:       "path/to/file.txt",
			expected:  "https://cdn.example.com/files/path/to/file.txt",
		},
		{
			name:      "root key",
			publicURL: "https://cdn.example.com",
			key:       "file.txt",
			expected:  "https://cdn.example.com/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &s3Store{
				ttl:       15 * time.Minute,
				publicURL: tt.publicURL,
			}

			url, err := store.SignedURL(context.Background(), tt.key, "GET", time.Hour)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if url != tt.expected {
				t.Errorf("SignedURL() = %q, want %q", url, tt.expected)
			}
		})
	}
}

// TestOSSStore_SignedURL_PublicURLPath tests OSS SignedURL with public URL
func TestOSSStore_SignedURL_PublicURLPath(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		key       string
		expected  string
	}{
		{
			name:      "basic public URL",
			publicURL: "https://cdn.example.com",
			key:       "uploads/file.pdf",
			expected:  "https://cdn.example.com/uploads/file.pdf",
		},
		{
			name:      "public URL with trailing slash",
			publicURL: "https://cdn.example.com/",
			key:       "uploads/file.pdf",
			expected:  "https://cdn.example.com/uploads/file.pdf",
		},
		{
			name:      "public URL with path",
			publicURL: "https://cdn.example.com/static",
			key:       "uploads/file.pdf",
			expected:  "https://cdn.example.com/static/uploads/file.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &ossStore{
				ttl:       15 * time.Minute,
				publicURL: tt.publicURL,
			}

			url, err := store.SignedURL(context.Background(), tt.key, "GET", time.Hour)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if url != tt.expected {
				t.Errorf("SignedURL() = %q, want %q", url, tt.expected)
			}
		})
	}
}

// TestCOSStore_SignedURL_PublicURLPath tests COS SignedURL with public URL
func TestCOSStore_SignedURL_PublicURLPath(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		key       string
		expected  string
	}{
		{
			name:      "CDN URL",
			publicURL: "https://cos.example.com",
			key:       "files/image.jpg",
			expected:  "https://cos.example.com/files/image.jpg",
		},
		{
			name:      "CDN URL with path",
			publicURL: "https://cos.example.com/assets",
			key:       "files/image.jpg",
			expected:  "https://cos.example.com/assets/files/image.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &cosStore{
				ttl:       15 * time.Minute,
				publicURL: tt.publicURL,
			}

			url, err := store.SignedURL(context.Background(), tt.key, "GET", time.Hour)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if url != tt.expected {
				t.Errorf("SignedURL() = %q, want %q", url, tt.expected)
			}
		})
	}
}

// TestOBSStore_SignedURL_PublicURLPath tests OBS SignedURL with public URL
func TestOBSStore_SignedURL_PublicURLPath(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		key       string
		expected  string
	}{
		{
			name:      "basic public URL",
			publicURL: "https://obs.example.com",
			key:       "data/file.bin",
			expected:  "https://obs.example.com/data/file.bin",
		},
		{
			name:      "public URL with trailing slash",
			publicURL: "https://obs.example.com/",
			key:       "data/file.bin",
			expected:  "https://obs.example.com/data/file.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &obsStore{
				ttl:       15 * time.Minute,
				publicURL: tt.publicURL,
			}

			url, err := store.SignedURL(context.Background(), tt.key, "GET", time.Hour)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if url != tt.expected {
				t.Errorf("SignedURL() = %q, want %q", url, tt.expected)
			}
		})
	}
}

// TestSignedURL_KeySanitizationAcrossStores tests key sanitization in SignedURL
func TestSignedURL_KeySanitizationAcrossStores(t *testing.T) {
	tests := []struct {
		name     string
		inputKey string
		contains string
	}{
		{
			name:     "with leading slash",
			inputKey: "/leading/file.txt",
			contains: "leading/file.txt",
		},
		{
			name:     "with dot segments",
			inputKey: "path/../cleaned/file.txt",
			contains: "cleaned/file.txt",
		},
		{
			name:     "with multiple slashes",
			inputKey: "path///to/file.txt",
			contains: "path/to/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run("S3_"+tt.name, func(t *testing.T) {
			store := &s3Store{
				ttl:       15 * time.Minute,
				publicURL: "https://cdn.example.com",
			}

			url, err := store.SignedURL(context.Background(), tt.inputKey, "GET", time.Hour)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if !strings.Contains(url, tt.contains) {
				t.Errorf("URL %q should contain %q", url, tt.contains)
			}
		})

		t.Run("OSS_"+tt.name, func(t *testing.T) {
			store := &ossStore{
				ttl:       15 * time.Minute,
				publicURL: "https://cdn.example.com",
			}

			url, err := store.SignedURL(context.Background(), tt.inputKey, "GET", time.Hour)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if !strings.Contains(url, tt.contains) {
				t.Errorf("URL %q should contain %q", url, tt.contains)
			}
		})

		t.Run("COS_"+tt.name, func(t *testing.T) {
			store := &cosStore{
				ttl:       15 * time.Minute,
				publicURL: "https://cdn.example.com",
			}

			url, err := store.SignedURL(context.Background(), tt.inputKey, "GET", time.Hour)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if !strings.Contains(url, tt.contains) {
				t.Errorf("URL %q should contain %q", url, tt.contains)
			}
		})

		t.Run("OBS_"+tt.name, func(t *testing.T) {
			store := &obsStore{
				ttl:       15 * time.Minute,
				publicURL: "https://cdn.example.com",
			}

			url, err := store.SignedURL(context.Background(), tt.inputKey, "GET", time.Hour)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if !strings.Contains(url, tt.contains) {
				t.Errorf("URL %q should contain %q", url, tt.contains)
			}
		})
	}
}

// TestSignedURL_ExpiryVariations tests expiry parameter handling
func TestSignedURL_ExpiryVariations(t *testing.T) {
	tests := []struct {
		name   string
		expiry time.Duration
	}{
		{name: "zero expiry", expiry: 0},
		{name: "1 second", expiry: 1 * time.Second},
		{name: "1 minute", expiry: 1 * time.Minute},
		{name: "1 hour", expiry: 1 * time.Hour},
		{name: "negative", expiry: -1 * time.Second},
	}

	for _, tt := range tests {
		t.Run("S3_"+tt.name, func(t *testing.T) {
			store := &s3Store{
				ttl:       15 * time.Minute,
				publicURL: "https://cdn.example.com",
			}

			url, err := store.SignedURL(context.Background(), "file.txt", "GET", tt.expiry)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if !strings.Contains(url, "file.txt") {
				t.Errorf("URL %q should contain 'file.txt'", url)
			}
		})

		t.Run("OSS_"+tt.name, func(t *testing.T) {
			store := &ossStore{
				ttl:       15 * time.Minute,
				publicURL: "https://cdn.example.com",
			}

			url, err := store.SignedURL(context.Background(), "file.txt", "GET", tt.expiry)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if !strings.Contains(url, "file.txt") {
				t.Errorf("URL %q should contain 'file.txt'", url)
			}
		})

		t.Run("COS_"+tt.name, func(t *testing.T) {
			store := &cosStore{
				ttl:       15 * time.Minute,
				publicURL: "https://cdn.example.com",
			}

			url, err := store.SignedURL(context.Background(), "file.txt", "GET", tt.expiry)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if !strings.Contains(url, "file.txt") {
				t.Errorf("URL %q should contain 'file.txt'", url)
			}
		})

		t.Run("OBS_"+tt.name, func(t *testing.T) {
			store := &obsStore{
				ttl:       15 * time.Minute,
				publicURL: "https://cdn.example.com",
			}

			url, err := store.SignedURL(context.Background(), "file.txt", "GET", tt.expiry)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			if !strings.Contains(url, "file.txt") {
				t.Errorf("URL %q should contain 'file.txt'", url)
			}
		})
	}
}

// TestSignedURL_EmptyKey tests empty key handling
func TestSignedURL_EmptyKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "empty key", key: ""},
		{name: "single slash", key: "/"},
		{name: "multiple slashes", key: "///"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &s3Store{
				ttl:       15 * time.Minute,
				publicURL: "https://cdn.example.com",
			}

			url, err := store.SignedURL(context.Background(), tt.key, "GET", time.Hour)
			if err != nil {
				t.Fatalf("SignedURL() error = %v", err)
			}

			// Empty key results in URL ending with /
			if !strings.HasSuffix(url, "/") && tt.key == "" {
				t.Errorf("URL for empty key should end with /, got %q", url)
			}
		})
	}
}

// TestSignedURL_ContextParameter tests context is accepted
func TestSignedURL_ContextParameter(t *testing.T) {
	ctx := context.Background()
	store := &s3Store{
		ttl:       15 * time.Minute,
		publicURL: "https://cdn.example.com",
	}

	url, err := store.SignedURL(ctx, "file.txt", "GET", time.Hour)
	if err != nil {
		t.Fatalf("SignedURL() error = %v", err)
	}

	if !strings.Contains(url, "file.txt") {
		t.Errorf("URL %q should contain 'file.txt'", url)
	}
}
