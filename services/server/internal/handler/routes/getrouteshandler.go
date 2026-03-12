// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package routes

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/routes"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取动态路由配置
func GetRoutesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := routes.NewGetRoutesLogic(r.Context(), svcCtx)
		resp, err := l.GetRoutes()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
