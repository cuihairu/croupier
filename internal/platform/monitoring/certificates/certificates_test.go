package certificates

import (
	"crypto/x509"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestNewStore(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	assert.NotNil(t, store)
	assert.Equal(t, db, store.db)
}

func TestStore_AutoMigrate(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	err := store.AutoMigrate()
	require.NoError(t, err)

	// Verify tables exist
	if err := db.Exec("SELECT * FROM certificates LIMIT 1").Error; err != nil {
		// Table doesn't exist or query failed
		assert.Fail(t, "certificates table should exist")
	}
	if err := db.Exec("SELECT * FROM certificate_alerts LIMIT 1").Error; err != nil {
		// Table doesn't exist or query failed
		assert.Fail(t, "certificate_alerts table should exist")
	}
}

func TestStore_AddDomain(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	t.Run("add new domain", func(t *testing.T) {
		err := store.AddDomain("example.com", 443, 30)
		require.NoError(t, err)

		var cert Certificate
		err = db.Where("domain = ?", "example.com").First(&cert).Error
		require.NoError(t, err)
		assert.Equal(t, "example.com", cert.Domain)
		assert.Equal(t, 443, cert.Port)
		assert.Equal(t, 30, cert.AlertDays)
		assert.True(t, cert.Enabled)
	})

	t.Run("add duplicate domain returns existing", func(t *testing.T) {
		err := store.AddDomain("test.com", 443, 30)
		require.NoError(t, err)

		// Add again
		err = store.AddDomain("test.com", 443, 30)
		require.NoError(t, err)

		var count int64
		db.Model(&Certificate{}).Where("domain = ?", "test.com").Count(&count)
		assert.Equal(t, int64(1), count)
	})
}

func TestStore_CheckCertificate_NonExistent(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	err := store.CheckCertificate(999)
	assert.Error(t, err)
}

func TestStore_CheckCertificate_Disabled(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	cert := &Certificate{
		Domain:    "example.com",
		Port:      443,
		Enabled:   false,
		Status:    "pending",
		AlertDays: 30,
	}
	require.NoError(t, db.Create(cert).Error)
	// Ensure Enabled is set to false via explicit update
	require.NoError(t, db.Model(cert).Update("enabled", false).Error)

	// Should not error even though disabled
	err := store.CheckCertificate(cert.ID)
	require.NoError(t, err)

	// Status should remain pending since certificate is disabled
	var updated Certificate
	require.NoError(t, db.First(&updated, cert.ID).Error)
	assert.Equal(t, "pending", updated.Status)
}

func TestStore_CheckAllCertificates(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	t.Run("no certificates", func(t *testing.T) {
		err := store.CheckAllCertificates()
		require.NoError(t, err)
	})

	t.Run("with disabled certificates", func(t *testing.T) {
		cert := &Certificate{
			Domain:    "example.com",
			Port:      443,
			Enabled:   false,
			Status:    "pending",
			AlertDays: 30,
		}
		require.NoError(t, db.Create(cert).Error)

		err := store.CheckAllCertificates()
		require.NoError(t, err)
	})
}

func TestStore_GetExpiringCertificates(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	t.Run("no certificates", func(t *testing.T) {
		certs, err := store.GetExpiringCertificates()
		require.NoError(t, err)
		assert.Empty(t, certs)
	})

	t.Run("with expiring certificates", func(t *testing.T) {
		cert1 := &Certificate{Domain: "expiring.com", Enabled: true, Status: "expiring"}
		cert2 := &Certificate{Domain: "expired.com", Enabled: true, Status: "expired"}
		cert3 := &Certificate{Domain: "valid.com", Enabled: true, Status: "valid"}
		cert4 := &Certificate{Domain: "disabled-expiring.com", Enabled: false, Status: "expiring"}

		require.NoError(t, db.Create(cert1).Error)
		require.NoError(t, db.Create(cert2).Error)
		require.NoError(t, db.Create(cert3).Error)
		require.NoError(t, db.Create(cert4).Error)
		// Ensure cert4 is actually disabled
		require.NoError(t, db.Model(cert4).Update("enabled", false).Error)

		certs, err := store.GetExpiringCertificates()
		require.NoError(t, err)
		assert.Len(t, certs, 2) // Only enabled expiring/expired
	})
}

func TestStore_ListCertificates(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	// Add test data
	cert1 := &Certificate{Domain: "cert1.com", Enabled: true, Status: "valid"}
	cert2 := &Certificate{Domain: "cert2.com", Enabled: true, Status: "expiring"}
	cert3 := &Certificate{Domain: "cert3.com", Enabled: true, Status: "expired"}
	require.NoError(t, db.Create(cert1).Error)
	require.NoError(t, db.Create(cert2).Error)
	require.NoError(t, db.Create(cert3).Error)

	t.Run("list all certificates", func(t *testing.T) {
		certs, total, err := store.ListCertificates(1, 10, "")
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, certs, 3)
	})

	t.Run("filter by status", func(t *testing.T) {
		certs, total, err := store.ListCertificates(1, 10, "valid")
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, certs, 1)
		assert.Equal(t, "valid", certs[0].Status)
	})

	t.Run("pagination", func(t *testing.T) {
		certs, total, err := store.ListCertificates(1, 2, "")
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, certs, 2)
	})
}

