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
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/logic/utils"
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
	UIConfig  interface{} `json:"uiConfig,omitempty"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

func resolveSchemasDir(cfg config.Config) string {
	dir := strings.TrimSpace(cfg.Schemas.Dir)
	if dir == "" {
		dir = "schemas"
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
		return nil, errorx.NewBadRequest("schema ID 不能为空")
	}
	rawPath := schemaFilePath(cfg, id)

	// 确保路径在 schemas 目录内，防止路径遍历攻击
	safePath, err := validateSchemaPath(cfg, rawPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errorx.NewNotFound("schema 不存在: " + id)
		}
		return nil, err
	}
	var file schemaFileModel
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return schemaDocFromFile(file), nil
}

func deleteSchemaByID(cfg config.Config, id string) error {
	if strings.TrimSpace(id) == "" {
		return errorx.NewBadRequest("schema ID 不能为空")
	}
	rawPath := schemaFilePath(cfg, id)

	// 确保路径在 schemas 目录内，防止路径遍历攻击
	safePath, err := validateSchemaPath(cfg, rawPath)
	if err != nil {
		return err
	}

	if err := os.Remove(safePath); err != nil {
		if os.IsNotExist(err) {
			return errorx.NewNotFound("schema 不存在: " + id)
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

func (doc *schemaDocument) toItem() SchemaItem {
	return SchemaItem{
		ID:        doc.ID,
		Name:      doc.Name,
		Schema:    doc.Schema,
		UIConfig:  doc.UIConfig,
		CreatedAt: utils.FormatTimestamp(doc.CreatedAt),
		UpdatedAt: utils.FormatTimestamp(doc.UpdatedAt),
	}
}

func (doc *schemaDocument) toCreateResponse() CreateResponse {
	return CreateResponse{
		ID:        doc.ID,
		Name:      doc.Name,
		Schema:    doc.Schema,
		UIConfig:  doc.UIConfig,
		CreatedAt: utils.FormatTimestamp(doc.CreatedAt),
		UpdatedAt: utils.FormatTimestamp(doc.UpdatedAt),
	}
}

func (doc *schemaDocument) toGetResponse() GetResponse {
	return GetResponse{
		ID:        doc.ID,
		Name:      doc.Name,
		Schema:    doc.Schema,
		UIConfig:  doc.UIConfig,
		CreatedAt: utils.FormatTimestamp(doc.CreatedAt),
		UpdatedAt: utils.FormatTimestamp(doc.UpdatedAt),
	}
}

func (doc *schemaDocument) toUpdateResponse() UpdateResponse {
	return UpdateResponse{
		ID:        doc.ID,
		Name:      doc.Name,
		Schema:    doc.Schema,
		UIConfig:  doc.UIConfig,
		CreatedAt: utils.FormatTimestamp(doc.CreatedAt),
		UpdatedAt: utils.FormatTimestamp(doc.UpdatedAt),
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
		id = id + "-" + uuid.NewString()[:8]
	}
}

func validateSchemaDefinition(schema interface{}) error {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	if err := compiler.AddResource("schema.json", schema); err != nil {
		return errorx.NewBadRequest("schema 解析失败")
	}
	if _, err := compiler.Compile("schema.json"); err != nil {
		return errorx.NewBadRequest("schema 校验失败")
	}
	return nil
}

func validatePayloadAgainst(schema interface{}, payload interface{}) (bool, []string, error) {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	if err := compiler.AddResource("schema.json", schema); err != nil {
		return false, nil, err
	}
	sch, err := compiler.Compile("schema.json")
	if err != nil {
		return false, nil, err
	}

	if err := sch.Validate(payload); err != nil {
		// jsonschema v6 的 Schema.Validate 出错时恒返回 *ValidationError
		//（见其 validator.go：err.(*ValidationError) 直接断言并重新构造），
		// 下方非 ValidationError 回退分支为死代码，已删除。
		ve := err.(*jsonschema.ValidationError)
		errors := extractErrors(ve)
		return false, errors, nil
	}
	return true, nil, nil
}

func extractErrors(err *jsonschema.ValidationError) []string {
	var errors []string
	if err.Error() != "" {
		errors = append(errors, err.Error())
	}
	for _, cause := range err.Causes {
		errors = append(errors, extractErrors(cause)...)
	}
	return errors
}

// validateSchemaPath ensures the path stays within the schemas directory and returns the cleaned path.
func validateSchemaPath(cfg config.Config, path string) (string, error) {
	schemasDir, err := ensureSchemasDir(cfg)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	// schemasDir 来自上方 ensureSchemasDir 的成功返回：要么是绝对路径
	//（filepath.Abs 不再依赖 Getwd，恒成功），要么是相对路径但 MkdirAll 已
	// 成功证明 cwd 有效（Getwd 恒成功）。两种情形 Abs(schemasDir) 均恒成功，
	// 原 err 分支为死代码，已删除。
	absSchemasDir, _ := filepath.Abs(schemasDir)
	if !strings.HasPrefix(absPath, absSchemasDir+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal detected: path is outside schemas directory")
	}
	return absPath, nil
}
