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

// 保存 Workspace 配置（创建或更新）
func WorkspaceConfigSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WorkspaceConfigSaveRequest
		if err := requireWorkspacePermission(r.Context(), svcCtx, "edit"); err != nil {
			writeWorkspaceError(w, r, err, "save")
			return
		}
		if err := httpx.Parse(r, &req); err != nil {
			writeWorkspaceError(w, r, err, "save")
			return
		}

		ctx := withWorkspaceRequestID(r.Context(), resolveRequestIDFromRequest(r))
		l := workspace.NewWorkspaceConfigSaveLogic(ctx, svcCtx)
		resp, err := l.WorkspaceConfigSave(&req)
		if err != nil {
			writeWorkspaceError(w, r, err, "save")
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
