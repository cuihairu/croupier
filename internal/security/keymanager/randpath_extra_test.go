package keymanager

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingReader 恒定失败，用于注入 crypto/rand.Reader 故障。
type failingReader struct{ err error }

func (r *failingReader) Read(p []byte) (int, error) { return 0, r.err }

// failAfterReader 前 n 次 Read 成功（返回全量确定性数据），之后恒失败。
type failAfterReader struct {
	remaining int
}

func (r *failAfterReader) Read(p []byte) (int, error) {
	if r.remaining > 0 {
		r.remaining--
		for i := range p {
			p[i] = byte(i)
		}
		return len(p), nil
	}
	return 0, errors.New("entropy exhausted")
}

// swapRandReader 临时替换 crypto/rand.Reader 并返回恢复函数。
func swapRandReader(r io.Reader) func() {
	old := rand.Reader
	rand.Reader = r
	return func() { rand.Reader = old }
}

// TestExtra_GenerateKey_RandReadFails 覆盖 GenerateKey 中
// keyMaterial 读取失败的分支（keymanager.go:151）。
func TestExtra_GenerateKey_RandReadFails(t *testing.T) {
	restore := swapRandReader(&failingReader{err: errors.New("no entropy")})
	defer restore()

	store := NewMemoryKeyStore()
	km, _ := newExtraManager(store)

	for _, keyType := range []KeyType{KeyTypeAES256, KeyTypeAES128, KeyTypeHMAC} {
		meta, err := km.GenerateKey(context.Background(), keyType, PurposeEncryption)
		require.Error(t, err)
		assert.Nil(t, meta)
		assert.Contains(t, err.Error(), "failed to generate key")
	}
}

// TestExtra_GenerateKey_IVReadFails 用"成功一次后失败"的 reader 覆盖：
//   - generateKeyID 随机后缀读取失败时回退到纯时间戳 ID（keymanager.go:632）
//   - encryptKeyMaterial 的 IV 读取失败（keymanager.go:467）
//   - GenerateKey 向上传播该错误（keymanager.go:177）
func TestExtra_GenerateKey_IVReadFails(t *testing.T) {
	// 第 1 次 Read 供 keyMaterial 使用，第 2 次（keyID）与第 3 次（IV）失败。
	restore := swapRandReader(&failAfterReader{remaining: 1})
	defer restore()

	store := NewMemoryKeyStore()
	km, _ := newExtraManager(store)

	meta, err := km.GenerateKey(context.Background(), KeyTypeAES256, PurposeEncryption)
	require.Error(t, err)
	assert.Nil(t, meta)

	// 失败发生在落库之前，store 不应有任何 key。
	keys, err := store.List(KeyFilter{})
	require.NoError(t, err)
	assert.Empty(t, keys)
}

// TestExtra_Encrypt_IVReadFails 覆盖 encryptAES 中 IV 读取失败（keymanager.go:499）：
// 密钥已生成并进入缓存，之后 rand 故障使 Encrypt 失败。
func TestExtra_Encrypt_IVReadFails(t *testing.T) {
	store := NewMemoryKeyStore()
	km, _ := newExtraManager(store)
	ctx := context.Background()

	meta, err := km.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption)
	require.NoError(t, err)

	restore := swapRandReader(&failingReader{err: errors.New("no entropy")})
	defer restore()

	out, err := km.Encrypt(ctx, meta.ID, []byte("payload"))
	require.Error(t, err)
	assert.Nil(t, out)
}

// TestExtra_Encrypt_InvalidStoredKeySize 覆盖 encryptAES 中
// aes.NewCipher(key) 失败分支（keymanager.go:493）：
// 落库物料为 20 字节（非合法 AES key 长度），元数据却声明 aes-256。
func TestExtra_Encrypt_InvalidStoredKeySize(t *testing.T) {
	store := NewMemoryKeyStore()
	seeder, master := newExtraManager(store)
	ctx := context.Background()

	bad := seedEncryptedEntry(t, seeder, store, KeyMetadata{
		ID: "bad-size-key", Type: KeyTypeAES256, Purpose: PurposeEncryption,
		State: KeyStateActive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, make([]byte, 20))

	reader := NewKeyManager(store, master)
	out, err := reader.Encrypt(ctx, bad.Metadata.ID, []byte("payload"))
	require.Error(t, err)
	assert.Nil(t, out)
}

// TestExtra_CheckAndRotate_RotateErrorContinues 覆盖 checkAndRotate 中
// RotateKey 失败后 continue 的分支（keymanager.go:600）。
func TestExtra_CheckAndRotate_RotateErrorContinues(t *testing.T) {
	store := NewMemoryKeyStore()
	seeder, master := newExtraManager(store)
	ctx := context.Background()

	// 构造一把已到期的轮换 key。
	due, err := seeder.GenerateKey(ctx, KeyTypeAES256, PurposeEncryption, WithRotation(1))
	require.NoError(t, err)
	entry, err := store.Get(due.ID)
	require.NoError(t, err)
	old := time.Now().AddDate(0, 0, -30)
	entry.Metadata.CreatedAt = old
	entry.Metadata.UpdatedAt = old
	require.NoError(t, store.Update(entry))

	// Update 恒失败 => RotateKey 在停用旧 key 时报错。
	st := &stubStore{KeyStore: store, updateErr: errors.New("locked")}
	km := NewKeyManager(st, master)

	job := &RotationJob{km: km, stopChan: make(chan struct{})}
	job.checkAndRotate() // 不得 panic

	// 轮换失败后旧 key 应保持 active。
	keys, err := store.List(KeyFilter{State: KeyStateActive})
	require.NoError(t, err)
	found := false
	for _, k := range keys {
		if k.ID == due.ID {
			found = true
		}
	}
	assert.True(t, found, "due key must stay active when rotation fails")
}
