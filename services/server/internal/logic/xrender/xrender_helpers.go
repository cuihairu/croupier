package xrender

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
)

type ComponentDefinition struct {
	ID           string
	Pack         string
	Schema       interface{}
	UIConfig     interface{}
	SchemaFile   string
	UIConfigFile string
	UpdatedAt    time.Time
}

func (c ComponentDefinition) toMap() map[string]interface{} {
	resp := map[string]interface{}{
		"id":        c.ID,
		"pack":      c.Pack,
		"schema":    c.Schema,
		"updatedAt": utils.FormatTimestamp(c.UpdatedAt),
	}
	if c.UIConfig != nil {
		resp["uiSchema"] = c.UIConfig
	}
	if c.SchemaFile != "" {
		resp["schemaFile"] = c.SchemaFile
	}
	if c.UIConfigFile != "" {
		resp["uiSchemaFile"] = c.UIConfigFile
	}
	return resp
}

type TemplateDefinition struct {
	ID        string
	Pack      string
	Function  string
	Renderer  string
	View      map[string]interface{}
	UpdatedAt time.Time
}

func (t TemplateDefinition) toMap() map[string]interface{} {
	return map[string]interface{}{
		"id":        t.ID,
		"pack":      t.Pack,
		"function":  t.Function,
		"renderer":  t.Renderer,
		"view":      t.View,
		"updatedAt": utils.FormatTimestamp(t.UpdatedAt),
	}
}

func resolvePacksDir(cfg config.Config) string {
	dir := strings.TrimSpace(cfg.Packs.Dir)
	if dir == "" {
		dir = "packs"
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func loadXRenderComponents(cfg config.Config) ([]ComponentDefinition, error) {
	base := resolvePacksDir(cfg)
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return []ComponentDefinition{}, nil
		}
		return nil, err
	}

	type componentKey struct {
		Pack string
		ID   string
	}
	components := make(map[componentKey]*ComponentDefinition)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packName := entry.Name()
		uiDir := filepath.Join(base, packName, "ui")
		files, err := os.ReadDir(uiDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			name := file.Name()
			lower := strings.ToLower(name)
			var (
				componentID string
				isSchema    bool
				isUISchema  bool
			)
			switch {
			case strings.HasSuffix(lower, ".schema.json") && !strings.HasSuffix(lower, ".uischema.json"):
				componentID = strings.TrimSuffix(strings.TrimSuffix(name, ".json"), ".schema")
				isSchema = true
			case strings.HasSuffix(lower, ".uischema.json"):
				componentID = strings.TrimSuffix(strings.TrimSuffix(name, ".json"), ".uischema")
				isUISchema = true
			default:
				continue
			}
			if componentID == "" {
				continue
			}

			content, err := os.ReadFile(filepath.Join(uiDir, name))
			if err != nil {
				return nil, err
			}
			var payload interface{}
			if err := json.Unmarshal(content, &payload); err != nil {
				return nil, err
			}

			info, err := file.Info()
			if err != nil {
				return nil, err
			}

			key := componentKey{Pack: packName, ID: componentID}
			comp, exists := components[key]
			if !exists {
				comp = &ComponentDefinition{
					ID:        componentID,
					Pack:      packName,
					UpdatedAt: info.ModTime(),
				}
				components[key] = comp
			}
			if comp.UpdatedAt.Before(info.ModTime()) {
				comp.UpdatedAt = info.ModTime()
			}

			if isSchema {
				comp.Schema = payload
				comp.SchemaFile = filepath.Join(packName, "ui", name)
				continue
			}
			if isUISchema {
				comp.UIConfig = payload
				comp.UIConfigFile = filepath.Join(packName, "ui", name)
			}
		}
	}

	items := make([]ComponentDefinition, 0, len(components))
	for _, comp := range components {
		if comp.Schema == nil {
			continue
		}
		items = append(items, *comp)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Pack == items[j].Pack {
			return items[i].ID < items[j].ID
		}
		return items[i].Pack < items[j].Pack
	})
	return items, nil
}

