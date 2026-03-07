package workspace

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/workspace"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type workspaceVersionDetailRequest struct {
	ObjectKey string `path:"objectKey"`
	VersionID string `path:"versionId"`
}

func WorkspaceVersionDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req workspaceVersionDetailRequest
		if err := requireWorkspacePermission(r.Context(), svcCtx, "read"); err != nil {
			writeWorkspaceError(w, r, err, "versions_detail")
			return
		}
		if err := httpx.Parse(r, &req); err != nil {
			writeWorkspaceError(w, r, err, "versions_detail")
			return
		}

		l := workspace.NewWorkspaceVersionsLogic(r.Context(), svcCtx)
		item, err := l.Detail(req.ObjectKey, req.VersionID)
		if err != nil {
			writeWorkspaceError(w, r, err, "versions_detail")
			return
		}
		httpx.OkJsonCtx(r.Context(), w, item)
	}
}
