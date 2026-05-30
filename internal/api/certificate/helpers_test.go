// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

// generateTestCertificate creates a test certificate with the specified issuer
func generateTestCertificate(t *testing.T, issuer pkix.Name) *x509.Certificate {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test.example.com"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		Issuer:                issuer,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	return cert
}

func certificateToPEM(cert *x509.Certificate) string {
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	return string(pem.EncodeToMemory(block))
}

func TestParseCertificatePEM(t *testing.T) {
	t.Parallel()

	validCert := generateTestCertificate(t, pkix.Name{
		Country:            []string{"US"},
		Organization:       []string{"Test Org"},
		OrganizationalUnit: []string{"IT"},
		CommonName:         "Test CA",
	})
	validPEM := certificateToPEM(validCert)

	tests := []struct {
		name    string
		pem     string
		wantErr bool
	}{
		{
			name:    "valid certificate PEM",
			pem:     validPEM,
			wantErr: false,
		},
		{
			name:    "invalid PEM - not a certificate",
			pem:     "not a pem",
			wantErr: true,
		},
		{
			name:    "empty string",
			pem:     "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			pem:     "   \n\t  ",
			wantErr: true,
		},
		{
			name:    "valid PEM with extra whitespace",
			pem:     "  \n  " + validPEM + "  \n  ",
			wantErr: false,
		},
		{
			name:    "invalid PEM block type",
			pem:     "-----BEGIN PRIVATE KEY-----\nMIIBOgIBAAJBAKj34GkxFhD90vcNLYLInFEX6Ppy1tPf9Cnzj4p4WGeKLs1Pt8Qu\n-----END PRIVATE KEY-----",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := ParseCertificatePEM(tt.pem)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCertificatePEM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cert == nil {
				t.Error("ParseCertificatePEM() returned nil cert without error")
			}
		})
	}
}

