package httpserver

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// addCertificateRoutes adds certificate monitoring routes
func (s *Server) addCertificateRoutes(r *gin.Engine) {
	certGroup := r.Group("/api/certificates")
	certGroup.Use(func(c *gin.Context) {
		if _, _, ok := s.require(c, "certificates:manage"); !ok {
			return
		}
		c.Next()
	})

	// List certificates
	certGroup.GET("", func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
		status := c.Query("status")

		certs, total, err := s.certStore.ListCertificates(page, size, status)
		if err != nil {
			s.respondError(c, 500, "internal_error", err.Error())
			return
		}

		c.JSON(200, gin.H{
			"certificates": certs,
			"total":        total,
			"page":         page,
			"size":         size,
		})
	})

	// Add domain for monitoring
	certGroup.POST("", func(c *gin.Context) {
		var req struct {
			Domain    string `json:"domain" binding:"required"`
			Port      int    `json:"port"`
			AlertDays int    `json:"alert_days"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			s.respondError(c, 400, "bad_request", err.Error())
			return
		}

		if req.Port == 0 {
			req.Port = 443
		}
		if req.AlertDays == 0 {
			req.AlertDays = 30
		}

		if err := s.certStore.AddDomain(req.Domain, req.Port, req.AlertDays); err != nil {
			s.respondError(c, 500, "internal_error", err.Error())
			return
		}

		c.JSON(200, gin.H{"message": "domain added for monitoring"})
	})

	// Check specific certificate
	certGroup.POST("/:id/check", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			s.respondError(c, 400, "bad_request", "invalid certificate id")
			return
		}

		if err := s.certStore.CheckCertificate(uint(id)); err != nil {
			s.respondError(c, 500, "internal_error", err.Error())
			return
		}

		c.JSON(200, gin.H{"message": "certificate checked"})
	})

	// Check all certificates
	certGroup.POST("/check-all", func(c *gin.Context) {
		if err := s.certStore.CheckAllCertificates(); err != nil {
			s.respondError(c, 500, "internal_error", err.Error())
			return
		}

		c.JSON(200, gin.H{"message": "all certificates checked"})
	})

	// Get certificate statistics
	certGroup.GET("/stats", func(c *gin.Context) {
		stats, err := s.certStore.GetCertificateStats()
		if err != nil {
			s.respondError(c, 500, "internal_error", err.Error())
			return
		}

		c.JSON(200, stats)
	})

	// Get expiring certificates
	certGroup.GET("/expiring", func(c *gin.Context) {
		certs, err := s.certStore.GetExpiringCertificates()
		if err != nil {
			s.respondError(c, 500, "internal_error", err.Error())
			return
		}

		c.JSON(200, gin.H{"certificates": certs})
	})

	// Add alert for certificate
	certGroup.POST("/:id/alerts", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			s.respondError(c, 400, "bad_request", "invalid certificate id")
			return
		}

		var req struct {
			AlertType string `json:"alert_type" binding:"required"` // email, sms, webhook, chat
			Target    string `json:"target" binding:"required"`     // email, phone, URL, chat ID
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			s.respondError(c, 400, "bad_request", err.Error())
			return
		}

		if err := s.certStore.AddAlert(uint(id), req.AlertType, req.Target); err != nil {
			s.respondError(c, 500, "internal_error", err.Error())
			return
		}

		c.JSON(200, gin.H{"message": "alert added"})
	})

	// Get alerts for certificate
	certGroup.GET("/:id/alerts", func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			s.respondError(c, 400, "bad_request", "invalid certificate id")
			return
		}

		alerts, err := s.certStore.GetAlertsForCertificate(uint(id))
		if err != nil {
			s.respondError(c, 500, "internal_error", err.Error())
			return
		}

		c.JSON(200, gin.H{"alerts": alerts})
	})

	// Get domain info
	certGroup.GET("/domain/:domain", func(c *gin.Context) {
		domain := c.Param("domain")

		info, err := s.certStore.GetDomainInfo(domain)
		if err != nil {
			s.respondError(c, 500, "internal_error", err.Error())
			return
		}

		c.JSON(200, info)
	})
}
