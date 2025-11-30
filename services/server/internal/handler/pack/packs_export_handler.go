// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pack

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/cuihairu/croupier/services/server/internal/logic/pack"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 导出功能包
func PacksExportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PacksExportRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := pack.NewPacksExportLogic(r.Context(), svcCtx)
		resp, err := l.PacksExport(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		filename := resp.Filename
		if filename == "" {
			filename = "packs-export.tar.gz"
		}
		disposition := fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", filename, url.PathEscape(filename))
		w.Header().Set("Content-Type", resp.ContentType)
		w.Header().Set("Content-Disposition", disposition)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp.Content)
	}
}
