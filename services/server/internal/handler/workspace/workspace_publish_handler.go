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

// 发布 Workspace 配置
func WorkspacePublishHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WorkspacePublishRequest
		if err := requireWorkspacePermission(r.Context(), svcCtx, "publish"); err != nil {
			writeWorkspaceError(w, r, err, "publish")
			return
		}
		if err := httpx.Parse(r, &req); err != nil {
			writeWorkspaceError(w, r, err, "publish")
			return
		}

		ctx := withWorkspaceRequestID(r.Context(), resolveRequestIDFromRequest(r))
		l := workspace.NewWorkspacePublishLogic(ctx, svcCtx)
		resp, err := l.WorkspacePublish(&req)
		if err != nil {
			writeWorkspaceError(w, r, err, "publish")
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
