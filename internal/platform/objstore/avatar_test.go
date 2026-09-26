package objstore

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeAvatarKey_AcceptsKeyRelativeAndAbsoluteURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"裸 key 原样保留", "avatars/123-abc.png", "avatars/123-abc.png"},
		{"file 驱动的相对路径剥掉 /uploads 前缀", "/uploads/avatars/123-abc.png", "avatars/123-abc.png"},
		{"无前导斜杠的相对路径", "uploads/avatars/123-abc.png", "avatars/123-abc.png"},
		{"带查询串的 URL 去掉 query", "https://cdn.example.com/avatars/123-abc.png?v=2", "avatars/123-abc.png"},
		{"带 fragment 的 URL 去掉 fragment", "https://cdn.example.com/avatars/123-abc.png#x", "avatars/123-abc.png"},
		{"带自定义 CDN 前缀取最后一段", "https://cdn.example.com/my-assets/avatars/123-abc.png", "avatars/123-abc.png"},
		{"http 亦可", "http://example.com/avatars/a.png", "avatars/a.png"},
		{"首尾空白被裁掉", "  avatars/a.png  ", "avatars/a.png"},
		{"内部多余斜杠被归一", "avatars//123.png", "avatars/123.png"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeAvatarKey(tc.input)
			if err != nil {
				t.Fatalf("NormalizeAvatarKey(%q) 返回错误: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeAvatarKey(%q) = %q, 期望 %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeAvatarKey_EmptyMeansCleared(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"", "   ", "\t\n"} {
		got, err := NormalizeAvatarKey(input)
		if err != nil {
			t.Fatalf("NormalizeAvatarKey(%q) 不该报错: %v", input, err)
		}
		if got != "" {
			t.Fatalf("NormalizeAvatarKey(%q) = %q, 期望空串（表示清空）", input, got)
		}
	}
}

func TestNormalizeAvatarKey_RejectsBadInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
	}{
		{"非 http 协议", "ftp://example.com/avatars/a.png"},
		{"非约定目录", "https://example.com/other/a.png"},
		{"裸的其它目录 key", "documents/a.pdf"},
		{"只是目录", "avatars/"},
		{"路径穿越无法逃出约定前缀", "https://example.com/avatars/../../etc/passwd"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got, err := NormalizeAvatarKey(tc.input); err == nil {
				t.Fatalf("NormalizeAvatarKey(%q) 应报错，却返回 %q", tc.input, got)
			}
		})
	}
}

// 路径穿越不能把 key 归一到约定目录之外。
func TestNormalizeAvatarKey_PathTraversalStaysInsidePrefix(t *testing.T) {
	t.Parallel()
	got, err := NormalizeAvatarKey("avatars/../../secret.txt")
	if err == nil && got != AvatarPrefix && !isUnderPrefix(got) {
		t.Fatalf("穿越输入归一到了约定目录之外: %q", got)
	}
}

func isUnderPrefix(key string) bool {
	return len(key) > len(AvatarPrefix) && key[:len(AvatarPrefix)] == AvatarPrefix
}

func TestResolveAvatarURL_EmptyKeyYieldsEmpty(t *testing.T) {
	t.Parallel()
	got, err := ResolveAvatarURL(context.Background(), nil, "  ")
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if got != "" {
		t.Fatalf("空 key 应解析为空串, 得到 %q", got)
	}
}

// 未接对象存储时退化为 file 驱动的相对路径形态，调用方无需感知 store 是否可用。
func TestResolveAvatarURL_NilStoreFallsBackToRelativePath(t *testing.T) {
	t.Parallel()
	got, err := ResolveAvatarURL(context.Background(), nil, "avatars/a.png")
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if want := PublicUploadPrefix + "avatars/a.png"; got != want {
		t.Fatalf("got %q, 期望 %q", got, want)
	}
}

// 有 store 时走签名：每次读取都现签，头像不会因为签名过期而失效。
func TestResolveAvatarURL_SignsOnEveryRead(t *testing.T) {
	t.Parallel()
	store := &countingStore{}
	for i := 0; i < 3; i++ {
		got, err := ResolveAvatarURL(context.Background(), store, "avatars/a.png")
		if err != nil {
			t.Fatalf("不应报错: %v", err)
		}
		if got == "" {
			t.Fatal("有 store 时应解析出非空 URL")
		}
	}
	if store.calls != 3 {
		t.Fatalf("SignedURL 调用次数 = %d, 期望 3（每次读取都现签）", store.calls)
	}
}

