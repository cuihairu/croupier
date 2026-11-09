package vault

// 密码箱补测：构造缺省值（路径/口令环境变量/TTL/时钟）、Build 走注册表闭包
// 与 local.Name、Unlock 的 KDF 失败臂（N=0 存储）、loadOrCreate 的损坏
// JSON/不支持格式/目录占位/父目录不可建、Read 的解密失败与非法路径、persist
// 的 rename 失败臂，以及 env 源的缺省前缀/Lock/非法路径。
//
// 诚实跳过：
//   - rand.Read 失败臂（local.go 129-131/136-138/308-309/323-324 与
//     nonce 生成）：crypto/rand 熵源不可注入，失败不可复现。
//   - newAEAD 的 NewGCM 失败臂（363-364）：aes 块大小恒 16，GCM 构造恒成功。
//   - Unlock/Read/Write/Delete 内 newAEAD 错误臂（125-126/207-208/233-234/
//     280-281）：会话密钥恒为 scryptKeyLen=32 字节，aes.NewCipher 恒成功。
//   - persist 的 json.Marshal 错误臂（329-330）：localFile 字段全可序列化。
//   - persist 的 bufio Writer.Write / Flush / Close 错误臂（338-340/342-344/
//     346-347）：tmp 为普通文件且载荷小于缓冲，写错误只在底层 I/O 失败时
//     出现，常规文件语义下不可复现（OpenFile 失败已由 persist_error_test 覆盖）。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Build 走注册表闭包：local/env 两键实际构造（registry.go 44-45）。
func TestBuild_LocalAndEnvClosures(t *testing.T) {
	t.Setenv("VAULT_BUILD_PASS", "build-pass")
	dir := t.TempDir()
	_, err := Build(ProviderLocal, SourceOptions{Path: filepath.Join(dir, "v.json")})
	require.NoError(t, err)
	_, err = Build(ProviderEnv, SourceOptions{Prefix: "VAULT_BUILD_"})
	require.NoError(t, err)
}

// NewLocalProvider：空路径/空口令环境变量/零 TTL/nil 时钟全部落到缺省值
// （79-80/83-84/86-88/90-92），且 Name 上报 local（106）。
func TestNewLocalProvider_DefaultsAndName(t *testing.T) {
	t.Chdir(t.TempDir())
	p, err := NewLocalProvider(SourceOptions{})
	require.NoError(t, err)
	assert.Equal(t, DefaultLocalPath, p.path)
	assert.Equal(t, DefaultPassphraseEnv, p.passphraseEnv)
	assert.Equal(t, DefaultSessionTTL, p.ttl)
	assert.NotNil(t, p.now)
	assert.Equal(t, ProviderLocal, p.Name())
}

// NewLocalProvider：父目录不可建（被普通文件占位）→ 构造失败（95-96）。
func TestNewLocalProvider_MkdirFail(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	_, err := NewLocalProvider(SourceOptions{Path: filepath.Join(blocker, "vault.json")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vault store dir")
}

// Unlock：KDF 参数不合法（N=0 但格式校验通过）→ scrypt 报错（121-122）。
func TestUnlock_KDFError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.json")
	store := localFile{Version: storeVersion,
		KDF:     localKDF{Algo: "scrypt", Salt: []byte("saltsaltsaltsalt"), N: 0, R: 8, P: 1},
		Entries: map[string]localBlob{}}
	raw, err := json.Marshal(store)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	t.Setenv("VAULT_TEST_PASS", "pass")
	p, err := NewLocalProvider(SourceOptions{Path: path, PassphraseEnv: "VAULT_TEST_PASS"})
	require.NoError(t, err)
	err = p.Unlock(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vault kdf")
}

// Unlock：存储文件为损坏 JSON → loadOrCreate 报错（296-297）。
func TestUnlock_CorruptStoreJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.json")
	require.NoError(t, os.WriteFile(path, []byte("{not-json"), 0o600))
	t.Setenv("VAULT_TEST_PASS", "pass")
	p, err := NewLocalProvider(SourceOptions{Path: path, PassphraseEnv: "VAULT_TEST_PASS"})
	require.NoError(t, err)
	require.ErrorContains(t, p.Unlock(context.Background()), "vault store")
}

// Unlock：存储格式不受支持（版本号不符）→ loadOrCreate 报错（299-300）。
func TestUnlock_UnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":99,"kdf":{"algo":"scrypt","salt":"c2FsdA=="}}`), 0o600))
	t.Setenv("VAULT_TEST_PASS", "pass")
	p, err := NewLocalProvider(SourceOptions{Path: path, PassphraseEnv: "VAULT_TEST_PASS"})
	require.NoError(t, err)
	require.ErrorContains(t, p.Unlock(context.Background()), "unsupported format")
}

// Unlock：存储路径被目录占位 → 读取非 ENOENT 错误（304-305）。
func TestUnlock_StorePathIsDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.json")
	require.NoError(t, os.Mkdir(path, 0o700))
	t.Setenv("VAULT_TEST_PASS", "pass")
	p, err := NewLocalProvider(SourceOptions{Path: path, PassphraseEnv: "VAULT_TEST_PASS"})
	require.NoError(t, err)
	require.Error(t, p.Unlock(context.Background()))
}

