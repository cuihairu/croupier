// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/storage"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 删除对象
func DeleteObjectHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DeleteObjectRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := storage.NewDeleteObjectLogic(r.Context(), svcCtx)
		resp, err := l.DeleteObject(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
