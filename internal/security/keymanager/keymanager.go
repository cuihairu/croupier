package keymanager

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// Key management errors
var (
	ErrKeyNotFound      = errors.New("key not found")
	ErrKeyExpired       = errors.New("key has expired")
	ErrKeyNotActive     = errors.New("key is not active")
	ErrInvalidKey       = errors.New("invalid key")
	ErrRotationFailed   = errors.New("key rotation failed")
	ErrEncryptionFailed = errors.New("encryption failed")
	ErrDecryptionFailed = errors.New("decryption failed")
)

// KeyType defines the type of encryption key
type KeyType string

const (
	KeyTypeAES256  KeyType = "aes-256"
	KeyTypeAES128  KeyType = "aes-128"
	KeyTypeRSA2048 KeyType = "rsa-2048"
	KeyTypeHMAC    KeyType = "hmac"
)

// KeyState defines the state of a key
type KeyState string

const (
	KeyStateActive   KeyState = "active"
	KeyStateInactive KeyState = "inactive"
	KeyStateExpired  KeyState = "expired"
	KeyStateRevoked  KeyState = "revoked"
)

// KeyPurpose defines the purpose of a key
type KeyPurpose string

const (
	PurposeEncryption   KeyPurpose = "encryption"
	PurposeSigning      KeyPurpose = "signing"
	PurposeVerification KeyPurpose = "verification"
	PurposeDerivation   KeyPurpose = "derivation"
)

// KeyMetadata contains metadata about a key
type KeyMetadata struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         KeyType           `json:"type"`
	Purpose      KeyPurpose        `json:"purpose"`
	State        KeyState          `json:"state"`
	Version      int               `json:"version"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
	RotatedAt    *time.Time        `json:"rotated_at,omitempty"`
	RotatedFrom  string            `json:"rotated_from,omitempty"`
	RotationDays int               `json:"rotation_days,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	CreatedBy    string            `json:"created_by"`
	Description  string            `json:"description,omitempty"`
}

// KeyEntry represents a stored key with its metadata
type KeyEntry struct {
	Metadata KeyMetadata `json:"metadata"`
	Key      []byte      `json:"key"` // Encrypted key material
	IV       []byte      `json:"iv,omitempty"`
	Nonce    []byte      `json:"nonce,omitempty"`
}

// KeyStore interface for key persistence
type KeyStore interface {
	Create(entry *KeyEntry) error
	Get(id string) (*KeyEntry, error)
	Update(entry *KeyEntry) error
	Delete(id string) error
	List(filter KeyFilter) ([]*KeyMetadata, error)
	GetActiveKey(purpose KeyPurpose) (*KeyEntry, error)
}

// KeyFilter for filtering keys
type KeyFilter struct {
	Purpose KeyPurpose
	State   KeyState
	Type    KeyType
}

// KeyManager manages encryption keys
type KeyManager struct {
	store       KeyStore
	masterKey   []byte
	cache       map[string]*KeyEntry
	cacheMu     sync.RWMutex
	rotationJob *RotationJob
	notifier    KeyRotationNotifier
}

// NewKeyManager creates a new key manager
func NewKeyManager(store KeyStore, masterKey []byte) *KeyManager {
	if len(masterKey) < 32 {
		panic("master key must be at least 32 bytes")
	}

	return &KeyManager{
		store:     store,
		masterKey: masterKey,
		cache:     make(map[string]*KeyEntry),
	}
}

// SetNotifier sets the key rotation notifier
func (km *KeyManager) SetNotifier(notifier KeyRotationNotifier) {
	km.notifier = notifier
}

