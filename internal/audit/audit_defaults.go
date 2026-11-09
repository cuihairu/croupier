package audit

import (
	"fmt"
)

// setDefaults sets default values based on event type
func (s *AuditService) setDefaults(record *AuditRecord) {
	// Set category based on event type
	if record.Category == "" {
		record.Category = s.inferCategory(record.EventType)
	}

	// Set severity based on event type
	if record.Severity == "" {
		record.Severity = s.inferSeverity(record.EventType)
	}

	// Set action if not set
	if record.Action == "" {
		record.Action = string(record.EventType)
	}
}

// inferCategory infers category from event type
func (s *AuditService) inferCategory(eventType AuditEventType) AuditCategory {
	switch eventType {
	case EventLogin, EventLogout, EventLoginFailed, EventAccessGranted, EventAccessDenied:
		return CategorySecurity
	case EventUserCreate, EventUserUpdate, EventUserDelete:
		return CategoryAdmin
	case EventFunctionInvoke, EventFunctionRegister, EventFunctionUnregister,
		EventFunctionUpdate, EventFunctionContractUpdated,
		EventPageDraftSave, EventPagePublish,
		EventPageUnpublish, EventPageRollback, EventPageExecute,
		EventOpenAPISourceCreate, EventOpenAPISourceBindingCreate,
		EventOpenAPISourceUpdate, EventOpenAPISourceBindingDelete,
		EventOpenAPISourceBindingAuto,
		EventConfigUpdate, EventConfigEmergencyEdit,
		EventConfigSourceChange:
		return CategoryOperational
	case EventDataAccess, EventDataExport, EventDataDelete:
		return CategoryData
	default:
		return CategoryCompliance
	}
}

// inferSeverity infers severity from event type
func (s *AuditService) inferSeverity(eventType AuditEventType) AuditSeverity {
	switch eventType {
	case EventLoginFailed, EventAccessDenied:
		return SeverityWarning
	case EventUserDelete, EventBackupRestore:
		return SeverityCritical
	default:
		return SeverityInfo
	}
}

// maskSensitiveData masks sensitive fields in the record
func (s *AuditService) maskSensitiveData(record *AuditRecord) {
	if record.Details != nil {
		record.Details = maskMap(record.Details, s.sensitiveFields)
	}
	if record.Changes != nil {
		if record.Changes.Before != nil {
			record.Changes.Before = maskMap(record.Changes.Before, s.sensitiveFields)
		}
		if record.Changes.After != nil {
			record.Changes.After = maskMap(record.Changes.After, s.sensitiveFields)
		}
	}
}

// defaultSensitiveFields 是平台统一的敏感字段清单（精确匹配）。
var defaultSensitiveFields = []string{
	"password", "secret", "token", "key", "credential",
	"api_key", "private_key", "access_token",
}

// MaskSensitiveValue 递归掩码任意 JSON 值中的敏感字段（与审计记录同一
// 敏感字段清单）。供执行留痕等需要同口径脱敏的场景复用。
func MaskSensitiveValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return maskMap(v, defaultSensitiveFields)
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = MaskSensitiveValue(item)
		}
		return out
	default:
		return value
	}
}

func maskMap(m map[string]interface{}, sensitiveFields []string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		if isSensitiveField(k, sensitiveFields) {
			result[k] = "***MASKED***"
		} else {
			if nested, ok := v.(map[string]interface{}); ok {
				result[k] = maskMap(nested, sensitiveFields)
			} else {
				result[k] = v
			}
		}
	}
	return result
}

func isSensitiveField(field string, sensitiveFields []string) bool {
	fieldLower := field
	for _, sf := range sensitiveFields {
		if fieldLower == sf {
			return true
		}
	}
	return false
}

func calculateDiffFields(before, after map[string]interface{}) []string {
	var diff []string
	allKeys := make(map[string]bool)

	for k := range before {
		allKeys[k] = true
	}
	for k := range after {
		allKeys[k] = true
	}

	for k := range allKeys {
		bv, bExists := before[k]
		av, aExists := after[k]

		if !bExists || !aExists {
			diff = append(diff, k)
			continue
		}

		// Simple comparison - in production, you'd do deep comparison
		if fmt.Sprintf("%v", bv) != fmt.Sprintf("%v", av) {
			diff = append(diff, k)
		}
	}

	return diff
}
