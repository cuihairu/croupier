package certificate

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
)

// ParseCertificatePEM parses a PEM-encoded certificate
func ParseCertificatePEM(certPEM string) (*x509.Certificate, error) {
	certPEM = strings.TrimSpace(certPEM)
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, errorx.NewBadRequest("failed to parse certificate PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

// FormatIssuer formats the certificate issuer for display
func FormatIssuer(cert *x509.Certificate) string {
	if cert == nil {
		return "Unknown"
	}
	parts := []string{}
	if len(cert.Issuer.Country) > 0 {
		parts = append(parts, cert.Issuer.Country[0])
	}
	if len(cert.Issuer.Organization) > 0 {
		parts = append(parts, cert.Issuer.Organization[0])
	}
	if len(cert.Issuer.OrganizationalUnit) > 0 {
		parts = append(parts, cert.Issuer.OrganizationalUnit[0])
	}
	if len(cert.Issuer.CommonName) > 0 {
		parts = append(parts, cert.Issuer.CommonName)
	}
	if len(parts) == 0 {
		return cert.Issuer.String()
	}
	return strings.Join(parts, ", ")
}

// FormatSubject renders a certificate subject as a compact string.
func FormatSubject(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName
	}
	return strings.Join(cert.Subject.Organization, ", ")
}

// certificateDaysLeft returns whole days until expiry (negative once expired).
func certificateDaysLeft(expiresAt time.Time) *int {
	if expiresAt.IsZero() {
		return nil
	}
	days := int(time.Until(expiresAt).Hours() / 24)
	return &days
}

// BuildCertificateDTO builds a certificate DTO from model
func BuildCertificateDTO(cert *model.Certificate) CertificateItem {
	return CertificateItem{
		ID:            cert.ID,
		Domain:        cert.Domain,
		Port:          cert.Port,
		Issuer:        cert.Issuer,
		Subject:       cert.Subject,
		NotBefore:     utils.FormatTimestampPtr(cert.StartsAt),
		NotAfter:      utils.FormatTimestamp(cert.ExpiresAt),
		ExpiresAt:     utils.FormatTimestamp(cert.ExpiresAt),
		Status:        cert.Status,
		DaysLeft:      certificateDaysLeft(cert.ExpiresAt),
		LastCheckedAt: utils.FormatTimestampPtr(cert.LastCheckedAt),
		ErrorMessage:  cert.ErrorMessage,
		CreatedAt:     utils.FormatTimestamp(cert.CreatedAt),
	}
}

// UpdateCertificateStatus updates the certificate status based on expiration
func UpdateCertificateStatus(cert *model.Certificate) {
	now := time.Now()
	cert.LastCheckedAt = &now
	cert.ErrorMessage = ""

	if cert.ExpiresAt.IsZero() {
		cert.Status = "invalid"
		cert.ErrorMessage = "missing expiration date"
		return
	}

	if cert.ExpiresAt.Before(now) {
		cert.Status = "expired"
	} else if cert.ExpiresAt.Before(now.Add(30 * 24 * time.Hour)) {
		cert.Status = "expiring"
	} else {
		cert.Status = "valid"
	}
}
