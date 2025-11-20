// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"io"
	"net/http"
	"os"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func PacksImportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer file.Close()
		tmp, err := os.CreateTemp("", "pack-*.tar.gz")
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		tmpPath := tmp.Name()
		if _, err := io.Copy(tmp, file); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		tmp.Close()
		defer os.Remove(tmpPath)
		l := logic.NewPacksImportLogic(r.Context(), svcCtx)
		resp, err := l.PacksImport(tmpPath)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
