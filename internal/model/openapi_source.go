package model

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// OpenAPISource stores an uploaded OpenAPI contract source. It is scoped by
// game/environment and is not a runtime executable function by itself.
type OpenAPISource struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	GameID          string         `gorm:"size:64;not null;default:'';uniqueIndex:uidx_openapi_sources_scope_id,priority:1;index:idx_openapi_sources_scope,priority:1" json:"gameId"`
	Env             string         `gorm:"size:64;not null;default:'';uniqueIndex:uidx_openapi_sources_scope_id,priority:2;index:idx_openapi_sources_scope,priority:2" json:"env"`
	SourceID        string         `gorm:"size:64;not null;uniqueIndex:uidx_openapi_sources_scope_id,priority:3" json:"sourceId"`
	Name            string         `gorm:"size:128;not null" json:"name"`
	Revision        int            `gorm:"default:1" json:"revision"`
	Format          string         `gorm:"size:16;not null" json:"format"`
	OpenAPIVersion  string         `gorm:"size:32;not null" json:"openapiVersion"`
	InfoTitle       string         `gorm:"size:256" json:"infoTitle,omitempty"`
	InfoVersion     string         `gorm:"size:64" json:"infoVersion,omitempty"`
	ContentHash     string         `gorm:"size:64;not null;index" json:"contentHash"`
	SpecJSON        string         `gorm:"type:json" json:"-"`
	OperationsJSON  string         `gorm:"type:json" json:"-"`
	DiagnosticsJSON string         `gorm:"type:json" json:"-"`
}

func (OpenAPISource) TableName() string {
	return "openapi_sources"
}

// OpenAPISourceBinding explicitly connects one source operation to an executor.
type OpenAPISourceBinding struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	GameID      string         `gorm:"size:64;not null;default:'';uniqueIndex:uidx_openapi_source_bindings_scope_id,priority:1;index:idx_openapi_source_bindings_scope,priority:1" json:"gameId"`
	Env         string         `gorm:"size:64;not null;default:'';uniqueIndex:uidx_openapi_source_bindings_scope_id,priority:2;index:idx_openapi_source_bindings_scope,priority:2" json:"env"`
	SourceID    string         `gorm:"size:64;not null;uniqueIndex:uidx_openapi_source_bindings_scope_id,priority:3;index" json:"sourceId"`
	BindingID   string         `gorm:"size:128;not null;uniqueIndex:uidx_openapi_source_bindings_scope_id,priority:4" json:"bindingId"`
	OperationID string         `gorm:"size:128;not null;index" json:"operationId"`
	Kind        string         `gorm:"size:32;not null" json:"kind"`
	FunctionID  string         `gorm:"size:128;index" json:"functionId,omitempty"`
	ProviderID  string         `gorm:"size:128;index" json:"providerId,omitempty"`
}

func (OpenAPISourceBinding) TableName() string {
	return "openapi_source_bindings"
}

// OpenAPISourceModel provides scoped access to uploaded OpenAPI sources.
type OpenAPISourceModel struct {
	db *gorm.DB
}

func NewOpenAPISourceModel(db *gorm.DB) *OpenAPISourceModel {
	return &OpenAPISourceModel{db: db}
}

func (m *OpenAPISourceModel) Create(ctx context.Context, source *OpenAPISource) error {
	return m.db.WithContext(ctx).Create(source).Error
}

func (m *OpenAPISourceModel) ListByScope(ctx context.Context, gameID, env string) ([]OpenAPISource, error) {
	var items []OpenAPISource
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("updated_at DESC, source_id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (m *OpenAPISourceModel) FindByScopeAndSourceID(ctx context.Context, gameID, env, sourceID string) (*OpenAPISource, error) {
	var source OpenAPISource
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND source_id = ?", gameID, env, sourceID).
		First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

func (s *OpenAPISource) GetSpec() json.RawMessage {
	return json.RawMessage(s.SpecJSON)
}

func (s *OpenAPISource) SetSpec(spec json.RawMessage) {
	s.SpecJSON = string(spec)
}

func (s *OpenAPISource) GetOperations(out interface{}) error {
	if s.OperationsJSON == "" {
		return nil
	}
	return json.Unmarshal([]byte(s.OperationsJSON), out)
}

func (s *OpenAPISource) SetOperations(operations interface{}) error {
	data, err := json.Marshal(operations)
	if err != nil {
		return err
	}
	s.OperationsJSON = string(data)
	return nil
}

func (s *OpenAPISource) GetDiagnostics(out interface{}) error {
	if s.DiagnosticsJSON == "" {
		return nil
	}
	return json.Unmarshal([]byte(s.DiagnosticsJSON), out)
}

func (s *OpenAPISource) SetDiagnostics(diagnostics interface{}) error {
	data, err := json.Marshal(diagnostics)
	if err != nil {
		return err
	}
	s.DiagnosticsJSON = string(data)
	return nil
}

// OpenAPISourceBindingModel provides scoped access to execution bindings.
type OpenAPISourceBindingModel struct {
	db *gorm.DB
}

func NewOpenAPISourceBindingModel(db *gorm.DB) *OpenAPISourceBindingModel {
	return &OpenAPISourceBindingModel{db: db}
}

func (m *OpenAPISourceBindingModel) Upsert(ctx context.Context, binding *OpenAPISourceBinding) error {
	var existing OpenAPISourceBinding
	err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND source_id = ? AND binding_id = ?",
			binding.GameID, binding.Env, binding.SourceID, binding.BindingID).
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return m.db.WithContext(ctx).Create(binding).Error
	}
	if err != nil {
		return err
	}
	binding.ID = existing.ID
	binding.CreatedAt = existing.CreatedAt
	return m.db.WithContext(ctx).Save(binding).Error
}

func (m *OpenAPISourceBindingModel) ListBySource(ctx context.Context, gameID, env, sourceID string) ([]OpenAPISourceBinding, error) {
	var items []OpenAPISourceBinding
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND source_id = ?", gameID, env, sourceID).
		Order("operation_id ASC, binding_id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (m *OpenAPISourceBindingModel) Delete(ctx context.Context, gameID, env, sourceID, bindingID string) error {
	return m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND source_id = ? AND binding_id = ?", gameID, env, sourceID, bindingID).
		Delete(&OpenAPISourceBinding{}).Error
}
