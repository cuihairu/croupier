package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/cuihairu/croupier/internal/loadbalancer"
	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// addOpsRoutes registers /api/ops/* endpoints.
func (s *Server) addOpsRoutes(r *gin.Engine) {
	// Services list (wrap registry output). Allow either ops:read or registry:read.
	r.GET("/api/ops/services", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read", "registry:read"); !ok {
			return
		}
		type Agent struct {
			AgentID      string            `json:"agent_id"`
			GameID       string            `json:"game_id"`
			Env          string            `json:"env"`
			RpcAddr      string            `json:"rpc_addr"`
			IP           string            `json:"ip"`
			Region       string            `json:"region,omitempty"`
			Zone         string            `json:"zone,omitempty"`
			Labels       map[string]string `json:"labels,omitempty"`
			Type         string            `json:"type"`
			Version      string            `json:"version"`
			Functions    int               `json:"functions"`
			Healthy      bool              `json:"healthy"`
			ExpiresInSec int               `json:"expires_in_sec"`
			// Stats (best-effort)
			ActiveConns    int64   `json:"active_conns"`
			TotalRequests  int64   `json:"total_requests"`
			FailedRequests int64   `json:"failed_requests"`
			ErrorRate      float64 `json:"error_rate"`
			AvgLatencyMs   int64   `json:"avg_latency_ms"`
			LastSeen       string  `json:"last_seen"`
			QPS1m          float64 `json:"qps_1m"`
			// Placeholder for per-service QPS limit (future)
			QPSLimit int `json:"qps_limit"`
		}
		var agents []Agent
		var stats map[string]*loadbalancer.AgentStats
		if s.statsProv != nil {
			stats = s.statsProv.GetStats()
		}
		if s.reg != nil {
			s.reg.Mu().RLock()
			now := time.Now()
			for _, a := range s.reg.AgentsUnsafe() {
				healthy := now.Before(a.ExpireAt)
				exp := int(time.Until(a.ExpireAt).Seconds())
				if exp < 0 {
					exp = 0
				}
				ip := ""
				if h, _, err := net.SplitHostPort(a.RPCAddr); err == nil {
					ip = h
				} else {
					if i := strings.LastIndex(a.RPCAddr, ":"); i > 0 {
						ip = a.RPCAddr[:i]
					} else {
						ip = a.RPCAddr
					}
				}
				var ac int64
				var tot int64
				var fail int64
				var avgMs int64
				var errRate float64
				var lastSeen string
				var qps1m float64
				if stats != nil {
					if st := stats[a.AgentID]; st != nil {
						ac = st.ActiveConns
						tot = st.TotalRequests
						fail = st.FailedRequests
						if st.AvgResponseTime > 0 {
							avgMs = st.AvgResponseTime.Milliseconds()
						}
						if tot > 0 {
							errRate = float64(fail) / float64(tot)
						}
						if !st.LastSeen.IsZero() {
							lastSeen = st.LastSeen.Format(time.RFC3339)
						}
						qps1m = st.QPS1m
					}
				}
				agents = append(agents, Agent{
					AgentID:        a.AgentID,
					GameID:         a.GameID,
					Env:            a.Env,
					RpcAddr:        a.RPCAddr,
					IP:             ip,
					Region:         a.Region,
					Zone:           a.Zone,
					Labels:         a.Labels,
					Type:           "agent",
					Version:        a.Version,
					Functions:      len(a.Functions),
					Healthy:        healthy,
					ExpiresInSec:   exp,
					ActiveConns:    ac,
					TotalRequests:  tot,
					FailedRequests: fail,
					ErrorRate:      errRate,
					AvgLatencyMs:   avgMs,
					QPS1m:          qps1m,
					LastSeen:       lastSeen,
				})
			}
			s.reg.Mu().RUnlock()
		}
		s.JSON(c, 200, gin.H{"agents": agents})
	})

	// Update agent meta (region/zone) manually (ops:manage)
	r.PUT("/api/ops/agents/:id/meta", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:manage"); !ok {
			return
		}
		id := strings.TrimSpace(c.Param("id"))
		var in struct {
			Region string
			Zone   string
		}
		if err := c.BindJSON(&in); err != nil {
			s.respondError(c, 400, "bad_request", "invalid payload")
			return
		}
		if s.reg == nil {
			s.respondError(c, 503, "unavailable", "registry not available")
			return
		}
		s.reg.Mu().Lock()
		if a := s.reg.AgentsUnsafe()[id]; a != nil {
			if strings.TrimSpace(in.Region) != "" {
				a.Region = strings.TrimSpace(in.Region)
			}
			if strings.TrimSpace(in.Zone) != "" {
				a.Zone = strings.TrimSpace(in.Zone)
			}
			s.reg.Mu().Unlock()
			c.Status(204)
			return
		}
		s.reg.Mu().Unlock()
		s.respondError(c, 404, "not_found", "agent not found")
	})

	// Agent meta report (token-gated, no RBAC). Agents can POST region/zone updates here periodically.
	r.POST("/api/agent/meta", func(c *gin.Context) {
		tok := strings.TrimSpace(os.Getenv("AGENT_META_TOKEN"))
		if tok == "" {
			s.respondError(c, 503, "unavailable", "agent meta disabled")
			return
		}
		if c.Request.Header.Get("X-Agent-Token") != tok {
			s.respondError(c, 401, "unauthorized", "unauthorized")
			return
		}
		var in struct {
			AgentID, Region, Zone string
			Labels                map[string]string
		}
		if err := c.BindJSON(&in); err != nil || strings.TrimSpace(in.AgentID) == "" {
			s.respondError(c, 400, "bad_request", "invalid payload")
			return
		}
		if s.reg == nil {
			s.respondError(c, 503, "unavailable", "registry not available")
			return
		}
		s.reg.Mu().Lock()
		if a := s.reg.AgentsUnsafe()[in.AgentID]; a != nil {
			if v := strings.TrimSpace(in.Region); v != "" {
				a.Region = v
			}
			if v := strings.TrimSpace(in.Zone); v != "" {
				a.Zone = v
			}
			if in.Labels != nil {
				if a.Labels == nil {
					a.Labels = map[string]string{}
				}
				for k, v := range in.Labels {
					a.Labels[k] = v
				}
			}
			s.reg.Mu().Unlock()
			c.Status(204)
			return
		}
		s.reg.Mu().Unlock()
		s.respondError(c, 404, "not_found", "agent not found")
	})

	// Rate limits (function-level MVP)
	r.GET("/api/ops/rate-limits", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read", "ops:manage"); !ok {
			return
		}
		type Rule struct {
			Scope    string            `json:"scope"`
			Key      string            `json:"key"`
			LimitQPS int               `json:"limit_qps"`
			Match    map[string]string `json:"match,omitempty"`
			Percent  int               `json:"percent,omitempty"`
		}
		out := []Rule{}
		s.mu.Lock()
		if len(s.fnRulesAdv) > 0 || len(s.svcRulesAdv) > 0 {
			for _, rr := range s.fnRulesAdv {
				out = append(out, Rule{Scope: "function", Key: rr.Key, LimitQPS: rr.LimitQPS, Match: rr.Match, Percent: rr.Percent})
			}
			for _, rr := range s.svcRulesAdv {
				out = append(out, Rule{Scope: "service", Key: rr.Key, LimitQPS: rr.LimitQPS, Match: rr.Match, Percent: rr.Percent})
			}
		} else {
			for k, v := range s.rateLimitRules {
				out = append(out, Rule{Scope: "function", Key: k, LimitQPS: v, Percent: 100})
			}
			for k, v := range s.serviceRateRules {
				out = append(out, Rule{Scope: "service", Key: k, LimitQPS: v, Percent: 100})
			}
		}
		s.mu.Unlock()
		s.JSON(c, 200, gin.H{"rules": out})
	})

	// MQ info (redis|kafka|noop)
	r.GET("/api/ops/mq", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read"); !ok {
			return
		}
		typ := strings.TrimSpace(os.Getenv("ANALYTICS_MQ_TYPE"))
		if typ == "" {
			typ = "noop"
		}
		out := gin.H{"type": typ}
		switch typ {
		case "redis":
			url := strings.TrimSpace(os.Getenv("REDIS_URL"))
			se := os.Getenv("ANALYTICS_REDIS_STREAM_EVENTS")
			if se == "" {
				se = "analytics:events"
			}
			sp := os.Getenv("ANALYTICS_REDIS_STREAM_PAYMENTS")
			if sp == "" {
				sp = "analytics:payments"
			}
			out["redis"] = gin.H{"url": url, "streams": gin.H{"events": se, "payments": sp}}
			// Optional lengths and consumer groups
			lens := gin.H{}
			groups := []gin.H{}
			if url != "" {
				if opt, err := redis.ParseURL(url); err == nil {
					cli := redis.NewClient(opt)
					ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
					defer cancel()
					if se != "" {
						if n, err := cli.XLen(ctx, se).Result(); err == nil {
							lens[se] = n
						}
						if gs, err := cli.XInfoGroups(ctx, se).Result(); err == nil {
							for _, g := range gs {
								groups = append(groups, gin.H{
									"stream":    se,
									"name":      g.Name,
									"consumers": g.Consumers,
									"pending":   g.Pending,
									// EntriesRead / Lag available in newer redis versions
									"entries_read": g.EntriesRead,
									"lag":          g.Lag,
								})
							}
						}
					}
					if sp != "" {
						if n, err := cli.XLen(ctx, sp).Result(); err == nil {
							lens[sp] = n
						}
						if gs, err := cli.XInfoGroups(ctx, sp).Result(); err == nil {
							for _, g := range gs {
								groups = append(groups, gin.H{
									"stream":       sp,
									"name":         g.Name,
									"consumers":    g.Consumers,
									"pending":      g.Pending,
									"entries_read": g.EntriesRead,
									"lag":          g.Lag,
								})
							}
						}
					}
					_ = cli.Close()
				}
			}
			if len(lens) > 0 {
				out["lengths"] = lens
			}
			if len(groups) > 0 {
				out["groups"] = groups
			}
		case "kafka":
			brokers := strings.TrimSpace(os.Getenv("KAFKA_BROKERS"))
			// Prefer ANALYTICS_KAFKA_TOPIC_* if present, fallback to KAFKA_TOPIC_*
			te := os.Getenv("ANALYTICS_KAFKA_TOPIC_EVENTS")
			if te == "" {
				te = os.Getenv("KAFKA_TOPIC_EVENTS")
			}
			if te == "" {
				te = "analytics.events"
			}
			tp := os.Getenv("ANALYTICS_KAFKA_TOPIC_PAYMENTS")
			if tp == "" {
				tp = os.Getenv("KAFKA_TOPIC_PAYMENTS")
			}
			if tp == "" {
				tp = "analytics.payments"
			}
			out["kafka"] = gin.H{"brokers": brokers, "topics": gin.H{"events": te, "payments": tp}}
		default:
		}
		c.JSON(200, out)
	})

	// Notifications config (channels + rules)
	r.GET("/api/ops/notifications", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:manage"); !ok {
			return
		}
		out := gin.H{"channels": s.notifyChannels, "rules": s.notifyRules}
		c.JSON(200, out)
	})

	// Nodes meta report (edge or others). Token-gated via AGENT_META_TOKEN.
	r.POST("/api/ops/nodes/meta", func(c *gin.Context) {
		ok := strings.TrimSpace(os.Getenv("AGENT_META_TOKEN"))
		if ok == "" {
			s.respondError(c, 503, "unavailable", "meta disabled")
			return
		}
		if c.Request.Header.Get("X-Agent-Token") != ok {
			s.respondError(c, 401, "unauthorized", "unauthorized")
			return
		}
		var in struct {
			Type     string `json:"type"` // edge|agent|other
			ID       string `json:"id"`
			Addr     string `json:"addr"`
			HTTPAddr string `json:"http_addr"`
			Version  string `json:"version"`
			Region   string `json:"region"`
			Zone     string `json:"zone"`
		}
		if err := c.BindJSON(&in); err != nil || strings.TrimSpace(in.ID) == "" {
			s.respondError(c, 400, "bad_request", "invalid payload")
			return
		}
		if strings.ToLower(strings.TrimSpace(in.Type)) != "edge" {
			c.Status(204)
			return // currently only record edge; agents are tracked via registry
		}
		ip := ""
		if h, _, err := net.SplitHostPort(in.Addr); err == nil {
			ip = h
		}
		s.edgeMu.Lock()
		if s.edgeNodes == nil {
			s.edgeNodes = map[string]struct {
				ID, Addr, HTTPAddr, Version, IP, Region, Zone string
				LastSeen                                      time.Time
			}{}
		}
		s.edgeNodes[in.ID] = struct {
			ID, Addr, HTTPAddr, Version, IP, Region, Zone string
			LastSeen                                      time.Time
		}{ID: in.ID, Addr: in.Addr, HTTPAddr: in.HTTPAddr, Version: in.Version, IP: ip, Region: strings.TrimSpace(in.Region), Zone: strings.TrimSpace(in.Zone), LastSeen: time.Now()}
		s.edgeMu.Unlock()
		c.Status(204)
	})

	// Nodes list: combine agents + edges (best-effort)
	r.GET("/api/ops/nodes", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read", "registry:read"); !ok {
			return
		}
		out := []gin.H{}
		// agents
		if s.reg != nil {
			s.reg.Mu().RLock()
			now := time.Now()
			for _, a := range s.reg.AgentsUnsafe() {
				healthy := now.Before(a.ExpireAt)
				exp := int(time.Until(a.ExpireAt).Seconds())
				if exp < 0 {
					exp = 0
				}
				ip := ""
				if h, _, err := net.SplitHostPort(a.RPCAddr); err == nil {
					ip = h
				}
				out = append(out, gin.H{"id": a.AgentID, "type": "agent", "game_id": a.GameID, "env": a.Env, "addr": a.RPCAddr, "ip": ip, "version": a.Version, "healthy": healthy, "expires_in_sec": exp})
			}
			s.reg.Mu().RUnlock()
		}
		// edges
		s.edgeMu.RLock()
		for _, e := range s.edgeNodes {
			// consider edge healthy if last seen within 90s
			healthy := time.Since(e.LastSeen) <= 90*time.Second
			out = append(out, gin.H{"id": e.ID, "type": "edge", "addr": e.Addr, "http_addr": e.HTTPAddr, "ip": e.IP, "version": e.Version, "region": e.Region, "zone": e.Zone, "healthy": healthy, "last_seen": e.LastSeen.Format(time.RFC3339)})
		}
		s.edgeMu.RUnlock()
		c.JSON(200, gin.H{"nodes": out})
	})
	r.PUT("/api/ops/notifications", func(c *gin.Context) {
		actor, _, ok := s.require(c, "ops:manage")
		if !ok {
			return
		}
		var in struct {
			Channels []struct{ ID, Type, URL, Secret, Provider, Account, From, To string } `json:"channels"`
			Rules    []struct {
				Event         string
				Channels      []string
				ThresholdDays int
			} `json:"rules"`
		}
		if err := c.BindJSON(&in); err != nil {
			s.respondError(c, 400, "bad_request", "invalid payload")
			return
		}
		// basic normalize: trim ids and types
		for i := range in.Channels {
			in.Channels[i].ID = strings.TrimSpace(in.Channels[i].ID)
			in.Channels[i].Type = strings.ToLower(strings.TrimSpace(in.Channels[i].Type))
		}
		// assign and persist
		// map to named types
		s.mu.Lock()
		chs := make([]NotifyChannel, 0, len(in.Channels))
		for _, c0 := range in.Channels {
			chs = append(chs, NotifyChannel{ID: c0.ID, Type: c0.Type, URL: c0.URL, Secret: c0.Secret, Provider: c0.Provider, Account: c0.Account, From: c0.From, To: c0.To})
		}
		rs := make([]NotifyRule, 0, len(in.Rules))
		for _, r0 := range in.Rules {
			rs = append(rs, NotifyRule{Event: r0.Event, Channels: r0.Channels, ThresholdDays: r0.ThresholdDays})
		}
		s.notifyChannels = chs
		s.notifyRules = rs
		s.mu.Unlock()
		_ = os.MkdirAll(filepath.Dir(s.notificationsPath), 0o755)
		b, _ := json.MarshalIndent(in, "", "  ")
		_ = os.WriteFile(s.notificationsPath, b, 0o644)
		if s.audit != nil {
			_ = s.audit.Log("ops.notifications.update", actor, "notifications", map[string]string{"ip": c.ClientIP()})
		}
		c.JSON(200, gin.H{"ok": true})
	})
	r.PUT("/api/ops/rate-limits", func(c *gin.Context) {
		user, _, ok := s.require(c, "ops:manage")
		if !ok {
			return
		}
		var in struct {
			Rules []struct {
				Scope, Key string
				LimitQPS   int
				Match      map[string]string
				Percent    int
			}
		}
		if err := c.BindJSON(&in); err != nil || len(in.Rules) == 0 {
			s.respondError(c, 400, "bad_request", "invalid payload")
			return
		}
		// snapshot old set for diff
		s.mu.Lock()
		oldKeys := []string{}
		if len(s.fnRulesAdv) > 0 || len(s.svcRulesAdv) > 0 {
			for _, rr := range s.fnRulesAdv {
				oldKeys = append(oldKeys, "function:"+rr.Key)
			}
			for _, rr := range s.svcRulesAdv {
				oldKeys = append(oldKeys, "service:"+rr.Key)
			}
		} else {
			for k := range s.rateLimitRules {
				oldKeys = append(oldKeys, "function:"+k)
			}
			for k := range s.serviceRateRules {
				oldKeys = append(oldKeys, "service:"+k)
			}
		}
		// Replace (idempotent) with new rules set
		s.fnRulesAdv = nil
		s.svcRulesAdv = nil
		for _, r0 := range in.Rules {
			scope := strings.ToLower(strings.TrimSpace(r0.Scope))
			k := strings.TrimSpace(r0.Key)
			if k == "" || r0.LimitQPS <= 0 {
				continue
			}
			pct := r0.Percent
			if pct <= 0 {
				pct = 100
			}
			rr := rateRuleAdv{Scope: scope, Key: k, LimitQPS: r0.LimitQPS, Match: r0.Match, Percent: pct}
			if scope == "function" {
				s.fnRulesAdv = append(s.fnRulesAdv, rr)
			} else if scope == "service" {
				s.svcRulesAdv = append(s.svcRulesAdv, rr)
			}
		}
		// rebuild simple maps for compatibility
		s.rateLimitRules = map[string]int{}
		s.serviceRateRules = map[string]int{}
		for _, rr := range s.fnRulesAdv {
			if len(rr.Match) == 0 && rr.Percent >= 100 {
				s.rateLimitRules[rr.Key] = rr.LimitQPS
			}
		}
		for _, rr := range s.svcRulesAdv {
			if len(rr.Match) == 0 && rr.Percent >= 100 {
				s.serviceRateRules[rr.Key] = rr.LimitQPS
			}
		}
		// build new set for diff
		newKeys := []string{}
		for _, rr := range s.fnRulesAdv {
			newKeys = append(newKeys, "function:"+rr.Key)
		}
		for _, rr := range s.svcRulesAdv {
			newKeys = append(newKeys, "service:"+rr.Key)
		}
		s.mu.Unlock()
		if err := s.saveRateLimitsToFile(); err != nil {
			s.respondError(c, 500, "internal_error", "persist failed")
			return
		}
		// sync per-agent rate lookup into function server if supported
		if setter, ok := s.invoker.(interface{ SetServiceRateLookup(func(string) int) }); ok {
			setter.SetServiceRateLookup(func(agentID string) int {
				s.mu.RLock()
				defer s.mu.RUnlock()
				// Look up agent's game/env
				gid, env, regn, zone := "", "", "", ""
				labels := map[string]string{}
				if s.reg != nil {
					s.reg.Mu().RLock()
					if a := s.reg.AgentsUnsafe()[agentID]; a != nil {
						gid, env, regn, zone, labels = a.GameID, a.Env, a.Region, a.Zone, a.Labels
					}
					s.reg.Mu().RUnlock()
				}
				// choose best matching service rule
				best := 0
				pct := 100
				score := -1
				for _, rr := range s.svcRulesAdv {
					if rr.Key != "" && rr.Key != agentID {
						continue
					}
					m := rr.Match
					sc := 0
					if m != nil {
						if v := strings.TrimSpace(m["game_id"]); v != "" {
							if v != gid {
								continue
							}
							sc++
						}
						if v := strings.TrimSpace(m["env"]); v != "" {
							if v != env {
								continue
							}
							sc++
						}
						if v := strings.TrimSpace(m["region"]); v != "" {
							if v != regn {
								continue
							}
							sc++
						}
						if v := strings.TrimSpace(m["zone"]); v != "" {
							if v != zone {
								continue
							}
							sc++
						}
						// other keys treated as label match
						for k, v := range m {
							if k == "game_id" || k == "env" || k == "region" || k == "zone" {
								continue
							}
							if labels == nil {
								labels = map[string]string{}
							}
							if labels[k] != v {
								sc = -1
								break
							}
						}
						if sc < 0 {
							continue
						}
					}
					if sc > score {
						best = rr.LimitQPS
						pct = rr.Percent
						score = sc
					}
				}
				if best <= 0 { // fallback map
					if v := s.serviceRateRules[agentID]; v > 0 {
						best = v
						pct = 100
					}
				}
				if best <= 0 {
					return 0
				}
				if pct < 100 {
					best = best * pct / 100
				}
				if best <= 0 {
					best = 1
				}
				return best
			})
		}
		// audit detail（附带规则数与示例）
		if s.audit != nil {
			// compute diff counts
			oldSet := map[string]struct{}{}
			for _, k := range oldKeys {
				oldSet[k] = struct{}{}
			}
			newSet := map[string]struct{}{}
			for _, k := range newKeys {
				newSet[k] = struct{}{}
			}
			added, removed := 0, 0
			for k := range newSet {
				if _, ok := oldSet[k]; !ok {
					added++
				}
			}
			for k := range oldSet {
				if _, ok := newSet[k]; !ok {
					removed++
				}
			}
			_ = s.audit.Log("ops.rate_limit_update", user, "batch", map[string]string{"ip": c.ClientIP(), "count": strconv.Itoa(len(in.Rules)), "added": strconv.Itoa(added), "removed": strconv.Itoa(removed)})
		}
		c.Status(204)
	})
	r.DELETE("/api/ops/rate-limits", func(c *gin.Context) {
		user, _, ok := s.require(c, "ops:manage")
		if !ok {
			return
		}
		scope := strings.ToLower(strings.TrimSpace(c.Query("scope")))
		key := strings.TrimSpace(c.Query("key"))
		if key == "" || (scope != "function" && scope != "service") {
			s.respondError(c, 400, "bad_request", "invalid scope/key")
			return
		}
		s.mu.Lock()
		if scope == "function" {
			delete(s.rateLimitRules, key)
		} else {
			delete(s.serviceRateRules, key)
		}
		s.mu.Unlock()
		if err := s.saveRateLimitsToFile(); err != nil {
			s.respondError(c, 500, "internal_error", "persist failed")
			return
		}
		if setter, ok := s.invoker.(interface{ SetServiceRateLookup(func(string) int) }); ok {
			setter.SetServiceRateLookup(func(agentID string) int { s.mu.RLock(); defer s.mu.RUnlock(); return s.serviceRateRules[agentID] })
		}
		if s.audit != nil {
			_ = s.audit.Log("ops.rate_limit_update", user, scope+":"+key, map[string]string{"ip": c.ClientIP(), "action": "delete"})
		}
		c.Status(204)
	})
	// Preview matching (service scope): returns matched agent_ids with effective qps
	r.GET("/api/ops/rate-limits/preview", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read", "ops:manage"); !ok {
			return
		}
		scope := strings.ToLower(strings.TrimSpace(c.Query("scope")))
		key := strings.TrimSpace(c.Query("key"))
		limitQPS, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit_qps", "0")))
		pct, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("percent", "100")))
		mg := strings.TrimSpace(c.Query("match_game_id"))
		me := strings.TrimSpace(c.Query("match_env"))
		mr := strings.TrimSpace(c.Query("match_region"))
		mz := strings.TrimSpace(c.Query("match_zone"))
		if scope != "service" {
			s.JSON(c, 200, gin.H{"matched": 0, "agents": []any{}})
			return
		}
		if s.reg == nil {
			s.JSON(c, 200, gin.H{"matched": 0, "agents": []any{}})
			return
		}
		var stats map[string]*loadbalancer.AgentStats
		if s.statsProv != nil {
			stats = s.statsProv.GetStats()
		}
		s.reg.Mu().RLock()
		out := []map[string]any{}
		for _, a := range s.reg.AgentsUnsafe() {
			if key != "" && a.AgentID != key {
				continue
			}
			if mg != "" && a.GameID != mg {
				continue
			}
			if me != "" && a.Env != me {
				continue
			}
			if mr != "" && a.Region != mr {
				continue
			}
			if mz != "" && a.Zone != mz {
				continue
			}
			eff := limitQPS
			if pct > 0 && pct < 100 {
				eff = eff * pct / 100
			}
			if eff <= 0 {
				eff = limitQPS
			}
			cur := 0.0
			if stats != nil {
				if st := stats[a.AgentID]; st != nil {
					cur = st.QPS1m
				}
			}
			out = append(out, map[string]any{"agent_id": a.AgentID, "game_id": a.GameID, "env": a.Env, "region": a.Region, "zone": a.Zone, "rpc_addr": a.RPCAddr, "qps": eff, "qps_1m": cur})
		}
		s.reg.Mu().RUnlock()
		s.JSON(c, 200, gin.H{"matched": len(out), "agents": out})
	})

	// Functions meta (for UI helpers like rate-limit editor)
	r.GET("/api/ops/functions", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read", "registry:read"); !ok {
			return
		}
		type Fn struct {
			ID       string `json:"id"`
			Category string `json:"category,omitempty"`
		}
		out := make([]Fn, 0, len(s.descIndex))
		for _, d := range s.descs {
			out = append(out, Fn{ID: d.ID, Category: d.Category})
		}
		s.JSON(c, 200, gin.H{"functions": out})
	})
	// Jobs list (from in-memory store)
	r.GET("/api/ops/jobs", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read"); !ok {
			return
		}
		status := strings.TrimSpace(c.Query("status"))
		fid := strings.TrimSpace(c.Query("function_id"))
		actor := strings.TrimSpace(c.Query("actor"))
		gid := strings.TrimSpace(c.Query("game_id"))
		env := strings.TrimSpace(c.Query("env"))
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
		if page <= 0 {
			page = 1
		}
		if size <= 0 || size > 200 {
			size = 20
		}
		// snapshot copy under lock
		s.jobsMu.Lock()
		order := append([]string(nil), s.jobsOrder...)
		jm := make(map[string]*jobInfo, len(s.jobs))
		for k, v := range s.jobs {
			cp := *v
			jm[k] = &cp
		}
		s.jobsMu.Unlock()
		// newest first
		// filter and collect
		list := make([]*jobInfo, 0, len(order))
		for i := len(order) - 1; i >= 0; i-- {
			id := order[i]
			ji := jm[id]
			if ji == nil {
				continue
			}
			if status != "" && ji.State != status {
				continue
			}
			if fid != "" && ji.FunctionID != fid {
				continue
			}
			if actor != "" && ji.Actor != actor {
				continue
			}
			if gid != "" && ji.GameID != gid {
				continue
			}
			if env != "" && ji.Env != env {
				continue
			}
			list = append(list, ji)
		}
		total := len(list)
		start := (page - 1) * size
		if start > total {
			start = total
		}
		end := start + size
		if end > total {
			end = total
		}
		window := list[start:end]
		// marshal with desired fields
		out := make([]map[string]any, 0, len(window))
		for _, ji := range window {
			out = append(out, map[string]any{
				"id": ji.ID, "function_id": ji.FunctionID, "actor": ji.Actor, "game_id": ji.GameID, "env": ji.Env,
				"state": ji.State, "started_at": ji.StartedAt.Format(time.RFC3339),
				"ended_at": func() string {
					if ji.EndedAt.IsZero() {
						return ""
					} else {
						return ji.EndedAt.Format(time.RFC3339)
					}
				}(),
				"duration_ms": ji.DurationMs, "error": ji.Error, "rpc_addr": ji.RPCAddr, "trace_id": ji.TraceID,
			})
		}
		s.JSON(c, 200, gin.H{"jobs": out, "total": total, "page": page, "size": size})
	})

	// Alerts (Alertmanager proxy, optional). Configure ALERTMANAGER_URL, ALERTMANAGER_BEARER, ALERTMANAGER_TIMEOUT_MS
	r.GET("/api/ops/alerts", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read"); !ok {
			return
		}
		am := strings.TrimSpace(os.Getenv("ALERTMANAGER_URL"))
		if am == "" {
			s.JSON(c, 200, gin.H{"alerts": []any{}})
			return
		}
		u := am
		if !strings.HasSuffix(u, "/") {
			u += "/"
		}
		u += "api/v2/alerts"
		req, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, u, nil)
		if v := strings.TrimSpace(os.Getenv("ALERTMANAGER_BEARER")); v != "" {
			req.Header.Set("Authorization", "Bearer "+v)
		}
		to := 1500 * time.Millisecond
		if tv := strings.TrimSpace(os.Getenv("ALERTMANAGER_TIMEOUT_MS")); tv != "" {
			if n, err := strconv.Atoi(tv); err == nil && n > 0 {
				to = time.Duration(n) * time.Millisecond
			}
		}
		cli := &http.Client{Timeout: to}
		resp, err := cli.Do(req)
		if err != nil || resp == nil || resp.Body == nil {
			s.respondError(c, 502, "bad_gateway", "alertmanager unavailable")
			return
		}
		defer resp.Body.Close()
		var arr []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
			s.respondError(c, 502, "bad_gateway", "invalid alertmanager response")
			return
		}
		out := make([]map[string]any, 0, len(arr))
		now := time.Now()
		for _, a := range arr {
			labels, _ := a["labels"].(map[string]any)
			ann, _ := a["annotations"].(map[string]any)
			status, _ := a["status"].(map[string]any)
			sev := ""
			if labels != nil {
				if v, ok := labels["severity"].(string); ok {
					sev = v
				}
			}
			inst := ""
			if labels != nil {
				if v, ok := labels["instance"].(string); ok {
					inst = v
				}
			}
			service := ""
			if labels != nil {
				if v, ok := labels["service"].(string); ok {
					service = v
				} else if v2, ok2 := labels["job"].(string); ok2 {
					service = v2
				}
			}
			summary := ""
			if ann != nil {
				if v, ok := ann["summary"].(string); ok {
					summary = v
				}
			}
			startsAt := ""
			if v, ok := a["startsAt"].(string); ok {
				startsAt = v
			}
			endsAt := ""
			if v, ok := a["endsAt"].(string); ok {
				endsAt = v
			}
			silenced := false
			if status != nil {
				if s2, ok := status["state"].(string); ok && s2 == "suppressed" {
					silenced = true
				}
			}
			dur := ""
			if t, err := time.Parse(time.RFC3339Nano, startsAt); err == nil {
				dur = now.Sub(t).Truncate(time.Second).String()
			}
			out = append(out, map[string]any{
				"labels": labels, "annotations": ann, "severity": sev, "instance": inst, "service": service,
				"summary": summary, "starts_at": startsAt, "ends_at": endsAt, "silenced": silenced, "duration": dur,
			})
		}
		s.JSON(c, 200, gin.H{"alerts": out})
	})
	r.POST("/api/ops/alerts/silence", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:manage"); !ok {
			return
		}
		am := strings.TrimSpace(os.Getenv("ALERTMANAGER_URL"))
		if am == "" {
			s.respondError(c, 503, "unavailable", "alertmanager not configured")
			return
		}
		var in struct {
			Matchers map[string]string
			Duration string
			Creator  string
			Comment  string
		}
		if err := c.BindJSON(&in); err != nil {
			s.respondError(c, 400, "bad_request", "invalid payload")
			return
		}
		if in.Duration == "" {
			in.Duration = "1h"
		}
		d, err := time.ParseDuration(in.Duration)
		if err != nil {
			s.respondError(c, 400, "bad_request", "invalid duration")
			return
		}
		now := time.Now().UTC()
		ms := []map[string]any{}
		for k, v := range in.Matchers {
			ms = append(ms, map[string]any{"name": k, "value": v, "isRegex": false})
		}
		payload := map[string]any{
			"matchers":  ms,
			"startsAt":  now.Format(time.RFC3339Nano),
			"endsAt":    now.Add(d).Format(time.RFC3339Nano),
			"createdBy": in.Creator,
			"comment":   in.Comment,
		}
		b, _ := json.Marshal(payload)
		u := am
		if !strings.HasSuffix(u, "/") {
			u += "/"
		}
		u += "api/v2/silences"
		req, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, u, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if v := strings.TrimSpace(os.Getenv("ALERTMANAGER_BEARER")); v != "" {
			req.Header.Set("Authorization", "Bearer "+v)
		}
		to := 1500 * time.Millisecond
		if tv := strings.TrimSpace(os.Getenv("ALERTMANAGER_TIMEOUT_MS")); tv != "" {
			if n, err := strconv.Atoi(tv); err == nil && n > 0 {
				to = time.Duration(n) * time.Millisecond
			}
		}
		cli := &http.Client{Timeout: to}
		resp, err := cli.Do(req)
		if err != nil {
			s.respondError(c, 502, "bad_gateway", "alertmanager unavailable")
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			s.respondError(c, 502, "bad_gateway", "silence create failed")
			return
		}
		c.Status(204)
	})
	r.GET("/api/ops/alerts/silences", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read"); !ok {
			return
		}
		am := strings.TrimSpace(os.Getenv("ALERTMANAGER_URL"))
		if am == "" {
			s.JSON(c, 200, gin.H{"silences": []any{}})
			return
		}
		u := strings.TrimRight(am, "/") + "/api/v2/silences"
		req, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, u, nil)
		if v := strings.TrimSpace(os.Getenv("ALERTMANAGER_BEARER")); v != "" {
			req.Header.Set("Authorization", "Bearer "+v)
		}
		to := 1500 * time.Millisecond
		if tv := strings.TrimSpace(os.Getenv("ALERTMANAGER_TIMEOUT_MS")); tv != "" {
			if n, err := strconv.Atoi(tv); err == nil && n > 0 {
				to = time.Duration(n) * time.Millisecond
			}
		}
		cli := &http.Client{Timeout: to}
		resp, err := cli.Do(req)
		if err != nil || resp == nil || resp.Body == nil {
			s.respondError(c, 502, "bad_gateway", "alertmanager unavailable")
			return
		}
		defer resp.Body.Close()
		var arr []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
			s.respondError(c, 502, "bad_gateway", "invalid alertmanager response")
			return
		}
		// passthrough essential fields
		out := make([]map[string]any, 0, len(arr))
		for _, si := range arr {
			out = append(out, map[string]any{
				"id": si["id"], "matchers": si["matchers"], "created_by": si["createdBy"], "comment": si["comment"],
				"starts_at": si["startsAt"], "ends_at": si["endsAt"], "status": si["status"],
			})
		}
		s.JSON(c, 200, gin.H{"silences": out})
	})
	r.DELETE("/api/ops/alerts/silences/:id", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:manage"); !ok {
			return
		}
		am := strings.TrimSpace(os.Getenv("ALERTMANAGER_URL"))
		if am == "" {
			s.respondError(c, 503, "unavailable", "alertmanager not configured")
			return
		}
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			s.respondError(c, 400, "bad_request", "missing id")
			return
		}
		u := strings.TrimRight(am, "/") + "/api/v2/silence/" + url.QueryEscape(id)
		req, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodDelete, u, nil)
		if v := strings.TrimSpace(os.Getenv("ALERTMANAGER_BEARER")); v != "" {
			req.Header.Set("Authorization", "Bearer "+v)
		}
		to := 1500 * time.Millisecond
		if tv := strings.TrimSpace(os.Getenv("ALERTMANAGER_TIMEOUT_MS")); tv != "" {
			if n, err := strconv.Atoi(tv); err == nil && n > 0 {
				to = time.Duration(n) * time.Millisecond
			}
		}
		cli := &http.Client{Timeout: to}
		resp, err := cli.Do(req)
		if err != nil {
			s.respondError(c, 502, "bad_gateway", "alertmanager unavailable")
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			s.respondError(c, 502, "bad_gateway", "delete failed")
			return
		}
		c.Status(204)
	})
	// Ops config for UI (base links)
	r.GET("/api/ops/config", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read", "registry:read"); !ok {
			return
		}
		s.JSON(c, 200, gin.H{
			"alertmanager_url":    strings.TrimSpace(os.Getenv("ALERTMANAGER_URL")),
			"grafana_explore_url": strings.TrimSpace(os.Getenv("GRAFANA_EXPLORE_URL")),
		})
	})

	// Prometheus metrics proxy (optional). Configure PROM_URL (+ optional PROM_BEARER, PROM_TIMEOUT_MS, and query templates)
	// Query templates (with {instance} placeholder) via env:
	//   PROM_QPS_QUERY, PROM_ERR_QUERY, PROM_P95_QUERY
	r.GET("/api/ops/metrics", func(c *gin.Context) {
		if _, _, ok := s.require(c, "ops:read"); !ok {
			return
		}
		base := strings.TrimSpace(os.Getenv("PROM_URL"))
		if base == "" {
			s.JSON(c, 200, gin.H{"qps": []any{}, "err_rate": []any{}, "p95_ms": []any{}})
			return
		}
		inst := strings.TrimSpace(c.Query("instance"))
		rng := strings.TrimSpace(c.DefaultQuery("range", "15m"))
		step := strings.TrimSpace(c.DefaultQuery("step", "15s"))
		// build queries from templates
		rep := func(tpl, inst string) string { return strings.ReplaceAll(tpl, "{instance}", inst) }
		qQPS := strings.TrimSpace(os.Getenv("PROM_QPS_QUERY"))
		qERR := strings.TrimSpace(os.Getenv("PROM_ERR_QUERY"))
		qP95 := strings.TrimSpace(os.Getenv("PROM_P95_QUERY"))
		// default conservative guesses if not provided (may return empty if metrics don't exist)
		if qQPS == "" {
			qQPS = `sum(rate(http_requests_total{instance="{instance}"}[1m]))`
		}
		if qERR == "" {
			qERR = `sum(rate(http_requests_total{instance="{instance}",status=~"5.."}[5m])) / sum(rate(http_requests_total{instance="{instance}"}[5m]))`
		}
		if qP95 == "" {
			qP95 = `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{instance="{instance}"}[5m])) by (le))`
		}
		// timeframe
		dur, err := time.ParseDuration(rng)
		if err != nil {
			s.respondError(c, 400, "bad_request", "invalid range")
			return
		}
		end := time.Now()
		start := end.Add(-dur)
		// helper to call prom
		urlQueryEscape := func(s string) string { return url.QueryEscape(s) }
		doRange := func(query string) ([][2]any, error) {
			u := strings.TrimRight(base, "/") + "/api/v1/query_range?query=" + urlQueryEscape(query) + "&start=" + urlQueryEscape(start.Format(time.RFC3339)) + "&end=" + urlQueryEscape(end.Format(time.RFC3339)) + "&step=" + urlQueryEscape(step)
			req, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, u, nil)
			if v := strings.TrimSpace(os.Getenv("PROM_BEARER")); v != "" {
				req.Header.Set("Authorization", "Bearer "+v)
			}
			to := 2000 * time.Millisecond
			if tv := strings.TrimSpace(os.Getenv("PROM_TIMEOUT_MS")); tv != "" {
				if n, err := strconv.Atoi(tv); err == nil && n > 0 {
					to = time.Duration(n) * time.Millisecond
				}
			}
			cli := &http.Client{Timeout: to}
			resp, err := cli.Do(req)
			if err != nil || resp == nil || resp.Body == nil {
				return nil, err
			}
			defer resp.Body.Close()
			var pr struct {
				Status string
				Data   struct {
					ResultType string `json:"resultType"`
					Result     []struct {
						Values [][2]any `json:"values"`
					} `json:"result"`
				}
			}
			if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
				return nil, err
			}
			if len(pr.Data.Result) == 0 {
				return [][2]any{}, nil
			}
			// Flatten: pick first series; UI只展示单线
			return pr.Data.Result[0].Values, nil
		}
		qps, _ := doRange(rep(qQPS, inst))
		errRate, _ := doRange(rep(qERR, inst))
		p95, _ := doRange(rep(qP95, inst))
		s.JSON(c, 200, gin.H{"qps": qps, "err_rate": errRate, "p95_ms": p95})
	})
}
