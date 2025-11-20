// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func PacksExportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewPacksExportLogic(r.Context(), svcCtx)
		if et := l.PackETag(); et != "" {
			w.Header().Set("ETag", et)
		}
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", "attachment; filename=pack.tgz")
		if err := l.PacksExport(w); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
	}
}
