package utils

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
)

// ParseCertificatePEM parses certificate PEM and returns x509 cert info.
func ParseCertificatePEM(pemData string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errorx.NewBadRequest("无效的证书内容")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errorx.NewBadRequest("解析证书失败")
	}
	return cert, nil
}

// BuildCertificateDTO converts model.Certificate to map.
func BuildCertificateDTO(cert *model.Certificate) map[string]interface{} {
	return map[string]interface{}{
		"id":           cert.ID,
		"domain":       cert.Domain,
		"issuer":       cert.Issuer,
		"expiresAt":    FormatTimestamp(cert.ExpiresAt),
		"status":       cert.Status,
		"lastChecked":  FormatTimestampPtr(cert.LastCheckedAt),
		"errorMessage": cert.ErrorMessage,
		"createdAt":    FormatTimestamp(cert.CreatedAt),
		"updatedAt":    FormatTimestamp(cert.UpdatedAt),
	}
}

// ValidateDomain ensures domain string present.
func ValidateDomain(domain string) (string, error) {
	d := strings.TrimSpace(domain)
	if d == "" {
		return "", errorx.NewBadRequest("域名不能为空")
	}
	return d, nil
}

// UpdateCertificateStatus updates status fields on model.
func UpdateCertificateStatus(cert *model.Certificate) {
	status := model.CertificateStatus(cert.ExpiresAt)
	cert.Status = status
	now := time.Now().UTC()
	cert.LastCheckedAt = &now
	if status == "expired" && cert.ErrorMessage == "" {
		cert.ErrorMessage = "证书已过期"
	} else if status != "expired" {
		cert.ErrorMessage = ""
	}
}

// FormatIssuer builds issuer string from certificate.
func FormatIssuer(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	if cert.Issuer.CommonName != "" {
		return cert.Issuer.CommonName
	}
	if len(cert.Issuer.Organization) > 0 {
		return cert.Issuer.Organization[0]
	}
	return cert.Issuer.String()
}
