// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/workspace"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 删除 Workspace 配置
func WorkspaceConfigDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WorkspaceConfigDeleteRequest
		if err := requireWorkspacePermission(r.Context(), svcCtx, "delete"); err != nil {
			writeWorkspaceError(w, r, err, "delete")
			return
		}
		if err := httpx.Parse(r, &req); err != nil {
			writeWorkspaceError(w, r, err, "delete")
			return
		}

		ctx := withWorkspaceRequestID(r.Context(), resolveRequestIDFromRequest(r))
		l := workspace.NewWorkspaceConfigDeleteLogic(ctx, svcCtx)
		resp, err := l.WorkspaceConfigDelete(&req)
		if err != nil {
			writeWorkspaceError(w, r, err, "delete")
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
