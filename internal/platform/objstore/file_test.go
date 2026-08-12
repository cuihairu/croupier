package objstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestOpenFile 测试打开文件存储
func TestOpenFile(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "有效配置",
			cfg: Config{
				Driver:  "file",
				BaseDir: os.TempDir(),
			},
			wantErr: false,
		},
		{
			name: "空 BaseDir",
			cfg: Config{
				Driver: "file",
			},
			wantErr: true,
		},
		{
			name: "包含 SignedURLTTL",
			cfg: Config{
				Driver:       "file",
				BaseDir:      os.TempDir(),
				SignedURLTTL: 30 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "包含 PublicURL",
			cfg: Config{
				Driver:    "file",
				BaseDir:   os.TempDir(),
				PublicURL: "https://cdn.example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := OpenFile(context.Background(), tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && store == nil {
				t.Error("OpenFile() should return non-nil store when no error")
			}
		})
	}
}

// TestFileStore_Put 测试文件上传
func TestFileStore_Put(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := OpenFile(context.Background(), Config{Driver: "file", BaseDir: tmpDir})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	ctx := context.Background()
	data := strings.NewReader("test content")

	// 测试基本上传
	err = store.Put(ctx, "test/file.txt", data, 12, "text/plain")
	if err != nil {
		t.Errorf("Put() error = %v", err)
	}

	// 验证文件存在
	path := filepath.Join(tmpDir, "test", "file.txt")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("File should exist after Put()")
	}

	// 测试路径清理（带前导斜杠）
	data2 := strings.NewReader("test content 2")
	err = store.Put(ctx, "/leading/slash/file.txt", data2, 15, "text/plain")
	if err != nil {
		t.Errorf("Put() with leading slash error = %v", err)
	}

	// 测试路径清理（包含 ..）
	data3 := strings.NewReader("test content 3")
	err = store.Put(ctx, "path/../cleaned/file.txt", data3, 15, "text/plain")
	if err != nil {
		t.Errorf("Put() with .. error = %v", err)
	}

	// 验证路径被正确清理
	path3 := filepath.Join(tmpDir, "path", "cleaned", "file.txt")
	if _, err := os.Stat(path3); os.IsNotExist(err) {
		t.Error("File should exist with cleaned path")
	}
}

// TestFileStore_SignedURL 测试签名 URL 生成
func TestFileStore_SignedURL(t *testing.T) {
	tests := []struct {
		name         string
		publicURL    string
		key          string
		method       string
		wantErr      bool
		wantContains string
	}{
		{
			name:         "相对路径",
			publicURL:    "",
			key:          "test/file.txt",
			method:       "GET",
			wantErr:      false,
			wantContains: "/uploads/test/file.txt",
		},
		{
			name:         "PublicURL 配置",
			publicURL:    "https://cdn.example.com",
			key:          "test/file.txt",
			method:       "GET",
			wantErr:      false,
			wantContains: "https://cdn.example.com/test/file.txt",
		},
		{
			name:         "PublicURL 带尾部斜杠",
			publicURL:    "https://cdn.example.com/",
			key:          "test/file.txt",
			method:       "GET",
			wantErr:      false,
			wantContains: "https://cdn.example.com/test/file.txt",
		},
		{
			name:      "DELETE 方法不支持",
			publicURL: "",
			key:       "test/file.txt",
			method:    "DELETE",
			wantErr:   true,
		},
		{
			name:         "PUT 方法支持",
			publicURL:    "",
			key:          "test/file.txt",
			method:       "PUT",
			wantErr:      false,
			wantContains: "/uploads/test/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store, err := OpenFile(context.Background(), Config{
				Driver:    "file",
				BaseDir:   tmpDir,
				PublicURL: tt.publicURL,
			})
			if err != nil {
				t.Fatalf("OpenFile() error = %v", err)
			}

			ctx := context.Background()
			url, err := store.SignedURL(ctx, tt.key, tt.method, 0)

			if (err != nil) != tt.wantErr {
				t.Errorf("SignedURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.wantContains != "" {
				if !strings.Contains(url, tt.wantContains) {
					t.Errorf("SignedURL() = %q, should contain %q", url, tt.wantContains)
				}
			}
		})
	}
}

