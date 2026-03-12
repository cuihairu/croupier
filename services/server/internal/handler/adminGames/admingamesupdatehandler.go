// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package adminGames

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/adminGames"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 更新管理员的游戏访问权限
func AdminGamesUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminGamesUpdateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := adminGames.NewAdminGamesUpdateLogic(r.Context(), svcCtx)
		err := l.AdminGamesUpdate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.Ok(w)
		}
	}
}
