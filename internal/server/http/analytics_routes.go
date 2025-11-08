package httpserver

import (
    "github.com/gin-gonic/gin"
)

// addAnalyticsRoutes registers placeholder analytics APIs.
// All endpoints require analytics:read for GET and analytics:manage for ingest POST.
// Return minimal structures to unblock frontend skeleton; replace with real implementations later.
func (s *Server) addAnalyticsRoutes(r *gin.Engine) {
    // Overview KPI
    r.GET("/api/analytics/overview", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:read"); !ok { return }
        s.JSON(c, 200, gin.H{
            "dau": 0, "wau": 0, "mau": 0, "new_users": 0,
            "d1": 0, "d7": 0, "d30": 0,
            "pay_rate": 0, "arpu": 0, "arppu": 0, "revenue_cents": 0,
            "series": gin.H{"dau": []},
        })
    })
    // Retention
    r.GET("/api/analytics/retention", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:read"); !ok { return }
        s.JSON(c, 200, gin.H{"cohorts": []})
    })
    // Realtime
    r.GET("/api/analytics/realtime", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:read"); !ok { return }
        s.JSON(c, 200, gin.H{"online": 0, "active_5m": 0, "rev_5m": 0, "pay_succ_rate": 0})
    })
    // Behavior: events + funnel
    r.GET("/api/analytics/behavior/events", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:read"); !ok { return }
        s.JSON(c, 200, gin.H{"events": [] , "total": 0})
    })
    r.GET("/api/analytics/behavior/funnel", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:read"); !ok { return }
        s.JSON(c, 200, gin.H{"steps": []})
    })
    // Payments
    r.GET("/api/analytics/payments/summary", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:read"); !ok { return }
        s.JSON(c, 200, gin.H{"totals": gin.H{"revenue_cents": 0, "refunds_cents": 0, "failed": 0, "success_rate": 0}, "by_channel": [] , "by_product": []})
    })
    r.GET("/api/analytics/payments/failures", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:read"); !ok { return }
        s.JSON(c, 200, gin.H{"top_reasons": []})
    })
    r.GET("/api/analytics/payments/transactions", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:read"); !ok { return }
        s.JSON(c, 200, gin.H{"transactions": [] , "total": 0})
    })
    // Levels (funnel + per-level stats)
    r.GET("/api/analytics/levels", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:read"); !ok { return }
        s.JSON(c, 200, gin.H{"funnel": [] , "per_level": []})
    })
    // Ingest endpoints (placeholder)
    r.POST("/api/analytics/ingest", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:manage"); !ok { return }
        c.Status(204)
    })
    r.POST("/api/analytics/payments/ingest", func(c *gin.Context) {
        if _, _, ok := s.require(c, "analytics:manage"); !ok { return }
        c.Status(204)
    })
}

