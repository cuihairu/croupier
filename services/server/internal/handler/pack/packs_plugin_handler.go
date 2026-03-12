// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/pack"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取 pack web 插件
func PacksPluginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PacksPluginRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := pack.NewPacksPluginLogic(r.Context(), svcCtx)
		resp, err := l.PacksPlugin(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