// Unlock：首次落盘失败（persist 的 OpenFile 错误）→ 建库中止（141-143）。
func TestUnlock_CreatePersistFail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.json")
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	t.Setenv("VAULT_TEST_PASS", "pass")
	p, err := NewLocalProvider(SourceOptions{Path: path, PassphraseEnv: "VAULT_TEST_PASS"})
	require.NoError(t, err)
	err = p.Unlock(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vault store write")
}

// Read：条目密文被篡改 → 解密失败（211-212）。
func TestRead_DecryptFailure(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "pass")
	p := localTestProvider(t, filepath.Join(t.TempDir(), "v.json"), 0, nil)
	unlockOK(t, p)
	require.NoError(t, p.Write(context.Background(), "a/b", "secret"))

	p.mu.Lock()
	blob := p.file.Entries["a/b"]
	blob.Data[0] ^= 0xff
	p.file.Entries["a/b"] = blob
	p.mu.Unlock()

	_, err := p.Read(context.Background(), "a/b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vault entry")
}

// Read / Delete：非法路径在校验层拒绝（193-194/265-266）。
func TestReadDelete_InvalidPath(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "pass")
	p := localTestProvider(t, filepath.Join(t.TempDir(), "v.json"), 0, nil)
	unlockOK(t, p)
	for _, bad := range []string{"", "..", "a/../b", "a//b", "/abs", " spaced"} {
		_, err := p.Read(context.Background(), bad)
		require.ErrorIs(t, err, ErrInvalidPath)
		require.ErrorIs(t, p.Delete(context.Background(), bad), ErrInvalidPath)
	}
}

// persist：目标路径被目录占位 → rename 失败（349-351），Write 回滚内存态。
func TestWrite_RenameFailRollsBack(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "pass")
	dir := t.TempDir()
	path := filepath.Join(dir, "v.json")
	p := localTestProvider(t, path, 0, nil)
	unlockOK(t, p)
	require.NoError(t, p.Write(context.Background(), "a", "1"))

	// 原存储文件换成同名目录：rename 到目录 → EISDIR。
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Mkdir(path, 0o700))

	err := p.Write(context.Background(), "b", "2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vault store rename")

	p.mu.Lock()
	_, hasA := p.file.Entries["a"]
	_, hasB := p.file.Entries["b"]
	p.mu.Unlock()
	assert.True(t, hasA, "既有条目保留")
	assert.False(t, hasB, "新条目回滚")
}

// newAEAD：密钥长度非法 → aes.NewCipher 报错（359-360）。
func TestNewAEAD_BadKeyLen(t *testing.T) {
	_, err := newAEAD([]byte("short"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vault cipher")
}

// Env 源：缺省前缀、Lock 空操作、非法路径（26-27/40/52-53）。
func TestEnvProvider_DefaultsLockInvalidPath(t *testing.T) {
	p := NewEnvProvider(SourceOptions{})
	assert.Equal(t, DefaultEnvPrefix, p.prefix)
	p.Lock()
	require.NoError(t, p.Unlock(context.Background()))
	assert.False(t, p.Locked())

	_, err := p.Read(context.Background(), "a//b")
	require.ErrorIs(t, err, ErrInvalidPath)
}

// Locked：滑动 TTL 到期后为真（158），未解锁前为真（155-158 前半）。
func TestLocked_ExpiryAndBeforeUnlock(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	clock := now
	p := localTestProvider(t, filepath.Join(t.TempDir(), "v.json"), 60*time.Second, func() time.Time { return clock })
	require.True(t, p.Locked(), "未解锁即为锁定态")
	t.Setenv("VAULT_TEST_PASS", "pass")
	unlockOK(t, p)
	require.False(t, p.Locked())

	clock = now.Add(61 * time.Second)
	require.True(t, p.Locked(), "TTL 过期后重新锁定")
}

// List：锁定会话报 ErrLocked（247-252 前半）。
func TestList_Locked(t *testing.T) {
	t.Setenv("VAULT_TEST_PASS", "pass")
	p := localTestProvider(t, filepath.Join(t.TempDir(), "v.json"), 0, nil)
	unlockOK(t, p)
	p.Lock()
	_, err := p.List(context.Background())
	require.ErrorIs(t, err, ErrLocked)
}

// Env List：前缀过滤含空值变量也列出（74-79 分支）。
func TestEnvProvider_ListFiltersPrefix(t *testing.T) {
	t.Setenv("CROUPIER_VAULT_ENV_AAA", "1")
	t.Setenv("CROUPIER_VAULT_ENV_BBB", "")
	t.Setenv("OTHER_VAULT_ENV_CCC", "1")
	p := NewEnvProvider(SourceOptions{})
	metas, err := p.List(context.Background())
	require.NoError(t, err)
	require.Len(t, metas, 2)
	assert.Equal(t, "aaa", metas[0].Path)
	assert.Equal(t, "bbb", metas[1].Path)
}

// Read：环境变量未设置（含空串）→ ErrNotFound（54-57 分支）。
func TestEnvProvider_ReadEmptyValueNotFound(t *testing.T) {
	t.Setenv("CROUPIER_VAULT_ENV_EMPTY", "")
	p := NewEnvProvider(SourceOptions{})
	_, err := p.Read(context.Background(), "empty")
	require.ErrorIs(t, err, ErrNotFound)
}

// Delete：env 只读源报 ErrReadOnly（64）。
func TestEnvProvider_DeleteReadOnly(t *testing.T) {
	p := NewEnvProvider(SourceOptions{})
	require.ErrorIs(t, p.Delete(context.Background(), "x/y"), ErrReadOnly)
}