func TestStore_AddAlert(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	cert := &Certificate{Domain: "example.com", Enabled: true}
	require.NoError(t, db.Create(cert).Error)

	t.Run("add alert", func(t *testing.T) {
		err := store.AddAlert(cert.ID, "email", "admin@example.com")
		require.NoError(t, err)

		var alert CertificateAlert
		err = db.Where("certificate_id = ?", cert.ID).First(&alert).Error
		require.NoError(t, err)
		assert.Equal(t, "email", alert.AlertType)
		assert.Equal(t, "admin@example.com", alert.Target)
		assert.True(t, alert.Enabled)
	})
}

func TestStore_GetAlertsForCertificate(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	cert := &Certificate{Domain: "example.com", Enabled: true}
	require.NoError(t, db.Create(cert).Error)

	alert1 := &CertificateAlert{CertificateID: cert.ID, AlertType: "email", Target: "admin@example.com", Enabled: true}
	alert2 := &CertificateAlert{CertificateID: cert.ID, AlertType: "webhook", Target: "http://example.com/hook", Enabled: true}
	alert3 := &CertificateAlert{CertificateID: cert.ID, AlertType: "sms", Target: "+1234567890", Enabled: false}
	require.NoError(t, db.Create(alert1).Error)
	require.NoError(t, db.Create(alert2).Error)
	require.NoError(t, db.Create(alert3).Error)
	// Ensure alert3 is actually disabled
	require.NoError(t, db.Model(alert3).Update("enabled", false).Error)

	alerts, err := store.GetAlertsForCertificate(cert.ID)
	require.NoError(t, err)
	assert.Len(t, alerts, 2) // Only enabled alerts
}

func TestStore_GetByID(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	t.Run("certificate exists", func(t *testing.T) {
		cert := &Certificate{Domain: "example.com", Enabled: true}
		require.NoError(t, db.Create(cert).Error)

		found, err := store.GetByID(cert.ID)
		require.NoError(t, err)
		assert.Equal(t, "example.com", found.Domain)
	})

	t.Run("certificate not found", func(t *testing.T) {
		_, err := store.GetByID(999)
		assert.Error(t, err)
	})
}

func TestStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	cert := &Certificate{Domain: "example.com", Enabled: true}
	require.NoError(t, db.Create(cert).Error)

	err := store.Delete(cert.ID)
	require.NoError(t, err)

	var found Certificate
	err = db.First(&found, cert.ID).Error
	assert.Error(t, err)
}

