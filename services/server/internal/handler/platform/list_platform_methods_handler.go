// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"net/http"

	platformlogic "github.com/cuihairu/croupier/services/server/internal/logic/platform"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取指定平台支持的方法列表
func ListPlatformMethodsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 从 URL 路径中获取 platform 参数
		platformName := r.PathValue("platform")
		l := platformlogic.NewListPlatformMethodsLogic(r.Context(), svcCtx)
		resp, err := l.ListPlatformMethods(platformName)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
