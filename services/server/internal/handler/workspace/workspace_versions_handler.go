package workspace

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/workspace"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type workspaceVersionsRequest struct {
	ObjectKey string `path:"objectKey"`
	From      string `form:"from,optional"`
	To        string `form:"to,optional"`
}

func WorkspaceVersionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req workspaceVersionsRequest
		if err := requireWorkspacePermission(r.Context(), svcCtx, "read"); err != nil {
			writeWorkspaceError(w, r, err, "versions_list")
			return
		}
		if err := httpx.Parse(r, &req); err != nil {
			writeWorkspaceError(w, r, err, "versions_list")
			return
		}

		l := workspace.NewWorkspaceVersionsLogic(r.Context(), svcCtx)
		items, err := l.List(req.ObjectKey, req.From, req.To)
		if err != nil {
			writeWorkspaceError(w, r, err, "versions_list")
			return
		}
		httpx.OkJsonCtx(r.Context(), w, map[string]interface{}{
			"items":                   items,
			"currentDraftVersion":     resolveCurrentVersion(items, "isCurrentDraft"),
			"currentPublishedVersion": resolveCurrentVersion(items, "isCurrentPublished"),
		})
	}
}

func resolveCurrentVersion(items []map[string]interface{}, flagKey string) int {
	for i := range items {
		flag, ok := items[i][flagKey].(bool)
		if !ok || !flag {
			continue
		}
		if version, ok := items[i]["version"].(int); ok {
			return version
		}
	}
	return 0
}