// TestFileStore_Delete 测试删除文件和目录
func TestFileStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := OpenFile(context.Background(), Config{Driver: "file", BaseDir: tmpDir})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	ctx := context.Background()

	// 创建测试文件
	data := strings.NewReader("test content")
	err = store.Put(ctx, "test/file1.txt", data, 12, "")
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	data2 := strings.NewReader("test content 2")
	err = store.Put(ctx, "test/file2.txt", data2, 14, "")
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// 测试删除单个文件
	err = store.Delete(ctx, "test/file1.txt")
	if err != nil {
		t.Errorf("Delete() file error = %v", err)
	}

	// 验证文件已删除
	path := filepath.Join(tmpDir, "test", "file1.txt")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("File should be deleted")
	}

	// 测试删除剩余的单个文件
	err = store.Delete(ctx, "test/file2.txt")
	if err != nil {
		t.Errorf("Delete() remaining file error = %v", err)
	}

	// 验证文件已删除
	path2 := filepath.Join(tmpDir, "test", "file2.txt")
	if _, err := os.Stat(path2); !os.IsNotExist(err) {
		t.Error("File should be deleted")
	}

	// 测试删除不存在的文件（应该是安全的）
	err = store.Delete(ctx, "nonexistent/file.txt")
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("Delete() nonexistent file should not error, got %v", err)
	}
}

// TestFileStore_List 测试列出文件
func TestFileStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := OpenFile(context.Background(), Config{Driver: "file", BaseDir: tmpDir})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	ctx := context.Background()

	// 创建测试文件
	files := []string{"test/file1.txt", "test/file2.txt", "test/subdir/file3.txt", "other/file4.txt"}
	for _, file := range files {
		data := strings.NewReader("content")
		if err := store.Put(ctx, file, data, 7, ""); err != nil {
			t.Fatalf("Put(%s) error = %v", file, err)
		}
	}

	// 测试基本列出
	result, err := store.List(ctx, "test/", "", "", 0)
	if err != nil {
		t.Errorf("List() error = %v", err)
	}
	if len(result.Objects) < 3 {
		t.Errorf("List() should return at least 3 objects, got %d", len(result.Objects))
	}

	// 测试带前缀过滤
	result, err = store.List(ctx, "test/subdir/", "", "", 0)
	if err != nil {
		t.Errorf("List() with prefix error = %v", err)
	}
	if len(result.Objects) < 1 {
		t.Error("List() with prefix should return at least 1 object")
	}

	// 测试限制
	result, err = store.List(ctx, "test/", "", "", 2)
	if err != nil {
		t.Errorf("List() with limit error = %v", err)
	}
	if len(result.Objects) != 2 {
		t.Errorf("List() with limit=2 should return 2 objects, got %d", len(result.Objects))
	}
	if !result.IsTruncated {
		t.Error("List() with limit should set IsTruncated=true")
	}

	// 测试带分隔符
	result, err = store.List(ctx, "test/", "", "/", 0)
	if err != nil {
		t.Errorf("List() with delimiter error = %v", err)
	}
	// 应该包含 subdir 作为前缀
	hasSubdir := false
	for _, prefix := range result.Prefixes {
		if strings.Contains(prefix, "subdir") {
			hasSubdir = true
			break
		}
	}
	if !hasSubdir {
		t.Error("List() with delimiter should include subdir in prefixes")
	}
}

func TestFileStore_List_NormalizesTraversalPrefixAndMarker(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := OpenFile(context.Background(), Config{Driver: "file", BaseDir: tmpDir})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	ctx := context.Background()
	if err := store.Put(ctx, "safe/file.txt", strings.NewReader("content"), 7, ""); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Both fields are normalized before they participate in filesystem paths
	// or pagination comparisons; neither may escape the configured base.
	result, err := store.List(ctx, "../safe/", "../safe/file.txt", "", 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Objects) != 0 {
		t.Fatalf("List() returned marker-or-earlier objects: %#v", result.Objects)
	}
}

// TestFileStore_CreatePrefix 测试创建前缀目录
func TestFileStore_CreatePrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := OpenFile(context.Background(), Config{Driver: "file", BaseDir: tmpDir})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	ctx := context.Background()

	// 测试创建带斜杠的前缀
	err = store.CreatePrefix(ctx, "test/prefix/")
	if err != nil {
		t.Errorf("CreatePrefix() with slash error = %v", err)
	}

	// 验证目录存在
	path := filepath.Join(tmpDir, "test", "prefix")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Directory should exist after CreatePrefix()")
	}

	// 测试创建不带斜杠的前缀（应该自动添加）
	err = store.CreatePrefix(ctx, "another/prefix")
	if err != nil {
		t.Errorf("CreatePrefix() without slash error = %v", err)
	}

	// 验证目录存在
	path2 := filepath.Join(tmpDir, "another", "prefix")
	if _, err := os.Stat(path2); os.IsNotExist(err) {
		t.Error("Directory should exist after CreatePrefix() without slash")
	}

	// 测试路径清理
	err = store.CreatePrefix(ctx, "/cleaned/path/")
	if err != nil {
		t.Errorf("CreatePrefix() with leading slash error = %v", err)
	}

	// 验证路径被正确清理
	path3 := filepath.Join(tmpDir, "cleaned", "path")
	if _, err := os.Stat(path3); os.IsNotExist(err) {
		t.Error("Directory should exist with cleaned path")
	}
}

