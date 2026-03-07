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

// 获取所有 workspace 配置列表
func WorkspaceConfigsListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WorkspaceConfigsListRequest
		if err := requireWorkspacePermission(r.Context(), svcCtx, "read"); err != nil {
			writeWorkspaceError(w, r, err, "list")
			return
		}
		l := workspace.NewWorkspaceConfigLogic(r.Context(), svcCtx)
		resp, err := l.ListConfigs(&req)
		if err != nil {
			writeWorkspaceError(w, r, err, "list")
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
