package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewCertificateModel(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	assert.NotNil(t, model)
}

func TestCertificateStatus(t *testing.T) {
	tests := []struct {
		name     string
		expiry   time.Time
		expected string
	}{
		{"expired", time.Now().Add(-24 * time.Hour), "expired"},
		{"expiring soon", time.Now().Add(15 * 24 * time.Hour), "expiring"},
		{"active", time.Now().Add(60 * 24 * time.Hour), "active"},
		{"unknown", time.Time{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, CertificateStatus(tt.expiry))
		})
	}
}

func TestCertificateModel_Create(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	cert := &Certificate{
		Domain:         "example.com",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		PrivateKeyPEM:  "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
		Issuer:         "Let's Encrypt",
		ExpiresAt:      time.Now().Add(90 * 24 * time.Hour),
		Status:         "active",
	}

	err := model.Create(ctx, cert)
	require.NoError(t, err)
	assert.NotZero(t, cert.ID)
}

func TestCertificateModel_Update(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	cert := &Certificate{
		Domain:    "update.example.com",
		Issuer:    "Test Issuer",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Status:    "active",
	}
	err := model.Create(ctx, cert)
	require.NoError(t, err)

	// Update
	updates := map[string]interface{}{
		"status":        "expiring",
		"error_message": "Certificate expiring soon",
	}
	err = model.Update(ctx, cert.ID, updates)
	require.NoError(t, err)

	// Verify
	found, err := model.FindOne(ctx, cert.ID)
	require.NoError(t, err)
	assert.Equal(t, "expiring", found.Status)
}

func TestCertificateModel_Delete(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	cert := &Certificate{
		Domain:    "delete.example.com",
		Issuer:    "Test",
		ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
		Status:    "active",
	}
	err := model.Create(ctx, cert)
	require.NoError(t, err)

	// Delete
	err = model.Delete(ctx, cert.ID)
	require.NoError(t, err)

	// Verify
	_, err = model.FindOne(ctx, cert.ID)
	assert.Error(t, err)
}

func TestCertificateModel_FindOne(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	cert := &Certificate{
		Domain:    "findone.example.com",
		Issuer:    "Test Issuer",
		ExpiresAt: time.Now().Add(60 * 24 * time.Hour),
		Status:    "active",
	}
	err := model.Create(ctx, cert)
	require.NoError(t, err)

	// Find existing
	found, err := model.FindOne(ctx, cert.ID)
	require.NoError(t, err)
	assert.Equal(t, "findone.example.com", found.Domain)

	// Find non-existent
	_, err = model.FindOne(ctx, 99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "证书不存在")
}

func TestCertificateModel_FindByDomain(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	cert := &Certificate{
		Domain:    "finddomain.example.com",
		Issuer:    "Test Issuer",
		ExpiresAt: time.Now().Add(60 * 24 * time.Hour),
		Status:    "active",
	}
	err := model.Create(ctx, cert)
	require.NoError(t, err)

	// Find by domain
	found, err := model.FindByDomain(ctx, "finddomain.example.com")
	require.NoError(t, err)
	assert.Equal(t, cert.ID, found.ID)

	// Find non-existent domain
	_, err = model.FindByDomain(ctx, "nonexistent.example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "证书不存在")
}

func TestCertificateModel_ExpiringWithin(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	// Create certificates with different expiry times
	certs := []*Certificate{
		{Domain: "expired.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(-1 * time.Hour), Status: "expired"},
		{Domain: "expiring.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(15 * 24 * time.Hour), Status: "expiring"},
		{Domain: "active.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(60 * 24 * time.Hour), Status: "active"},
	}
	for _, c := range certs {
		err := model.Create(ctx, c)
		require.NoError(t, err)
	}

	// Find expiring within 30 days
	expiring, err := model.ExpiringWithin(ctx, 30*24*time.Hour)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(expiring), 2) // expired + expiring
}

func TestCertificateModel_Stats(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	certs := []*Certificate{
		{Domain: "cert1.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(-1 * time.Hour), Status: "expired"},
		{Domain: "cert2.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(15 * 24 * time.Hour), Status: "expiring"},
		{Domain: "cert3.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(15 * 24 * time.Hour), Status: "expiring"},
		{Domain: "cert4.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(90 * 24 * time.Hour), Status: "active"},
	}
	for _, c := range certs {
		err := model.Create(ctx, c)
		require.NoError(t, err)
	}

	// Get stats
	stats, err := model.Stats(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats["total"], int64(4))
	assert.GreaterOrEqual(t, stats["expiring"], int64(2))
	assert.GreaterOrEqual(t, stats["expired"], int64(1))
	assert.GreaterOrEqual(t, stats["active"], int64(1))
}

func TestCertificateModel_ListAlerts(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	alerts := []*CertificateAlert{
		{Domain: "alert1.example.com", ThresholdDays: 30, Active: true},
		{Domain: "alert2.example.com", ThresholdDays: 15, Active: true},
		{Domain: "alert3.example.com", ThresholdDays: 7, Active: false},
	}
	for _, a := range alerts {
		err := model.Create(ctx, &Certificate{
			Domain:    a.Domain,
			Issuer:    "Test",
			ExpiresAt: time.Now().Add(60 * 24 * time.Hour),
			Status:    "active",
		})
		require.NoError(t, err)
		err = model.AddAlert(ctx, a)
		require.NoError(t, err)
	}

	// List alerts
	list, total, err := model.ListAlerts(ctx, 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(3))
	assert.GreaterOrEqual(t, len(list), 3)
}

func TestCertificateModel_AddAlert(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	alert := &CertificateAlert{
		Domain:        "addalert.example.com",
		ThresholdDays: 30,
		Active:        true,
	}

	err := model.AddAlert(ctx, alert)
	require.NoError(t, err)
	assert.NotZero(t, alert.ID)
}

func TestCertificateModel_ListAll(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	certs := []*Certificate{
		{Domain: "all1.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(60 * 24 * time.Hour), Status: "active"},
		{Domain: "all2.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(90 * 24 * time.Hour), Status: "active"},
	}
	for _, c := range certs {
		err := model.Create(ctx, c)
		require.NoError(t, err)
	}

	// List all
	all, err := model.ListAll(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 2)
}

func TestCertificateModel_List(t *testing.T) {
	db := setupCertificateTestDB(t)
	model := NewCertificateModel(db)
	ctx := context.Background()

	certs := []*Certificate{
		{Domain: "list1.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(-1 * time.Hour), Status: "expired"},
		{Domain: "list2.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(15 * 24 * time.Hour), Status: "expiring"},
		{Domain: "list3.example.com", Issuer: "Test", ExpiresAt: time.Now().Add(90 * 24 * time.Hour), Status: "active"},
	}
	for _, c := range certs {
		err := model.Create(ctx, c)
		require.NoError(t, err)
	}

	// List all
	_, total, err := model.List(ctx, ListCertificatesOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(3))

	// Filter by status
	items, total, err := model.List(ctx, ListCertificatesOptions{Status: "expiring"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	_ = items // items is used for verification
}

// ===== ConfigVersionModel Tests =====
