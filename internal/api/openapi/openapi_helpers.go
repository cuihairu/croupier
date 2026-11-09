package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
	"go.opentelemetry.io/otel/attribute"
)

// extensionInt 读取数值型扩展（json number 或数字字符串）。非法值产出
// error 诊断并按未声明处理（0）。
func extensionInt(extensions map[string]interface{}, key, field string, diags *[]spec.Diagnostic) int {
	if extensions == nil {
		return 0
	}
	value, ok := extensions[key]
	if !ok {
		return 0
	}
	var n int
	switch v := value.(type) {
	case float64:
		n = int(v)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			*diags = append(*diags, sourceDiagnostic(
				"openapi_timeout_ms_invalid",
				spec.SeverityError,
				key+" must be an integer (milliseconds)",
				field+"."+key,
			))
			return 0
		}
		n = parsed
	default:
		*diags = append(*diags, sourceDiagnostic(
			"openapi_timeout_ms_invalid",
			spec.SeverityError,
			key+" must be an integer (milliseconds)",
			field+"."+key,
		))
		return 0
	}
	return n
}

func extensionString(extensions map[string]interface{}, key string) string {
	if extensions == nil {
		return ""
	}
	value, ok := extensions[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func extensionLocalized(extensions map[string]interface{}, key string) spec.LocalizedText {
	if extensions == nil {
		return nil
	}
	value, ok := extensions[key]
	if !ok {
		return nil
	}
	out := spec.LocalizedText{}
	switch typed := value.(type) {
	case map[string]string:
		for locale, text := range typed {
			if strings.TrimSpace(locale) != "" && strings.TrimSpace(text) != "" {
				out[strings.TrimSpace(locale)] = strings.TrimSpace(text)
			}
		}
	case map[string]interface{}:
		for locale, rawText := range typed {
			text, ok := rawText.(string)
			if ok && strings.TrimSpace(locale) != "" && strings.TrimSpace(text) != "" {
				out[strings.TrimSpace(locale)] = strings.TrimSpace(text)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isValidRisk(risk spec.RiskLevel) bool {
	switch risk {
	case spec.RiskSafe, spec.RiskWarning, spec.RiskHigh, spec.RiskDanger:
		return true
	default:
		return false
	}
}

func sourceDiagnostic(code string, severity spec.DiagnosticSeverity, message, field string) spec.Diagnostic {
	return spec.Diagnostic{
		Code:     code,
		Severity: severity,
		Message:  message,
		Field:    field,
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func mustMarshalRaw(value interface{}) json.RawMessage {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(data)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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

func (s *Service) requireSourceRead(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(
		ctx,
		s.svcCtx,
		"无权查看 OpenAPI Source",
		"admin:all",
		"openapi_sources:read",
		"openapi_sources:write",
		"resources:read",
		"resources:diagnose",
		"functions:read",
		"functions:manage",
		"pages:read",
		"pages:edit",
	)
	return err
}

func (s *Service) requireSourceWrite(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(
		ctx,
		s.svcCtx,
		"无权管理 OpenAPI Source",
		"admin:all",
		"openapi_sources:write",
		"resources:diagnose",
		"functions:manage",
		"pages:edit",
	)
	return err
}

func (s *Service) auditSourceEvent(ctx context.Context, eventType audit.AuditEventType, gameID, env, sourceID, sourceName string, details map[string]interface{}) {
	if s == nil || s.svcCtx == nil || s.svcCtx.AuditService == nil {
		return
	}
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		actor = "unknown"
	}
	if details == nil {
		details = map[string]interface{}{}
	}
	details["game_id"] = gameID
	details["env"] = env
	details["source_id"] = sourceID
	_, err = s.svcCtx.AuditService.Log(ctx, eventType,
		audit.WithActorID(actor, "user", actor),
		audit.WithResource(audit.ResourceInfo{
			Type:        "openapi_source",
			ID:          sourceID,
			Name:        sourceName,
			GameID:      gameID,
			Environment: env,
		}),
		audit.WithDetails(details),
		audit.WithOutcome("success", ""),
	)
	if err != nil {
		s.svcCtx.AuditService.ReportWriteGap(ctx, eventType, err)
	}
}

func (s *Service) startSourceSpan(ctx context.Context, name, gameID, env string, attrs ...attribute.KeyValue) (context.Context, func(error, ...attribute.KeyValue)) {
	if s == nil || s.svcCtx == nil || s.svcCtx.Telemetry == nil {
		return ctx, func(error, ...attribute.KeyValue) {}
	}
	startedAt := time.Now()
	baseAttrs := []attribute.KeyValue{
		attribute.String("game.id", gameID),
		attribute.String("game.env", env),
	}
	baseAttrs = append(baseAttrs, attrs...)
	nextCtx, span := s.svcCtx.Telemetry.StartSpan(ctx, name, baseAttrs...)
	return nextCtx, func(err error, extraAttrs ...attribute.KeyValue) {
		s.svcCtx.Telemetry.EndSpan(span, startedAt, err, extraAttrs...)
	}
}
