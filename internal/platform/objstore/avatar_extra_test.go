package objstore

// 覆盖率巡检补测（avatar.go 91.1% → 98.2%，5 块未覆盖中 4 块补齐）。
//
// 已知边界（按仓库既有口径登记，不造假用例、不删防御分支）：
// NormalizeAvatarKey 的 `key == AvatarPrefix`（key 不能是目录）防御分支
// 不可达——sanitizeKey 基于 filepath.Clean，Clean 恒去除尾斜杠（仅根 "/"
// 例外，而 TrimPrefix 后为空串、过不了 HasPrefix("avatars/") 前置校验），
// 归一结果不可能等于 "avatars/"，等值分支无法构造输入触达。
//
// 另两处依赖 os.Getwd() 失败才能触达（Abs 错误分支、mustGetwd 失败分支），
// 用「chdir 进已删除目录」构造；chdir 是进程态，本文件测试不得 t.Parallel。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 相对输入形态的 query/fragment 裁剪：绝对 URL 经 url.Parse 后 u.Path 已
// 不含 ?/#，永远到不了这两个 Index 分支，必须用相对 key 触达。
func TestNormalizeAvatarKey_TrimsQueryAndFragmentOnRelativeInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"相对 key 带查询串", "avatars/123.png?Expires=1&Signature=x", "avatars/123.png"},
		{"相对 key 带 fragment", "avatars/123.png#sig", "avatars/123.png"},
		{"查询与 fragment 并存只裁到 ?", "avatars/123.png?a=1#sig", "avatars/123.png"},
		{"uploads 历史值带 fragment", "/uploads/avatars/123.png#tok", "avatars/123.png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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

// LocalBaseDir 的两个 Getwd 依赖分支：
//   - 相对输入 → filepath.Abs 内部 Getwd 失败 → 「base_dir 解析失败」；
//   - 绝对输入 → Abs 不调 Getwd 直接放行，守卫里的 mustGetwd 失败退化为
//     空串比较，绝对路径不被拦截（CWD 不可知时守卫退化为不命中）。
func TestLocalBaseDir_GetwdFailureBranches(t *testing.T) {
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("前置 Getwd 失败: %v", err)
	}

	tmp := t.TempDir()
	t.Chdir(tmp)
	if err := os.Remove(tmp); err != nil {
		t.Fatalf("删除当前目录构造 getcwd 失败: %v", err)
	}

	if _, err := LocalBaseDir("data/uploads"); err == nil || !strings.Contains(err.Error(), "base_dir 解析失败") {
		t.Fatalf("相对输入在 CWD 失效时应报「base_dir 解析失败」，got %v", err)
	}

	abs := filepath.Join(orig, "avatar-dir-probe")
	got, err := LocalBaseDir(abs)
	if err != nil {
		t.Fatalf("绝对输入在 CWD 失效时应放行: %v", err)
	}
	if got != abs {
		t.Fatalf("绝对输入应原样归一返回，got %q want %q", got, abs)
	}
}
