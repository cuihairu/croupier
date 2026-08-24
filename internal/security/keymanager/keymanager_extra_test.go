package keymanager

import (
	"context"
	"encoding/base64"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubStore wraps a real store and lets tests inject failures per method.
type stubStore struct {
	KeyStore

	createErr error
	updateErr error
	getErr    error
	listErr   error
	updateFn  func(*KeyEntry) error
}

func (s *stubStore) Create(entry *KeyEntry) error {
	if s.createErr != nil {
		return s.createErr
	}
	return s.KeyStore.Create(entry)
}

func (s *stubStore) Get(id string) (*KeyEntry, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.KeyStore.Get(id)
}

func (s *stubStore) Update(entry *KeyEntry) error {
	if s.updateFn != nil {
		return s.updateFn(entry)
	}
	if s.updateErr != nil {
		return s.updateErr
	}
	return s.KeyStore.Update(entry)
}

func (s *stubStore) List(filter KeyFilter) ([]*KeyMetadata, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.KeyStore.List(filter)
}

// seedEncryptedEntry stores an entry whose key material is encrypted with the
// given master key so GetKey on a fresh (cache-empty) manager can decrypt it.
func seedEncryptedEntry(t *testing.T, km *KeyManager, store KeyStore, meta KeyMetadata, material []byte) *KeyEntry {
	t.Helper()
	enc, iv, err := km.encryptKeyMaterial(material)
	require.NoError(t, err)
	entry := &KeyEntry{Metadata: meta, Key: enc, IV: iv}
	require.NoError(t, store.Create(entry))
	return entry
}

func newExtraManager(store KeyStore) (*KeyManager, []byte) {
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i + 1)
	}
	return NewKeyManager(store, master), master
}

func TestExtra_GenerateKey_StoreCreateError(t *testing.T) {
	store := &stubStore{KeyStore: NewMemoryKeyStore(), createErr: errors.New("disk full")}
	km, _ := newExtraManager(store)

	meta, err := km.GenerateKey(context.Background(), KeyTypeAES256, PurposeEncryption)
	assert.Error(t, err)
	assert.Nil(t, meta)
	assert.Contains(t, err.Error(), "failed to store key")
}

func TestExtra_GetKey_LoadsFromStoreWhenCacheEmpty(t *testing.T) {
	store := NewMemoryKeyStore()
	seeder, master := newExtraManager(store)
	ctx := context.Background()

	meta, err := seeder.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithName("stored"))
	require.NoError(t, err)

	// A second manager with the same master key has an empty cache.
	reader := NewKeyManager(store, master)
	entry, err := reader.GetKey(ctx, meta.ID)
	require.NoError(t, err)
	assert.Equal(t, meta.ID, entry.Metadata.ID)
	assert.Equal(t, "stored", entry.Metadata.Name)
	// The returned key material must be decrypted plaintext.
	assert.Len(t, entry.Key, 32)

	// Second call hits the cache.
	cached, err := reader.GetKey(ctx, meta.ID)
	require.NoError(t, err)
	assert.Equal(t, entry.Key, cached.Key)
}

