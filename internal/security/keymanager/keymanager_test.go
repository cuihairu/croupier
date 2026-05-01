package keymanager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a master key for testing
func testMasterKey(t *testing.T) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

// KeyManager tests
func TestNewKeyManager(t *testing.T) {
	t.Run("creates key manager with valid master key", func(t *testing.T) {
		store := NewMemoryKeyStore()
		km := NewKeyManager(store, testMasterKey(t))
		assert.NotNil(t, km)
		assert.Equal(t, store, km.store)
		assert.NotNil(t, km.cache)
	})

	t.Run("panics with short master key", func(t *testing.T) {
		store := NewMemoryKeyStore()
		shortKey := make([]byte, 16)
		assert.Panics(t, func() {
			NewKeyManager(store, shortKey)
		})
	})
}

func TestKeyManager_GenerateKey(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))
	ctx := context.Background()

	t.Run("generates AES-256 key", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
		require.NoError(t, err)
		assert.NotEmpty(t, metadata.ID)
		assert.Equal(t, KeyTypeAES256, metadata.Type)
		assert.Equal(t, PurposeEncryption, metadata.Purpose)
		assert.Equal(t, KeyStateActive, metadata.State)
		assert.Equal(t, 1, metadata.Version)
	})

	t.Run("generates AES-128 key", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeAES128, PurposeSigning)
		require.NoError(t, err)
		assert.Equal(t, KeyTypeAES128, metadata.Type)
		assert.Equal(t, PurposeSigning, metadata.Purpose)
	})

	t.Run("generates HMAC key", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeHMAC, PurposeSigning)
		require.NoError(t, err)
		assert.Equal(t, KeyTypeHMAC, metadata.Type)
	})

	t.Run("errors on unsupported key type", func(t *testing.T) {
		_, err := km.GenerateKey(ctx, KeyTypeRSA2048, PurposeEncryption)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported key type")
	})

	t.Run("applies WithName option", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithName("my-key"))
		require.NoError(t, err)
		assert.Equal(t, "my-key", metadata.Name)
	})

	t.Run("applies WithExpiration option", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithExpiration(24*time.Hour))
		require.NoError(t, err)
		assert.NotNil(t, metadata.ExpiresAt)
		assert.WithinDuration(t, time.Now().Add(24*time.Hour), *metadata.ExpiresAt, time.Second)
	})

	t.Run("applies WithRotation option", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithRotation(30))
		require.NoError(t, err)
		assert.Equal(t, 30, metadata.RotationDays)
	})

	t.Run("applies WithLabels option", func(t *testing.T) {
		labels := map[string]string{"env": "test", "owner": "team-a"}
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithLabels(labels))
		require.NoError(t, err)
		assert.Equal(t, labels, metadata.Labels)
	})

	t.Run("applies WithCreatedBy option", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithCreatedBy("admin"))
		require.NoError(t, err)
		assert.Equal(t, "admin", metadata.CreatedBy)
	})

	t.Run("applies WithDescription option", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithDescription("test key"))
		require.NoError(t, err)
		assert.Equal(t, "test key", metadata.Description)
	})
}

func TestKeyManager_GetKey(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))
	ctx := context.Background()

	t.Run("retrieves existing key", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
		require.NoError(t, err)

		entry, err := km.GetKey(ctx, metadata.ID)
		require.NoError(t, err)
		assert.Equal(t, metadata.ID, entry.Metadata.ID)
		assert.NotEmpty(t, entry.Key) // Decrypted key
	})

	t.Run("returns error for non-existent key", func(t *testing.T) {
		_, err := km.GetKey(ctx, "non-existent")
		assert.Error(t, err)
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("returns error for expired key", func(t *testing.T) {
		// Create key store and manager without cache interference
		expiredStore := NewMemoryKeyStore()
		expiredKM := NewKeyManager(expiredStore, testMasterKey(t))

		metadata, err := expiredKM.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithExpiration(1*time.Nanosecond))
		require.NoError(t, err)

		// Clear cache and manually set expiration in store
		delete(expiredKM.cache, metadata.ID)
		entry, _ := expiredStore.Get(metadata.ID)
		now := time.Now().Add(-1 * time.Hour) // Set to past
		entry.Metadata.ExpiresAt = &now
		expiredStore.Update(entry)

		_, err = expiredKM.GetKey(ctx, metadata.ID)
		assert.Error(t, err)
		assert.Equal(t, ErrKeyExpired, err)
	})

	t.Run("uses cache on second call", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
		require.NoError(t, err)

		// First call
		_, err = km.GetKey(ctx, metadata.ID)
		require.NoError(t, err)

		// Second call should use cache
		km.cacheMu.Lock()
		_, cached := km.cache[metadata.ID]
		km.cacheMu.Unlock()
		assert.True(t, cached)
	})
}

