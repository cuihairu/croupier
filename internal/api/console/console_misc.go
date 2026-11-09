package console

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

func sanitizeApprovalID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "binding"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.', r == '_', r == '-':
			builder.WriteRune('_')
		}
	}
	if builder.Len() == 0 {
		return "binding"
	}
	return builder.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s *Service) requireConsoleRead(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权查看运行控制台", "admin:all", "console:read", "pages:read", "function:invoke")
	return err
}

func (s *Service) requireConsoleExecute(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权执行运行控制台操作", "admin:all", "function:invoke")
	return err
}

func requireScope(ctx context.Context) (string, string, error) {
	scope := svc.GameScopeFromContext(ctx)
	gameID := strings.TrimSpace(scope.GameID)
	env := strings.TrimSpace(scope.Env)
	if gameID == "" {
		return "", "", errorx.NewBadRequest("X-Game-ID is required")
	}
	if env == "" {
		return "", "", errorx.NewBadRequest("X-Env is required")
	}
	return gameID, env, nil
}

// normalizeLanguage 归一化为 BCP47 canonical 形态（语言小写、region 大写，
// 如 zh-tw → zh-TW）。此前盲目 ToLower 使 ja-JP 等请求永不命中 map 里
// canonical 键，多语言排序静默失效。
func normalizeLanguage(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return "zh-CN"
	}
	lang = strings.ReplaceAll(lang, "_", "-")
	parts := strings.Split(lang, "-")
	parts[0] = strings.ToLower(parts[0])
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i])
	}
	switch parts[0] {
	case "zh":
		if len(parts) == 1 {
			return "zh-CN"
		}
	case "en":
		if len(parts) == 1 {
			return "en-US"
		}
	}
	return strings.Join(parts, "-")
}

func getLocalizedText(labels spec.LocalizedText, lang, fallback string) string {
	if labels == nil {
		return fallback
	}
	if v, ok := labels[lang]; ok && v != "" {
		return v
	}
	if v, ok := labels["zh-CN"]; ok && v != "" {
		return v
	}
	if v, ok := labels["en-US"]; ok && v != "" {
		return v
	}
	for _, v := range labels {
		if v != "" {
			return v
		}
	}
	return fallback
}

// pageEntry 是挂载到菜单的已发布页面在控制台菜单中的条目形态。
type pageEntry struct {
	key   string
	title spec.LocalizedText
	icon  string
	order int
}

func ErrPageNotFound(key string) error {
	return &PageNotFoundError{Key: key}
}

type PageNotFoundError struct {
	Key string
}

func (e *PageNotFoundError) Error() string {
	return "page not found: " + e.Key
}
