// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package openapi

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/openapi"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取函数的 OpenAPI spec
func FunctionOpenAPISpecHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OpenAPISpecRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := openapi.NewFunctionOpenAPISpecLogic(r.Context(), svcCtx)
		resp, err := l.FunctionOpenAPISpec(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
