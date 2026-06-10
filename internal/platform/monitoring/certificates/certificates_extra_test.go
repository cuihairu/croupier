package certificates

import (
	"crypto/x509"
	"fmt"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDBExtra(t *testing.T) *gorm.DB {
	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestStore_ListCertificates_SecondPage(t *testing.T) {
	db := setupTestDBExtra(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	// Add 5 certificates
	for i := 0; i < 5; i++ {
		cert := &Certificate{
			Domain:  fmt.Sprintf("cert%d.example.com", i),
			Enabled: true,
			Status:  "valid",
		}
		require.NoError(t, db.Create(cert).Error)
	}

	// Get second page
	certs, total, err := store.ListCertificates(2, 2, "")
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, certs, 2)
}

func TestStore_ListCertificates_ThirdPage(t *testing.T) {
	db := setupTestDBExtra(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	// Add 5 certificates
	for i := 0; i < 5; i++ {
		cert := &Certificate{
			Domain:  fmt.Sprintf("cert%d.example.com", i),
			Enabled: true,
			Status:  "valid",
		}
		require.NoError(t, db.Create(cert).Error)
	}

	// Get third page (should have 1 item)
	certs, total, err := store.ListCertificates(3, 2, "")
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, certs, 1)
}

func TestStore_ListCertificates_EmptyPage(t *testing.T) {
	db := setupTestDBExtra(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	// Add 2 certificates
	for i := 0; i < 2; i++ {
		cert := &Certificate{
			Domain:  fmt.Sprintf("cert%d.example.com", i),
			Enabled: true,
			Status:  "valid",
		}
		require.NoError(t, db.Create(cert).Error)
	}

	// Get page beyond data
	certs, total, err := store.ListCertificates(10, 10, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, certs, 0)
}

func TestStore_formatKeyUsage_DataEncipherment(t *testing.T) {
	store := NewStore(nil)

	// Test Data Encipherment only
	result := store.formatKeyUsage(x509.KeyUsageDataEncipherment, []x509.ExtKeyUsage{})
	assert.Equal(t, "Data Encipherment", result)
}

func TestStore_formatKeyUsage_DigitalSignatureAndDataEncipherment(t *testing.T) {
	store := NewStore(nil)

	// Test Digital Signature + Data Encipherment
	result := store.formatKeyUsage(
		x509.KeyUsageDigitalSignature|x509.KeyUsageDataEncipherment,
		[]x509.ExtKeyUsage{},
	)
	assert.Contains(t, result, "Digital Signature")
	assert.Contains(t, result, "Data Encipherment")
	assert.NotContains(t, result, "Key Encipherment")
}

func TestStore_formatKeyUsage_ExtKeyUsageOnly(t *testing.T) {
	store := NewStore(nil)

	// Test with only ExtKeyUsage
	result := store.formatKeyUsage(0, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	assert.Equal(t, "Server Authentication", result)
}

func TestStore_formatKeyUsage_ClientAuthOnly(t *testing.T) {
	store := NewStore(nil)

	// Test with only Client Auth
	result := store.formatKeyUsage(0, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	assert.Equal(t, "Client Authentication", result)
}

func TestStore_formatKeyUsage_UnknownExtKeyUsage(t *testing.T) {
	store := NewStore(nil)

	// Test with unknown ExtKeyUsage (should be ignored)
	result := store.formatKeyUsage(0, []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping})
	assert.Equal(t, "", result)
}

func TestStore_CheckCertificate_ErrorPath(t *testing.T) {
	db := setupTestDBExtra(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	// Add certificate with non-existent domain
	cert := &Certificate{
		Domain:    "nonexistent.example.com",
		Port:      9999,
		Enabled:   true,
		Status:    "pending",
		AlertDays: 30,
	}
	require.NoError(t, db.Create(cert).Error)

	// Check should fail and set status to error
	err := store.CheckCertificate(cert.ID)
	require.NoError(t, err) // CheckCertificate returns nil even on fetch error

	var updated Certificate
	require.NoError(t, db.First(&updated, cert.ID).Error)
	assert.Equal(t, "error", updated.Status)
	assert.NotEmpty(t, updated.ErrorMsg)
	assert.Contains(t, updated.ErrorMsg, "failed to connect")
}

func TestStore_CheckAllCertificates_WithDisabledCerts(t *testing.T) {
	db := setupTestDBExtra(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	// Add disabled certificates
	for i := 0; i < 3; i++ {
		cert := &Certificate{
			Domain:  fmt.Sprintf("disabled%d.example.com", i),
			Enabled: false,
			Status:  "pending",
		}
		require.NoError(t, db.Create(cert).Error)
		// Ensure disabled
		require.NoError(t, db.Model(cert).Update("enabled", false).Error)
	}

	// Check all should succeed (disabled certs are skipped)
	err := store.CheckAllCertificates()
	require.NoError(t, err)
}

func TestStore_CheckAllCertificates_MixedEnabled(t *testing.T) {
	db := setupTestDBExtra(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	// Add enabled cert with non-existent domain (will fail)
	enabledCert := &Certificate{
		Domain:    "nonexistent.example.com",
		Port:      9999,
		Enabled:   true,
		Status:    "pending",
		AlertDays: 30,
	}
	require.NoError(t, db.Create(enabledCert).Error)

	// Add disabled cert
	disabledCert := &Certificate{
		Domain:  "disabled.example.com",
		Enabled: false,
		Status:  "pending",
	}
	require.NoError(t, db.Create(disabledCert).Error)
	require.NoError(t, db.Model(disabledCert).Update("enabled", false).Error)

	// Check all should succeed (enabled cert fails but continues)
	err := store.CheckAllCertificates()
	require.NoError(t, err)

	// Verify enabled cert was marked as error
	var updated Certificate
	require.NoError(t, db.First(&updated, enabledCert.ID).Error)
	assert.Equal(t, "error", updated.Status)
}

func TestStore_GetDomainInfo_WithDNSLookup(t *testing.T) {
	store := NewStore(nil)

	// Test with a domain that has NS records
	// Note: This test depends on DNS availability
	info, err := store.GetDomainInfo("example.com")
	require.NoError(t, err)
	assert.Equal(t, "example.com", info.Domain)
	assert.Equal(t, "active", info.Status)
	// NS lookup may or may not succeed depending on network
}

func TestStore_GetDomainInfo_Localhost(t *testing.T) {
	store := NewStore(nil)

	info, err := store.GetDomainInfo("localhost")
	require.NoError(t, err)
	assert.Equal(t, "localhost", info.Domain)
	assert.Equal(t, "active", info.Status)
}

func TestStore_fetchCertificateInfo_ConnectionError(t *testing.T) {
	store := NewStore(nil)

	// Test connection to non-existent server
	_, err := store.fetchCertificateInfo("192.0.2.1", 1) // RFC 5737 test address
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
}

func TestStore_fetchCertificateInfo_InvalidDomain(t *testing.T) {
	store := NewStore(nil)

	// Test with invalid domain
	_, err := store.fetchCertificateInfo("invalid.domain.that.doesnt.exist.example", 443)
	assert.Error(t, err)
}