func TestStore_GetDomainInfo(t *testing.T) {
	store := NewStore(nil)

	t.Run("valid domain", func(t *testing.T) {
		info, err := store.GetDomainInfo("example.com")
		require.NoError(t, err)
		assert.Equal(t, "example.com", info.Domain)
		assert.Equal(t, "active", info.Status)
	})

	t.Run("empty domain", func(t *testing.T) {
		info, err := store.GetDomainInfo("")
		require.NoError(t, err)
		assert.Equal(t, "", info.Domain)
		assert.Equal(t, "active", info.Status)
	})
}

func TestStore_GetCertificateStats(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())

	t.Run("no certificates", func(t *testing.T) {
		stats, err := store.GetCertificateStats()
		require.NoError(t, err)
		assert.Equal(t, int64(0), stats.Total)
		assert.Equal(t, int64(0), stats.Valid)
		assert.Equal(t, int64(0), stats.Expiring)
		assert.Equal(t, int64(0), stats.Expired)
		assert.Equal(t, int64(0), stats.Errors)
	})

	t.Run("with various statuses", func(t *testing.T) {
		now := time.Now()
		cert1 := &Certificate{Domain: "valid.com", Enabled: true, Status: "valid", LastChecked: now}
		cert2 := &Certificate{Domain: "expiring.com", Enabled: true, Status: "expiring", LastChecked: now.Add(-time.Hour)}
		cert3 := &Certificate{Domain: "expired.com", Enabled: true, Status: "expired", LastChecked: now.Add(-2 * time.Hour)}
		cert4 := &Certificate{Domain: "error.com", Enabled: true, Status: "error", LastChecked: now.Add(-3 * time.Hour)}
		cert5 := &Certificate{Domain: "disabled.com", Enabled: false, Status: "valid"} // Should not be counted

		require.NoError(t, db.Create(cert1).Error)
		require.NoError(t, db.Create(cert2).Error)
		require.NoError(t, db.Create(cert3).Error)
		require.NoError(t, db.Create(cert4).Error)
		require.NoError(t, db.Create(cert5).Error)
		// Ensure cert5 is actually disabled
		require.NoError(t, db.Model(cert5).Update("enabled", false).Error)

		stats, err := store.GetCertificateStats()
		require.NoError(t, err)
		assert.Equal(t, int64(4), stats.Total)
		assert.Equal(t, int64(1), stats.Valid)
		assert.Equal(t, int64(1), stats.Expiring)
		assert.Equal(t, int64(1), stats.Expired)
		assert.Equal(t, int64(1), stats.Errors)
		assert.False(t, stats.LastChecked.IsZero())
	})
}

func TestStore_formatKeyUsage(t *testing.T) {
	store := NewStore(nil)

	t.Run("digital signature only", func(t *testing.T) {
		result := store.formatKeyUsage(x509.KeyUsageDigitalSignature, []x509.ExtKeyUsage{})
		assert.Equal(t, "Digital Signature", result)
	})

	t.Run("key encipherment", func(t *testing.T) {
		result := store.formatKeyUsage(x509.KeyUsageKeyEncipherment, []x509.ExtKeyUsage{})
		assert.Equal(t, "Key Encipherment", result)
	})

	t.Run("combined usages", func(t *testing.T) {
		result := store.formatKeyUsage(
			x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment,
			[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		)
		assert.Contains(t, result, "Digital Signature")
		assert.Contains(t, result, "Key Encipherment")
		assert.Contains(t, result, "Server Authentication")
		assert.Contains(t, result, "Client Authentication")
	})

	t.Run("no usage", func(t *testing.T) {
		result := store.formatKeyUsage(0, []x509.ExtKeyUsage{})
		assert.Equal(t, "", result)
	})
}

func TestCertificate_TableName(t *testing.T) {
	cert := Certificate{}
	assert.Equal(t, "certificates", cert.TableName())
}

func TestCertificateAlert_TableName(t *testing.T) {
	alert := CertificateAlert{}
	assert.Equal(t, "certificate_alerts", alert.TableName())
}
