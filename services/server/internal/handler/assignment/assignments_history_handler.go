// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package assignment

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/assignment"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取分配历史
func AssignmentsHistoryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AssignmentsHistoryRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := assignment.NewAssignmentsHistoryLogic(r.Context(), svcCtx)
		resp, err := l.AssignmentsHistory(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
