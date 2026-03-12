// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package feedback

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/feedback"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取反馈列表
func FeedbackListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.FeedbackListRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := feedback.NewFeedbackListLogic(r.Context(), svcCtx)
		resp, err := l.FeedbackList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
