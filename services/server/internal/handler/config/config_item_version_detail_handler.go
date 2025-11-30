package config

import (
	"net/http"
	"strconv"

	configlogic "github.com/cuihairu/croupier/services/server/internal/logic/config"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ConfigItemVersionDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		versionStr := r.PathValue("version")
		version, _ := strconv.Atoi(versionStr)
		l := configlogic.NewConfigItemVersionDetailLogic(r.Context(), svcCtx)
		resp, err := l.ConfigItemVersionDetail(id, version)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
