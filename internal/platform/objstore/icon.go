package objstore

import (
	"path/filepath"
)

// IconPrefix 是游戏图标对象 key 的约定前缀。
const IconPrefix = "icons/"

// IconGamesPrefix 是游戏图标在 icons 子树下的业务目录（key = games/<hash>.<ext>）。
const IconGamesPrefix = IconPrefix + "games/"

// IconPublicPrefix 是游戏图标在 file 驱动下被 HTTP 静态路由暴露的路径前缀。
//
// 与 avatars 同款决策：**只**暴露 icons 子树，而不是整个 uploads 目录——通用
// 存储 API（POST /api/v1/storage/objects）也往同一 baseDir 写文件，缺陷附件、
// 导出等敏感内容不得被 <img src>/直链访问。图标必须能被 <img> 直接加载
// （浏览器发 <img> 请求不带 Authorization 头，鉴权路由不可能命中），因此该
// 子树进免认证白名单（见 AuthMiddleware publicReadPrefixes）。
const IconPublicPrefix = PublicUploadPrefix + IconPrefix

// LocalIconDir 返回 file 驱动下 icons 子目录的绝对路径（供静态路由挂载）。
func LocalIconDir(baseDir string) (string, error) {
	abs, err := LocalBaseDir(baseDir)
	if err != nil {
		return "", err
	}
	// 目录名是包内常量而非外部输入，这里只需拼接后再做一次穿越校验。
	return LocalBaseDir(filepath.Join(abs, IconPrefix))
}