func loadXRenderTemplates(cfg config.Config) ([]TemplateDefinition, error) {
	base := resolvePacksDir(cfg)
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return []TemplateDefinition{}, nil
		}
		return nil, err
	}

	type descriptorFile struct {
		ID          string                 `json:"id"`
		Description string                 `json:"description"`
		Name        string                 `json:"name"`
		Outputs     map[string]interface{} `json:"outputs"`
	}

	var templates []TemplateDefinition
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packName := entry.Name()
		descDir := filepath.Join(base, packName, "descriptors")
		files, err := os.ReadDir(descDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(descDir, file.Name()))
			if err != nil {
				return nil, err
			}
			var desc descriptorFile
			if err := json.Unmarshal(data, &desc); err != nil {
				return nil, err
			}
			viewsRaw, ok := desc.Outputs["views"]
			if !ok {
				continue
			}
			viewList, ok := viewsRaw.([]interface{})
			if !ok {
				continue
			}
			info, err := file.Info()
			if err != nil {
				return nil, err
			}
			for idx, item := range viewList {
				viewMap, err := normalizeMap(item)
				if err != nil {
					continue
				}
				viewID := ""
				if idVal, ok := viewMap["id"].(string); ok && strings.TrimSpace(idVal) != "" {
					viewID = idVal
				} else {
					viewID = fmt.Sprintf("%s_%d", strings.TrimSuffix(file.Name(), ".json"), idx)
				}
				renderer := ""
				if r, ok := viewMap["renderer"].(string); ok {
					renderer = r
				}
				templateID := fmt.Sprintf("%s.%s", desc.ID, viewID)
				clone := deepCopyMap(viewMap)
				templates = append(templates, TemplateDefinition{
					ID:        templateID,
					Pack:      packName,
					Function:  desc.ID,
					Renderer:  renderer,
					View:      clone,
					UpdatedAt: info.ModTime(),
				})
			}
		}
	}
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Pack == templates[j].Pack {
			if templates[i].Function == templates[j].Function {
				return templates[i].ID < templates[j].ID
			}
			return templates[i].Function < templates[j].Function
		}
		return templates[i].Pack < templates[j].Pack
	})
	return templates, nil
}