// GenerateKey generates a new key
func (km *KeyManager) GenerateKey(ctx context.Context, keyType KeyType, purpose KeyPurpose, opts ...KeyOption) (*KeyMetadata, error) {
	// Generate key material
	var keyMaterial []byte
	var err error

	switch keyType {
	case KeyTypeAES256:
		keyMaterial = make([]byte, 32)
		_, err = io.ReadFull(rand.Reader, keyMaterial)
	case KeyTypeAES128:
		keyMaterial = make([]byte, 16)
		_, err = io.ReadFull(rand.Reader, keyMaterial)
	case KeyTypeHMAC:
		keyMaterial = make([]byte, 64)
		_, err = io.ReadFull(rand.Reader, keyMaterial)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Create key entry
	now := time.Now()
	entry := &KeyEntry{
		Metadata: KeyMetadata{
			ID:        generateKeyID(string(purpose)),
			Type:      keyType,
			Purpose:   purpose,
			State:     KeyStateActive,
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Key: keyMaterial,
	}

	// Apply options
	for _, opt := range opts {
		opt(&entry.Metadata)
	}

	// Encrypt the key material before storing
	encryptedKey, iv, err := km.encryptKeyMaterial(keyMaterial)
	if err != nil {
		return nil, err
	}
	entry.Key = encryptedKey
	entry.IV = iv

	// Store the key
	if err := km.store.Create(entry); err != nil {
		return nil, fmt.Errorf("failed to store key: %w", err)
	}

	// Cache the key
	km.cacheMu.Lock()
	km.cache[entry.Metadata.ID] = entry
	km.cacheMu.Unlock()

	return &entry.Metadata, nil
}

// KeyOption is a function that modifies key metadata
type KeyOption func(*KeyMetadata)

// WithName sets the key name
func WithName(name string) KeyOption {
	return func(m *KeyMetadata) {
		m.Name = name
	}
}

// WithExpiration sets the key expiration
func WithExpiration(duration time.Duration) KeyOption {
	return func(m *KeyMetadata) {
		exp := m.CreatedAt.Add(duration)
		m.ExpiresAt = &exp
	}
}

// WithRotation sets the key rotation period
func WithRotation(days int) KeyOption {
	return func(m *KeyMetadata) {
		m.RotationDays = days
	}
}

// WithLabels sets the key labels
func WithLabels(labels map[string]string) KeyOption {
	return func(m *KeyMetadata) {
		m.Labels = labels
	}
}

// WithCreatedBy sets who created the key
func WithCreatedBy(createdBy string) KeyOption {
	return func(m *KeyMetadata) {
		m.CreatedBy = createdBy
	}
}

// WithDescription sets the key description
func WithDescription(description string) KeyOption {
	return func(m *KeyMetadata) {
		m.Description = description
	}
}

// GetKey retrieves a key by ID
func (km *KeyManager) GetKey(ctx context.Context, id string) (*KeyEntry, error) {
	// Check cache first
	km.cacheMu.RLock()
	if entry, exists := km.cache[id]; exists {
		km.cacheMu.RUnlock()
		return entry, nil
	}
	km.cacheMu.RUnlock()

	// Load from store
	entry, err := km.store.Get(id)
	if err != nil {
		return nil, ErrKeyNotFound
	}

	// Check if key is expired
	if entry.Metadata.ExpiresAt != nil && time.Now().After(*entry.Metadata.ExpiresAt) {
		return nil, ErrKeyExpired
	}

	// Check if key is active
	if entry.Metadata.State != KeyStateActive {
		return nil, ErrKeyNotActive
	}

	// Decrypt key material
	decryptedKey, err := km.decryptKeyMaterial(entry.Key, entry.IV)
	if err != nil {
		return nil, err
	}

	// Create a copy with decrypted key
	decryptedEntry := &KeyEntry{
		Metadata: entry.Metadata,
		Key:      decryptedKey,
		IV:       entry.IV,
	}

	// Cache the entry
	km.cacheMu.Lock()
	km.cache[id] = decryptedEntry
	km.cacheMu.Unlock()

	return decryptedEntry, nil
}

// GetActiveKey gets the current active key for a purpose
func (km *KeyManager) GetActiveKey(ctx context.Context, purpose KeyPurpose) (*KeyEntry, error) {
	km.cacheMu.RLock()
	for _, entry := range km.cache {
		if entry.Metadata.Purpose == purpose && entry.Metadata.State == KeyStateActive {
			km.cacheMu.RUnlock()
			return entry, nil
		}
	}
	km.cacheMu.RUnlock()

	// Load from store
	entry, err := km.store.GetActiveKey(purpose)
	if err != nil {
		return nil, err
	}

	// Decrypt key material
	decryptedKey, err := km.decryptKeyMaterial(entry.Key, entry.IV)
	if err != nil {
		return nil, err
	}

	decryptedEntry := &KeyEntry{
		Metadata: entry.Metadata,
		Key:      decryptedKey,
		IV:       entry.IV,
	}

	// Cache
	km.cacheMu.Lock()
	km.cache[entry.Metadata.ID] = decryptedEntry
	km.cacheMu.Unlock()

	return decryptedEntry, nil
}

// RotateKey rotates a key
func (km *KeyManager) RotateKey(ctx context.Context, id string) (*KeyMetadata, error) {
	// Get the current key
	currentKey, err := km.GetKey(ctx, id)
	if err != nil {
		return nil, err
	}

	// Generate new key with same parameters
	newMetadata, err := km.GenerateKey(ctx,
		currentKey.Metadata.Type,
		currentKey.Metadata.Purpose,
		WithName(currentKey.Metadata.Name),
		WithRotation(currentKey.Metadata.RotationDays),
		WithLabels(currentKey.Metadata.Labels),
	)
	if err != nil {
		return nil, err
	}

	// Mark old key as inactive
	now := time.Now()
	currentKey.Metadata.State = KeyStateInactive
	currentKey.Metadata.UpdatedAt = now

	// Update the old key
	if err := km.store.Update(currentKey); err != nil {
		return nil, fmt.Errorf("failed to deactivate old key: %w", err)
	}

	// Update new key metadata to track rotation
	newKey, _ := km.store.Get(newMetadata.ID)
	newKey.Metadata.RotatedFrom = id
	newKey.Metadata.RotatedAt = &now
	if err := km.store.Update(newKey); err != nil {
		return nil, fmt.Errorf("failed to update new key: %w", err)
	}

	// Invalidate cache
	km.cacheMu.Lock()
	delete(km.cache, id)
	km.cacheMu.Unlock()

	// Notify
	if km.notifier != nil {
		km.notifier.OnKeyRotated(ctx, &currentKey.Metadata, newMetadata)
	}

	return newMetadata, nil
}

// RevokeKey revokes a key
func (km *KeyManager) RevokeKey(ctx context.Context, id string, reason string) error {
	entry, err := km.store.Get(id)
	if err != nil {
		return err
	}

	now := time.Now()
	entry.Metadata.State = KeyStateRevoked
	entry.Metadata.UpdatedAt = now
	entry.Metadata.Description = fmt.Sprintf("Revoked: %s", reason)

	if err := km.store.Update(entry); err != nil {
		return err
	}

	// Invalidate cache
	km.cacheMu.Lock()
	delete(km.cache, id)
	km.cacheMu.Unlock()

	return nil
}

// ListKeys lists keys with optional filtering
func (km *KeyManager) ListKeys(ctx context.Context, filter KeyFilter) ([]*KeyMetadata, error) {
	return km.store.List(filter)
}

// Encrypt encrypts data using a key
func (km *KeyManager) Encrypt(ctx context.Context, keyID string, plaintext []byte) ([]byte, error) {
	entry, err := km.GetKey(ctx, keyID)
	if err != nil {
		return nil, err
	}

	switch entry.Metadata.Type {
	case KeyTypeAES256, KeyTypeAES128:
		return km.encryptAES(entry.Key, plaintext)
	default:
		return nil, fmt.Errorf("unsupported encryption type: %s", entry.Metadata.Type)
	}
}

// Decrypt decrypts data using a key
func (km *KeyManager) Decrypt(ctx context.Context, keyID string, ciphertext []byte) ([]byte, error) {
	entry, err := km.GetKey(ctx, keyID)
	if err != nil {
		return nil, err
	}

	switch entry.Metadata.Type {
	case KeyTypeAES256, KeyTypeAES128:
		return km.decryptAES(entry.Key, ciphertext)
	default:
		return nil, fmt.Errorf("unsupported encryption type: %s", entry.Metadata.Type)
	}
}

// EncryptString encrypts a string and returns base64 encoded ciphertext
func (km *KeyManager) EncryptString(ctx context.Context, keyID string, plaintext string) (string, error) {
	ciphertext, err := km.Encrypt(ctx, keyID, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts a base64 encoded string
func (km *KeyManager) DecryptString(ctx context.Context, keyID string, ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	plaintext, err := km.Decrypt(ctx, keyID, data)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Internal encryption methods

func (km *KeyManager) encryptKeyMaterial(key []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(km.masterKey[:32])
	if err != nil {
		return nil, nil, err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, nil, err
	}

	ciphertext := make([]byte, len(key))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext, key)

	return ciphertext, iv, nil
}

func (km *KeyManager) decryptKeyMaterial(ciphertext, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(km.masterKey[:32])
	if err != nil {
		return nil, err
	}

	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

func (km *KeyManager) encryptAES(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Generate IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// Pad plaintext
	paddedPlaintext := pkcs7Pad(plaintext, aes.BlockSize)

	ciphertext := make([]byte, len(paddedPlaintext))
	stream := cipher.NewCBCEncrypter(block, iv)
	stream.CryptBlocks(ciphertext, paddedPlaintext)

	// Prepend IV
	result := make([]byte, len(iv)+len(ciphertext))
	copy(result, iv)
	copy(result[len(iv):], ciphertext)

	return result, nil
}

func (km *KeyManager) decryptAES(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, ErrDecryptionFailed
	}

	// Extract IV
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCBCDecrypter(block, iv)
	stream.CryptBlocks(plaintext, ciphertext)

	// Remove padding
	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// RotationJob handles automatic key rotation
type RotationJob struct {
	km       *KeyManager
	interval time.Duration
	stopChan chan struct{}
	running  bool
	mu       sync.Mutex
}

// StartRotation starts the automatic key rotation job
func (km *KeyManager) StartRotation(interval time.Duration) *RotationJob {
	job := &RotationJob{
		km:       km,
		interval: interval,
		stopChan: make(chan struct{}),
	}

	go job.run()
	return job
}

func (j *RotationJob) run() {
	j.mu.Lock()
	j.running = true
	j.mu.Unlock()

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.checkAndRotate()
		case <-j.stopChan:
			return
		}
	}
}

func (j *RotationJob) checkAndRotate() {
	ctx := context.Background()

	// Get all active keys
	keys, err := j.km.ListKeys(ctx, KeyFilter{State: KeyStateActive})
	if err != nil {
		return
	}

	now := time.Now()
	for _, meta := range keys {
		// Check if rotation is needed
		if meta.RotationDays > 0 {
			rotateAt := meta.CreatedAt.AddDate(0, 0, meta.RotationDays)
			if now.After(rotateAt) {
				_, err := j.km.RotateKey(ctx, meta.ID)
				if err != nil {
					// Log error
					continue
				}
			}
		}
	}
}

// Stop stops the rotation job
func (j *RotationJob) Stop() {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.running {
		j.stopChan <- struct{}{}
		j.running = false
	}
}

// KeyRotationNotifier interface for key rotation notifications
type KeyRotationNotifier interface {
	OnKeyRotated(ctx context.Context, oldKey, newKey *KeyMetadata)
	OnKeyExpiring(ctx context.Context, key *KeyMetadata, daysRemaining int)
}

// Helper functions

func generateKeyID(purpose string) string {
	// 时间戳提供可读的时间顺序，但单独依赖 UnixNano 在低精度时钟（如 Windows）
	// 上会在紧密循环中产生重复值；追加随机后缀确保唯一性。
	b := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return fmt.Sprintf("key_%s_%d", purpose, time.Now().UnixNano())
	}
	return fmt.Sprintf("key_%s_%d_%x", purpose, time.Now().UnixNano(), b)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return padded
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	padding := int(data[len(data)-1])
	if padding > len(data) {
		return nil, errors.New("invalid padding")
	}
	return data[:len(data)-padding], nil
}

// MemoryKeyStore is an in-memory key store for testing
type MemoryKeyStore struct {
	keys map[string]*KeyEntry
	mu   sync.RWMutex
}

// NewMemoryKeyStore creates a new memory key store
func NewMemoryKeyStore() *MemoryKeyStore {
	return &MemoryKeyStore{
		keys: make(map[string]*KeyEntry),
	}
}

func (s *MemoryKeyStore) Create(entry *KeyEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[entry.Metadata.ID] = entry
	return nil
}

func (s *MemoryKeyStore) Get(id string) (*KeyEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, exists := s.keys[id]
	if !exists {
		return nil, ErrKeyNotFound
	}
	return entry, nil
}

func (s *MemoryKeyStore) Update(entry *KeyEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.keys[entry.Metadata.ID]; !exists {
		return ErrKeyNotFound
	}
	s.keys[entry.Metadata.ID] = entry
	return nil
}

func (s *MemoryKeyStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, id)
	return nil
}

func (s *MemoryKeyStore) List(filter KeyFilter) ([]*KeyMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*KeyMetadata
	for _, entry := range s.keys {
		if filter.Purpose != "" && entry.Metadata.Purpose != filter.Purpose {
			continue
		}
		if filter.State != "" && entry.Metadata.State != filter.State {
			continue
		}
		if filter.Type != "" && entry.Metadata.Type != filter.Type {
			continue
		}
		result = append(result, &entry.Metadata)
	}
	return result, nil
}

func (s *MemoryKeyStore) GetActiveKey(purpose KeyPurpose) (*KeyEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []*KeyEntry
	for _, entry := range s.keys {
		if entry.Metadata.Purpose == purpose && entry.Metadata.State == KeyStateActive {
			candidates = append(candidates, entry)
		}
	}

	if len(candidates) == 0 {
		return nil, ErrKeyNotFound
	}

	// Return the most recent active key
	var latest *KeyEntry
	for _, entry := range candidates {
		if latest == nil || entry.Metadata.CreatedAt.After(latest.Metadata.CreatedAt) {
			latest = entry
		}
	}

	return latest, nil
}

// DataEncryption provides high-level data encryption utilities
type DataEncryption struct {
	km    *KeyManager
	keyID string
}

// NewDataEncryption creates a new data encryption helper
func NewDataEncryption(km *KeyManager, keyID string) *DataEncryption {
	return &DataEncryption{
		km:    km,
		keyID: keyID,
	}
}

// EncryptMap encrypts a map of sensitive data
func (de *DataEncryption) EncryptMap(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for k, v := range data {
		jsonData, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		encrypted, err := de.km.EncryptString(ctx, de.keyID, string(jsonData))
		if err != nil {
			return nil, err
		}
		result[k] = encrypted
	}
	return result, nil
}

// DecryptMap decrypts a map of encrypted data
func (de *DataEncryption) DecryptMap(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for k, v := range data {
		encrypted, ok := v.(string)
		if !ok {
			result[k] = v
			continue
		}
		decrypted, err := de.km.DecryptString(ctx, de.keyID, encrypted)
		if err != nil {
			return nil, err
		}
		var value interface{}
		if err := json.Unmarshal([]byte(decrypted), &value); err != nil {
			result[k] = decrypted
		} else {
			result[k] = value
		}
	}
	return result, nil
}
