package handler

// 游戏图标静态路由（registerIconStaticRoute）用例：挂载主链（GET/HEAD 注册 +
// icons 目录自动创建）、GET 响应头三件套（immutable 缓存 / nosniff / SVG 兜底
// CSP）与 Content-Type、HEAD 可用、路径穿越不逃逸 icons 子树、LocalIconDir
// 解析失败与 MkdirAll 失败翼不挂载。

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func iconRouteCount(r *gin.Engine, method string) int {
	n := 0
	for _, rt := range r.Routes() {
		if rt.Method == method && strings.HasPrefix(rt.Path, "/uploads/icons") {
			n++
		}
	}
	return n
}

func TestRegisterIconStaticRoute_MountAndHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	base := t.TempDir()
	r := gin.New()
	registerIconStaticRoute(r, base)

	assert.Equal(t, 1, iconRouteCount(r, http.MethodGet), "应挂载图标 GET 静态路由")
	assert.Equal(t, 1, iconRouteCount(r, http.MethodHead), "应挂载图标 HEAD 静态路由")
	assert.DirExists(t, filepath.Join(base, "icons"), "目录未建也可挂载，此处应被自动创建")

	// 落一枚真 PNG 到 icons/games/<hash>.png（上传端点的 key 形态）。
	body := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01}
	name := "games/0123456789abcdef.png"
	require.NoError(t, os.MkdirAll(filepath.Join(base, "icons", "games"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "icons", name), body, 0o644))

	// GET：200 + Content-Type + 三道响应头。
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/uploads/icons/"+name, nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=31536000, immutable", w.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "default-src 'none'; style-src 'unsafe-inline'", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, body, w.Body.Bytes())

	// HEAD：200（无体）。
	wh := httptest.NewRecorder()
	r.ServeHTTP(wh, httptest.NewRequest(http.MethodHead, "/uploads/icons/"+name, nil))
	assert.Equal(t, http.StatusOK, wh.Code)

	// 缺失文件 → 404。
	wm := httptest.NewRecorder()
	r.ServeHTTP(wm, httptest.NewRequest(http.MethodGet, "/uploads/icons/games/deadbeef.png", nil))
	assert.Equal(t, http.StatusNotFound, wm.Code)

	// 路径穿越：icons 子树外的文件不可达。两层防线叠加——handler 的
	// Clean("/"+param)+前缀校验把 ".." 钳回 icons 子树内（此时目标不存在，
	// 意图返回 404）；net/http.ServeFile 对原始 URL 含 ".." 会先于存在性
	// 检查直接 400 "invalid URL path"。两种码都安全，这里断言不 200 且
	// 子树外内容绝不泄露。
	require.NoError(t, os.WriteFile(filepath.Join(base, "secret.txt"), []byte("top-secret"), 0o644))
	wt := httptest.NewRecorder()
	r.ServeHTTP(wt, httptest.NewRequest(http.MethodGet, "/uploads/icons/%2e%2e/secret.txt", nil))
	assert.NotEqual(t, http.StatusOK, wt.Code)
	assert.NotContains(t, wt.Body.String(), "top-secret")
}

func TestRegisterIconStaticRoute_FailureWings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// MkdirAll 失败翼：icons 路径被同名文件占位 → warn 后不挂载。
	base := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(base, "icons"), []byte("x"), 0o644))
	r := gin.New()
	registerIconStaticRoute(r, base)
	assert.Zero(t, iconRouteCount(r, http.MethodGet))

	// LocalIconDir 失败翼：相对 baseDir 须 Getwd 解析绝对路径；chdir 进已删
	// 目录使 os.Getwd 失败（本包无并行用例，cwd 恢复安全）。
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	dead := t.TempDir()
	require.NoError(t, os.Chdir(dead))
	require.NoError(t, os.RemoveAll(dead))
	r2 := gin.New()
	registerIconStaticRoute(r2, "uploads")
	assert.Zero(t, iconRouteCount(r2, http.MethodGet), "目录解析失败不挂载")
	require.NoError(t, os.Chdir(oldWd))
}
