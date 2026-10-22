package handler

// 覆盖率巡检第十四轮（wt-api）：routes.go registerUploadStaticRoute
// 残余 5 块收口——LocalAvatarDir 解析失败翼（相对 baseDir + Getwd 失效：
// chdir 进已删目录构造，本包无并行用例、顺序执行安全）、os.MkdirAll
// 失败翼（avatars 路径被同名文件占位）、挂载成功主链（路由注册 + 目录
// 自动创建）。file 驱动/空 baseDir 守卫翼既有用例已覆盖。

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func uploadStaticCtx(driver, baseDir string) *svc.ServiceContext {
	return &svc.ServiceContext{Config: config.Config{
		Storage: config.StorageConfig{Driver: driver, BaseDir: baseDir},
	}}
}

func avatarRouteCount(r *gin.Engine) int {
	n := 0
	for _, rt := range r.Routes() {
		if rt.Method == http.MethodGet && strings.HasPrefix(rt.Path, "/uploads/avatars") {
			n++
		}
	}
	return n
}

func TestRegisterUploadStaticRoute_Paths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 挂载成功主链：路由注册 + avatars 目录自动创建
	r := gin.New()
	base := t.TempDir()
	registerUploadStaticRoute(r, uploadStaticCtx("file", base))
	assert.Equal(t, 1, avatarRouteCount(r), "应挂载头像静态路由")
	assert.DirExists(t, filepath.Join(base, "avatars"), "目录未建也可挂载，此处应被自动创建")

	// MkdirAll 失败翼：avatars 路径被同名文件占位 → warn 后不挂载
	r2 := gin.New()
	base2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(base2, "avatars"), []byte("x"), 0o644))
	registerUploadStaticRoute(r2, uploadStaticCtx("file", base2))
	assert.Zero(t, avatarRouteCount(r2))

	// LocalAvatarDir 失败翼：相对 baseDir 须 Getwd 解析绝对路径；
	// chdir 进已删目录使 os.Getwd 失败（本包无并行用例，cwd 恢复安全）
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	dead := t.TempDir()
	require.NoError(t, os.Chdir(dead))
	require.NoError(t, os.RemoveAll(dead))
	r3 := gin.New()
	registerUploadStaticRoute(r3, uploadStaticCtx("file", "uploads"))
	assert.Zero(t, avatarRouteCount(r3), "目录解析失败不挂载")
	require.NoError(t, os.Chdir(oldWd))
}