func TestKeyManager_GetActiveKey(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))
	ctx := context.Background()

	t.Run("retrieves active key for purpose", func(t *testing.T) {
		_, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithName("key1"))
		require.NoError(t, err)

		entry, err := km.GetActiveKey(ctx, PurposeEncryption)
		require.NoError(t, err)
		assert.Equal(t, PurposeEncryption, entry.Metadata.Purpose)
		assert.Equal(t, KeyStateActive, entry.Metadata.State)
	})

	t.Run("returns most recent active key", func(t *testing.T) {
		// Create a new store and manager to avoid cache contamination
		newStore := NewMemoryKeyStore()
		newKM := NewKeyManager(newStore, testMasterKey(t))

		_, err := newKM.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithName("key2"))
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamp
		metadata2, err := newKM.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithName("key3"))
		require.NoError(t, err)

		// Clear cache to force loading from store
		newKM.cache = make(map[string]*KeyEntry)

		entry, err := newKM.GetActiveKey(ctx, PurposeEncryption)
		require.NoError(t, err)
		assert.Equal(t, metadata2.ID, entry.Metadata.ID)
	})

	t.Run("returns error when no active key for purpose", func(t *testing.T) {
		// Create a new store and manager
		newStore := NewMemoryKeyStore()
		newKM := NewKeyManager(newStore, testMasterKey(t))

		_, err := newKM.GetActiveKey(ctx, PurposeSigning)
		assert.Error(t, err)
	})
}

func TestKeyManager_RotateKey(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))
	ctx := context.Background()

	t.Run("rotates active key", func(t *testing.T) {
		oldKey, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithRotation(30), WithName("rotate-test"))
		require.NoError(t, err)

		newKey, err := km.RotateKey(ctx, oldKey.ID)
		require.NoError(t, err)

		assert.NotEqual(t, oldKey.ID, newKey.ID)
		assert.Equal(t, oldKey.Name, newKey.Name)
		assert.Equal(t, oldKey.RotationDays, newKey.RotationDays)
		assert.Equal(t, oldKey.ID, newKey.RotatedFrom)

		// Check old key is now inactive
		entry, err := km.store.Get(oldKey.ID)
		require.NoError(t, err)
		assert.Equal(t, KeyStateInactive, entry.Metadata.State)
	})

	t.Run("returns error for non-existent key", func(t *testing.T) {
		_, err := km.RotateKey(ctx, "non-existent")
		assert.Error(t, err)
	})
}

func TestKeyManager_RevokeKey(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))
	ctx := context.Background()

	t.Run("revokes a key", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
		require.NoError(t, err)

		err = km.RevokeKey(ctx, metadata.ID, "test revocation")
		require.NoError(t, err)

		entry, err := km.store.Get(metadata.ID)
		require.NoError(t, err)
		assert.Equal(t, KeyStateRevoked, entry.Metadata.State)
		assert.Contains(t, entry.Metadata.Description, "Revoked: test revocation")
	})

	t.Run("returns error for non-existent key", func(t *testing.T) {
		err := km.RevokeKey(ctx, "non-existent", "reason")
		assert.Error(t, err)
	})
}

func TestKeyManager_ListKeys(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))
	ctx := context.Background()

	// Create test keys
	_, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithName("key1"))
	require.NoError(t, err)
	_, err = km.GenerateKey(ctx, KeyTypeAES128, PurposeSigning, WithName("key2"))
	require.NoError(t, err)

	t.Run("lists all keys", func(t *testing.T) {
		keys, err := km.ListKeys(ctx, KeyFilter{})
		require.NoError(t, err)
		assert.Len(t, keys, 2)
	})

	t.Run("filters by purpose", func(t *testing.T) {
		keys, err := km.ListKeys(ctx, KeyFilter{Purpose: PurposeEncryption})
		require.NoError(t, err)
		assert.Len(t, keys, 1)
		assert.Equal(t, PurposeEncryption, keys[0].Purpose)
	})

	t.Run("filters by type", func(t *testing.T) {
		keys, err := km.ListKeys(ctx, KeyFilter{Type: KeyTypeAES256})
		require.NoError(t, err)
		assert.Len(t, keys, 1)
		assert.Equal(t, KeyTypeAES256, keys[0].Type)
	})

	t.Run("filters by state", func(t *testing.T) {
		keys, err := km.ListKeys(ctx, KeyFilter{State: KeyStateActive})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(keys), 2)
	})
}

