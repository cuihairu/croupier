package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/gin-gonic/gin"
)

// softFeatureGuard 是五域功能的 L3 运行时软开关
// （docs/architecture/config-layering.md §4 features.*）。
//
// 与 L2 的物理裁剪（不注册路由）不同：路由照常注册，middleware 在每次
// 请求时查询 Layered.FeatureEnabled（L2 ∧ L3 合成），被禁域返回
// 403 {"error":"feature_disabled"}。前端菜单由 GET /api/v1 features
// 同步隐藏，二者读同一合成值。
type softFeatureGuard struct {
	layered *settings.Layered
}

func newSoftFeatureGuard(layered *settings.Layered) *softFeatureGuard {
	return &softFeatureGuard{layered: layered}
}

// guard 返回拦截指定域的 gin middleware。
func (g *softFeatureGuard) guard(flag string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !g.enabled(flag) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "feature_disabled",
				"message": "该功能域已被管理员停用",
				"details": gin.H{"feature": flag},
			})
			return
		}
		c.Next()
	}
}

func (g *softFeatureGuard) enabled(flag string) bool {
	if g == nil || g.layered == nil {
		return true // fail-open：settings 未初始化不得锁死全部域
	}
	return g.layered.FeatureEnabled(flag)
}
