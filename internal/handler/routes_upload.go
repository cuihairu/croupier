package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/svc"

	"log/slog"

	"github.com/gin-gonic/gin"
)

// registerUploadStaticRoute 为 file 存储驱动挂载头像静态目录。
//
// fileStore.SignedURL 在未配置 PublicURL 时返回 `/uploads/<key>` 相对路径，
// 但此前没有任何 handler 提供该路径——于是上传成功的头像一律 404，页面只能
// 显示空白占位（docs/BUGS.md BUG-012）。S3/OSS/COS 驱动返回的是带签名的绝对
// 地址，不需要这条路由。
//
// 只挂 **avatars 子树**：通用存储 API 也往同一目录写文件（缺陷附件、导出等），
// 把整个 uploads 目录静态暴露会一并泄露；而头像必须能被 <img src> 直接加载
// （浏览器发 <img> 请求不带 Authorization 头，走不了鉴权路由）。
func registerUploadStaticRoute(r *gin.Engine, serverCtx *svc.ServiceContext) {
	baseDir := strings.TrimSpace(serverCtx.Config.Storage.BaseDir)
	if baseDir == "" || !strings.EqualFold(strings.TrimSpace(serverCtx.Config.Storage.Driver), "file") {
		return
	}
	dir, err := objstore.LocalAvatarDir(baseDir)
	if err != nil {
		slog.Default().Warn("本地头像静态目录无效，未挂载 "+objstore.AvatarPublicPrefix,
			"baseDir", baseDir, "error", err)
		return
	}
	// 目录可能尚未创建（还没人上传过头像）：StaticFS 仍可挂载，文件缺失时 404。
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Default().Warn("本地头像目录创建失败，未挂载 "+objstore.AvatarPublicPrefix,
			"dir", dir, "error", err)
		return
	}
	r.StaticFS(objstore.AvatarPublicPrefix, gin.Dir(dir, false))
	slog.Default().Info("已挂载本地头像静态目录",
		"prefix", objstore.AvatarPublicPrefix, "dir", dir)

	registerIconStaticRoute(r, baseDir)
}

// registerIconStaticRoute 挂载游戏图标静态目录（file 驱动，键空间
// icons/**，上传入口 POST /api/v1/games/icons）。
//
// 不用 StaticFS：SVG 可内嵌 <script>，直开 URL 即存储型 XSS——手动注册
// GET/HEAD 并附加响应头（CSP 禁脚本与外链 + nosniff；内联 style 保留，图标
// 渐变/填充常用）。图标 key 是内容寻址哈希名（上传端点产出），可放心
// immutable 长缓存。
func registerIconStaticRoute(r *gin.Engine, baseDir string) {
	dir, err := objstore.LocalIconDir(baseDir)
	if err != nil {
		slog.Default().Warn("本地图标静态目录无效，未挂载 "+objstore.IconPublicPrefix,
			"baseDir", baseDir, "error", err)
		return
	}
	// 目录可能尚未创建（还没人上传过图标）：仍挂载，文件缺失时 404。
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Default().Warn("本地图标目录创建失败，未挂载 "+objstore.IconPublicPrefix,
			"dir", dir, "error", err)
		return
	}
	handler := func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		// 前置 "/" 使 Clean 以虚拟根为锚，".." 折叠后不会逃出目录；再兜一层
		// 前缀校验双保险。
		name := filepath.Clean("/" + c.Param("filepath"))
		full := filepath.Join(dir, filepath.FromSlash(name))
		if abs, err := filepath.Abs(full); err != nil ||
			!strings.HasPrefix(abs, dir+string(filepath.Separator)) {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.File(full)
	}
	pattern := objstore.IconPublicPrefix + "*filepath"
	r.GET(pattern, handler)
	r.HEAD(pattern, handler)
	slog.Default().Info("已挂载本地游戏图标静态目录",
		"prefix", objstore.IconPublicPrefix, "dir", dir)
}