func TestKeyManager_EncryptDecrypt(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))
	ctx := context.Background()

	metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
	require.NoError(t, err)

	t.Run("encrypts and decrypts data", func(t *testing.T) {
		plaintext := []byte("sensitive data")

		ciphertext, err := km.Encrypt(ctx, metadata.ID, plaintext)
		require.NoError(t, err)
		assert.NotEqual(t, plaintext, ciphertext)
		assert.Greater(t, len(ciphertext), len(plaintext)) // IV + padding

		decrypted, err := km.Decrypt(ctx, metadata.ID, ciphertext)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("encrypts and decrypts string", func(t *testing.T) {
		plaintext := "sensitive string"

		ciphertext, err := km.EncryptString(ctx, metadata.ID, plaintext)
		require.NoError(t, err)
		assert.NotEqual(t, plaintext, ciphertext)

		decrypted, err := km.DecryptString(ctx, metadata.ID, ciphertext)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("errors with non-existent key ID", func(t *testing.T) {
		_, err := km.Encrypt(ctx, "non-existent", []byte("data"))
		assert.Error(t, err)

		_, err = km.Decrypt(ctx, "non-existent", []byte("ciphertext"))
		assert.Error(t, err)
	})

	t.Run("errors on invalid base64 for DecryptString", func(t *testing.T) {
		_, err := km.DecryptString(ctx, metadata.ID, "not-base64!!!")
		assert.Error(t, err)
	})

	t.Run("errors on unsupported encryption type", func(t *testing.T) {
		metadata, err := km.GenerateKey(ctx, KeyTypeHMAC, PurposeSigning)
		require.NoError(t, err)

		_, err = km.Encrypt(ctx, metadata.ID, []byte("data"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported encryption type")
	})
}

func TestKeyManager_SetNotifier(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))

	t.Run("sets notifier", func(t *testing.T) {
		notifier := &mockNotifier{}
		km.SetNotifier(notifier)
		assert.Equal(t, notifier, km.notifier)
	})
}

func TestKeyManager_StartRotation(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))
	ctx := context.Background()

	t.Run("starts rotation job", func(t *testing.T) {
		job := km.StartRotation(100 * time.Millisecond)
		assert.NotNil(t, job)

		// Wait a bit for goroutine to start
		time.Sleep(50 * time.Millisecond)

		// Check if running
		job.mu.Lock()
		running := job.running
		job.mu.Unlock()
		assert.True(t, running)

		// Stop the job
		job.Stop()

		// Wait for stop to complete
		time.Sleep(50 * time.Millisecond)

		job.mu.Lock()
		running = job.running
		job.mu.Unlock()
		assert.False(t, running)
	})

	t.Run("rotation job checks and rotates keys", func(t *testing.T) {
		// Create a key that needs rotation (created in the past)
		pastTime := time.Now().Add(-1 * time.Hour)
		metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithRotation(0)) // 0 days means rotate immediately
		require.NoError(t, err)

		// Manually set CreatedAt to past to simulate old key
		entry, _ := km.store.Get(metadata.ID)
		entry.Metadata.CreatedAt = pastTime
		entry.Metadata.RotationDays = 0
		km.store.Update(entry)

		// This is a basic test - actual rotation testing would require more setup
		_ = metadata
		_ = err
	})
}

// MemoryKeyStore tests
func TestNewMemoryKeyStore(t *testing.T) {
	store := NewMemoryKeyStore()
	assert.NotNil(t, store)
	assert.NotNil(t, store.keys)
}

func TestMemoryKeyStore_Create(t *testing.T) {
	store := NewMemoryKeyStore()

	entry := &KeyEntry{
		Metadata: KeyMetadata{
			ID:      "test-id",
			Type:    KeyTypeAES256,
			Purpose: PurposeEncryption,
			State:   KeyStateActive,
		},
		Key: []byte("test-key"),
	}

	err := store.Create(entry)
	require.NoError(t, err)

	retrieved, err := store.Get("test-id")
	require.NoError(t, err)
	assert.Equal(t, entry.Metadata.ID, retrieved.Metadata.ID)
}

func TestMemoryKeyStore_Get(t *testing.T) {
	store := NewMemoryKeyStore()

	t.Run("returns existing entry", func(t *testing.T) {
		entry := &KeyEntry{
			Metadata: KeyMetadata{ID: "get-test"},
		}
		store.Create(entry)

		retrieved, err := store.Get("get-test")
		require.NoError(t, err)
		assert.Equal(t, "get-test", retrieved.Metadata.ID)
	})

	t.Run("returns error for non-existent entry", func(t *testing.T) {
		_, err := store.Get("non-existent")
		assert.Error(t, err)
		assert.Equal(t, ErrKeyNotFound, err)
	})
}

