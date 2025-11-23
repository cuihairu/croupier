// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package approval

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/approval"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 通过审批
func ApprovalApproveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ApprovalApproveRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := approval.NewApprovalApproveLogic(r.Context(), svcCtx)
		resp, err := l.ApprovalApprove(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
