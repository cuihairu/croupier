package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func UploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "uploads:write") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		if svcCtx.ObjStore() == nil {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusServiceUnavailable, map[string]string{"message": "storage unavailable"})
			return
		}
		const maxSize = 120 * 1024 * 1024
		r.Body = http.MaxBytesReader(w, r.Body, maxSize)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusBadRequest, map[string]string{"message": "invalid multipart form"})
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusBadRequest, map[string]string{"message": "missing file"})
			return
		}
		l := logic.NewUploadLogic(r.Context(), svcCtx)
		resp, err := l.Upload(user, file, header)
		if err != nil {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
