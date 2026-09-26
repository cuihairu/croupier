package objstore

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// avatarTTL 是为头像生成读取签名 URL 的有效期。
//
// 头像 URL 在**每次读取 profile 时**现算，而不是把签名 URL 存进库：签名 URL
// 带过期时间，存库就等于存了一条定时失效的死链（S3/OSS/COS 驱动默认 15min
// TTL，configs 里的 STORAGE_SIGNED_URL_TTL 还能再调），用户过一会儿刷新
// 页面头像就裂了。
const avatarTTL = 30 * time.Minute

// AvatarPrefix 是头像对象 key 的约定前缀。
const AvatarPrefix = "avatars/"

// PublicUploadPrefix 是 file 驱动下相对 URL 的对外路径前缀。
const PublicUploadPrefix = "/uploads/"

// AvatarPublicPrefix 是头像在 file 驱动下被 HTTP 静态路由暴露的路径前缀。
//
// 刻意**只**暴露 avatars 子树，而不是整个 uploads 目录：通用存储 API
// （POST /api/v1/storage/objects）也能写入同一个目录，里面可能有缺陷附件、
// 导出文件等敏感内容；而头像只有时间戳 + 随机段组成的 key，且必须能被
// <img src> 直接加载（浏览器发 <img> 请求时不会带 Authorization 头，鉴权过的
// 路由在这里根本不可能命中）。
const AvatarPublicPrefix = PublicUploadPrefix + AvatarPrefix

// LocalBaseDir 校验并归一 file 驱动的本地存储目录。
//
// 该目录会被 HTTP 静态路由直接暴露，因此拒绝会放大暴露面的取值：空串（等价于
// 服务工作目录）与 "." / "./" 这类相对当前目录的写法。返回绝对化后的路径。
func LocalBaseDir(baseDir string) (string, error) {
	dir := strings.TrimSpace(baseDir)
	if dir == "" {
		return "", fmt.Errorf("base_dir 不能为空")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("base_dir 解析失败: %w", err)
	}
	// filepath.Abs(".") = 当前工作目录：整目录静态暴露不是可接受的默认值。
	if strings.TrimSuffix(abs, string(filepath.Separator)) == strings.TrimSuffix(mustGetwd(), string(filepath.Separator)) {
		return "", fmt.Errorf("base_dir 不能是当前工作目录（会暴露整个服务目录）")
	}
	return abs, nil
}

// LocalAvatarDir 返回 file 驱动下头像子目录的绝对路径（供静态路由挂载）。
func LocalAvatarDir(baseDir string) (string, error) {
	abs, err := LocalBaseDir(baseDir)
	if err != nil {
		return "", err
	}
	// 目录名是包内常量而非外部输入，这里只需拼接后再做一次穿越校验。
	return LocalBaseDir(filepath.Join(abs, AvatarPrefix))
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

// NormalizeAvatarKey 把「用户提交的任意头像值」归一成**裸对象 key**。
//
// 允许三种输入，统一存成第三种（裸 key）：
//
//	"avatars/123-abc.png"                → "avatars/123-abc.png"
//	"/uploads/avatars/123-abc.png"       → "avatars/123-abc.png"   （file 驱动历史值）
//	"https://cdn.example.com/prefix/avatars/123-abc.png" → "avatars/123-abc.png"
//
// 返回空串表示「用户显式清空头像」（调用方据此落库空串）。
//
// 绝对 URL 只截取 AvatarPrefix 之后的部分：S3/OSS/COS 的 PublicURL 常带自定义
// 前缀（如 https://cdn.example.com/my-assets），裸 key 无法可靠反推前缀位置，
// 因此以约定前缀为准——上传接口产出的 key 一律以它开头。
//
// 路径穿越（".."）在任何输入形态下都被拒绝。
func NormalizeAvatarKey(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}

	// 绝对 URL：先剥 scheme://host，再剥 public prefix，最后取 AvatarPrefix 段。
	if u, err := url.Parse(value); err == nil && u.IsAbs() {
		if u.Scheme != "http" && u.Scheme != "https" {
			return "", fmt.Errorf("头像地址仅支持 http/https，收到 %q", u.Scheme)
		}
		value = u.Path
	}

	// 剥掉 file 驱动的 /uploads/ 前缀（历史值）与结尾的查询串/片段。
	if idx := strings.Index(value, "?"); idx >= 0 {
		value = value[:idx]
	}
	if idx := strings.Index(value, "#"); idx >= 0 {
		value = value[:idx]
	}
	if strings.HasPrefix(value, PublicUploadPrefix) {
		value = strings.TrimPrefix(value, PublicUploadPrefix)
	}

	// 定位约定前缀：取**最后**一次出现，兼容带自定义前缀的 CDN 路径。
	idx := strings.LastIndex(value, AvatarPrefix)
	if idx < 0 {
		return "", fmt.Errorf("头像地址必须位于 %q 目录下：%q", AvatarPrefix, raw)
	}
	value = value[idx:]

	// 归一 + 穿越校验。sanitizeKey 会把 "a/../../etc" 折叠成 "etc"，
	// 因此这里额外要求归一结果仍以 AvatarPrefix 开头。
	key := sanitizeKey(value)
	if !strings.HasPrefix(key, AvatarPrefix) || strings.Contains(key, "..") {
		return "", fmt.Errorf("非法的头像对象 key：%q", raw)
	}
	if key == AvatarPrefix {
		return "", fmt.Errorf("头像对象 key 不能是目录：%q", raw)
	}
	return key, nil
}

// ResolveAvatarURL 把库里的裸对象 key 解析成**当前可访问**的 URL。
//
// key 为空返回空串。store 为 nil 时（未接入对象存储的纯本地部署、单元测试）
// 退化为 file 驱动的相对路径形态，使调用方无需感知 store 是否可用。
//
// 已经是绝对 URL 的值原样返回：归一化（NormalizeAvatarKey）是在本次修复之后
// 才 introduced 的，存量库里存的是签名 URL 或 CDN 地址。它们虽会随签名过期而
// 失效，但**不能**在这里被拼成 `/uploads/https://…` 这种明显错误的地址——
// 那会把「可能失效」变成「必然 404」。保留原值，用户下次保存资料时会被归一成
// 裸 key，从而自动修复。
func ResolveAvatarURL(ctx context.Context, store Store, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", nil
	}
	if u, err := url.Parse(key); err == nil && u.IsAbs() {
		return key, nil
	}
	// 存量相对路径（file 驱动历史值）已是可直接访问的形态，仅补齐缺失的前缀。
	if strings.HasPrefix(key, PublicUploadPrefix) {
		return key, nil
	}
	if store == nil {
		return PublicUploadPrefix + strings.TrimPrefix(key, "/"), nil
	}
	url, err := store.SignedURL(ctx, key, "GET", avatarTTL)
	if err != nil {
		return "", fmt.Errorf("生成头像访问地址失败: %w", err)
	}
	return url, nil
}