func TestFormatIssuer(t *testing.T) {
	t.Parallel()

	// Helper to create test certificate directly without signing
	makeCert := func(issuer pkix.Name) *x509.Certificate {
		return &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: "test.example.com"},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(365 * 24 * time.Hour),
			KeyUsage:              x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
			Issuer:                issuer,
		}
	}

	tests := []struct {
		name string
		cert *x509.Certificate
		want string
	}{
		{
			name: "nil certificate",
			cert: nil,
			want: "Unknown",
		},
		{
			name: "full issuer details",
			cert: makeCert(pkix.Name{
				Country:            []string{"US"},
				Organization:       []string{"Example Inc"},
				OrganizationalUnit: []string{"IT"},
				CommonName:         "example.com",
			}),
			want: "US, Example Inc, IT, example.com",
		},
		{
			name: "partial issuer - org and CN",
			cert: makeCert(pkix.Name{
				Organization: []string{"Test Org"},
				CommonName:   "test.com",
			}),
			want: "Test Org, test.com",
		},
		{
			name: "only common name",
			cert: makeCert(pkix.Name{
				CommonName: "localhost",
			}),
			want: "localhost",
		},
		{
			name: "empty issuer - should return issuer string",
			cert: makeCert(pkix.Name{}),
			want: "",
		},
		{
			name: "multiple countries - should use first",
			cert: makeCert(pkix.Name{
				Country:    []string{"US", "UK"},
				CommonName: "test.com",
			}),
			want: "US, test.com",
		},
		{
			name: "multiple organizations - should use first",
			cert: makeCert(pkix.Name{
				Organization: []string{"Org1", "Org2"},
				CommonName:   "test.com",
			}),
			want: "Org1, test.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatIssuer(tt.cert)
			if got != tt.want {
				t.Errorf("FormatIssuer() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildCertificateDTO(t *testing.T) {
	t.Parallel()

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	cert := &model.Certificate{
		Domain:        "example.com",
		Issuer:        "Test CA",
		ExpiresAt:     now,
		Status:        "valid",
		LastCheckedAt: &yesterday,
		ErrorMessage:  "",
	}
	cert.ID = 123
	cert.CreatedAt = now

	dto := BuildCertificateDTO(cert)

	if dto.ID != 123 {
		t.Errorf("ID = %d, want 123", dto.ID)
	}
	if dto.Domain != "example.com" {
		t.Errorf(`Domain = %s, want "example.com"`, dto.Domain)
	}
	if dto.Issuer != "Test CA" {
		t.Errorf(`Issuer = %s, want "Test CA"`, dto.Issuer)
	}
	if dto.Status != "valid" {
		t.Errorf(`Status = %s, want "valid"`, dto.Status)
	}
	if dto.ExpiresAt == "" {
		t.Error("ExpiresAt should not be empty")
	}
	if dto.LastCheckedAt == "" {
		t.Error("LastCheckedAt should not be empty")
	}
	if dto.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
}

func TestBuildCertificateDTOWithNilLastChecked(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cert := &model.Certificate{
		Domain:        "test.com",
		Issuer:        "CA",
		ExpiresAt:     now,
		Status:        "valid",
		LastCheckedAt: nil,
		ErrorMessage:  "",
	}
	cert.ID = 1
	cert.CreatedAt = now

	dto := BuildCertificateDTO(cert)

	if dto.LastCheckedAt != "" {
		t.Errorf(`LastCheckedAt = %s, want empty string for nil time`, dto.LastCheckedAt)
	}
}

func TestUpdateCertificateStatus(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tests := []struct {
		name          string
		expiresAtFunc func(time.Time) time.Time
		wantStatus    string
		wantErrMsg    string
		wantLastCheck bool
	}{
		{
			name:          "expired certificate",
			expiresAtFunc: func(t time.Time) time.Time { return t.Add(-24 * time.Hour) },
			wantStatus:    "expired",
			wantErrMsg:    "",
			wantLastCheck: true,
		},
		{
			name:          "expiring soon (within 30 days)",
			expiresAtFunc: func(t time.Time) time.Time { return t.Add(15 * 24 * time.Hour) },
			wantStatus:    "expiring",
			wantErrMsg:    "",
			wantLastCheck: true,
		},
		{
			name:          "valid certificate",
			expiresAtFunc: func(t time.Time) time.Time { return t.Add(60 * 24 * time.Hour) },
			wantStatus:    "valid",
			wantErrMsg:    "",
			wantLastCheck: true,
		},
		{
			name:          "zero expiration date - invalid",
			expiresAtFunc: func(t time.Time) time.Time { return time.Time{} },
			wantStatus:    "invalid",
			wantErrMsg:    "missing expiration date",
			wantLastCheck: true,
		},
		{
			name:          "just over 30 days - valid",
			expiresAtFunc: func(t time.Time) time.Time { return t.Add(30*24*time.Hour + time.Second) },
			wantStatus:    "valid",
			wantErrMsg:    "",
			wantLastCheck: true,
		},
		{
			name:          "just under 30 days - expiring",
			expiresAtFunc: func(t time.Time) time.Time { return t.Add(30*24*time.Hour - time.Second) },
			wantStatus:    "expiring",
			wantErrMsg:    "",
			wantLastCheck: true,
		},
		{
			name:          "31 days - valid",
			expiresAtFunc: func(t time.Time) time.Time { return t.Add(31 * 24 * time.Hour) },
			wantStatus:    "valid",
			wantErrMsg:    "",
			wantLastCheck: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := &model.Certificate{
				ExpiresAt:    tt.expiresAtFunc(now),
				Status:       "old_status",
				ErrorMessage: "old error",
			}

			UpdateCertificateStatus(cert)

			if cert.Status != tt.wantStatus {
				t.Errorf("Status = %s, want %s", cert.Status, tt.wantStatus)
			}
			if cert.ErrorMessage != tt.wantErrMsg {
				t.Errorf(`ErrorMessage = %s, want "%s"`, cert.ErrorMessage, tt.wantErrMsg)
			}
			if cert.LastCheckedAt == nil {
				t.Error("LastCheckedAt should be set")
			} else if time.Since(*cert.LastCheckedAt) > time.Second {
				t.Error("LastCheckedAt should be recent")
			}
		})
	}
}

func TestUpdateCertificateStatusErrorMessageCleared(t *testing.T) {
	t.Parallel()

	cert := &model.Certificate{
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		Status:       "expired",
		ErrorMessage: "previous error message",
	}

	UpdateCertificateStatus(cert)

	if cert.ErrorMessage != "" {
		t.Errorf(`ErrorMessage should be cleared, got "%s"`, cert.ErrorMessage)
	}
}

func TestFormatIssuerEdgeCases(t *testing.T) {
	t.Parallel()

	// Helper to create test certificate directly without signing
	makeCert := func(issuer pkix.Name) *x509.Certificate {
		return &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: "test.example.com"},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(365 * 24 * time.Hour),
			KeyUsage:              x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
			Issuer:                issuer,
		}
	}

	// Test with certificate having only organizational unit
	cert := makeCert(pkix.Name{
		OrganizationalUnit: []string{"Engineering"},
	})
	got := FormatIssuer(cert)
	want := "Engineering"
	if got != want {
		t.Errorf("FormatIssuer() with only OU = %q, want %q", got, want)
	}

	// Test with certificate having only country
	cert2 := makeCert(pkix.Name{
		Country: []string{"CN"},
	})
	got2 := FormatIssuer(cert2)
	want2 := "CN"
	if got2 != want2 {
		t.Errorf("FormatIssuer() with only Country = %q, want %q", got2, want2)
	}
}
