// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic/analytics"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 获取等级分析
func AnalyticsLevelsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AnalyticsLevelsRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := analytics.NewAnalyticsLevelsLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsLevels(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
