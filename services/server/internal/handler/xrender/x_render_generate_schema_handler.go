// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/xrender"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 生成XRender模式
func XRenderGenerateSchemaHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.XRenderGenerateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := xrender.NewXRenderGenerateSchemaLogic(r.Context(), svcCtx)
		resp, err := l.XRenderGenerateSchema(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
