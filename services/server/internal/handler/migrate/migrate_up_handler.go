// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package migrate

import (
	"net/http"

	migratelogic "github.com/cuihairu/croupier/services/server/internal/logic/migrate"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 执行迁移
func MigrateUpHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MigrateUpRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := migratelogic.NewMigrateUpLogic(r.Context(), svcCtx)
		resp, err := l.MigrateUp(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
