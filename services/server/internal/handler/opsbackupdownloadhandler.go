package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func OpsBackupDownloadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Id string `path:"id"`
		}
		if err := httpx.ParsePath(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewOpsBackupDownloadLogic(r.Context(), svcCtx)
		if path, ok := l.OpsBackupDownload(req.Id); ok {
			http.ServeFile(w, r, path)
		} else {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusNotFound, map[string]any{"message": "backup not found"})
		}
	}
}
