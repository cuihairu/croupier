// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package faq

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/faq"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 删除FAQ
func FAQDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.FAQDeleteRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := faq.NewFAQDeleteLogic(r.Context(), svcCtx)
		err := l.FAQDelete(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.Ok(w)
		}
	}
}