func normalizeMap(v interface{}) (map[string]interface{}, error) {
	if v == nil {
		return map[string]interface{}{}, nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	b, _ := json.Marshal(m)
	var out map[string]interface{}
	_ = json.Unmarshal(b, &out)
	return out
}

func normalizeSchemaInput(payload interface{}) (map[string]interface{}, error) {
	if payload == nil {
		return map[string]interface{}{}, nil
	}
	if schema, ok := payload.(map[string]interface{}); ok {
		return schema, nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(b, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}

func ensureObjectSchema(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		schema = map[string]interface{}{}
	}
	schemaType, _ := schema["type"].(string)
	if schemaType == "" {
		schema["type"] = "object"
	} else if schemaType != "object" {
		schema["type"] = "object"
	}
	if _, ok := schema["properties"].(map[string]interface{}); !ok {
		schema["properties"] = map[string]interface{}{}
	}
	return schema
}

func buildDefaultUISchema(schema map[string]interface{}) map[string]interface{} {
	props, _ := schema["properties"].(map[string]interface{})
	required := buildRequiredSet(schema["required"])
	fields := make(map[string]interface{}, len(props))
	order := make([]string, 0, len(props))
	for name, raw := range props {
		propSchema, _ := normalizeMap(raw)
		fieldUI := map[string]interface{}{
			"label": deriveFieldLabel(name, propSchema),
		}
		switch inferWidget(propSchema) {
		case "textarea":
			fieldUI["widget"] = "textarea"
		case "select":
			fieldUI["widget"] = "select"
		case "switch":
			fieldUI["widget"] = "switch"
		case "date":
			fieldUI["widget"] = "date"
		case "datetime":
			fieldUI["widget"] = "datetime"
		}
		if _, ok := required[name]; ok {
			fieldUI["required"] = true
		}
		if hint := buildPlaceholder(propSchema); hint != "" {
			fieldUI["placeholder"] = hint
		}
		fields[name] = fieldUI
		order = append(order, name)
	}

	layoutCols := 1
	switch {
	case len(order) >= 6:
		layoutCols = 3
	case len(order) >= 3:
		layoutCols = 2
	}

	return map[string]interface{}{
		"fields":   fields,
		"ui:order": order,
		"ui:layout": map[string]interface{}{
			"type": "grid",
			"cols": layoutCols,
		},
	}
}

func buildRequiredSet(raw interface{}) map[string]struct{} {
	set := make(map[string]struct{})
	switch v := raw.(type) {
	case []string:
		for _, item := range v {
			if strings.TrimSpace(item) != "" {
				set[item] = struct{}{}
			}
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				set[s] = struct{}{}
			}
		}
	}
	return set
}

func deriveFieldLabel(name string, schema map[string]interface{}) string {
	if title, ok := schema["title"].(string); ok && strings.TrimSpace(title) != "" {
		return title
	}
	return strings.Title(strings.ReplaceAll(name, "_", " "))
}

func inferWidget(schema map[string]interface{}) string {
	typ, _ := schema["type"].(string)
	format, _ := schema["format"].(string)
	if format == "date" {
		return "date"
	}
	if format == "date-time" {
		return "datetime"
	}
	if typ == "boolean" {
		return "switch"
	}
	if enumVals, ok := schema["enum"]; ok {
		if arr, ok := enumVals.([]interface{}); ok && len(arr) > 0 {
			return "select"
		}
	}
	if typ == "string" {
		if maxLen, ok := schema["maxLength"].(float64); ok && maxLen > 120 {
			return "textarea"
		}
	}
	return ""
}

func buildPlaceholder(schema map[string]interface{}) string {
	if format, ok := schema["format"].(string); ok {
		switch format {
		case "date-time":
			return "例如：2024-01-01T00:00:00Z"
		case "date":
			return "例如：2024-01-01"
		case "email":
			return "例如：user@example.com"
		}
	}
	if example, ok := schema["example"].(string); ok && example != "" {
		return example
	}
	return ""
}

func extractSchemaFields(schema map[string]interface{}) []map[string]interface{} {
	props, _ := schema["properties"].(map[string]interface{})
	required := buildRequiredSet(schema["required"])
	order := make([]string, 0, len(props))
	for name := range props {
		order = append(order, name)
	}
	sort.Strings(order)

	fields := make([]map[string]interface{}, 0, len(order))
	for _, name := range order {
		raw := props[name]
		propSchema, _ := normalizeMap(raw)
		field := map[string]interface{}{
			"name":        name,
			"type":        propSchema["type"],
			"description": propSchema["description"],
		}
		if _, ok := required[name]; ok {
			field["required"] = true
		}
		if enumVals, ok := propSchema["enum"]; ok {
			field["enum"] = enumVals
		}
		fields = append(fields, field)
	}
	return fields
}

func generateSampleData(schema map[string]interface{}) interface{} {
	if schema == nil {
		return map[string]interface{}{}
	}
	typ, _ := schema["type"].(string)
	switch typ {
	case "array":
		items, _ := normalizeMap(schema["items"])
		return []interface{}{generateSampleData(items)}
	case "string":
		if format, ok := schema["format"].(string); ok {
			switch format {
			case "date-time":
				return time.Now().UTC().Format(time.RFC3339)
			case "date":
				return time.Now().UTC().Format("2006-01-02")
			case "email":
				return "user@example.com"
			}
		}
		if enumVals, ok := schema["enum"].([]interface{}); ok && len(enumVals) > 0 {
			return enumVals[0]
		}
		return "example"
	case "integer", "number":
		return 0
	case "boolean":
		return false
	default:
		props, _ := schema["properties"].(map[string]interface{})
		result := make(map[string]interface{}, len(props))
		for name, raw := range props {
			propSchema, _ := normalizeMap(raw)
			result[name] = generateSampleData(propSchema)
		}
		return result
	}
}

// Exported helper wrappers for other logic packages.
func ListComponentDefinitions(cfg config.Config) ([]ComponentDefinition, error) {
	return loadXRenderComponents(cfg)
}

func FindComponentDefinition(cfg config.Config, id string) (*ComponentDefinition, error) {
	components, err := loadXRenderComponents(cfg)
	if err != nil {
		return nil, err
	}
	for i := range components {
		if components[i].ID == id {
			comp := components[i]
			return &comp, nil
		}
	}
	return nil, fmt.Errorf("component %s not found", id)
}

func ListTemplateDefinitions(cfg config.Config) ([]TemplateDefinition, error) {
	return loadXRenderTemplates(cfg)
}

func NormalizeSchemaInput(payload interface{}) (map[string]interface{}, error) {
	return normalizeSchemaInput(payload)
}

func EnsureObjectSchema(schema map[string]interface{}) map[string]interface{} {
	return ensureObjectSchema(schema)
}

func BuildDefaultUISchema(schema map[string]interface{}) map[string]interface{} {
	return buildDefaultUISchema(schema)
}

func ExtractSchemaFields(schema map[string]interface{}) []map[string]interface{} {
	return extractSchemaFields(schema)
}

func GenerateSampleData(schema map[string]interface{}) interface{} {
	return generateSampleData(schema)
}