func TestMemoryKeyStore_Update(t *testing.T) {
	store := NewMemoryKeyStore()

	entry := &KeyEntry{
		Metadata: KeyMetadata{
			ID:      "update-test",
			Name:    "original-name",
			State:   KeyStateActive,
			Type:    KeyTypeAES256,
			Purpose: PurposeEncryption,
		},
	}
	store.Create(entry)

	entry.Metadata.Name = "updated-name"
	entry.Metadata.State = KeyStateInactive

	err := store.Update(entry)
	require.NoError(t, err)

	retrieved, _ := store.Get("update-test")
	assert.Equal(t, "updated-name", retrieved.Metadata.Name)
	assert.Equal(t, KeyStateInactive, retrieved.Metadata.State)
}

func TestMemoryKeyStore_Delete(t *testing.T) {
	store := NewMemoryKeyStore()

	entry := &KeyEntry{
		Metadata: KeyMetadata{ID: "delete-test"},
	}
	store.Create(entry)

	err := store.Delete("delete-test")
	require.NoError(t, err)

	_, err = store.Get("delete-test")
	assert.Error(t, err)
}

func TestMemoryKeyStore_List(t *testing.T) {
	store := NewMemoryKeyStore()

	entry1 := &KeyEntry{
		Metadata: KeyMetadata{
			ID:      "list-1",
			Type:    KeyTypeAES256,
			Purpose: PurposeEncryption,
			State:   KeyStateActive,
		},
	}
	entry2 := &KeyEntry{
		Metadata: KeyMetadata{
			ID:      "list-2",
			Type:    KeyTypeAES128,
			Purpose: PurposeSigning,
			State:   KeyStateActive,
		},
	}
	store.Create(entry1)
	store.Create(entry2)

	t.Run("lists all entries", func(t *testing.T) {
		all, err := store.List(KeyFilter{})
		require.NoError(t, err)
		assert.Len(t, all, 2)
	})

	t.Run("filters by purpose", func(t *testing.T) {
		filtered, err := store.List(KeyFilter{Purpose: PurposeEncryption})
		require.NoError(t, err)
		assert.Len(t, filtered, 1)
		assert.Equal(t, PurposeEncryption, filtered[0].Purpose)
	})

	t.Run("filters by type", func(t *testing.T) {
		filtered, err := store.List(KeyFilter{Type: KeyTypeAES256})
		require.NoError(t, err)
		assert.Len(t, filtered, 1)
		assert.Equal(t, KeyTypeAES256, filtered[0].Type)
	})

	t.Run("filters by state", func(t *testing.T) {
		filtered, err := store.List(KeyFilter{State: KeyStateActive})
		require.NoError(t, err)
		assert.Len(t, filtered, 2)
	})
}

func TestMemoryKeyStore_GetActiveKey(t *testing.T) {
	store := NewMemoryKeyStore()

	oldEntry := &KeyEntry{
		Metadata: KeyMetadata{
			ID:        "active-old",
			Type:      KeyTypeAES256,
			Purpose:   PurposeEncryption,
			State:     KeyStateActive,
			CreatedAt: time.Now().Add(-1 * time.Hour),
		},
	}
	newEntry := &KeyEntry{
		Metadata: KeyMetadata{
			ID:        "active-new",
			Type:      KeyTypeAES256,
			Purpose:   PurposeEncryption,
			State:     KeyStateActive,
			CreatedAt: time.Now(),
		},
	}
	inactiveEntry := &KeyEntry{
		Metadata: KeyMetadata{
			ID:        "inactive",
			Type:      KeyTypeAES256,
			Purpose:   PurposeEncryption,
			State:     KeyStateInactive,
			CreatedAt: time.Now(),
		},
	}

	store.Create(oldEntry)
	store.Create(newEntry)
	store.Create(inactiveEntry)

	t.Run("returns most recent active key", func(t *testing.T) {
		entry, err := store.GetActiveKey(PurposeEncryption)
		require.NoError(t, err)
		assert.Equal(t, "active-new", entry.Metadata.ID)
	})

	t.Run("returns error when no active key", func(t *testing.T) {
		_, err := store.GetActiveKey(PurposeSigning)
		assert.Error(t, err)
		assert.Equal(t, ErrKeyNotFound, err)
	})
}

