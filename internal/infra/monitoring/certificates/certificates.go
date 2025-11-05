package certificates

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"gorm.io/gorm"
	"net"
	"strings"
	"time"
)

// Certificate represents a monitored SSL certificate
type Certificate struct {
	ID          uint   `gorm:"primaryKey"`
	Domain      string `gorm:"column:domain;uniqueIndex;size:255"`
	Port        int    `gorm:"column:port;default:443"`
	Issuer      string `gorm:"column:issuer;size:500"`
	Subject     string `gorm:"column:subject;size:500"`
	Algorithm   string `gorm:"column:algorithm;size:100"`
	KeyUsage    string `gorm:"column:key_usage;size:200"`
	ValidFrom   time.Time `gorm:"column:valid_from"`
	ValidTo     time.Time `gorm:"column:valid_to"`
	DaysLeft    int    `gorm:"column:days_left"`
	Status      string `gorm:"column:status;size:50"` // valid, expired, expiring, error
	LastChecked time.Time `gorm:"column:last_checked"`
	ErrorMsg    string `gorm:"column:error_msg;type:text"`
	AlertDays   int    `gorm:"column:alert_days;default:30"` // Alert when days left <= this value
	Enabled     bool   `gorm:"column:enabled;default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Certificate) TableName() string {
	return "certificates"
}

// CertificateAlert represents alert configuration
type CertificateAlert struct {
	ID           uint   `gorm:"primaryKey"`
	CertificateID uint   `gorm:"column:certificate_id;index"`
	AlertType    string `gorm:"column:alert_type;size:50"` // email, sms, webhook, chat
	Target       string `gorm:"column:target;size:500"`    // email address, phone, webhook URL, chat ID
	Enabled      bool   `gorm:"column:enabled;default:true"`
	LastSent     *time.Time `gorm:"column:last_sent"`
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Certificate Certificate `gorm:"foreignKey:CertificateID"`
}

func (CertificateAlert) TableName() string {
	return "certificate_alerts"
}

// Store handles certificate monitoring
type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// AutoMigrate creates certificate tables
func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&Certificate{}, &CertificateAlert{})
}

// AddDomain adds a domain to monitor
func (s *Store) AddDomain(domain string, port int, alertDays int) error {
	cert := &Certificate{
		Domain:    domain,
		Port:      port,
		AlertDays: alertDays,
		Enabled:   true,
		Status:    "pending",
	}

	return s.db.Where("domain = ? AND port = ?", domain, port).FirstOrCreate(cert).Error
}

// CheckCertificate checks a single certificate
func (s *Store) CheckCertificate(certID uint) error {
	var cert Certificate
	if err := s.db.First(&cert, certID).Error; err != nil {
		return err
	}

	if !cert.Enabled {
		return nil
	}

	certInfo, err := s.fetchCertificateInfo(cert.Domain, cert.Port)
	if err != nil {
		cert.Status = "error"
		cert.ErrorMsg = err.Error()
		cert.LastChecked = time.Now()
		return s.db.Save(&cert).Error
	}

	// Update certificate information
	cert.Issuer = certInfo.Issuer.CommonName
	cert.Subject = certInfo.Subject.CommonName
	cert.Algorithm = certInfo.SignatureAlgorithm.String()
	cert.KeyUsage = s.formatKeyUsage(certInfo.KeyUsage, certInfo.ExtKeyUsage)
	cert.ValidFrom = certInfo.NotBefore
	cert.ValidTo = certInfo.NotAfter
	cert.DaysLeft = int(time.Until(certInfo.NotAfter).Hours() / 24)
	cert.LastChecked = time.Now()
	cert.ErrorMsg = ""

	// Determine status
	now := time.Now()
	if certInfo.NotAfter.Before(now) {
		cert.Status = "expired"
	} else if cert.DaysLeft <= cert.AlertDays {
		cert.Status = "expiring"
	} else {
		cert.Status = "valid"
	}

	return s.db.Save(&cert).Error
}

// CheckAllCertificates checks all enabled certificates
func (s *Store) CheckAllCertificates() error {
	var certs []Certificate
	if err := s.db.Where("enabled = ?", true).Find(&certs).Error; err != nil {
		return err
	}

	for _, cert := range certs {
		if err := s.CheckCertificate(cert.ID); err != nil {
			// Log error but continue with other certificates
			continue
		}
	}

	return nil
}

// GetExpiringCertificates returns certificates that are expiring or expired
func (s *Store) GetExpiringCertificates() ([]Certificate, error) {
	var certs []Certificate
	err := s.db.Where("enabled = ? AND status IN (?)", true, []string{"expiring", "expired"}).Find(&certs).Error
	return certs, err
}

// ListCertificates returns all certificates with pagination
func (s *Store) ListCertificates(page, size int, status string) ([]Certificate, int64, error) {
	var certs []Certificate
	var total int64

	query := s.db.Model(&Certificate{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	err := query.Offset(offset).Limit(size).Order("created_at DESC").Find(&certs).Error
	return certs, total, err
}

// fetchCertificateInfo connects to domain and retrieves certificate
func (s *Store) fetchCertificateInfo(domain string, port int) (*x509.Certificate, error) {
	address := fmt.Sprintf("%s:%d", domain, port)

	// Set timeout for connection
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
		ServerName: domain,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates found for %s", domain)
	}

	return certs[0], nil
}

// formatKeyUsage formats certificate key usage for display
func (s *Store) formatKeyUsage(keyUsage x509.KeyUsage, extKeyUsage []x509.ExtKeyUsage) string {
	var usages []string

	if keyUsage&x509.KeyUsageDigitalSignature != 0 {
		usages = append(usages, "Digital Signature")
	}
	if keyUsage&x509.KeyUsageKeyEncipherment != 0 {
		usages = append(usages, "Key Encipherment")
	}
	if keyUsage&x509.KeyUsageDataEncipherment != 0 {
		usages = append(usages, "Data Encipherment")
	}

	for _, ext := range extKeyUsage {
		switch ext {
		case x509.ExtKeyUsageServerAuth:
			usages = append(usages, "Server Authentication")
		case x509.ExtKeyUsageClientAuth:
			usages = append(usages, "Client Authentication")
		}
	}

	return strings.Join(usages, ", ")
}

// AddAlert adds an alert for a certificate
func (s *Store) AddAlert(certID uint, alertType, target string) error {
	alert := &CertificateAlert{
		CertificateID: certID,
		AlertType:     alertType,
		Target:        target,
		Enabled:       true,
	}

	return s.db.Create(alert).Error
}

// GetAlertsForCertificate returns alerts for a specific certificate
func (s *Store) GetAlertsForCertificate(certID uint) ([]CertificateAlert, error) {
	var alerts []CertificateAlert
	err := s.db.Where("certificate_id = ? AND enabled = ?", certID, true).Find(&alerts).Error
	return alerts, err
}

// DomainInfo contains domain registration information
type DomainInfo struct {
	Domain         string    `json:"domain"`
	Registrar      string    `json:"registrar"`
	RegistrationDate *time.Time `json:"registration_date,omitempty"`
	ExpirationDate   *time.Time `json:"expiration_date,omitempty"`
	NameServers    []string  `json:"name_servers"`
	DaysToExpiry   int       `json:"days_to_expiry"`
	Status         string    `json:"status"`
}

// GetDomainInfo attempts to get domain registration info (basic implementation)
func (s *Store) GetDomainInfo(domain string) (*DomainInfo, error) {
	// This is a simplified implementation
	// In production, you would use WHOIS APIs or services like:
	// - WHOIS API providers
	// - DNS record lookups
	// - Domain registration APIs

	info := &DomainInfo{
		Domain: domain,
		Status: "active", // Default status
	}

	// Try to get nameservers via DNS lookup
	if nameservers, err := net.LookupNS(domain); err == nil {
		for _, ns := range nameservers {
			info.NameServers = append(info.NameServers, ns.Host)
		}
	}

	return info, nil
}

// CertificateStats contains certificate statistics
type CertificateStats struct {
	Total       int64 `json:"total"`
	Valid       int64 `json:"valid"`
	Expiring    int64 `json:"expiring"`
	Expired     int64 `json:"expired"`
	Errors      int64 `json:"errors"`
	LastChecked time.Time `json:"last_checked"`
}

// GetCertificateStats returns certificate statistics
func (s *Store) GetCertificateStats() (*CertificateStats, error) {
	stats := &CertificateStats{}

	// Total count
	s.db.Model(&Certificate{}).Where("enabled = ?", true).Count(&stats.Total)

	// Count by status
	var statusCounts []struct {
		Status string
		Count  int64
	}
	s.db.Model(&Certificate{}).
		Select("status, count(*) as count").
		Where("enabled = ?", true).
		Group("status").
		Scan(&statusCounts)

	for _, sc := range statusCounts {
		switch sc.Status {
		case "valid":
			stats.Valid = sc.Count
		case "expiring":
			stats.Expiring = sc.Count
		case "expired":
			stats.Expired = sc.Count
		case "error":
			stats.Errors = sc.Count
		}
	}

	// Last checked time
	var lastCert Certificate
	if err := s.db.Where("enabled = ?", true).Order("last_checked DESC").First(&lastCert).Error; err == nil {
		stats.LastChecked = lastCert.LastChecked
	}

	return stats, nil
}