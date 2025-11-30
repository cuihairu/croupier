package xrender_schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/config"
)

type customSchemaDocument struct {
	ID        string
	Name      string
	Schema    interface{}
	UIConfig  interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

func resolveCustomSchemaDir(cfg config.Config) string {
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

func loadCustomSchema(cfg config.Config, id string) (*customSchemaDocument, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("schema ID 不能为空")
	}
	dir := resolveCustomSchemaDir(cfg)
	path := filepath.Join(dir, fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file struct {
		ID        string      `json:"id"`
		Name      string      `json:"name"`
		Schema    interface{} `json:"schema"`
		UIConfig  interface{} `json:"ui_config"`
		CreatedAt string      `json:"created_at"`
		UpdatedAt string      `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	createdAt, _ := time.Parse(time.RFC3339, file.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, file.UpdatedAt)
	return &customSchemaDocument{
		ID:        file.ID,
		Name:      file.Name,
		Schema:    file.Schema,
		UIConfig:  file.UIConfig,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}