func TestResolveAvatarURL_StoreErrorPropagates(t *testing.T) {
	t.Parallel()
	store := &countingStore{err: errors.New("boom")}
	if _, err := ResolveAvatarURL(context.Background(), store, "avatars/a.png"); err == nil {
		t.Fatal("store 报错时应向上传递")
	}
}

func TestLocalBaseDir_RejectsUnsafeValues(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"", "   ", ".", "./", "./."} {
		if _, err := LocalBaseDir(bad); err == nil {
			t.Fatalf("LocalBaseDir(%q) 应报错", bad)
		}
	}
}

func TestLocalBaseDir_ReturnsAbsolutePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got, err := LocalBaseDir(dir)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("应返回绝对路径, 得到 %q", got)
	}
	abs, _ := filepath.Abs(dir)
	if got != abs {
		t.Fatalf("got %q, 期望 %q", got, abs)
	}
}

// countingStore 记录 SignedURL 调用次数的最小 Store 实现。
type countingStore struct {
	calls int
	err   error
}

func (s *countingStore) Put(context.Context, string, ReadSeeker, int64, string) error { return nil }
func (s *countingStore) SignedURL(_ context.Context, key string, _ string, _ time.Duration) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	return PublicUploadPrefix + key, nil
}
func (s *countingStore) Delete(context.Context, string) error      { return nil }
func (s *countingStore) List(context.Context, string, string, string, int) (ListResult, error) {
	return ListResult{}, nil
}
func (s *countingStore) CreatePrefix(context.Context, string) error          { return nil }
func (s *countingStore) RenamePrefix(context.Context, string, string) error  { return nil }

// 存量库里存的是绝对 URL（归一化是本次修复之后才引入的）。这类值必须原样返回——
// 把它拼成 /uploads/https://… 会把「可能过期」变成「必然 404」。
func TestResolveAvatarURL_PassesThroughAbsoluteURL(t *testing.T) {
	t.Parallel()
	const legacy = "https://cdn.example.com/avatars/old.png?X-Amz-Signature=abc"
	store := &countingStore{}
	got, err := ResolveAvatarURL(context.Background(), store, legacy)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if got != legacy {
		t.Fatalf("绝对 URL 应原样返回, 得到 %q", got)
	}
	if store.calls != 0 {
		t.Fatalf("绝对 URL 不应触发签名, 调用次数 = %d", store.calls)
	}
}

// 存量 file 驱动的相对路径已是可访问形态，不应重复加前缀。
func TestResolveAvatarURL_PassesThroughExistingRelativePath(t *testing.T) {
	t.Parallel()
	store := &countingStore{}
	got, err := ResolveAvatarURL(context.Background(), store, PublicUploadPrefix+"avatars/old.png")
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if want := PublicUploadPrefix + "avatars/old.png"; got != want {
		t.Fatalf("got %q, 期望 %q（不应重复加前缀）", got, want)
	}
	if store.calls != 0 {
		t.Fatalf("已是可访问形态时不应签名, 调用次数 = %d", store.calls)
	}
}

// 只暴露 avatars 子树：静态路由挂载点是 /uploads/avatars/，不是整个 uploads。
func TestAvatarPublicPrefix_IsAvatarsSubtreeOnly(t *testing.T) {
	t.Parallel()
	if AvatarPublicPrefix != "/uploads/avatars/" {
		t.Fatalf("AvatarPublicPrefix = %q, 期望 /uploads/avatars/", AvatarPublicPrefix)
	}
	if !strings.HasPrefix(AvatarPublicPrefix, PublicUploadPrefix) {
		t.Fatalf("头像前缀应在 uploads 前缀之下: %q", AvatarPublicPrefix)
	}
}

func TestLocalAvatarDir_JoinsAvatarsSubdir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	got, err := LocalAvatarDir(base)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if filepath.Base(got) != "avatars" {
		t.Fatalf("应指向 avatars 子目录, 得到 %q", got)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("应返回绝对路径, 得到 %q", got)
	}
}

func TestLocalAvatarDir_RejectsUnsafeBase(t *testing.T) {
	t.Parallel()
	if _, err := LocalAvatarDir("."); err == nil {
		t.Fatal("base 为当前工作目录时应报错")
	}
	if _, err := LocalAvatarDir("  "); err == nil {
		t.Fatal("base 为空时应报错")
	}
}