// TestFileStore_RenamePrefix 测试重命名前缀目录
func TestFileStore_RenamePrefix(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := OpenFile(context.Background(), Config{Driver: "file", BaseDir: tmpDir})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	ctx := context.Background()

	// 创建测试文件
	files := []string{"old/file1.txt", "old/file2.txt"}
	for _, file := range files {
		data := strings.NewReader("content")
		if err := store.Put(ctx, file, data, 7, ""); err != nil {
			t.Fatalf("Put(%s) error = %v", file, err)
		}
	}

	// 测试重命名目录
	err = store.RenamePrefix(ctx, "old/", "new/")
	if err != nil {
		t.Errorf("RenamePrefix() error = %v", err)
	}

	// 验证旧目录不存在
	oldPath := filepath.Join(tmpDir, "old")
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Old directory should not exist after rename")
	}

	// 验证新目录存在
	newPath := filepath.Join(tmpDir, "new")
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("New directory should exist after rename")
	}

	// 验证文件已移动
	newFile1 := filepath.Join(tmpDir, "new", "file1.txt")
	if _, err := os.Stat(newFile1); os.IsNotExist(err) {
		t.Error("File should exist in new directory")
	}

	// 测试不带斜杠的前缀（应该自动添加）
	err = store.RenamePrefix(ctx, "new", "renamed")
	if err != nil {
		t.Errorf("RenamePrefix() without slash error = %v", err)
	}

	// 验证重命名成功
	renamedPath := filepath.Join(tmpDir, "renamed")
	if _, err := os.Stat(renamedPath); os.IsNotExist(err) {
		t.Error("Directory should exist after rename without slash")
	}
}

// TestFileStore_Integration 测试完整的文件操作流程
func TestFileStore_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := OpenFile(context.Background(), Config{
		Driver:       "file",
		BaseDir:      tmpDir,
		SignedURLTTL: 30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	ctx := context.Background()

	// 1. 创建前缀
	err = store.CreatePrefix(ctx, "uploads/")
	if err != nil {
		t.Fatalf("CreatePrefix() error = %v", err)
	}

	// 2. 上传文件
	data := strings.NewReader("Hello, World!")
	err = store.Put(ctx, "uploads/test.txt", data, 13, "text/plain")
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// 3. 获取签名 URL
	url, err := store.SignedURL(ctx, "uploads/test.txt", "GET", 0)
	if err != nil {
		t.Fatalf("SignedURL() error = %v", err)
	}
	if !strings.Contains(url, "/uploads/uploads/test.txt") {
		t.Errorf("SignedURL() = %q, should contain '/uploads/uploads/test.txt'", url)
	}

	// 4. 列出文件
	result, err := store.List(ctx, "uploads/", "", "", 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Objects) != 1 {
		t.Errorf("List() should return 1 object, got %d", len(result.Objects))
	}

	// 5. 删除文件
	err = store.Delete(ctx, "uploads/test.txt")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// 6. 验证文件已删除
	result, err = store.List(ctx, "uploads/", "", "", 0)
	if err != nil {
		t.Fatalf("List() after delete error = %v", err)
	}
	if len(result.Objects) != 0 {
		t.Errorf("List() after delete should return 0 objects, got %d", len(result.Objects))
	}
}

// BenchmarkFileStore_Put 性能基准测试
func BenchmarkFileStore_Put(b *testing.B) {
	tmpDir := b.TempDir()
	store, err := OpenFile(context.Background(), Config{Driver: "file", BaseDir: tmpDir})
	if err != nil {
		b.Fatalf("OpenFile() error = %v", err)
	}

	ctx := context.Background()
	data := strings.NewReader("test content for benchmarking")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "benchmark/file.txt"
		store.Put(ctx, key, data, 28, "")
	}
}

// BenchmarkFileStore_List 性能基准测试
func BenchmarkFileStore_List(b *testing.B) {
	tmpDir := b.TempDir()
	store, err := OpenFile(context.Background(), Config{Driver: "file", BaseDir: tmpDir})
	if err != nil {
		b.Fatalf("OpenFile() error = %v", err)
	}

	ctx := context.Background()
	// 创建一些测试文件
	for i := 0; i < 100; i++ {
		data := strings.NewReader("content")
		store.Put(ctx, "test/file.txt", data, 7, "")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.List(ctx, "test/", "", "", 0)
	}
}
