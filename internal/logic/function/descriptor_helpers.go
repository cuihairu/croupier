package function

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

var nodeSeparatorDupRE = regexp.MustCompile(`[_\-.]{2,}`)

func sanitizeNodeKey(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
			continue
		}
		if r == ' ' || r == '/' || r == ':' {
			b.WriteRune('_')
		}
	}
	s := b.String()
	s = strings.Trim(s, "._-")
	s = nodeSeparatorDupRE.ReplaceAllString(s, "_")
	return s
}

func normalizeTerm(aliasMap map[string]map[string]string, domain, raw string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	value := strings.TrimSpace(strings.ToLower(raw))
	if domain == "" || value == "" {
		return value
	}
	if m, ok := aliasMap[domain]; ok {
		if canonical, ok := m[value]; ok && canonical != "" {
			return canonical
		}
	}
	return value
}

func termDisplay(displayMap map[string]map[string]map[string]string, domain, key string) map[string]string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	key = strings.TrimSpace(strings.ToLower(key))
	if domain == "" || key == "" {
		return nil
	}
	dm, ok := displayMap[domain]
	if !ok {
		return nil
	}
	disp, ok := dm[key]
	if !ok || len(disp) == 0 {
		return nil
	}
	out := map[string]string{}
	if zh := strings.TrimSpace(disp["zh"]); zh != "" {
		out["zh"] = zh
	}
	if en := strings.TrimSpace(disp["en"]); en != "" {
		out["en"] = en
	}
	return out
}

func extractOperationRequestSchema(op *openapi3.Operation) map[string]interface{} {
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil
	}
	content := op.RequestBody.Value.Content
	if len(content) == 0 {
		return nil
	}

	var media *openapi3.MediaType
	if mt, ok := content["application/json"]; ok && mt != nil {
		media = mt
	} else {
		for _, mt := range content {
			if mt != nil {
				media = mt
				break
			}
		}
	}
	if media == nil || media.Schema == nil {
		return nil
	}
	return schemaRefToMap(media.Schema)
}

func schemaRefToMap(ref *openapi3.SchemaRef) map[string]interface{} {
	if ref == nil {
		return nil
	}
	if ref.Value != nil {
		var out map[string]interface{}
		raw, err := json.Marshal(ref.Value)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil
		}
		return out
	}
	if ref.Ref != "" {
		return map[string]interface{}{"$ref": ref.Ref}
	}
	return nil
}
