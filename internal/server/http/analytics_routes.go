package httpserver

import (
	"github.com/gin-gonic/gin"
	"strings"
	"time"
)

// addAnalyticsRoutes registers analytics APIs.
func (s *Server) addAnalyticsRoutes(r *gin.Engine) {
	// Overview KPI (best-effort CH queries)
	r.GET("/api/analytics/overview", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"series": gin.H{"new_users": []any{}, "peak_online": []any{}, "revenue_cents": []any{}}})
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			t2 := time.Now()
			t1 := t2.Add(-7 * 24 * time.Hour)
			start = t1.Format(time.RFC3339)
			end = t2.Format(time.RFC3339)
		}
		wGame := ""
		wEnv := ""
		var args []any
		args = append(args, start, end)
		if game != "" {
			wGame = " AND game_id = ?"
			args = append(args, game)
		}
		if env != "" {
			wEnv = " AND env = ?"
			args = append(args, env)
		}
		newRows := []any{}
		if rows, err := s.ch.Query(c, "SELECT toDate(d) AS d, sum(new_users) FROM analytics.daily_users WHERE d BETWEEN toDate(?) AND toDate(?)"+wGame+wEnv+" GROUP BY d ORDER BY d", args...); err == nil {
			for rows.Next() {
				var d time.Time
				var v uint64
				_ = rows.Scan(&d, &v)
				newRows = append(newRows, []any{d.Format(time.RFC3339), v})
			}
			rows.Close()
		}
		peakRows := []any{}
		if rows, err := s.ch.Query(c, "SELECT d, maxMerge(peak_online) FROM analytics.daily_online_peak WHERE d BETWEEN toDate(?) AND toDate(?)"+wGame+wEnv+" GROUP BY d ORDER BY d", args...); err == nil {
			for rows.Next() {
				var d time.Time
				var v uint64
				_ = rows.Scan(&d, &v)
				peakRows = append(peakRows, []any{d.Format(time.RFC3339), v})
			}
			rows.Close()
		}
		revRows := []any{}
		if rows, err := s.ch.Query(c, "SELECT toDate(d) AS d, sum(revenue_cents) FROM analytics.daily_revenue WHERE d BETWEEN toDate(?) AND toDate(?)"+wGame+wEnv+" GROUP BY d ORDER BY d", args...); err == nil {
			for rows.Next() {
				var d time.Time
				var v uint64
				_ = rows.Scan(&d, &v)
				revRows = append(revRows, []any{d.Format(time.RFC3339), v})
			}
			rows.Close()
		}
		// summary today
		today := time.Now().Format("2006-01-02")
		dau, newUsers, revenue := uint64(0), uint64(0), uint64(0)
		_ = s.ch.QueryRow(c, "SELECT sum(dau) FROM analytics.daily_users WHERE d=toDate(?)"+wGame+wEnv, append([]any{today}, args[2:]...)...).Scan(&dau)
		_ = s.ch.QueryRow(c, "SELECT sum(new_users) FROM analytics.daily_users WHERE d=toDate(?)"+wGame+wEnv, append([]any{today}, args[2:]...)...).Scan(&newUsers)
		_ = s.ch.QueryRow(c, "SELECT sum(revenue_cents) FROM analytics.daily_revenue WHERE d=toDate(?)"+wGame+wEnv, append([]any{today}, args[2:]...)...).Scan(&revenue)
		var payers uint64
		_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.payments WHERE status='success' AND time>=toDateTime(?) AND time<toDateTime(?)"+wGame+wEnv, append([]any{today + " 00:00:00", today + " 23:59:59"}, args[2:]...)...).Scan(&payers)
		payRate, arpu, arppu := 0.0, 0.0, 0.0
		if dau > 0 {
			arpu = float64(revenue) / float64(dau)
		}
		if payers > 0 {
			arppu = float64(revenue) / float64(payers)
		}
		if dau > 0 {
			payRate = float64(payers) * 100.0 / float64(dau)
		}
		c.JSON(200, gin.H{"dau": dau, "new_users": newUsers, "revenue_cents": revenue, "pay_rate": payRate, "arpu": arpu, "arppu": arppu, "series": gin.H{"new_users": newRows, "peak_online": peakRows, "revenue_cents": revRows}})
	})

	// Realtime (best-effort CH queries)
	r.GET("/api/analytics/realtime", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		out := gin.H{"online": 0, "active_5m": 0, "rev_5m": 0, "pay_succ_rate": 0, "registered_total": 0, "online_peak_today": 0, "online_peak_all_time": 0}
		if s.ch == nil {
			c.JSON(200, out)
			return
		}
		wGame := ""
		wEnv := ""
		var args []any
		if game != "" {
			wGame = " AND game_id=?"
			args = append(args, game)
		}
		if env != "" {
			wEnv = " AND env=?"
			args = append(args, env)
		}
		_ = s.ch.QueryRow(c, "SELECT online FROM analytics.minute_online WHERE 1=1"+wGame+wEnv+" ORDER BY m DESC LIMIT 1", args...).Scan(&out["online"])
		_ = s.ch.QueryRow(c, "SELECT sum(online) FROM analytics.minute_online WHERE m>now()-interval 5 minute"+wGame+wEnv, args...).Scan(&out["active_5m"])
		var succ, total, rev5 uint64
		_ = s.ch.QueryRow(c, "SELECT sumIf(amount_cents, status='success') FROM analytics.payments WHERE time>now()-interval 5 minute"+wGame+wEnv, args...).Scan(&rev5)
		_ = s.ch.QueryRow(c, "SELECT countIf(status='success'), count() FROM analytics.payments WHERE time>now()-interval 5 minute"+wGame+wEnv, args...).Scan(&succ, &total)
		out["rev_5m"] = rev5
		if total > 0 {
			out["pay_succ_rate"] = float64(succ) * 100.0 / float64(total)
		}
		_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events WHERE event IN ('register','first_active')"+wGame+wEnv, args...).Scan(&out["registered_total"])
		_ = s.ch.QueryRow(c, "SELECT maxMerge(peak_online) FROM analytics.daily_online_peak WHERE d=today()"+wGame+wEnv, args...).Scan(&out["online_peak_today"])
		_ = s.ch.QueryRow(c, "SELECT maxMerge(peak_online) FROM analytics.daily_online_peak WHERE 1=1"+wGame+wEnv, args...).Scan(&out["online_peak_all_time"])
		c.JSON(200, out)
	})

	// Behavior (placeholders)
	r.GET("/api/analytics/behavior/events", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		c.JSON(200, gin.H{"events": []any{}, "total": 0})
	})
	r.GET("/api/analytics/behavior/funnel", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		c.JSON(200, gin.H{"steps": []any{}})
	})
	// Payments (placeholders)
	r.GET("/api/analytics/payments/summary", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		c.JSON(200, gin.H{"totals": gin.H{"revenue_cents": 0, "refunds_cents": 0, "failed": 0, "success_rate": 0}, "by_channel": []any{}, "by_product": []any{}})
	})
	r.GET("/api/analytics/payments/failures", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		c.JSON(200, gin.H{"top_reasons": []any{}})
	})
	r.GET("/api/analytics/payments/transactions", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		c.JSON(200, gin.H{"transactions": []any{}, "total": 0})
	})
	// Levels (placeholder)
	r.GET("/api/analytics/levels", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		c.JSON(200, gin.H{"funnel": []any{}, "per_level": []any{}})
	})
	// Ingest endpoints
	r.POST("/api/analytics/ingest", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:manage"); !ok {
			return
		}
		var arr []map[string]any
		if err := c.BindJSON(&arr); err != nil {
			s.respondError(c, 400, "bad_request", "invalid payload")
			return
		}
		for _, e := range arr {
			_ = s.analyticsMQ.PublishEvent(e)
		}
		c.Status(202)
	})
	r.POST("/api/analytics/payments/ingest", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:manage"); !ok {
			return
		}
		var arr []map[string]any
		if err := c.BindJSON(&arr); err != nil {
			s.respondError(c, 400, "bad_request", "invalid payload")
			return
		}
		for _, e := range arr {
			_ = s.analyticsMQ.PublishPayment(e)
		}
		c.Status(202)
	})
}
