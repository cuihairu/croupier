package httpserver

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
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
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
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
		// WAU/MAU（去重用户数，直接从 events 去重）
		wau, mau := uint64(0), uint64(0)
		_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events WHERE event_time>=now()-interval 7 day"+wGame+wEnv, args[2:]...).Scan(&wau)
		_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events WHERE event_time>=now()-interval 30 day"+wGame+wEnv, args[2:]...).Scan(&mau)
		// Retention（基于昨天 cohort：注册或首次活跃，D1/D7/D30；任意事件视为活跃）
		y := time.Now().Add(-24 * time.Hour)
		ymd := y.Format("2006-01-02")
		var total uint64
		_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events WHERE toDate(event_time)=toDate(?) AND event IN ('register','first_active')"+wGame+wEnv, append([]any{ymd}, args[2:]...)...).Scan(&total)
		calcRet := func(offsetDays int) float64 {
			if total == 0 {
				return 0
			}
			tgt := y.Add(time.Duration(offsetDays) * 24 * time.Hour).Format("2006-01-02")
			var kept uint64
			_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events WHERE toDate(event_time)=toDate(?)"+wGame+wEnv+" AND user_id IN (SELECT user_id FROM analytics.events WHERE toDate(event_time)=toDate(?) AND event IN ('register','first_active')"+wGame+wEnv+")",
				append(append([]any{tgt, ymd}, args[2:]...), args[2:]...)...).Scan(&kept)
			return math.Round((float64(kept) * 10000.0 / float64(total))) / 100.0
		}
		d1 := calcRet(1)
		d7 := calcRet(7)
		d30 := calcRet(30)
		c.JSON(200, gin.H{
			"dau": dau, "wau": wau, "mau": mau,
			"new_users": newUsers, "revenue_cents": revenue, "pay_rate": payRate, "arpu": arpu, "arppu": arppu,
			"d1": d1, "d7": d7, "d30": d30,
			"series": gin.H{"new_users": newRows, "peak_online": peakRows, "revenue_cents": revRows},
		})
	})

	// Realtime (best-effort CH queries)
	r.GET("/api/analytics/realtime", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
        game := strings.TrimSpace(c.Query("game_id"))
        env := strings.TrimSpace(c.Query("env"))
        if game == "" { game = strings.TrimSpace(c.Request.Header.Get("X-Game-ID")) }
        if env == "" { env = strings.TrimSpace(c.Request.Header.Get("X-Env")) }
    // extend realtime payload: include today's recharge total (cents)
    // NOTE: UI expects *_yuan fields to always exist with two decimals even when CH is disabled.
    // Initialize with zeros so the nil-CH branch still returns "0.00".
    out := gin.H{"online": 0, "active_1m": 0, "active_5m": 0, "active_15m": 0, "rev_5m": 0, "rev_today": 0, "rev_5m_yuan": "0.00", "rev_today_yuan": "0.00", "pay_succ_rate": 0.0, "registered_total": 0, "online_peak_today": 0, "online_peak_all_time": 0}
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
		var online, active1, active5, active15 uint64
		_ = s.ch.QueryRow(c, "SELECT online FROM analytics.minute_online WHERE 1=1"+wGame+wEnv+" ORDER BY m DESC LIMIT 1", args...).Scan(&online)
		_ = s.ch.QueryRow(c, "SELECT sum(online) FROM analytics.minute_online WHERE m>now()-interval 1 minute"+wGame+wEnv, args...).Scan(&active1)
		_ = s.ch.QueryRow(c, "SELECT sum(online) FROM analytics.minute_online WHERE m>now()-interval 5 minute"+wGame+wEnv, args...).Scan(&active5)
		_ = s.ch.QueryRow(c, "SELECT sum(online) FROM analytics.minute_online WHERE m>now()-interval 15 minute"+wGame+wEnv, args...).Scan(&active15)
    var succ, total, rev5 uint64
    _ = s.ch.QueryRow(c, "SELECT sumIf(amount_cents, status='success') FROM analytics.payments WHERE time>now()-interval 5 minute"+wGame+wEnv, args...).Scan(&rev5)
    _ = s.ch.QueryRow(c, "SELECT countIf(status='success'), count() FROM analytics.payments WHERE time>now()-interval 5 minute"+wGame+wEnv, args...).Scan(&succ, &total)
    // Today's recharge (successful payments only), cents since start of day
    var revToday uint64
    _ = s.ch.QueryRow(c, "SELECT sumIf(amount_cents, status='success') FROM analytics.payments WHERE time>=toStartOfDay(now())"+wGame+wEnv, args...).Scan(&revToday)
		// Try Redis HLL overlay for low-latency online/active_5m (optional)
		// Keys: hll:online:<game_id>:<env>:YYYYMMDDHHmm (current minute and previous 4 minutes)
		if url := strings.TrimSpace(os.Getenv("REDIS_URL")); url != "" {
			if opt, err := redis.ParseURL(url); err == nil {
				rc := redis.NewClient(opt)
				ctx, cancel := context.WithTimeout(c, 600*time.Millisecond)
				defer cancel()
				now := time.Now()
				// Online = PFCOUNT(current minute key)
				curKey := fmt.Sprintf("hll:online:%s:%s:%s", game, env, now.Truncate(time.Minute).Format("200601021504"))
				if n, err2 := rc.PFCount(ctx, curKey).Result(); err2 == nil && n >= 0 {
					online = uint64(n)
				}
				// Active_1m = PFCOUNT(current minute)
				if n, err2 := rc.PFCount(ctx, curKey).Result(); err2 == nil && n >= 0 {
					active1 = uint64(n)
				}
				// Active_5m/15m = PFMERGE of recent keys -> temp -> PFCOUNT
				mergeCount := func(minutes int) uint64 {
					keys := []string{}
					for i := 0; i < minutes; i++ {
						t := now.Add(time.Duration(-i) * time.Minute)
						keys = append(keys, fmt.Sprintf("hll:online:%s:%s:%s", game, env, t.Truncate(time.Minute).Format("200601021504")))
					}
					tmp := fmt.Sprintf("tmp:hll:online:%d:%d", minutes, now.UnixNano())
					if err2 := rc.PFMerge(ctx, tmp, keys...).Err(); err2 == nil {
						if n, err3 := rc.PFCount(ctx, tmp).Result(); err3 == nil && n >= 0 {
							_ = rc.Expire(ctx, tmp, 2*time.Second).Err()
							return uint64(n)
						}
						_ = rc.Expire(ctx, tmp, 2*time.Second).Err()
					}
					return 0
				}
				active5 = mergeCount(5)
				active15 = mergeCount(15)
				_ = rc.Close()
			}
		}
		out["online"] = online
		out["active_1m"] = active1
		out["active_5m"] = active5
    out["active_15m"] = active15
    out["rev_5m"] = rev5
    out["rev_today"] = revToday
    // Also expose yuan amounts with 2 decimals for UI convenience
    out["rev_5m_yuan"] = fmt.Sprintf("%.2f", float64(rev5)/100.0)
    out["rev_today_yuan"] = fmt.Sprintf("%.2f", float64(revToday)/100.0)
		if total > 0 {
			out["pay_succ_rate"] = float64(succ) * 100.0 / float64(total)
		}
		var regTotal, peakToday, peakAll uint64
		_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events WHERE event IN ('register','first_active')"+wGame+wEnv, args...).Scan(&regTotal)
		_ = s.ch.QueryRow(c, "SELECT maxMerge(peak_online) FROM analytics.daily_online_peak WHERE d=today()"+wGame+wEnv, args...).Scan(&peakToday)
		_ = s.ch.QueryRow(c, "SELECT maxMerge(peak_online) FROM analytics.daily_online_peak WHERE 1=1"+wGame+wEnv, args...).Scan(&peakAll)
		out["registered_total"] = regTotal
		out["online_peak_today"] = peakToday
		out["online_peak_all_time"] = peakAll
		// Today DAU and New Users (prefer Redis HLL, fallback CH)
		today := time.Now().Format("2006-01-02")
		var dauToday, newToday uint64
		if url := strings.TrimSpace(os.Getenv("REDIS_URL")); url != "" {
			if opt, err := redis.ParseURL(url); err == nil {
				rc := redis.NewClient(opt)
				ctx, cancel := context.WithTimeout(c, 500*time.Millisecond)
				defer cancel()
				kDau := fmt.Sprintf("hll:dau:%s:%s:%s", game, env, today)
				kNew := fmt.Sprintf("hll:new:%s:%s:%s", game, env, today)
				if n, err2 := rc.PFCount(ctx, kDau).Result(); err2 == nil && n >= 0 {
					dauToday = uint64(n)
				}
				if n, err2 := rc.PFCount(ctx, kNew).Result(); err2 == nil && n >= 0 {
					newToday = uint64(n)
				}
				_ = rc.Close()
			}
		}
		if dauToday == 0 {
			_ = s.ch.QueryRow(c, "SELECT sum(dau) FROM analytics.daily_users WHERE d=toDate(?)"+wGame+wEnv, append([]any{today}, args...)...).Scan(&dauToday)
		}
		if newToday == 0 {
			_ = s.ch.QueryRow(c, "SELECT sum(new_users) FROM analytics.daily_users WHERE d=toDate(?)"+wGame+wEnv, append([]any{today}, args...)...).Scan(&newToday)
		}
		out["dau_today"] = dauToday
		out["new_today"] = newToday
		c.JSON(200, out)
	})

	// Realtime series (minute buckets): online per minute, revenue_cents per minute
	r.GET("/api/analytics/realtime/series", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"online": []any{}, "revenue_cents": []any{}})
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			c.JSON(400, gin.H{"code": "bad_request", "message": "start/end required"})
			return
		}
		wGame := ""
		wEnv := ""
		var args []any
		args = append(args, start, end)
		if game != "" {
			wGame = " AND game_id=?"
			args = append(args, game)
		}
		if env != "" {
			wEnv = " AND env=?"
			args = append(args, env)
		}
		// online per minute + rolling sums (approx active_5m/active_15m by sum)
		onRows := []any{}
		act5Rows := []any{}
		act15Rows := []any{}
		if rows, err := s.ch.Query(c, "SELECT m, online, sum(online) OVER (ORDER BY m ROWS BETWEEN 4 PRECEDING AND CURRENT ROW) AS a5, sum(online) OVER (ORDER BY m ROWS BETWEEN 14 PRECEDING AND CURRENT ROW) AS a15 FROM analytics.minute_online WHERE m>=toDateTime(?) AND m<=toDateTime(?)"+wGame+wEnv+" ORDER BY m", args...); err == nil {
			for rows.Next() {
				var m time.Time
				var v, a5, a15 uint64
				_ = rows.Scan(&m, &v, &a5, &a15)
				onRows = append(onRows, []any{m.Format(time.RFC3339), v})
				act5Rows = append(act5Rows, []any{m.Format(time.RFC3339), a5})
				act15Rows = append(act15Rows, []any{m.Format(time.RFC3339), a15})
			}
			rows.Close()
		}
		// revenue per minute (success only)
		revRows := []any{}
		if rows, err := s.ch.Query(c, "SELECT toStartOfMinute(time) AS m, sumIf(amount_cents,status='success') AS rev FROM analytics.payments WHERE time>=toDateTime(?) AND time<=toDateTime(?)"+wGame+wEnv+" GROUP BY m ORDER BY m", args...); err == nil {
			for rows.Next() {
				var m time.Time
				var v uint64
				_ = rows.Scan(&m, &v)
				revRows = append(revRows, []any{m.Format(time.RFC3339), v})
			}
			rows.Close()
		}
		c.JSON(200, gin.H{"online": onRows, "active_5m_sum": act5Rows, "active_15m_sum": act15Rows, "revenue_cents": revRows})
	})

	// Behavior (placeholders)
	r.GET("/api/analytics/behavior/events", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"events": []any{}, "total": 0})
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		event := strings.TrimSpace(c.Query("event"))
		propKey := strings.TrimSpace(c.Query("prop_key"))
		propVal := strings.TrimSpace(c.Query("prop_val"))
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
		if page <= 0 {
			page = 1
		}
		if size <= 0 || size > 500 {
			size = 50
		}
		off := (page - 1) * size
		if start == "" || end == "" {
			t2 := time.Now()
			t1 := t2.Add(-24 * time.Hour)
			start = t1.Format(time.RFC3339)
			end = t2.Format(time.RFC3339)
		}
		w := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
		var args []any
		args = append(args, start, end)
		if game != "" {
			w += " AND game_id=?"
			args = append(args, game)
		}
		if env != "" {
			w += " AND env=?"
			args = append(args, env)
		}
		if event != "" {
			w += " AND event=?"
			args = append(args, event)
		}
		// Optional property exact match，限制 key 为 [A-Za-z0-9_]
		if propKey != "" && propVal != "" {
			valid := true
			for _, ch := range propKey {
				if !(ch == '_' || ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z') {
					valid = false
					break
				}
			}
			if valid {
				// JSON_VALUE(props_json,'$.key') = propVal
				w += " AND JSON_VALUE(props_json, '$." + propKey + "') = ?"
				args = append(args, propVal)
			}
		}
		// total
		var total uint64
		_ = s.ch.QueryRow(c, "SELECT count() FROM analytics.events"+w, args...).Scan(&total)
		qry := "SELECT event_time, event, user_id FROM analytics.events" + w + " ORDER BY event_time DESC LIMIT " + strconv.Itoa(size) + " OFFSET " + strconv.Itoa(off)
		rows, err := s.ch.Query(c, qry, args...)
		if err != nil {
			c.JSON(200, gin.H{"events": []any{}, "total": total})
			return
		}
		out := []any{}
		for rows.Next() {
			var t time.Time
			var ev, uid string
			_ = rows.Scan(&t, &ev, &uid)
			out = append(out, gin.H{"time": t.Format(time.RFC3339), "event": ev, "user_id": uid})
		}
		rows.Close()
		c.JSON(200, gin.H{"events": out, "total": total})
	})
	// Funnel: steps=a,b,c（逗号）；支持 sequential=1（严格顺序，可设 same_session=1 与 gap_sec）或简化模式（后续步骤属于第一步集合）
	r.GET("/api/analytics/behavior/funnel", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"steps": []any{}})
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		stepsStr := strings.TrimSpace(c.Query("steps"))
		if stepsStr == "" {
			c.JSON(200, gin.H{"steps": []any{}})
			return
		}
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			t2 := time.Now()
			t1 := t2.Add(-7 * 24 * time.Hour)
			start = t1.Format(time.RFC3339)
			end = t2.Format(time.RFC3339)
		}
		steps := []string{}
		for _, s1 := range strings.Split(stepsStr, ",") {
			s1 = strings.TrimSpace(s1)
			if s1 != "" {
				steps = append(steps, s1)
			}
		}
		if len(steps) == 0 {
			c.JSON(200, gin.H{"steps": []any{}})
			return
		}
		sequential := strings.TrimSpace(c.DefaultQuery("sequential", "0")) == "1"
		sameSession := strings.TrimSpace(c.DefaultQuery("same_session", "0")) == "1"
		gapSec, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("gap_sec", "0"))) // 0 表示不限制
		maxUsers, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("max_users", "50000")))
		if maxUsers <= 0 || maxUsers > 200000 {
			maxUsers = 50000
		}

		wBase := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
		var baseArgs []any
		baseArgs = append(baseArgs, start, end)
		if game != "" {
			wBase += " AND game_id=?"
			baseArgs = append(baseArgs, game)
		}
		if env != "" {
			wBase += " AND env=?"
			baseArgs = append(baseArgs, env)
		}

		if !sequential {
			// 简化版：后续步骤用户属于第一步集合
			var first uint64
			_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events "+wBase+" AND event=?", append(baseArgs, steps[0])...).Scan(&first)
			out := make([]any, 0, len(steps))
			firstSetSQL := "SELECT user_id FROM analytics.events" + wBase + " AND event=?"
			firstArgs := append([]any{}, baseArgs...)
			firstArgs = append(firstArgs, steps[0])
			for i, st := range steps {
				var n uint64
				if i == 0 {
					n = first
				} else {
					qry := "SELECT uniqExact(user_id) FROM analytics.events" + wBase + " AND event=? AND user_id IN (" + firstSetSQL + ")"
					args := append([]any{}, baseArgs...)
					args = append(args, st)
					args = append(args, firstArgs...)
					_ = s.ch.QueryRow(c, qry, args...).Scan(&n)
				}
				rate := 0.0
				if first > 0 {
					rate = math.Round((float64(n) * 10000.0 / float64(first))) / 100.0
				}
				out = append(out, gin.H{"step": st, "users": n, "rate": rate})
			}
			c.JSON(200, gin.H{"steps": out})
			return
		}

		// 严格顺序：对每个用户拉取该时间范围内 steps 集合事件的数组（时间/事件/会话），在服务端按顺序规则判定
		// 注意：此方法对大区间可能较重，max_users 默认限制 5 万用户组
		placeholders := make([]string, len(steps))
		for i := range steps {
			placeholders[i] = "?"
		}
		inClause := strings.Join(placeholders, ",")
		qry := "SELECT user_id, groupArray(toUnixTimestamp(event_time)) AS ts, groupArray(event) AS ev, groupArray(session_id) AS sid FROM analytics.events" + wBase + " AND event IN (" + inClause + ") GROUP BY user_id LIMIT " + strconv.Itoa(maxUsers)
		args := append([]any{}, baseArgs...)
		for _, st := range steps {
			args = append(args, st)
		}
		rows, err := s.ch.Query(c, qry, args...)
		if err != nil {
			s.respondError(c, 500, "internal_error", "funnel query failed")
			return
		}
		defer rows.Close()
		counts := make([]uint64, len(steps))
		for rows.Next() {
			var uid string
			var ts []uint64
			var ev []string
			var sid []string
			if err := rows.Scan(&uid, &ts, &ev, &sid); err != nil {
				continue
			}
			// sort by ts if not already
			// simple insertion sort indices
			n := len(ts)
			idx := make([]int, n)
			for i := 0; i < n; i++ {
				idx[i] = i
			}
			// small n expected; O(n^2) is acceptable
			for i := 1; i < n; i++ {
				j := i
				for j > 0 && ts[idx[j-1]] > ts[idx[j]] {
					idx[j-1], idx[j] = idx[j], idx[j-1]
					j--
				}
			}
			// scan in order
			matchedStep := -1
			var lastTs uint64
			sess := ""
			for _, k := range idx {
				e := ev[k]
				t := ts[k]
				s0 := ""
				if k < len(sid) {
					s0 = sid[k]
				}
				// next expected step
				need := steps[matchedStep+1]
				if e != need {
					continue
				}
				if matchedStep >= 0 {
					if gapSec > 0 && t < lastTs+uint64(gapSec) {
						// within gap constraint: OK; if negative gap means no constraint
					}
					if gapSec > 0 && t > lastTs+uint64(gapSec) {
						continue
					}
					if sameSession {
						if sess == "" || s0 == "" || s0 != sess {
							continue
						}
					}
				}
				// accept this step
				matchedStep++
				lastTs = t
				if matchedStep == 0 && sameSession {
					sess = s0
				}
				if matchedStep+1 == len(steps) {
					break
				}
			}
			if matchedStep >= 0 {
				for i := 0; i <= matchedStep && i < len(counts); i++ {
					counts[i]++
				}
			}
		}
		first := counts[0]
		out := make([]any, 0, len(steps))
		for i, st := range steps {
			rate := 0.0
			if first > 0 {
				rate = math.Round((float64(counts[i]) * 10000.0 / float64(first))) / 100.0
			}
			out = append(out, gin.H{"step": st, "users": counts[i], "rate": rate})
		}
		c.JSON(200, gin.H{"steps": out})
	})
	// Payments
	r.GET("/api/analytics/payments/summary", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"totals": gin.H{"revenue_cents": 0, "refunds_cents": 0, "failed": 0, "success_rate": 0}, "by_channel": []any{}, "by_platform": []any{}, "by_country": []any{}})
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		channel := strings.TrimSpace(c.Query("channel"))
		platform := strings.TrimSpace(c.Query("platform"))
		country := strings.TrimSpace(c.Query("country"))
		region := strings.TrimSpace(c.Query("region"))
		city := strings.TrimSpace(c.Query("city"))
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			t2 := time.Now()
			t1 := t2.Add(-7 * 24 * time.Hour)
			start = t1.Format(time.RFC3339)
			end = t2.Format(time.RFC3339)
		}
		w := " WHERE time>=toDateTime(?) AND time<=toDateTime(?)"
		var args []any
		args = append(args, start, end)
		if game != "" {
			w += " AND game_id=?"
			args = append(args, game)
		}
		if env != "" {
			w += " AND env=?"
			args = append(args, env)
		}
		if channel != "" {
			w += " AND channel=?"
			args = append(args, channel)
		}
		if platform != "" {
			w += " AND platform=?"
			args = append(args, platform)
		}
		if country != "" {
			w += " AND country=?"
			args = append(args, country)
		}
		if region != "" {
			w += " AND region=?"
			args = append(args, region)
		}
		if city != "" {
			w += " AND city=?"
			args = append(args, city)
		}
		var revSucc, revRefund uint64
		var succCnt, totalCnt uint64
		_ = s.ch.QueryRow(c, "SELECT sumIf(amount_cents,status='success'), sumIf(amount_cents,status='refund'), countIf(status='success'), count() FROM analytics.payments"+w, args...).Scan(&revSucc, &revRefund, &succCnt, &totalCnt)
		succRate := 0.0
		if totalCnt > 0 {
			succRate = math.Round((float64(succCnt) * 10000.0 / float64(totalCnt))) / 100.0
		}
		// by_channel
		chanRows := []any{}
		if rows, err := s.ch.Query(c, "SELECT channel, sumIf(amount_cents,status='success') as revenue, countIf(status='success') as succ, count() as total FROM analytics.payments"+w+" GROUP BY channel ORDER BY revenue DESC", args...); err == nil {
			for rows.Next() {
				var ch string
				var revenue uint64
				var succ, tot uint64
				_ = rows.Scan(&ch, &revenue, &succ, &tot)
				rate := 0.0
				if tot > 0 {
					rate = math.Round((float64(succ) * 10000.0 / float64(tot))) / 100.0
				}
				chanRows = append(chanRows, gin.H{"channel": ch, "revenue_cents": revenue, "success": succ, "total": tot, "success_rate": rate})
			}
			rows.Close()
		}
		// by_platform
		platRows := []any{}
		if rows, err := s.ch.Query(c, "SELECT platform, sumIf(amount_cents,status='success') as revenue, countIf(status='success') as succ, count() as total FROM analytics.payments"+w+" GROUP BY platform ORDER BY revenue DESC", args...); err == nil {
			for rows.Next() {
				var dim string
				var revenue uint64
				var succ, tot uint64
				_ = rows.Scan(&dim, &revenue, &succ, &tot)
				rate := 0.0
				if tot > 0 {
					rate = math.Round((float64(succ) * 10000.0 / float64(tot))) / 100.0
				}
				platRows = append(platRows, gin.H{"platform": dim, "revenue_cents": revenue, "success": succ, "total": tot, "success_rate": rate})
			}
			rows.Close()
		}
		// by_country
		countryRows := []any{}
		if rows, err := s.ch.Query(c, "SELECT country, sumIf(amount_cents,status='success') as revenue, countIf(status='success') as succ, count() as total FROM analytics.payments"+w+" GROUP BY country ORDER BY revenue DESC", args...); err == nil {
			for rows.Next() {
				var dim string
				var revenue uint64
				var succ, tot uint64
				_ = rows.Scan(&dim, &revenue, &succ, &tot)
				rate := 0.0
				if tot > 0 {
					rate = math.Round((float64(succ) * 10000.0 / float64(tot))) / 100.0
				}
				countryRows = append(countryRows, gin.H{"country": dim, "revenue_cents": revenue, "success": succ, "total": tot, "success_rate": rate})
			}
			rows.Close()
		}
		// by_region (province/state)
		regionRows := []any{}
		if rows, err := s.ch.Query(c, "SELECT region, sumIf(amount_cents,status='success') as revenue, countIf(status='success') as succ, count() as total FROM analytics.payments"+w+" GROUP BY region ORDER BY revenue DESC", args...); err == nil {
			for rows.Next() {
				var dim string
				var revenue uint64
				var succ, tot uint64
				_ = rows.Scan(&dim, &revenue, &succ, &tot)
				rate := 0.0
				if tot > 0 {
					rate = math.Round((float64(succ) * 10000.0 / float64(tot))) / 100.0
				}
				regionRows = append(regionRows, gin.H{"region": dim, "revenue_cents": revenue, "success": succ, "total": tot, "success_rate": rate})
			}
			rows.Close()
		}
		// by_city
		cityRows := []any{}
		if rows, err := s.ch.Query(c, "SELECT city, sumIf(amount_cents,status='success') as revenue, countIf(status='success') as succ, count() as total FROM analytics.payments"+w+" GROUP BY city ORDER BY revenue DESC", args...); err == nil {
			for rows.Next() {
				var dim string
				var revenue uint64
				var succ, tot uint64
				_ = rows.Scan(&dim, &revenue, &succ, &tot)
				rate := 0.0
				if tot > 0 {
					rate = math.Round((float64(succ) * 10000.0 / float64(tot))) / 100.0
				}
				cityRows = append(cityRows, gin.H{"city": dim, "revenue_cents": revenue, "success": succ, "total": tot, "success_rate": rate})
			}
			rows.Close()
		}
		// by_product (if column exists)
		prodRows := []any{}
		if rows, err := s.ch.Query(c, "SELECT product_id, sumIf(amount_cents,status='success') as revenue, countIf(status='success') as succ, count() as total FROM analytics.payments"+w+" GROUP BY product_id ORDER BY revenue DESC", args...); err == nil {
			for rows.Next() {
				var dim string
				var revenue uint64
				var succ, tot uint64
				_ = rows.Scan(&dim, &revenue, &succ, &tot)
				rate := 0.0
				if tot > 0 {
					rate = math.Round((float64(succ) * 10000.0 / float64(tot))) / 100.0
				}
				prodRows = append(prodRows, gin.H{"product_id": dim, "revenue_cents": revenue, "success": succ, "total": tot, "success_rate": rate})
			}
			rows.Close()
		}
		c.JSON(200, gin.H{"totals": gin.H{"revenue_cents": revSucc, "refunds_cents": revRefund, "failed": totalCnt - succCnt, "success_rate": succRate}, "by_channel": chanRows, "by_platform": platRows, "by_country": countryRows, "by_region": regionRows, "by_city": cityRows, "by_product": prodRows})
	})
	r.GET("/api/analytics/payments/transactions", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"transactions": []any{}, "total": 0})
			return
		}
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
		if page <= 0 {
			page = 1
		}
		if size <= 0 || size > 200 {
			size = 20
		}
		off := (page - 1) * size
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		channel := strings.TrimSpace(c.Query("channel"))
		platform := strings.TrimSpace(c.Query("platform"))
		country := strings.TrimSpace(c.Query("country"))
		region := strings.TrimSpace(c.Query("region"))
		city := strings.TrimSpace(c.Query("city"))
		status := strings.TrimSpace(c.Query("status"))
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			t2 := time.Now()
			t1 := t2.Add(-7 * 24 * time.Hour)
			start = t1.Format(time.RFC3339)
			end = t2.Format(time.RFC3339)
		}
		w := " WHERE time>=toDateTime(?) AND time<=toDateTime(?)"
		var args []any
		args = append(args, start, end)
		if game != "" {
			w += " AND game_id=?"
			args = append(args, game)
		}
		if env != "" {
			w += " AND env=?"
			args = append(args, env)
		}
		if channel != "" {
			w += " AND channel=?"
			args = append(args, channel)
		}
		if platform != "" {
			w += " AND platform=?"
			args = append(args, platform)
		}
		if country != "" {
			w += " AND country=?"
			args = append(args, country)
		}
		if region != "" {
			w += " AND region=?"
			args = append(args, region)
		}
		if city != "" {
			w += " AND city=?"
			args = append(args, city)
		}
		if status != "" {
			w += " AND status=?"
			args = append(args, status)
		}
		var total uint64
		_ = s.ch.QueryRow(c, "SELECT count() FROM analytics.payments"+w, args...).Scan(&total)
		qry := "SELECT time, order_id, user_id, amount_cents, status, channel FROM analytics.payments" + w + " ORDER BY time DESC LIMIT " + strconv.Itoa(size) + " OFFSET " + strconv.Itoa(off)
		rows, err := s.ch.Query(c, qry, args...)
		if err != nil {
			c.JSON(200, gin.H{"transactions": []any{}, "total": total})
			return
		}
		out := []any{}
		for rows.Next() {
			var t time.Time
			var oid, uid, st, ch string
			var amt uint64
			_ = rows.Scan(&t, &oid, &uid, &amt, &st, &ch)
			out = append(out, gin.H{"time": t.Format(time.RFC3339), "order_id": oid, "user_id": uid, "amount_cents": amt, "status": st, "channel": ch})
		}
		rows.Close()
		c.JSON(200, gin.H{"transactions": out, "total": total})
	})

	// Payments: product trend (succ/total/revenue over time) for one or multiple product_id
	r.GET("/api/analytics/payments/product_trend", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"products": []any{}})
			return
		}
		productsStr := strings.TrimSpace(c.Query("product_id"))
		if productsStr == "" {
			s.respondError(c, 400, "bad_request", "product_id required")
			return
		}
		gran := strings.ToLower(strings.TrimSpace(c.DefaultQuery("granularity", "hour"))) // minute|hour
		if gran != "minute" {
			gran = "hour"
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		channel := strings.TrimSpace(c.Query("channel"))
		platform := strings.TrimSpace(c.Query("platform"))
		country := strings.TrimSpace(c.Query("country"))
		region := strings.TrimSpace(c.Query("region"))
		city := strings.TrimSpace(c.Query("city"))
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			s.respondError(c, 400, "bad_request", "start/end required")
			return
		}
		prods := []string{}
		for _, p := range strings.Split(productsStr, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				prods = append(prods, p)
			}
		}
		if len(prods) == 0 {
			c.JSON(200, gin.H{"products": []any{}})
			return
		}
		w := " WHERE time>=toDateTime(?) AND time<=toDateTime(?)"
		var args []any
		args = append(args, start, end)
		if game != "" {
			w += " AND game_id=?"
			args = append(args, game)
		}
		if env != "" {
			w += " AND env=?"
			args = append(args, env)
		}
		if channel != "" {
			w += " AND channel=?"
			args = append(args, channel)
		}
		if platform != "" {
			w += " AND platform=?"
			args = append(args, platform)
		}
		if country != "" {
			w += " AND country=?"
			args = append(args, country)
		}
		if region != "" {
			w += " AND region=?"
			args = append(args, region)
		}
		if city != "" {
			w += " AND city=?"
			args = append(args, city)
		}
		// product filter placeholders
		placeholders := make([]string, len(prods))
		for i := range prods {
			placeholders[i] = "?"
			args = append(args, prods[i])
		}
		bucket := "toStartOfHour(time)"
		if gran == "minute" {
			bucket = "toStartOfMinute(time)"
		}
		sql := "SELECT " + bucket + " AS t, product_id, sumIf(amount_cents,status='success') AS rev, countIf(status='success') AS succ, count() AS total FROM analytics.payments" + w + " AND product_id IN (" + strings.Join(placeholders, ",") + ") GROUP BY t, product_id ORDER BY t"
		rows, err := s.ch.Query(c, sql, args...)
		if err != nil {
			s.respondError(c, 500, "internal_error", "trend query failed")
			return
		}
		defer rows.Close()
		type pt struct {
			T              time.Time
			Rev, Succ, Tot uint64
		}
		m := map[string][]pt{}
		for rows.Next() {
			var t time.Time
			var prod string
			var rev, succ, tot uint64
			_ = rows.Scan(&t, &prod, &rev, &succ, &tot)
			m[prod] = append(m[prod], pt{T: t, Rev: rev, Succ: succ, Tot: tot})
		}
		out := []any{}
		for _, p := range prods {
			pts := []any{}
			for _, v := range m[p] {
				pts = append(pts, []any{v.T.Format(time.RFC3339), v.Succ, v.Tot, v.Rev})
			}
			out = append(out, gin.H{"product_id": p, "points": pts})
		}
		c.JSON(200, gin.H{"products": out})
	})
	// Levels (placeholder)
	r.GET("/api/analytics/levels", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"funnel": []any{}, "per_level": []any{}})
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		episode := strings.TrimSpace(c.Query("episode"))
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			t2 := time.Now()
			t1 := t2.Add(-7 * 24 * time.Hour)
			start = t1.Format(time.RFC3339)
			end = t2.Format(time.RFC3339)
		}
		w := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
		var args []any
		args = append(args, start, end)
		if game != "" {
			w += " AND game_id=?"
			args = append(args, game)
		}
		if env != "" {
			w += " AND env=?"
			args = append(args, env)
		}
		if episode != "" {
			w += " AND JSON_VALUE(props_json,'$.episode') = ?"
			args = append(args, episode)
		}
		// overall funnel（按唯一用户）
		var s1, s2, s3 uint64
		_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_start','level_enter','level_begin')", args...).Scan(&s1)
		_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_clear','level_pass','level_win')", args...).Scan(&s2)
		_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_fail','level_lose','level_dead')", args...).Scan(&s3)
		rate := func(n uint64) float64 {
			if s1 == 0 {
				return 0
			}
			return math.Round((float64(n) * 10000.0 / float64(s1))) / 100.0
		}
		funnel := []any{gin.H{"step": "开始关卡", "users": s1, "rate": 100.0}, gin.H{"step": "完成关卡", "users": s2, "rate": rate(s2)}, gin.H{"step": "失败过", "users": s3, "rate": rate(s3)}}
		// per-level stats (all players)
		type KV struct {
			K string
			V uint64
		}
		attempts := map[string]uint64{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_start','level_enter','level_begin') GROUP BY lvl", args...); err == nil {
			for rows.Next() {
				var lvl string
				var n uint64
				_ = rows.Scan(&lvl, &n)
				if strings.TrimSpace(lvl) != "" {
					attempts[lvl] = n
				}
			}
			rows.Close()
		}
		clears := map[string]uint64{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_clear','level_pass','level_win') GROUP BY lvl", args...); err == nil {
			for rows.Next() {
				var lvl string
				var n uint64
				_ = rows.Scan(&lvl, &n)
				if strings.TrimSpace(lvl) != "" {
					clears[lvl] = n
				}
			}
			rows.Close()
		}
		type Stat struct {
			L     string
			Dur   *float64
			Retry *float64
		}
		detail := map[string]Stat{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.level') as lvl, avgOrNull(toFloat64OrNull(JSON_VALUE(props_json,'$.duration_sec'))), avgOrNull(toFloat64OrNull(JSON_VALUE(props_json,'$.retries'))) FROM analytics.events"+w+" AND event IN ('level_clear','level_pass','level_win') GROUP BY lvl", args...); err == nil {
			for rows.Next() {
				var lvl string
				var d, r *float64
				_ = rows.Scan(&lvl, &d, &r)
				if strings.TrimSpace(lvl) != "" {
					detail[lvl] = Stat{L: lvl, Dur: d, Retry: r}
				}
			}
			rows.Close()
		}
		per := []any{}
		for lvl, n := range attempts {
			clr := clears[lvl]
			wr := 0.0
			if n > 0 {
				wr = math.Round((float64(clr) * 10000.0 / float64(n))) / 100.0
			}
			st := detail[lvl]
			avgDur := 0.0
			if st.Dur != nil {
				avgDur = math.Round((*st.Dur)*100.0) / 100.0
			}
			avgRetry := 0.0
			if st.Retry != nil {
				avgRetry = math.Round((*st.Retry)*100.0) / 100.0
			}
			diff := "-"
			if wr > 70 {
				diff = "低"
			} else if wr >= 40 {
				diff = "中"
			} else {
				diff = "高"
			}
			per = append(per, gin.H{"level": lvl, "players": n, "win_rate": wr, "avg_duration_sec": avgDur, "avg_retries": avgRetry, "difficulty": diff})
		}
		// segments: new / payer / returning (approx)
		// new: users with register/first_active in window
		attemptsNew := map[string]uint64{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_start','level_enter','level_begin') AND user_id IN (SELECT user_id FROM analytics.events"+w+" AND event IN ('register','first_active')) GROUP BY lvl", append(args, args...)...); err == nil {
			for rows.Next() {
				var lvl string
				var n uint64
				_ = rows.Scan(&lvl, &n)
				attemptsNew[lvl] = n
			}
			rows.Close()
		}
		clearsNew := map[string]uint64{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_clear','level_pass','level_win') AND user_id IN (SELECT user_id FROM analytics.events"+w+" AND event IN ('register','first_active')) GROUP BY lvl", append(args, args...)...); err == nil {
			for rows.Next() {
				var lvl string
				var n uint64
				_ = rows.Scan(&lvl, &n)
				clearsNew[lvl] = n
			}
			rows.Close()
		}
		// payer: users with success payments in window
		attemptsPayer := map[string]uint64{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_start','level_enter','level_begin') AND user_id IN (SELECT user_id FROM analytics.payments WHERE time>=toDateTime(?) AND time<=toDateTime(?) AND status='success'"+strings.ReplaceAll(strings.ReplaceAll(w, " WHERE", " AND"), "event_time", "time")+") GROUP BY lvl", append([]any{start, end}, args...)...); err == nil {
			for rows.Next() {
				var lvl string
				var n uint64
				_ = rows.Scan(&lvl, &n)
				attemptsPayer[lvl] = n
			}
			rows.Close()
		}
		clearsPayer := map[string]uint64{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_clear','level_pass','level_win') AND user_id IN (SELECT user_id FROM analytics.payments WHERE time>=toDateTime(?) AND time<=toDateTime(?) AND status='success'"+strings.ReplaceAll(strings.ReplaceAll(w, " WHERE", " AND"), "event_time", "time")+") GROUP BY lvl", append([]any{start, end}, args...)...); err == nil {
			for rows.Next() {
				var lvl string
				var n uint64
				_ = rows.Scan(&lvl, &n)
				clearsPayer[lvl] = n
			}
			rows.Close()
		}
		// returning = all - new（按层级做差，避免 uint 下溢）
		// new
		perNew := []any{}
		for lvl, a := range attemptsNew {
			c0 := clearsNew[lvl]
			wr := 0.0
			if a > 0 {
				wr = math.Round((float64(c0) * 10000.0 / float64(a))) / 100.0
			}
			perNew = append(perNew, gin.H{"level": lvl, "players": a, "win_rate": wr})
		}
		// payer
		perPayer := []any{}
		for lvl, a := range attemptsPayer {
			c0 := clearsPayer[lvl]
			wr := 0.0
			if a > 0 {
				wr = math.Round((float64(c0) * 10000.0 / float64(a))) / 100.0
			}
			perPayer = append(perPayer, gin.H{"level": lvl, "players": a, "win_rate": wr})
		}
		// returning = all - new
		attemptsRet := map[string]uint64{}
		clearsRet := map[string]uint64{}
		for lvl, n := range attempts {
			if n > attemptsNew[lvl] {
				attemptsRet[lvl] = n - attemptsNew[lvl]
			} else {
				attemptsRet[lvl] = 0
			}
		}
		for lvl, n := range clears {
			if n > clearsNew[lvl] {
				clearsRet[lvl] = n - clearsNew[lvl]
			} else {
				clearsRet[lvl] = 0
			}
		}
		perRet := []any{}
		for lvl, a := range attemptsRet {
			c0 := clearsRet[lvl]
			wr := 0.0
			if a > 0 {
				wr = math.Round((float64(c0) * 10000.0 / float64(a))) / 100.0
			}
			perRet = append(perRet, gin.H{"level": lvl, "players": a, "win_rate": wr})
		}
		c.JSON(200, gin.H{"funnel": funnel, "per_level": per, "per_level_segments": gin.H{"new": perNew, "payer": perPayer, "returning": perRet}})
	})

	// Levels per-episode facets: for each episode, list per_level stats (players & win_rate)
	r.GET("/api/analytics/levels/episodes", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"episodes": []any{}})
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
		w := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
		var args []any
		args = append(args, start, end)
		if game != "" {
			w += " AND game_id=?"
			args = append(args, game)
		}
		if env != "" {
			w += " AND env=?"
			args = append(args, env)
		}
		// attempts per episode/level
		type key struct{ Ep, Lvl string }
		att := map[key]uint64{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.episode') as ep, JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_start','level_enter','level_begin') GROUP BY ep, lvl", args...); err == nil {
			for rows.Next() {
				var ep, lvl string
				var n uint64
				_ = rows.Scan(&ep, &lvl, &n)
				if strings.TrimSpace(ep) != "" && strings.TrimSpace(lvl) != "" {
					att[key{Ep: ep, Lvl: lvl}] = n
				}
			}
			rows.Close()
		}
		// clears per episode/level
		clr := map[key]uint64{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.episode') as ep, JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_clear','level_pass','level_win') GROUP BY ep, lvl", args...); err == nil {
			for rows.Next() {
				var ep, lvl string
				var n uint64
				_ = rows.Scan(&ep, &lvl, &n)
				if strings.TrimSpace(ep) != "" && strings.TrimSpace(lvl) != "" {
					clr[key{Ep: ep, Lvl: lvl}] = n
				}
			}
			rows.Close()
		}
		eps := map[string][]map[string]any{}
		for k, n := range att {
			c0 := clr[k]
			wr := 0.0
			if n > 0 {
				wr = math.Round((float64(c0) * 10000.0 / float64(n))) / 100.0
			}
			eps[k.Ep] = append(eps[k.Ep], map[string]any{"level": k.Lvl, "players": n, "win_rate": wr})
		}
		out := []any{}
		// stable sort episodes by name
		names := make([]string, 0, len(eps))
		for ep := range eps {
			names = append(names, ep)
		}
		sort.Strings(names)
		for _, ep := range names {
			// sort levels by natural order or by name
			arr := eps[ep]
			sort.Slice(arr, func(i, j int) bool {
				return strings.Compare(strings.TrimSpace(arr[i]["level"].(string)), strings.TrimSpace(arr[j]["level"].(string))) < 0
			})
			out = append(out, gin.H{"episode": ep, "per_level": arr})
		}
		c.JSON(200, gin.H{"episodes": out})
	})

	// Levels per-map facets: for each map (props_json.map), list per_level stats
	r.GET("/api/analytics/levels/maps", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"maps": []any{}})
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
		w := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
		var args []any
		args = append(args, start, end)
		if game != "" {
			w += " AND game_id=?"
			args = append(args, game)
		}
		if env != "" {
			w += " AND env=?"
			args = append(args, env)
		}
		type key struct{ Mp, Lvl string }
		att := map[key]uint64{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.map') as mp, JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_start','level_enter','level_begin') GROUP BY mp, lvl", args...); err == nil {
			for rows.Next() {
				var mp, lvl string
				var n uint64
				_ = rows.Scan(&mp, &lvl, &n)
				if strings.TrimSpace(mp) != "" && strings.TrimSpace(lvl) != "" {
					att[key{Mp: mp, Lvl: lvl}] = n
				}
			}
			rows.Close()
		}
		clr := map[key]uint64{}
		if rows, err := s.ch.Query(c, "SELECT JSON_VALUE(props_json,'$.map') as mp, JSON_VALUE(props_json,'$.level') as lvl, uniqExact(user_id) FROM analytics.events"+w+" AND event IN ('level_clear','level_pass','level_win') GROUP BY mp, lvl", args...); err == nil {
			for rows.Next() {
				var mp, lvl string
				var n uint64
				_ = rows.Scan(&mp, &lvl, &n)
				if strings.TrimSpace(mp) != "" && strings.TrimSpace(lvl) != "" {
					clr[key{Mp: mp, Lvl: lvl}] = n
				}
			}
			rows.Close()
		}
		maps := map[string][]map[string]any{}
		for k, n := range att {
			c0 := clr[k]
			wr := 0.0
			if n > 0 {
				wr = math.Round((float64(c0) * 10000.0 / float64(n))) / 100.0
			}
			maps[k.Mp] = append(maps[k.Mp], map[string]any{"level": k.Lvl, "players": n, "win_rate": wr})
		}
		names := make([]string, 0, len(maps))
		for mp := range maps {
			names = append(names, mp)
		}
		sort.Strings(names)
		out := []any{}
		for _, mp := range names {
			arr := maps[mp]
			sort.Slice(arr, func(i, j int) bool {
				return strings.Compare(strings.TrimSpace(arr[i]["level"].(string)), strings.TrimSpace(arr[j]["level"].(string))) < 0
			})
			out = append(out, gin.H{"map": mp, "per_level": arr})
		}
		c.JSON(200, gin.H{"maps": out})
	})
	// Retention（每日 cohort：D1/D7/D30）
	r.GET("/api/analytics/retention", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"cohorts": []any{}})
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		cohort := strings.TrimSpace(c.DefaultQuery("cohort", "signup"))
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			t2 := time.Now()
			t1 := t2.Add(-14 * 24 * time.Hour)
			start = t1.Format("2006-01-02")
			end = t2.Format("2006-01-02")
		}
		t1, _ := time.Parse("2006-01-02", start[:10])
		t2, _ := time.Parse("2006-01-02", end[:10])
		if t2.Before(t1) {
			t1, t2 = t2, t1
		}
		wGame := ""
		wEnv := ""
		var extra []any
		if game != "" {
			wGame = " AND game_id=?"
			extra = append(extra, game)
		}
		if env != "" {
			wEnv = " AND env=?"
			extra = append(extra, env)
		}
		baseEvent := "register"
		if cohort == "first_active" {
			baseEvent = "first_active"
		}
		out := []any{}
		for d := t1; !d.After(t2); d = d.Add(24 * time.Hour) {
			day := d.Format("2006-01-02")
			var total uint64
			_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events WHERE toDate(event_time)=toDate(?) AND event='"+baseEvent+"'"+wGame+wEnv, append([]any{day}, extra...)...).Scan(&total)
			kept := func(off int) uint64 {
				tgt := d.Add(time.Duration(off) * 24 * time.Hour).Format("2006-01-02")
				var n uint64
				_ = s.ch.QueryRow(c, "SELECT uniqExact(user_id) FROM analytics.events WHERE toDate(event_time)=toDate(?)"+wGame+wEnv+" AND user_id IN (SELECT user_id FROM analytics.events WHERE toDate(event_time)=toDate(?) AND event='"+baseEvent+"'"+wGame+wEnv+")",
					append(append([]any{tgt, day}, extra...), extra...)...).Scan(&n)
				return n
			}
			var d1p, d7p, d30p float64
			if total > 0 {
				d1p = math.Round((float64(kept(1)) * 10000.0 / float64(total))) / 100.0
				d7p = math.Round((float64(kept(7)) * 10000.0 / float64(total))) / 100.0
				d30p = math.Round((float64(kept(30)) * 10000.0 / float64(total))) / 100.0
			}
			out = append(out, gin.H{"date": day, "total": total, "d1": d1p, "d7": d7p, "d30": d30p})
		}
		c.JSON(200, gin.H{"cohorts": out})
	})

	// Behavior: Top Paths (user/session)
	r.GET("/api/analytics/behavior/paths", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"paths": []any{}})
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		per := strings.ToLower(strings.TrimSpace(c.DefaultQuery("per", "session"))) // session|user
		steps, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("steps", "5")))
		if steps <= 0 || steps > 10 {
			steps = 5
		}
		limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "50")))
		if limit <= 0 || limit > 500 {
			limit = 50
		}
		sameSession := strings.TrimSpace(c.DefaultQuery("same_session", "0")) == "1"
		gapSec, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("gap_sec", "0")))
		maxGroups, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("max_groups", "50000")))
		if maxGroups <= 0 || maxGroups > 200000 {
			maxGroups = 50000
		}
		incStr := strings.TrimSpace(c.Query("include")) // comma list of events to include (optional)
		excStr := strings.TrimSpace(c.Query("exclude")) // comma list of events to exclude (optional)
		pathReStr := strings.TrimSpace(c.Query("path_re"))
		pathNotReStr := strings.TrimSpace(c.Query("path_not_re"))
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			t2 := time.Now()
			t1 := t2.Add(-7 * 24 * time.Hour)
			start = t1.Format(time.RFC3339)
			end = t2.Format(time.RFC3339)
		}
		w := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
		var args []any
		args = append(args, start, end)
		if game != "" {
			w += " AND game_id=?"
			args = append(args, game)
		}
		if env != "" {
			w += " AND env=?"
			args = append(args, env)
		}
		// include/exclude filters
		if incStr != "" {
			inc := []string{}
			placeholders := []string{}
			for _, s1 := range strings.Split(incStr, ",") {
				s1 = strings.TrimSpace(s1)
				if s1 != "" {
					inc = append(inc, s1)
				}
			}
			if len(inc) > 0 {
				for range inc {
					placeholders = append(placeholders, "?")
				}
				w += " AND event IN (" + strings.Join(placeholders, ",") + ")"
				for _, s1 := range inc {
					args = append(args, s1)
				}
			}
		}
		if excStr != "" {
			exc := []string{}
			placeholders := []string{}
			for _, s1 := range strings.Split(excStr, ",") {
				s1 = strings.TrimSpace(s1)
				if s1 != "" {
					exc = append(exc, s1)
				}
			}
			if len(exc) > 0 {
				for range exc {
					placeholders = append(placeholders, "?")
				}
				w += " AND event NOT IN (" + strings.Join(placeholders, ",") + ")"
				for _, s1 := range exc {
					args = append(args, s1)
				}
			}
		}
		grp := "user_id"
		if per == "session" {
			grp = "concat(user_id, '\\u007c', session_id)"
		}
		// Get arrays per group: timestamps/events/sessions
		sub := "SELECT " + grp + " AS grp, groupArray(toUnixTimestamp(event_time) ORDER BY event_time) AS ts, groupArray(event ORDER BY event_time) AS ev, groupArray(session_id ORDER BY event_time) AS sid FROM analytics.events" + w + " GROUP BY grp LIMIT " + strconv.Itoa(maxGroups)
		rows, err := s.ch.Query(c, sub, args...)
		if err != nil {
			c.JSON(200, gin.H{"paths": []any{}})
			return
		}
		defer rows.Close()
		// Optional regex filters on final path
		var incRe, excRe *regexp.Regexp
		if pathReStr != "" {
			incRe, _ = regexp.Compile(pathReStr)
		}
		if pathNotReStr != "" {
			excRe, _ = regexp.Compile(pathNotReStr)
		}
		counts := map[string]uint64{}
		for rows.Next() {
			var grpID string
			var ts []uint64
			var ev []string
			var sid []string
			if err := rows.Scan(&grpID, &ts, &ev, &sid); err != nil {
				continue
			}
			// Build constrained path of length <= steps
			pathEvents := make([]string, 0, steps)
			var baseSess string
			var lastTs uint64
			for i := 0; i < len(ev) && len(pathEvents) < steps; i++ {
				if sameSession {
					if len(pathEvents) == 0 {
						baseSess = safeSid(sid, i)
					} else {
						if s1 := safeSid(sid, i); baseSess == "" || s1 == "" || s1 != baseSess {
							continue
						}
					}
				}
				if gapSec > 0 && len(pathEvents) > 0 {
					if ts[i] > lastTs+uint64(gapSec) {
						break
					}
				}
				pathEvents = append(pathEvents, ev[i])
				lastTs = ts[i]
			}
			if len(pathEvents) == 0 {
				continue
			}
			p := strings.Join(pathEvents, ">")
			if incRe != nil && !incRe.MatchString(p) {
				continue
			}
			if excRe != nil && excRe.MatchString(p) {
				continue
			}
			counts[p]++
		}
		// Sort and limit
		type kv struct {
			K string
			V uint64
		}
		arr := make([]kv, 0, len(counts))
		for k, v := range counts {
			arr = append(arr, kv{k, v})
		}
		sort.Slice(arr, func(i, j int) bool {
			if arr[i].V == arr[j].V {
				return arr[i].K < arr[j].K
			}
			return arr[i].V > arr[j].V
		})
		if len(arr) > limit {
			arr = arr[:limit]
		}
		out := []any{}
		for _, it := range arr {
			out = append(out, gin.H{"path": it.K, "groups": it.V})
		}
		c.JSON(200, gin.H{"paths": out})
	})

	// Behavior: Feature Adoption (user/session baseline)
	r.GET("/api/analytics/behavior/adoption", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"features": []any{}, "baseline": 0})
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		per := strings.ToLower(strings.TrimSpace(c.DefaultQuery("per", "user"))) // user|session
		featsStr := strings.TrimSpace(c.Query("features"))
		if featsStr == "" {
			c.JSON(200, gin.H{"features": []any{}, "baseline": 0})
			return
		}
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			t2 := time.Now()
			t1 := t2.Add(-7 * 24 * time.Hour)
			start = t1.Format(time.RFC3339)
			end = t2.Format(time.RFC3339)
		}
		feats := []string{}
		for _, f := range strings.Split(featsStr, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				feats = append(feats, f)
			}
		}
		if len(feats) == 0 {
			c.JSON(200, gin.H{"features": []any{}, "baseline": 0})
			return
		}
		w := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
		var baseArgs []any
		baseArgs = append(baseArgs, start, end)
		if game != "" {
			w += " AND game_id=?"
			baseArgs = append(baseArgs, game)
		}
		if env != "" {
			w += " AND env=?"
			baseArgs = append(baseArgs, env)
		}
		grp := "user_id"
		if per == "session" {
			grp = "concat(user_id, '\\u007c', session_id)"
		}
		var baseline uint64
		_ = s.ch.QueryRow(c, "SELECT uniqExact("+grp+") FROM analytics.events"+w, baseArgs...).Scan(&baseline)
		// counts by feature
		placeholders := make([]string, len(feats))
		featArgs := append([]any{}, baseArgs...)
		for i := range feats {
			placeholders[i] = "?"
			featArgs = append(featArgs, feats[i])
		}
		sql := "SELECT event, uniqExact(" + grp + ") FROM analytics.events" + w + " AND event IN (" + strings.Join(placeholders, ",") + ") GROUP BY event"
		rows, err := s.ch.Query(c, sql, featArgs...)
		if err != nil {
			c.JSON(200, gin.H{"features": []any{}, "baseline": baseline})
			return
		}
		defer rows.Close()
		m := map[string]uint64{}
		for rows.Next() {
			var e string
			var n uint64
			_ = rows.Scan(&e, &n)
			m[e] = n
		}
		out := []any{}
		for _, f := range feats {
			n := m[f]
			rate := 0.0
			if baseline > 0 {
				rate = math.Round((float64(n) * 10000.0 / float64(baseline))) / 100.0
			}
			out = append(out, gin.H{"feature": f, "groups": n, "rate": rate})
		}
		c.JSON(200, gin.H{"features": out, "baseline": baseline})
	})

	// Feature adoption breakdown by dimension (channel|platform|country)
	r.GET("/api/analytics/behavior/adoption_breakdown", func(c *gin.Context) {
		if _, _, ok := s.require(c, "analytics:read"); !ok {
			return
		}
		if s.ch == nil {
			c.JSON(200, gin.H{"by": "", "rows": []any{}})
			return
		}
		game := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		per := strings.ToLower(strings.TrimSpace(c.DefaultQuery("per", "user"))) // user|session
		featsStr := strings.TrimSpace(c.Query("features"))
		by := strings.ToLower(strings.TrimSpace(c.DefaultQuery("by", "channel"))) // channel|platform|country
		if featsStr == "" {
			c.JSON(200, gin.H{"by": by, "rows": []any{}})
			return
		}
		start := strings.TrimSpace(c.Query("start"))
		end := strings.TrimSpace(c.Query("end"))
		if start == "" || end == "" {
			t2 := time.Now()
			t1 := t2.Add(-7 * 24 * time.Hour)
			start = t1.Format(time.RFC3339)
			end = t2.Format(time.RFC3339)
		}
		feats := []string{}
		for _, f := range strings.Split(featsStr, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				feats = append(feats, f)
			}
		}
		if len(feats) == 0 {
			c.JSON(200, gin.H{"by": by, "rows": []any{}})
			return
		}
		w := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
		var baseArgs []any
		baseArgs = append(baseArgs, start, end)
		if game != "" {
			w += " AND game_id=?"
			baseArgs = append(baseArgs, game)
		}
		if env != "" {
			w += " AND env=?"
			baseArgs = append(baseArgs, env)
		}
		grp := "user_id"
		if per == "session" {
			grp = "concat(user_id, '\\u007c', session_id)"
		}
		dim := "channel"
		switch by {
		case "platform":
			dim = "platform"
		case "country":
			dim = "country"
		default:
			dim = "channel"
		}
		// Baseline: uniq groups by dim
		baseSQL := "SELECT " + dim + ", uniqExact(" + grp + ") FROM analytics.events" + w + " GROUP BY " + dim
		baseRows, err := s.ch.Query(c, baseSQL, baseArgs...)
		if err != nil {
			c.JSON(200, gin.H{"by": by, "rows": []any{}})
			return
		}
		baseMap := map[string]uint64{}
		for baseRows.Next() {
			var d string
			var n uint64
			_ = baseRows.Scan(&d, &n)
			baseMap[d] = n
		}
		baseRows.Close()
		// Features: uniq groups by dim for features
		placeholders := make([]string, len(feats))
		featArgs := append([]any{}, baseArgs...)
		for i := range feats {
			placeholders[i] = "?"
			featArgs = append(featArgs, feats[i])
		}
		featSQL := "SELECT " + dim + ", uniqExact(" + grp + ") FROM analytics.events" + w + " AND event IN (" + strings.Join(placeholders, ",") + ") GROUP BY " + dim
		featRows, err := s.ch.Query(c, featSQL, featArgs...)
		if err != nil {
			c.JSON(200, gin.H{"by": by, "rows": []any{}})
			return
		}
		featMap := map[string]uint64{}
		for featRows.Next() {
			var d string
			var n uint64
			_ = featRows.Scan(&d, &n)
			featMap[d] = n
		}
		featRows.Close()
		// Merge
		out := []any{}
		for d, base := range baseMap {
			num := featMap[d]
			rate := 0.0
			if base > 0 {
				rate = math.Round((float64(num) * 10000.0 / float64(base))) / 100.0
			}
			out = append(out, gin.H{"dim": d, "baseline": base, "groups": num, "rate": rate})
		}
		c.JSON(200, gin.H{"by": by, "rows": out})
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

// safeSid returns sid[i] if exists, else empty string
func safeSid(sid []string, i int) string {
	if i >= 0 && i < len(sid) {
		return sid[i]
	}
	return ""
}
