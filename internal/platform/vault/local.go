package vault

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"golang.org/x/crypto/scrypt"
)

// 本地加密存储缺省值（目录/文件权限 0600/0700；口令只能经环境变量注入）。
const (
	DefaultLocalPath     = "data/vault.json"
	DefaultPassphraseEnv = "CROUPIER_VAULT_PASSPHRASE"
	DefaultSessionTTL    = 900 * time.Second // 15 分钟滑动窗口
	defaultScryptN       = 32768
	defaultScryptR       = 8
	defaultScryptP       = 1
	scryptKeyLen         = 32 // AES-256
	storeVersion         = 1
	checkPlaintext       = "croupier-vault-check"
)

// LocalProvider 本地加密文件密码箱（缺省实现，todo 4250）：scrypt 主口令
// 派生密钥 + AES-256-GCM 逐条加密，存储为单文件 JSON（0600）。会话滑动
// TTL——每次成功读写续期，空闲超时自动锁定（Read/Write 返回 ErrLocked，
// 调用方重新 Unlock）。写路径全量重写 + tmp+rename 原子落盘。
type LocalProvider struct {
	path          string
	passphraseEnv string
	ttl           time.Duration
	now           func() time.Time
	logger        *slog.Logger

	mu    sync.Mutex
	key   []byte     // 会话密钥（Lock 清零）
	until time.Time  // 会话到期时刻（滑动）
	file  *localFile // 内存态（解锁时载入）
}

