// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package xrender_schema

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/xrender_schema"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取UI模式
func UiSchemaHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UISchemaRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := xrender_schema.NewUiSchemaLogic(r.Context(), svcCtx)
		resp, err := l.UiSchema(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
