package handler

import (
	"errors"
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func OpsRateLimitsUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RateLimitRulesRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewOpsRateLimitsUpdateLogic(r.Context(), svcCtx)
		resp, err := l.OpsRateLimitsUpdate(&req)
		if err != nil {
			if errors.Is(err, logic.ErrRateRuleInvalid) {
				httpx.WriteJsonCtx(r.Context(), w, http.StatusBadRequest, map[string]any{"message": err.Error()})
			} else {
				httpx.ErrorCtx(r.Context(), w, err)
			}
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