// localFile 存储文件形态（v1）：KDF 参数 + 校验块 + 条目密文。明文凭据
// 与口令不出现于任何字段。
type localFile struct {
	Version   int                  `json:"version"`
	KDF       localKDF             `json:"kdf"`
	Check     localBlob            `json:"check"`
	Entries   map[string]localBlob `json:"entries"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

type localKDF struct {
	Algo string `json:"algo"`
	Salt []byte `json:"salt"`
	N    int    `json:"n"`
	R    int    `json:"r"`
	P    int    `json:"p"`
}

// localBlob 单块密文（nonce + ciphertext，GCM 无独立 tag 字段）。
type localBlob struct {
	Nonce []byte `json:"nonce"`
	Data  []byte `json:"data"`
}

// NewLocalProvider 构造本地密码箱（不落盘；首次 Unlock 时建/载存储）。
func NewLocalProvider(opts SourceOptions) (*LocalProvider, error) {
	path := opts.Path
	if path == "" {
		path = DefaultLocalPath
	}
	passphraseEnv := opts.PassphraseEnv
	if passphraseEnv == "" {
		passphraseEnv = DefaultPassphraseEnv
	}
	ttl := opts.SessionTTL
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("vault store dir: %w", err)
	}
	return &LocalProvider{
		path:          path,
		passphraseEnv: passphraseEnv,
		ttl:           ttl,
		now:           now,
		logger:        slog.Default(),
	}, nil
}

func (p *LocalProvider) Name() string { return ProviderLocal }

// Unlock 从主口令环境变量取口令：口令缺失 → ErrPassphraseMissing；存储
// 不存在则初始化（随机盐 + 校验块）；口令经 scrypt 派生后解校验块验证，
// 失败 → ErrPassphraseMismatch（消息不含口令）。成功=建立滑动 TTL 会话。
func (p *LocalProvider) Unlock(context.Context) error {
	pass := os.Getenv(p.passphraseEnv)
	if pass == "" {
		return ErrPassphraseMissing
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	file, created, err := p.loadOrCreate()
	if err != nil {
		return err
	}
	key, err := scrypt.Key([]byte(pass), file.KDF.Salt, file.KDF.N, file.KDF.R, file.KDF.P, scryptKeyLen)
	if err != nil {
		return fmt.Errorf("vault kdf: %w", err)
	}
	aead, err := newAEAD(key)
	if err != nil {
		clear(key)
		return err
	}
	if created {
		// 新存储：用派生密钥封校验块并落盘（后续 Unlock 以解校验块验口令）。
		nonce := make([]byte, aead.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			clear(key)
			return fmt.Errorf("vault nonce: %w", err)
		}
		file.Check = localBlob{Nonce: nonce, Data: aead.Seal(nil, nonce, []byte(checkPlaintext), nil)}
		if err := p.persist(file, aead); err != nil {
			clear(key)
			return err
		}
	} else if _, err := aead.Open(nil, file.Check.Nonce, file.Check.Data, nil); err != nil {
		clear(key)
		return ErrPassphraseMismatch
	}
	p.key = key
	p.until = p.now().Add(p.ttl)
	p.file = file
	return nil
}

// Locked 报告会话是否锁定（未解锁或滑动 TTL 已过）。
func (p *LocalProvider) Locked() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.locked()
}

func (p *LocalProvider) locked() bool {
	return p.key == nil || !p.now().Before(p.until)
}

// Lock 立即锁定并清零会话密钥。
func (p *LocalProvider) Lock() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lockNow()
}

func (p *LocalProvider) lockNow() {
	if p.key != nil {
		clear(p.key)
		p.key = nil
	}
	p.until = time.Time{}
	p.file = nil
}

// session 取会话密钥（锁定=ErrLocked）；命中即滑动续期。
func (p *LocalProvider) session() ([]byte, *localFile, error) {
	if p.locked() {
		return nil, nil, ErrLocked
	}
	p.until = p.now().Add(p.ttl)
	return p.key, p.file, nil
}

// Read 解密取回凭据明文。
func (p *LocalProvider) Read(_ context.Context, path string) (string, error) {
	if err := ValidatePath(path); err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	key, file, err := p.session()
	if err != nil {
		return "", err
	}
	blob, ok := file.Entries[path]
	if !ok {
		return "", ErrNotFound
	}
	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}
	plain, err := aead.Open(nil, blob.Nonce, blob.Data, []byte(path))
	if err != nil {
		return "", fmt.Errorf("vault entry %q: %w", path, err)
	}
	return string(plain), nil
}

// Write 加密写入/覆盖凭据并原子落盘。
func (p *LocalProvider) Write(_ context.Context, path, value string) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	key, file, err := p.session()
	if err != nil {
		return err
	}
	aead, err := newAEAD(key)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("vault nonce: %w", err)
	}
	file.Entries[path] = localBlob{
		Nonce: nonce,
		Data:  aead.Seal(nil, nonce, []byte(value), []byte(path)),
	}
	if err := p.persist(file, aead); err != nil {
		delete(file.Entries, path) // 落盘失败回滚内存态，保持与存储一致
		return err
	}
	return nil
}

// List 列凭据路径与更新时间（不含明文）。
func (p *LocalProvider) List(_ context.Context) ([]EntryMeta, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, file, err := p.session()
	if err != nil {
		return nil, err
	}
	metas := make([]EntryMeta, 0, len(file.Entries))
	for path := range file.Entries {
		metas = append(metas, EntryMeta{Path: path, UpdatedAt: file.UpdatedAt})
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Path < metas[j].Path })
	return metas, nil
}

// Delete 删除凭据并原子落盘。
func (p *LocalProvider) Delete(_ context.Context, path string) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	key, file, err := p.session()
	if err != nil {
		return err
	}
	if _, ok := file.Entries[path]; !ok {
		return ErrNotFound
	}
	removed := file.Entries[path]
	delete(file.Entries, path)
	aead, aeadErr := newAEAD(key)
	if aeadErr != nil {
		return aeadErr
	}
	if err := p.persist(file, aead); err != nil {
		file.Entries[path] = removed // 落盘失败回滚内存态
		return err
	}
	return nil
}

// loadOrCreate 载入存储；文件不存在则初始化（随机盐 + 空条目，不落盘——
// 校验块由 Unlock 派生密钥后封印并首次落盘）。
func (p *LocalProvider) loadOrCreate() (*localFile, bool, error) {
	raw, err := os.ReadFile(p.path)
	if err == nil {
		var file localFile
		if err := json.Unmarshal(raw, &file); err != nil {
			return nil, false, fmt.Errorf("vault store %s: %w", p.path, err)
		}
		if file.Version != storeVersion || file.KDF.Algo != "scrypt" || len(file.KDF.Salt) == 0 {
			return nil, false, fmt.Errorf("vault store %s: unsupported format", p.path)
		}
		return &file, false, nil
	}
	if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("vault store %s: %w", p.path, err)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, false, fmt.Errorf("vault salt: %w", err)
	}
	file := &localFile{
		Version: storeVersion,
		KDF:     localKDF{Algo: "scrypt", Salt: salt, N: defaultScryptN, R: defaultScryptR, P: defaultScryptP},
		Entries: map[string]localBlob{},
	}
	return file, true, nil
}

// persist 校验块重封 + 全量重写（tmp+rename 原子替换，0600）。校验块以
// 当前会话密钥重封——换口令重开存储后旧校验块自然失效。
func (p *LocalProvider) persist(file *localFile, aead cipher.AEAD) error {
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("vault nonce: %w", err)
	}
	file.Check = localBlob{Nonce: nonce, Data: aead.Seal(nil, nonce, []byte(checkPlaintext), nil)}
	file.UpdatedAt = p.now().UTC()
	raw, err := json.Marshal(file)
	if err != nil {
		return fmt.Errorf("vault store marshal: %w", err)
	}
	tmp := p.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("vault store write: %w", err)
	}
	w := bufio.NewWriter(f)
	if _, err := w.Write(raw); err != nil {
		f.Close()
		return fmt.Errorf("vault store write: %w", err)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("vault store write: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("vault store write: %w", err)
	}
	if err := os.Rename(tmp, p.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("vault store rename: %w", err)
	}
	return nil
}

// newAEAD AES-256-GCM（key 必须恰 32 字节，scryptKeyLen 保证）。
func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("vault cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("vault gcm: %w", err)
	}
	return aead, nil
}

// clear 清零密钥字节（best effort；防 swap/残留是尽力而为语义）。
func clear(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
