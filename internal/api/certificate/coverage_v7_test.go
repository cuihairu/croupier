package certificate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestParseCertificatePEM_V7(t *testing.T) {
	// Generate a test certificate
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test.example.com", Organization: []string{"Test Org"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"test.example.com"},
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Valid PEM
	cert, err := ParseCertificatePEM(string(certPEM))
	assert.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, "test.example.com", cert.Subject.CommonName)

	// Invalid PEM
	_, err = ParseCertificatePEM("not a certificate")
	assert.Error(t, err)

	// Empty PEM
	_, err = ParseCertificatePEM("")
	assert.Error(t, err)

	// Valid PEM block but invalid certificate bytes
	badBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{0x00}})
	_, err = ParseCertificatePEM(string(badBlock))
	assert.Error(t, err)
}

func TestFormatIssuer_V7(t *testing.T) {
	// nil cert
	assert.Equal(t, "Unknown", FormatIssuer(nil))

	// cert with all fields
	cert := &x509.Certificate{
		Issuer: pkix.Name{
			Country:            []string{"US"},
			Organization:       []string{"ACME"},
			OrganizationalUnit: []string{"IT"},
			CommonName:         "Root CA",
		},
	}
	result := FormatIssuer(cert)
	assert.Contains(t, result, "US")
	assert.Contains(t, result, "ACME")
	assert.Contains(t, result, "IT")
	assert.Contains(t, result, "Root CA")

	// cert with no fields: pkix.Name.String() renders an empty string
	cert = &x509.Certificate{Issuer: pkix.Name{}}
	result = FormatIssuer(cert)
	assert.Empty(t, result)
}

func TestFormatSubject_V7(t *testing.T) {
	// nil cert
	assert.Equal(t, "", FormatSubject(nil))

	// cert with common name
	cert := &x509.Certificate{
		Subject: pkix.Name{CommonName: "test.example.com"},
	}
	assert.Equal(t, "test.example.com", FormatSubject(cert))

	// cert without common name, with organization
	cert = &x509.Certificate{
		Subject: pkix.Name{Organization: []string{"Org1", "Org2"}},
	}
	assert.Equal(t, "Org1, Org2", FormatSubject(cert))

	// empty cert
	cert = &x509.Certificate{}
	assert.Equal(t, "", FormatSubject(cert))
}

func TestCertificateDaysLeft_V7(t *testing.T) {
	// zero time
	result := certificateDaysLeft(time.Time{})
	assert.Nil(t, result)

	// future date
	result = certificateDaysLeft(time.Now().Add(30 * 24 * time.Hour))
	assert.NotNil(t, result)
	assert.True(t, *result > 28, "should be ~30 days")

	// past date
	result = certificateDaysLeft(time.Now().Add(-24 * time.Hour))
	assert.NotNil(t, result)
	assert.True(t, *result < 0, "should be negative")
}

func TestBuildCertificateDTO_V7(t *testing.T) {
	now := time.Now()
	cert := &model.Certificate{
		Model:         gorm.Model{ID: 1, CreatedAt: now},
		Domain:        "example.com",
		Port:          443,
		Issuer:        "Test CA",
		Subject:       "example.com",
		StartsAt:      &now,
		ExpiresAt:     now.Add(365 * 24 * time.Hour),
		Status:        "valid",
		LastCheckedAt: &now,
		ErrorMessage:  "",
	}
	dto := BuildCertificateDTO(cert)
	assert.Equal(t, uint(1), dto.ID)
	assert.Equal(t, "example.com", dto.Domain)
	assert.Equal(t, 443, dto.Port)
	assert.Equal(t, "valid", dto.Status)
	assert.NotNil(t, dto.DaysLeft)
	assert.True(t, *dto.DaysLeft > 360)
}

func TestUpdateCertificateStatus_V7(t *testing.T) {
	// Zero expiry
	cert := &model.Certificate{ExpiresAt: time.Time{}}
	UpdateCertificateStatus(cert)
	assert.Equal(t, "invalid", cert.Status)
	assert.NotEmpty(t, cert.ErrorMessage)

	// Expired
	cert = &model.Certificate{ExpiresAt: time.Now().Add(-24 * time.Hour)}
	UpdateCertificateStatus(cert)
	assert.Equal(t, "expired", cert.Status)
	assert.Empty(t, cert.ErrorMessage)

	// Expiring (within 30 days)
	cert = &model.Certificate{ExpiresAt: time.Now().Add(10 * 24 * time.Hour)}
	UpdateCertificateStatus(cert)
	assert.Equal(t, "expiring", cert.Status)

	// Valid (more than 30 days)
	cert = &model.Certificate{ExpiresAt: time.Now().Add(60 * 24 * time.Hour)}
	UpdateCertificateStatus(cert)
	assert.Equal(t, "valid", cert.Status)

	// LastCheckedAt is set
	assert.NotNil(t, cert.LastCheckedAt)
}

func TestParseUintID_V7(t *testing.T) {
	id, err := parseUintID("123", "test")
	assert.NoError(t, err)
	assert.Equal(t, uint(123), id)

	_, err = parseUintID("abc", "test")
	assert.Error(t, err)

	_, err = parseUintID("", "test")
	assert.Error(t, err)

	_, err = parseUintID("-1", "test")
	assert.Error(t, err)
}
