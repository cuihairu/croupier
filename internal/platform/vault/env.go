package vault

import (
	"context"
	"os"
	"sort"
	"strings"
	"time"
)

// DefaultEnvPrefix env 源环境变量名前缀缺省值。
const DefaultEnvPrefix = "CROUPIER_VAULT_ENV_"

// EnvProvider 环境变量源（只读）：凭据路径映射到带前缀的环境变量名。
// 容器部署零加密面（编排层管 secret），本机开发零口令。路径归一："/"
// 与 "-" 归并为 "_" 后大写——"smtp/password" 与 "smtp_password" 同指
// CROUPIER_VAULT_ENV_SMTP_PASSWORD。
type EnvProvider struct {
	prefix string
}

// NewEnvProvider 构造 env 源（prefix 空取 DefaultEnvPrefix）。
func NewEnvProvider(opts SourceOptions) *EnvProvider {
	prefix := opts.Prefix
	if prefix == "" {
		prefix = DefaultEnvPrefix
	}
	return &EnvProvider{prefix: prefix}
}

func (p *EnvProvider) Name() string { return ProviderEnv }

// Unlock 恒成功（环境即存储，无会话概念）。
func (p *EnvProvider) Unlock(context.Context) error { return nil }

// Locked 恒 false。
func (p *EnvProvider) Locked() bool { return false }

// Lock 空操作。
func (p *EnvProvider) Lock() {}

// envName 路径 → 环境变量名。
func (p *EnvProvider) envName(path string) string {
	n := strings.ReplaceAll(path, "/", "_")
	n = strings.ReplaceAll(n, "-", "_")
	return p.prefix + strings.ToUpper(n)
}

// Read 取环境变量值；未设置/空 = ErrNotFound。
func (p *EnvProvider) Read(_ context.Context, path string) (string, error) {
	if err := ValidatePath(path); err != nil {
		return "", err
	}
	if v, ok := os.LookupEnv(p.envName(path)); ok && v != "" {
		return v, nil
	}
	return "", ErrNotFound
}

// Write 只读源不支持。
func (p *EnvProvider) Write(context.Context, string, string) error { return ErrReadOnly }

// Delete 只读源不支持。
func (p *EnvProvider) Delete(context.Context, string) error { return ErrReadOnly }

// List 枚举带前缀的环境变量（只报名，不报值）；反向映射有损——"_" 不
// 还原为 "/"，读侧用 "smtp_password" 形态与 List 输出一致（"smtp/password"
// 是读侧等价别名）。
func (p *EnvProvider) List(context.Context) ([]EntryMeta, error) {
	now := time.Now().UTC()
	var metas []EntryMeta
	for _, kv := range os.Environ() {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(name, p.prefix) {
			continue
		}
		suffix := strings.ToLower(strings.TrimPrefix(name, p.prefix))
		metas = append(metas, EntryMeta{Path: suffix, UpdatedAt: now})
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Path < metas[j].Path })
	return metas, nil
}
