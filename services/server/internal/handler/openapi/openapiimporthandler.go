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

// 导入 OpenAPI spec
func OpenAPIImportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OpenAPIImportRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := openapi.NewOpenAPIImportLogic(r.Context(), svcCtx)
		resp, err := l.OpenAPIImport(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
