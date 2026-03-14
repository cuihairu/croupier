package schema

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns a list of schemas
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	docs, err := listSchemas(s.svcCtx.Config)
	if err != nil {
		return nil, err
	}

	items := make([]SchemaItem, 0, len(docs))
	for _, doc := range docs {
		items = append(items, doc.toItem())
	}

	return &ListResponse{
		Items: items,
		Total: len(items),
	}, nil
}

// Create creates a new schema
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errorx.NewBadRequest("Schema 名称不能为空")
	}

	if err := validateSchemaDefinition(req.Schema); err != nil {
		return nil, err
	}

	doc := &schemaDocument{
		ID:        ensureUniqueSchemaID(s.svcCtx.Config, name),
		Name:      name,
		Schema:    req.Schema,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := saveSchema(s.svcCtx.Config, doc); err != nil {
		return nil, err
	}

	resp := doc.toCreateResponse()
	return &resp, nil
}

// Get returns a schema by ID
func (s *Service) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	doc, err := loadSchema(s.svcCtx.Config, req.ID)
	if err != nil {
		return nil, err
	}

	resp := doc.toGetResponse()
	return &resp, nil
}

// Update updates a schema
func (s *Service) Update(ctx context.Context, req *UpdateRequest) (*UpdateResponse, error) {
	if err := validateSchemaDefinition(req.Schema); err != nil {
		return nil, err
	}

	doc, err := loadSchema(s.svcCtx.Config, req.ID)
	if err != nil {
		return nil, err
	}

	doc.Schema = req.Schema
	doc.UpdatedAt = time.Now()

	if err := saveSchema(s.svcCtx.Config, doc); err != nil {
		return nil, err
	}

	resp := doc.toUpdateResponse()
	return &resp, nil
}

// Delete deletes a schema
func (s *Service) Delete(ctx context.Context, req *DeleteRequest) error {
	if err := deleteSchemaByID(s.svcCtx.Config, req.ID); err != nil {
		return err
	}
	return nil
}

// Validate validates data against a schema
func (s *Service) Validate(ctx context.Context, req *ValidateRequest) (*ValidateResponse, error) {
	doc, err := loadSchema(s.svcCtx.Config, req.ID)
	if err != nil {
		return nil, err
	}

	valid, issues, err := validatePayloadAgainst(doc.Schema, req.Data)
	if err != nil {
		return nil, err
	}

	return &ValidateResponse{
		Valid:  valid,
		Errors: issues,
	}, nil
}

// RawValidate validates data against a raw schema
func (s *Service) RawValidate(ctx context.Context, req *RawValidateRequest) (*RawValidateResponse, error) {
	if err := validateSchemaDefinition(req.Schema); err != nil {
		return nil, err
	}

	valid, issues, err := validatePayloadAgainst(req.Schema, req.Data)
	if err != nil {
		return nil, err
	}

	return &RawValidateResponse{
		Valid:  valid,
		Errors: issues,
	}, nil
}

// GetUIConfig returns the UI config for a schema
func (s *Service) GetUIConfig(ctx context.Context, req *GetUIConfigRequest) (*GetUIConfigResponse, error) {
	doc, err := loadSchema(s.svcCtx.Config, req.ID)
	if err != nil {
		return nil, err
	}

	return &GetUIConfigResponse{
		ID:       doc.ID,
		UIConfig: doc.UIConfig,
	}, nil
}

// UpdateUIConfig updates the UI config for a schema
func (s *Service) UpdateUIConfig(ctx context.Context, req *UpdateUIConfigRequest) (*UpdateUIConfigResponse, error) {
	doc, err := loadSchema(s.svcCtx.Config, req.ID)
	if err != nil {
		return nil, err
	}

	doc.UIConfig = req.Config
	doc.UpdatedAt = time.Now()

	if err := saveSchema(s.svcCtx.Config, doc); err != nil {
		return nil, err
	}

	return &UpdateUIConfigResponse{
		ID:       doc.ID,
		UIConfig: doc.UIConfig,
	}, nil
}
