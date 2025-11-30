package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xeipuuv/gojsonschema"

	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
)

type schemaDocument struct {
	ID        string
	Name      string
	Schema    interface{}
	UIConfig  interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

type schemaFileModel struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Schema    interface{} `json:"schema"`
	UIConfig  interface{} `json:"ui_config,omitempty"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
}

func resolveSchemasDir(cfg config.Config) string {
	dir := strings.TrimSpace(cfg.Schemas.Dir)
	if dir == "" {
		dir = filepath.Join("packs", "ui")
	}
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
	}
	return filepath.Join(dir, "custom")
}

func ensureSchemasDir(cfg config.Config) (string, error) {
	dir := resolveSchemasDir(cfg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func schemaFilePath(cfg config.Config, id string) string {
	dir := resolveSchemasDir(cfg)
	return filepath.Join(dir, fmt.Sprintf("%s.json", id))
}

func saveSchema(cfg config.Config, doc *schemaDocument) error {
	dir, err := ensureSchemasDir(cfg)
	if err != nil {
		return err
	}
	file := schemaFileModel{
		ID:        doc.ID,
		Name:      doc.Name,
		Schema:    doc.Schema,
		UIConfig:  doc.UIConfig,
		CreatedAt: doc.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: doc.UpdatedAt.UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s.json", doc.ID)), data, 0o644)
}

func loadSchema(cfg config.Config, id string) (*schemaDocument, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("schema ID 不能为空")
	}
	path := schemaFilePath(cfg, id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("schema %s 不存在", id)
		}
		return nil, err
	}
	var file schemaFileModel
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return schemaDocFromFile(file), nil
}

func deleteSchema(cfg config.Config, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("schema ID 不能为空")
	}
	path := schemaFilePath(cfg, id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("schema %s 不存在", id)
		}
		return err
	}
	return nil
}

func listSchemas(cfg config.Config) ([]*schemaDocument, error) {
	dir := resolveSchemasDir(cfg)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*schemaDocument{}, nil
		}
		return nil, err
	}
	docs := make([]*schemaDocument, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var file schemaFileModel
		if err := json.Unmarshal(data, &file); err != nil {
			continue
		}
		docs = append(docs, schemaDocFromFile(file))
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Name < docs[j].Name
	})
	return docs, nil
}

func schemaDocFromFile(file schemaFileModel) *schemaDocument {
	createdAt, _ := time.Parse(time.RFC3339, file.CreatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	updatedAt, _ := time.Parse(time.RFC3339, file.UpdatedAt)
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	return &schemaDocument{
		ID:        file.ID,
		Name:      file.Name,
		Schema:    file.Schema,
		UIConfig:  file.UIConfig,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

func (doc *schemaDocument) toMap() map[string]interface{} {
	return map[string]interface{}{
		"id":        doc.ID,
		"name":      doc.Name,
		"schema":    doc.Schema,
		"uiConfig":  doc.UIConfig,
		"createdAt": utils.FormatTimestamp(doc.CreatedAt),
		"updatedAt": utils.FormatTimestamp(doc.UpdatedAt),
	}
}

func generateSchemaID(name string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	builder := strings.Builder{}
	prevDash := false
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			builder.WriteByte('-')
			prevDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		slug = uuid.NewString()
	}
	return slug
}

func ensureUniqueSchemaID(cfg config.Config, name string) string {
	id := generateSchemaID(name)
	for {
		if _, err := os.Stat(schemaFilePath(cfg, id)); os.IsNotExist(err) {
			return id
		}
		id = fmt.Sprintf("%s-%s", id, uuid.NewString()[:8])
	}
}

func validateSchemaDefinition(schema interface{}) error {
	data, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("schema 不是有效的 JSON: %w", err)
	}
	loader := gojsonschema.NewBytesLoader(data)
	if _, err := gojsonschema.NewSchema(loader); err != nil {
		return fmt.Errorf("schema 校验失败: %w", err)
	}
	return nil
}

func validatePayloadAgainst(schema interface{}, payload interface{}) (bool, []string, error) {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return false, nil, fmt.Errorf("schema JSON 无效: %w", err)
	}
	dataBytes, err := json.Marshal(payload)
	if err != nil {
		return false, nil, fmt.Errorf("数据 JSON 无效: %w", err)
	}
	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	payloadLoader := gojsonschema.NewBytesLoader(dataBytes)
	result, err := gojsonschema.Validate(schemaLoader, payloadLoader)
	if err != nil {
		return false, nil, err
	}
	if result.Valid() {
		return true, nil, nil
	}
	errors := make([]string, 0, len(result.Errors()))
	for _, issue := range result.Errors() {
		errors = append(errors, issue.String())
	}
	return false, errors, nil
}
