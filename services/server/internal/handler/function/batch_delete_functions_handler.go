// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/function"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 批量删除函数
func BatchDeleteFunctionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BatchDeleteFunctionsRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := function.NewBatchDeleteFunctionsLogic(r.Context(), svcCtx)
		resp, err := l.BatchDeleteFunctions(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}