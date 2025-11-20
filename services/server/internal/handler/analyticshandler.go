package handler

import (
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/logic"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AnalyticsOverviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsOverviewQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsOverviewLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsOverview(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsRealtimeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsRealtimeQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsRealtimeLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsRealtime(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsRealtimeSeriesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsRealtimeSeriesQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsRealtimeSeriesLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsRealtimeSeries(&req)
		if err != nil {
			if err == logic.ErrInvalidRequest {
				httpx.WriteJsonCtx(r.Context(), w, http.StatusBadRequest, map[string]string{"message": "invalid request"})
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsBehaviorEventsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsBehaviorEventsQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsBehaviorEventsLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsBehaviorEvents(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsBehaviorFunnelHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsBehaviorFunnelQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsBehaviorFunnelLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsBehaviorFunnel(&req)
		if err != nil {
			if err == logic.ErrInvalidRequest {
				httpx.WriteJsonCtx(r.Context(), w, http.StatusBadRequest, map[string]string{"message": "invalid request"})
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsBehaviorPathsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsBehaviorPathsQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsBehaviorPathsLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsBehaviorPaths(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsBehaviorAdoptionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsBehaviorAdoptionQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsBehaviorAdoptionLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsBehaviorAdoption(&req)
		if err != nil {
			if err == logic.ErrInvalidRequest {
				httpx.WriteJsonCtx(r.Context(), w, http.StatusBadRequest, map[string]string{"message": "invalid request"})
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsBehaviorAdoptionBreakdownHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsBehaviorAdoptionBreakdownQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsBehaviorAdoptionBreakdownLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsBehaviorAdoptionBreakdown(&req)
		if err != nil {
			if err == logic.ErrInvalidRequest {
				httpx.WriteJsonCtx(r.Context(), w, http.StatusBadRequest, map[string]string{"message": "invalid request"})
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsIngestHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:manage") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsIngestRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewAnalyticsIngestLogic(r.Context(), svcCtx)
		if err := l.AnalyticsIngest(&req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		httpx.OkJsonCtx(r.Context(), w, map[string]bool{"ok": true})
	}
}

func AnalyticsPaymentsIngestHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:manage") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsPaymentsIngestRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := logic.NewAnalyticsPaymentsIngestLogic(r.Context(), svcCtx)
		if err := l.AnalyticsPaymentsIngest(&req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		httpx.OkJsonCtx(r.Context(), w, map[string]bool{"ok": true})
	}
}

func AnalyticsPaymentsSummaryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsPaymentsSummaryQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsPaymentsSummaryLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsPaymentsSummary(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsPaymentsTransactionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsPaymentsTransactionsQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsPaymentsTransactionsLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsPaymentsTransactions(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsPaymentsProductTrendHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsPaymentsProductTrendQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsPaymentsProductTrendLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsPaymentsProductTrend(&req)
		if err != nil {
			if err == logic.ErrInvalidRequest {
				httpx.WriteJsonCtx(r.Context(), w, http.StatusBadRequest, map[string]string{"message": "invalid request"})
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsLevelsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsLevelsQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsLevelsLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsLevels(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsLevelsEpisodesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsLevelsEpisodesQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsLevelsEpisodesLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsLevelsEpisodes(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsLevelsMapsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsLevelsEpisodesQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsLevelsMapsLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsLevelsMaps(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

func AnalyticsRetentionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := svcCtx.Authenticate(r)
		if !ok {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}
		if !svcCtx.EnforcePermission(user, roles, "analytics:read") {
			httpx.WriteJsonCtx(r.Context(), w, http.StatusForbidden, map[string]string{"message": "forbidden"})
			return
		}
		var req types.AnalyticsRetentionQuery
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		req.GameId, req.Env = resolveAnalyticsScope(r, req.GameId, req.Env)
		l := logic.NewAnalyticsRetentionLogic(r.Context(), svcCtx)
		resp, err := l.AnalyticsRetention(&req)
		if err != nil {
			if err == logic.ErrInvalidRequest {
				httpx.WriteJsonCtx(r.Context(), w, http.StatusBadRequest, map[string]string{"message": "invalid request"})
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
