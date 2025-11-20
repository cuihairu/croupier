package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/api/internal/logic"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GameUpsertHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GameUpsertRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewGameUpsertLogic(r.Context(), svcCtx)
		resp, err := l.GameUpsert(&req)
		if err != nil {
			writeGameError(r.Context(), w, err)
		} else {
			if req.Id <= 0 {
				w.WriteHeader(http.StatusCreated)
			}
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