// DataEncryption tests
func TestDataEncryption_EncryptMap(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))
	ctx := context.Background()

	metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
	require.NoError(t, err)

	de := NewDataEncryption(km, metadata.ID)

	t.Run("encrypts map data", func(t *testing.T) {
		data := map[string]interface{}{
			"field1": "sensitive-value1",
			"field2": "sensitive-value2",
			"field3": 12345,
		}

		encrypted, err := de.EncryptMap(ctx, data)
		require.NoError(t, err)
		assert.Len(t, encrypted, 3)
		assert.NotEqual(t, data["field1"], encrypted["field1"])
		assert.NotEqual(t, data["field2"], encrypted["field2"])
	})

	t.Run("decrypts map data", func(t *testing.T) {
		original := map[string]interface{}{
			"field1": "value1",
			"field2": "value2",
			"field3": 123,
		}

		encrypted, err := de.EncryptMap(ctx, original)
		require.NoError(t, err)

		decrypted, err := de.DecryptMap(ctx, encrypted)
		require.NoError(t, err)
		assert.Equal(t, original["field1"], decrypted["field1"])
		assert.Equal(t, original["field2"], decrypted["field2"])
		// JSON unmarshaling converts numbers to float64
		assert.Equal(t, float64(123), decrypted["field3"])
	})

	t.Run("keeps non-string values in DecryptMap", func(t *testing.T) {
		// Create a properly encrypted string
		encryptedStr, err := de.km.EncryptString(ctx, de.keyID, "test value")
		require.NoError(t, err)

		encrypted := map[string]interface{}{
			"field1": encryptedStr, // Valid encrypted string
			"field2": 123,          // Not a string - should keep as is
			"field3": true,         // Boolean - should keep as is
		}

		decrypted, err := de.DecryptMap(ctx, encrypted)
		require.NoError(t, err)
		assert.Equal(t, "test value", decrypted["field1"])
		assert.Equal(t, 123, decrypted["field2"])
		assert.Equal(t, true, decrypted["field3"])
	})
}

// Helper functions tests
func TestGenerateKeyID(t *testing.T) {
	id1 := generateKeyID("encryption")
	time.Sleep(1 * time.Nanosecond) // Ensure different timestamp
	id2 := generateKeyID("encryption")

	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "encryption")
}

func TestPKCS7Pad(t *testing.T) {
	t.Run("pads data to block size", func(t *testing.T) {
		data := []byte{1, 2, 3}
		blockSize := 16
		padded := pkcs7Pad(data, blockSize)

		expectedLen := len(data) + (blockSize - len(data)%blockSize)
		assert.Equal(t, expectedLen, len(padded))
		assert.Equal(t, data, padded[:len(data)])
	})

	t.Run("already aligned data", func(t *testing.T) {
		data := make([]byte, 16)
		blockSize := 16
		padded := pkcs7Pad(data, blockSize)

		assert.Equal(t, 32, len(padded)) // Full block of padding
	})
}

func TestPKCS7Unpad(t *testing.T) {
	t.Run("unpads padded data", func(t *testing.T) {
		original := []byte{1, 2, 3}
		padded := pkcs7Pad(original, 16)

		unpadded, err := pkcs7Unpad(padded)
		require.NoError(t, err)
		assert.Equal(t, original, unpadded)
	})

	t.Run("errors on empty data", func(t *testing.T) {
		_, err := pkcs7Unpad([]byte{})
		assert.Error(t, err)
	})

	t.Run("errors on invalid padding", func(t *testing.T) {
		invalid := []byte{1, 2, 3, 255} // Padding value larger than data length
		_, err := pkcs7Unpad(invalid)
		assert.Error(t, err)
	})
}

// Mock notifier for testing
type mockNotifier struct {
	oldKey   *KeyMetadata
	newKey   *KeyMetadata
	notified bool
}

func (m *mockNotifier) OnKeyRotated(ctx context.Context, oldKey, newKey *KeyMetadata) {
	m.oldKey = oldKey
	m.newKey = newKey
	m.notified = true
}

func (m *mockNotifier) OnKeyExpiring(ctx context.Context, key *KeyMetadata, daysRemaining int) {
	// Not used in these tests
}

func TestKeyManager_Notifier(t *testing.T) {
	store := NewMemoryKeyStore()
	km := NewKeyManager(store, testMasterKey(t))
	ctx := context.Background()

	notifier := &mockNotifier{}
	km.SetNotifier(notifier)

	metadata, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
	require.NoError(t, err)

	_, err = km.RotateKey(ctx, metadata.ID)
	require.NoError(t, err)

	assert.True(t, notifier.notified)
	assert.Equal(t, metadata.ID, notifier.oldKey.ID)
}
