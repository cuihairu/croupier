package settings

import (
	"context"
	"log/slog"

	"github.com/cuihairu/croupier/internal/platform/vault"
)

var secretResolver vault.Resolver

// SetSecretResolver 注入凭据展开函数（server 装配期调用一次；测试用
// SetSecretResolver(nil) 复位）。Layered 单例尚未初始化时同样安全——
// 展开函数是包级装配，getter 侧按需取用。
func SetSecretResolver(r vault.Resolver) { secretResolver = r }

// secretStr 读取 secretKeys 类键并展开值内引用：vault:// 引用经 resolver
// 展开，内联值直通。解析失败 → 空串 + warn 日志（fail-closed：消费侧按
// 「未配置」降级，绝不把引用串当真凭据外发）。日志只含键名与引用路径，
// 不含明文。
func (l *Layered) secretStr(ctx context.Context, key string) string {
	v, _, ok := l.GetString(ctx, key)
	if !ok || v == "" {
		return ""
	}
	if secretResolver == nil || !vault.IsVaultRef(v) {
		return v
	}
	resolved, err := secretResolver(ctx, v)
	if err != nil {
		slog.WarnContext(ctx, "settings: secret resolve failed; treat as unset",
			"key", key, "error", err)
		return ""
	}
	return resolved
}