func TestExtra_GetKey_Errors(t *testing.T) {
	store := NewMemoryKeyStore()
	seeder, master := newExtraManager(store)
	ctx := context.Background()
	reader := NewKeyManager(store, master)

	t.Run("not found", func(t *testing.T) {
		_, err := reader.GetKey(ctx, "missing-id")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	expiredAt := time.Now().Add(-time.Hour)
	expired := seedEncryptedEntry(t, seeder, store, KeyMetadata{
		ID: "expired-key", Type: KeyTypeAES256, Purpose: PurposeEncryption,
		State: KeyStateActive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		ExpiresAt: &expiredAt,
	}, make([]byte, 32))

	t.Run("expired", func(t *testing.T) {
		_, err := reader.GetKey(ctx, expired.Metadata.ID)
		assert.ErrorIs(t, err, ErrKeyExpired)
	})

	inactive := seedEncryptedEntry(t, seeder, store, KeyMetadata{
		ID: "inactive-key", Type: KeyTypeAES256, Purpose: PurposeEncryption,
		State: KeyStateInactive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, make([]byte, 32))

	t.Run("inactive", func(t *testing.T) {
		_, err := reader.GetKey(ctx, inactive.Metadata.ID)
		assert.ErrorIs(t, err, ErrKeyNotActive)
	})
}

func TestExtra_RotateKey_UnsupportedKeyType(t *testing.T) {
	store := NewMemoryKeyStore()
	seeder, master := newExtraManager(store)
	ctx := context.Background()

	seedEncryptedEntry(t, seeder, store, KeyMetadata{
		ID: "odd-key", Type: KeyTypeRSA2048, Purpose: PurposeEncryption,
		State: KeyStateActive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, make([]byte, 32))

	reader := NewKeyManager(store, master)
	_, err := reader.RotateKey(ctx, "odd-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported key type")
}

func TestExtra_RotateKey_UpdateErrors(t *testing.T) {
	ctx := context.Background()

	newErr := errors.New("update failed")

	setup := func(t *testing.T) (*stubStore, *KeyManager, string) {
		store := NewMemoryKeyStore()
		seeder, master := newExtraManager(store)
		meta, err := seeder.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithRotation(7))
		require.NoError(t, err)
		st := &stubStore{KeyStore: store}
		reader := NewKeyManager(st, master)
		return st, reader, meta.ID
	}

	t.Run("deactivate old key fails", func(t *testing.T) {
		st, reader, id := setup(t)
		st.updateErr = newErr
		_, err := reader.RotateKey(ctx, id)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to deactivate old key")
	})

	t.Run("update new key fails", func(t *testing.T) {
		st, reader, id := setup(t)
		// First Update deactivates the old key; second persists rotation info.
		count := 0
		st.updateFn = func(entry *KeyEntry) error {
			count++
			if count == 1 {
				return st.KeyStore.Update(entry)
			}
			return errors.New("update new key failed")
		}
		_, err := reader.RotateKey(ctx, id)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update new key")
	})
}

func TestExtra_RevokeKey_UpdateError(t *testing.T) {
	store := NewMemoryKeyStore()
	seeder, master := newExtraManager(store)
	ctx := context.Background()
	meta, err := seeder.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
	require.NoError(t, err)

	st := &stubStore{KeyStore: store, updateErr: errors.New("locked")}
	lockedManager := NewKeyManager(st, master)
	err = lockedManager.RevokeKey(ctx, meta.ID, "compromised")
	assert.Error(t, err)
}

func TestExtra_EncryptDecrypt_UnsupportedAndBadInput(t *testing.T) {
	store := NewMemoryKeyStore()
	seeder, master := newExtraManager(store)
	ctx := context.Background()

	// HMAC-typed active key with valid encrypted material.
	hmacMeta, err := seeder.GenerateKey(ctx, KeyTypeHMAC, PurposeSigning)
	require.NoError(t, err)

	reader := NewKeyManager(store, master)

	t.Run("encrypt unsupported type", func(t *testing.T) {
		_, err := reader.Encrypt(ctx, hmacMeta.ID, []byte("data"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported encryption type")
	})

	t.Run("decrypt unsupported type", func(t *testing.T) {
		_, err := reader.Decrypt(ctx, hmacMeta.ID, []byte("data"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported encryption type")
	})

	aesMeta, err := seeder.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
	require.NoError(t, err)

	t.Run("encrypt unknown key", func(t *testing.T) {
		_, err := reader.EncryptString(ctx, "no-such-key", "secret")
		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	t.Run("decrypt invalid base64", func(t *testing.T) {
		_, err := reader.DecryptString(ctx, aesMeta.ID, "!!!not-base64!!!")
		assert.Error(t, err)
	})

	t.Run("decrypt short ciphertext", func(t *testing.T) {
		_, err := reader.DecryptString(ctx, aesMeta.ID, "AAAA")
		assert.ErrorIs(t, err, ErrDecryptionFailed)
	})

	t.Run("decrypt corrupt padding", func(t *testing.T) {
		plaintext, err := reader.EncryptString(ctx, aesMeta.ID, "hello world")
		require.NoError(t, err)
		data, err := base64.StdEncoding.DecodeString(plaintext)
		require.NoError(t, err)
		data[len(data)-1] = 0xFF // corrupt padding byte
		_, err = reader.Decrypt(ctx, aesMeta.ID, data)
		assert.Error(t, err)
	})

	t.Run("decrypt with malformed stored material", func(t *testing.T) {
		badMaterial := seedEncryptedEntry(t, seeder, store, KeyMetadata{
			ID: "bad-size", Type: KeyTypeAES256, Purpose: PurposeEncryption,
			State: KeyStateActive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}, make([]byte, 15)) // not a valid AES key size
		_, err := reader.Decrypt(ctx, badMaterial.Metadata.ID, []byte("0123456789abcdef"))
		assert.Error(t, err)
	})
}

func TestExtra_MemoryKeyStore_UpdateMissing_ReturnsNotFound(t *testing.T) {
	store := NewMemoryKeyStore()
	err := store.Update(&KeyEntry{Metadata: KeyMetadata{ID: "ghost"}})
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestExtra_MemoryKeyStore_ListFilterByType(t *testing.T) {
	store := NewMemoryKeyStore()
	km, _ := newExtraManager(store)
	ctx := context.Background()
	_, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
	require.NoError(t, err)
	_, err = km.GenerateKey(ctx, KeyTypeHMAC, PurposeSigning)
	require.NoError(t, err)

	keys, err := store.List(KeyFilter{Type: KeyTypeHMAC})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, KeyTypeHMAC, keys[0].Type)
}

func TestExtra_DataEncryption_ErrorPaths(t *testing.T) {
	store := NewMemoryKeyStore()
	km, _ := newExtraManager(store)
	ctx := context.Background()
	meta, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
	require.NoError(t, err)
	de := NewDataEncryption(km, meta.ID)

	t.Run("EncryptMap marshal error", func(t *testing.T) {
		out, err := de.EncryptMap(ctx, map[string]interface{}{
			"bad": math.Inf(1),
		})
		assert.Error(t, err)
		assert.Nil(t, out)
	})

	t.Run("EncryptMap encryption error", func(t *testing.T) {
		bad := NewDataEncryption(km, "missing-key")
		out, err := bad.EncryptMap(ctx, map[string]interface{}{"k": "v"})
		assert.Error(t, err)
		assert.Nil(t, out)
	})

	t.Run("DecryptMap non-string passthrough", func(t *testing.T) {
		out, err := de.DecryptMap(ctx, map[string]interface{}{"n": float64(42)})
		require.NoError(t, err)
		assert.Equal(t, float64(42), out["n"])
	})

	t.Run("DecryptMap decrypt error", func(t *testing.T) {
		out, err := de.DecryptMap(ctx, map[string]interface{}{"k": "%%%invalid"})
		assert.Error(t, err)
		assert.Nil(t, out)
	})
}

type recordingNotifier struct {
	rotated int
}

func (n *recordingNotifier) OnKeyRotated(context.Context, *KeyMetadata, *KeyMetadata) {
	n.rotated++
}

func (n *recordingNotifier) OnKeyExpiring(context.Context, *KeyMetadata, int) {}

func TestExtra_CheckAndRotate_RotatesDueKeys_AndTickerRun(t *testing.T) {
	store := NewMemoryKeyStore()
	km, _ := newExtraManager(store)
	ctx := context.Background()

	// Due for rotation: created well in the past with RotationDays=1.
	old := time.Now().AddDate(0, 0, -30)
	due, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithRotation(1))
	require.NoError(t, err)
	entry, err := store.Get(due.ID)
	require.NoError(t, err)
	entry.Metadata.CreatedAt = old
	entry.Metadata.UpdatedAt = old
	require.NoError(t, store.Update(entry))

	// Not due: no rotation policy.
	_, err = km.GenerateKey(ctx, KeyTypeAES128, PurposeEncryption)
	require.NoError(t, err)

	notifier := &recordingNotifier{}
	km.SetNotifier(notifier)

	job := &RotationJob{km: km, stopChan: make(chan struct{})}
	job.checkAndRotate()

	keys, err := store.List(KeyFilter{State: KeyStateActive})
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Equal(t, 1, notifier.rotated)

	inactive, err := store.List(KeyFilter{State: KeyStateInactive})
	require.NoError(t, err)
	require.Len(t, inactive, 1)
	assert.Equal(t, due.ID, inactive[0].ID)
}

func TestExtra_CheckAndRotate_ListError(t *testing.T) {
	st := &stubStore{KeyStore: NewMemoryKeyStore(), listErr: errors.New("list failed")}
	km, _ := newExtraManager(st)

	job := &RotationJob{km: km, stopChan: make(chan struct{})}
	job.checkAndRotate() // must not panic
}

func TestExtra_StartRotation_TickerFiresAndStops(t *testing.T) {
	store := NewMemoryKeyStore()
	km, _ := newExtraManager(store)
	ctx := context.Background()

	old := time.Now().AddDate(0, 0, -90)
	due, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithRotation(1))
	require.NoError(t, err)
	entry, err := store.Get(due.ID)
	require.NoError(t, err)
	entry.Metadata.CreatedAt = old
	require.NoError(t, store.Update(entry))

	job := km.StartRotation(5 * time.Millisecond)
	time.Sleep(40 * time.Millisecond)
	job.Stop()

	keys, err := store.List(KeyFilter{State: KeyStateActive})
	require.NoError(t, err)
	var found bool
	for _, k := range keys {
		if k.RotatedFrom == due.ID {
			found = true
		}
	}
	assert.True(t, found, "expected due key to be rotated by ticker job")
}
