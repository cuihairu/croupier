package config

import (
	"net/http"

	configlogic "github.com/cuihairu/croupier/services/server/internal/logic/config"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ConfigItemVersionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		l := configlogic.NewConfigItemVersionsLogic(r.Context(), svcCtx)
		resp, err := l.ConfigItemVersions(id)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
