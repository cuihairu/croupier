package workspace

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/workspace"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type workspaceRollbackRequest struct {
	ObjectKey string `path:"objectKey"`
	VersionID string `json:"versionId"`
}

func WorkspaceRollbackHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req workspaceRollbackRequest
		if err := requireWorkspacePermission(r.Context(), svcCtx, "rollback"); err != nil {
			writeWorkspaceError(w, r, err, "rollback")
			return
		}
		if err := httpx.Parse(r, &req); err != nil {
			writeWorkspaceError(w, r, err, "rollback")
			return
		}

		ctx := withWorkspaceRequestID(r.Context(), resolveRequestIDFromRequest(r))
		l := workspace.NewWorkspaceRollbackLogic(ctx, svcCtx)
		resp, err := l.Rollback(req.ObjectKey, req.VersionID)
		if err != nil {
			writeWorkspaceError(w, r, err, "rollback")
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
