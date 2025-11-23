// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package meta

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/meta"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 根路径 - API 信息和版本
func RootHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RootRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := meta.NewRootLogic(r.Context(), svcCtx)
		resp, err := l.Root(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
