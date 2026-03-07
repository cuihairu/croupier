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

// 获取已发布的 Workspace 配置列表
func WorkspacePublishedListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WorkspacePublishedListRequest
		if err := requireWorkspacePermission(r.Context(), svcCtx, "read"); err != nil {
			writeWorkspaceError(w, r, err, "published_list")
			return
		}
		if err := httpx.Parse(r, &req); err != nil {
			writeWorkspaceError(w, r, err, "published_list")
			return
		}

		l := workspace.NewWorkspacePublishedListLogic(r.Context(), svcCtx)
		resp, err := l.WorkspacePublishedList(&req)
		if err != nil {
			writeWorkspaceError(w, r, err, "published_list")
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
