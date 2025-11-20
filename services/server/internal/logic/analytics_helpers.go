package logic

import (
	"strings"
	"time"
)

func buildAnalyticsFilters(game, env string) (string, string, []any) {
	game = strings.TrimSpace(game)
	env = strings.TrimSpace(env)
	wGame, wEnv := "", ""
	args := []any{}
	if game != "" {
		wGame = " AND game_id = ?"
		args = append(args, game)
	}
	if env != "" {
		wEnv = " AND env = ?"
		args = append(args, env)
	}
	return wGame, wEnv, args
}

func validAnalyticsPropKey(key string) bool {
	if key == "" {
		return false
	}
	for _, ch := range key {
		if ch == '_' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		return false
	}
	return true
}

func buildPaymentsWhere(start, end string, filters map[string]string) (string, []any) {
	where := " WHERE time>=toDateTime(?) AND time<=toDateTime(?)"
	args := []any{start, end}
	order := []string{"game_id", "env", "channel", "platform", "country", "region", "city", "status"}
	for _, key := range order {
		if val := strings.TrimSpace(filters[key]); val != "" {
			where += " AND " + key + "=?"
			args = append(args, val)
		}
	}
	return where, args
}

func defaultRange(start, end string, days int) (string, string) {
	s := strings.TrimSpace(start)
	e := strings.TrimSpace(end)
	if s != "" && e != "" {
		return s, e
	}
	if days <= 0 {
		days = 7
	}
	t2 := time.Now()
	t1 := t2.Add(-time.Duration(days) * 24 * time.Hour)
	return t1.Format(time.RFC3339), t2.Format(time.RFC3339)
}

func buildEventsWhere(start, end string, filters map[string]string) (string, []any) {
	where := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
	args := []any{start, end}
	order := []string{"game_id", "env"}
	for _, key := range order {
		if val := strings.TrimSpace(filters[key]); val != "" {
			where += " AND " + key + "=?"
			args = append(args, val)
		}
	}
	return where, args
}

func cloneArgs(args []any) []any {
	out := make([]any, len(args))
	copy(out, args)
	return out
}

func splitCSV(val string) []string {
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}
