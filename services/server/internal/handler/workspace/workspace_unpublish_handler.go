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

// 取消发布 Workspace 配置
func WorkspaceUnpublishHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WorkspaceUnpublishRequest
		if err := requireWorkspacePermission(r.Context(), svcCtx, "publish"); err != nil {
			writeWorkspaceError(w, r, err, "unpublish")
			return
		}
		if err := httpx.Parse(r, &req); err != nil {
			writeWorkspaceError(w, r, err, "unpublish")
			return
		}

		ctx := withWorkspaceRequestID(r.Context(), resolveRequestIDFromRequest(r))
		l := workspace.NewWorkspaceUnpublishLogic(ctx, svcCtx)
		resp, err := l.WorkspaceUnpublish(&req)
		if err != nil {
			writeWorkspaceError(w, r, err, "unpublish")
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
