package admin

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/admin"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取管理员游戏范围
func AdminGamesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminGamesRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := admin.NewAdminGamesLogic(r.Context(), svcCtx)
		resp, err := l.AdminGames(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
