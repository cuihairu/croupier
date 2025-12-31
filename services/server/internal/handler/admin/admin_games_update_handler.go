package admin

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/admin"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 更新管理员游戏范围
func AdminGamesUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminGamesUpdateRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := admin.NewAdminGamesUpdateLogic(r.Context(), svcCtx)
		resp, err := l.AdminGamesUpdate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
