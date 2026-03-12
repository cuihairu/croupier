// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/component"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 安装组件
func ComponentsInstallHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ComponentsInstallRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := component.NewComponentsInstallLogic(r.Context(), svcCtx)
		resp, err := l.ComponentsInstall(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
