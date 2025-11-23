// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/support"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 工单状态转换
func SupportTicketTransitionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SupportTicketTransitionRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := support.NewSupportTicketTransitionLogic(r.Context(), svcCtx)
		resp, err := l.SupportTicketTransition(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
