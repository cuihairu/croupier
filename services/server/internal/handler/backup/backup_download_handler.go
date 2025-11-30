// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package backup

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cuihairu/croupier/services/server/internal/logic/backup"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 下载备份
func BackupDownloadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BackupDownloadRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := backup.NewBackupDownloadLogic(r.Context(), svcCtx)
		payload, err := l.BackupDownload(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		if payload.RedirectURL != "" {
			http.Redirect(w, r, payload.RedirectURL, http.StatusFound)
			return
		}

		if payload.Reader == nil {
			httpx.ErrorCtx(r.Context(), w, fmt.Errorf("备份文件不可用"))
			return
		}
		defer payload.Reader.Close()

		filename := payload.Filename
		if filename == "" {
			filename = req.ID + ".bak"
		}
		disposition := fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s",
			filename, url.PathEscape(filename))

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", disposition)
		if payload.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(payload.Size, 10))
		}

		if _, err := io.Copy(w, payload.Reader); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
	}
}
