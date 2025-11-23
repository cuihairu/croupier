// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/provider"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取提供者描述符
func ProvidersDescriptorsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ProvidersDescriptorsRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := provider.NewProvidersDescriptorsLogic(r.Context(), svcCtx)
		resp, err := l.ProvidersDescriptors(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
