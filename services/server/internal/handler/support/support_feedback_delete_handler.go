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

// 删除反馈
func SupportFeedbackDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SupportFeedbackDeleteRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := support.NewSupportFeedbackDeleteLogic(r.Context(), svcCtx)
		resp, err := l.SupportFeedbackDelete(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
