package function_calls

import (
	"net/http"
	"strconv"

	"github.com/cuihairu/croupier/services/server/internal/logic/function_calls"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func FunctionCallsListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.FunctionCallsListRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// Parse page from path if provided
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if page, err := strconv.Atoi(pageStr); err == nil {
				req.Page = page
			}
		}

		l := function_calls.NewFunctionCallsListLogic(r.Context(), svcCtx)
		resp, err := l.FunctionCallsList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
