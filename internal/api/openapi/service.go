package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// GetSpec returns the OpenAPI spec for a function
func (s *Service) GetSpec(ctx context.Context, req *GetSpecRequest) (*GetSpecResponse, error) {
	spec, err := s.svcCtx.RegistryStore.GetOpenAPI(req.ID)
	if err != nil {
		if hasRegisteredFunction(s.svcCtx, req.ID) {
			spec = logicfunction.BuildFallbackOpenAPIOperation(req.ID)
		} else {
			return nil, err
		}
	}
	return &GetSpecResponse{
		Spec: spec,
	}, nil
}

// Import imports OpenAPI spec and creates functions
func (s *Service) Import(ctx context.Context, req *ImportRequest) (*ImportResponse, error) {
	// Parameter validation
	if req.Spec == nil {
		return nil, errorx.NewBadRequest("spec is required")
	}

	// Convert interface{} to OpenAPI document
	specBytes, err := json.Marshal(req.Spec)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal spec", "error", err)
		return nil, err
	}

	// Load OpenAPI document
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specBytes)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load OpenAPI spec", "error", err)
		return &ImportResponse{
			Imported: 0,
			Failed:   []string{err.Error()},
		}, nil
	}

	// Validate document
	normalizeOpenAPIDoc(doc)
	if err := doc.Validate(loader.Context); err != nil {
		slog.ErrorContext(ctx, "invalid OpenAPI spec", "error", err)
		return &ImportResponse{
			Imported: 0,
			Failed:   []string{err.Error()},
		}, nil
	}

	// Extract all operations and import to registry
	imported := 0
	failed := []string{}

	for path, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}

		// Process all HTTP method operations
		operations := map[string]*openapi3.Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"PATCH":  pathItem.Patch,
			"DELETE": pathItem.Delete,
		}

		for method, op := range operations {
			if op == nil {
				continue
			}

			// Generate function ID: {method}{path} (e.g., getUsers, createUser)
			funcID := method + path

			// Store to registry
			if err := s.svcCtx.RegistryStore.UpsertOpenAPI(funcID, op); err != nil {
				slog.ErrorContext(ctx, "failed to upsert operation", "funcID", funcID, "error", err)
				failed = append(failed, funcID+": "+err.Error())
			} else {
				imported++
			}
		}
	}

	slog.InfoContext(ctx, "OpenAPI import completed", "imported", imported, "failed", len(failed))

	return &ImportResponse{
		Imported: imported,
		Failed:   failed,
	}, nil
}

// EntityFunctions returns functions associated with an entity
func (s *Service) EntityFunctions(ctx context.Context, req *EntityFunctionsRequest) (*EntityFunctionsResponse, error) {
	// Parameter validation
	if req.ID == "" {
		return nil, errorx.NewBadRequest("entity ID is required")
	}

	entityType := strings.TrimSpace(req.ID)
	if s.svcCtx != nil && s.svcCtx.EntityModel != nil {
		if parsed, err := utils.ParseUintID(req.ID, "实体ID"); err == nil {
			entity, findErr := s.svcCtx.EntityModel.FindOne(ctx, parsed)
			if findErr == nil && entity != nil && strings.TrimSpace(entity.Type) != "" {
				entityType = strings.TrimSpace(entity.Type)
			}
		}
	}

	items := make([]EntityFunction, 0)
	operations := s.svcCtx.RegistryStore.ListOpenAPIOperations()
	for funcID, op := range operations {
		if op == nil {
			continue
		}
		if entityType != "" && !matchesEntity(op.Extensions["x-entity"], entityType, req.ID) {
			continue
		}
		operation := "custom"
		if opType, ok := op.Extensions["x-operation"].(string); ok && strings.TrimSpace(opType) != "" {
			operation = opType
		}
		name := strings.TrimSpace(op.Summary)
		if name == "" {
			name = funcID
		}
		items = append(items, EntityFunction{
			ID:        funcID,
			Operation: operation,
			Name:      name,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})

	return &EntityFunctionsResponse{
		Items: items,
	}, nil
}

func matchesEntity(raw interface{}, entityType, entityID string) bool {
	value, ok := raw.(string)
	if !ok {
		return false
	}
	value = strings.TrimSpace(value)
	return strings.EqualFold(value, strings.TrimSpace(entityType)) || strings.EqualFold(value, strings.TrimSpace(entityID))
}

// EntityIndex returns all entities derived from function registrations.
// Entities are grouped by x-entity extension, descriptor metadata, or inferred from function ID.
func (s *Service) EntityIndex(ctx context.Context, req *EntityIndexRequest) (*EntityIndexResponse, error) {
	type entityAcc struct {
		name       string
		category   string
		operations map[string]struct{}
		functions  []string
	}

	entities := map[string]*entityAcc{}
	seen := map[string]bool{}

	addFunction := func(funcID, entityName, category, operation string) {
		if entityName == "" {
			entityName = inferEntityFromID(funcID)
		}
		if entityName == "" {
			return
		}
		entityName = strings.ToLower(strings.TrimSpace(entityName))
		if operation == "" {
			operation = "custom"
		}

		acc := entities[entityName]
		if acc == nil {
			acc = &entityAcc{
				name:       entityName,
				operations: map[string]struct{}{},
			}
			entities[entityName] = acc
		}
		if category != "" && acc.category == "" {
			acc.category = category
		}
		acc.operations[operation] = struct{}{}
		acc.functions = append(acc.functions, funcID)
		seen[funcID] = true
	}

	// 1) Scan OpenAPI operations (runtime registry)
	if s.svcCtx != nil && s.svcCtx.RegistryStore != nil {
		operations := s.svcCtx.RegistryStore.ListOpenAPIOperations()
		for funcID, op := range operations {
			if op == nil {
				continue
			}
			entityName := ""
			if ext, ok := op.Extensions["x-entity"].(string); ok {
				entityName = strings.TrimSpace(ext)
			}
			category := ""
			if ext, ok := op.Extensions["x-category"].(string); ok {
				category = strings.TrimSpace(ext)
			}
			operation := ""
			if ext, ok := op.Extensions["x-operation"].(string); ok {
				operation = strings.TrimSpace(ext)
			}
			addFunction(funcID, entityName, category, operation)
		}
	}

	// 2) Scan descriptors from logic layer (DB + pack files)
	descLogic := logicfunction.NewDescriptorsLogic(ctx, s.svcCtx)
	descs, err := descLogic.Descriptors(&logicfunction.DescriptorsRequest{})
	if err == nil {
		for _, d := range descs {
			fid, _ := d["id"].(string)
			if fid == "" || seen[fid] {
				continue
			}
			entityName, _ := d["entity"].(string)
			category, _ := d["category"].(string)
			operation, _ := d["operation"].(string)
			addFunction(fid, entityName, category, operation)
		}
	}

	// Filter by category
	categoryFilter := strings.TrimSpace(req.Category)

	items := make([]EntityIndexItem, 0, len(entities))
	for _, acc := range entities {
		if categoryFilter != "" && !strings.EqualFold(acc.category, categoryFilter) {
			continue
		}
		ops := make([]string, 0, len(acc.operations))
		for op := range acc.operations {
			ops = append(ops, op)
		}
		sort.Strings(ops)
		sort.Strings(acc.functions)

		displayName := acc.name
		if len(displayName) > 0 {
			displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
		}

		items = append(items, EntityIndexItem{
			Name:          acc.name,
			DisplayName:   displayName,
			Category:      acc.category,
			Operations:    ops,
			Functions:     acc.functions,
			FunctionCount: len(acc.functions),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return &EntityIndexResponse{
		Items: items,
		Total: len(items),
	}, nil
}

func inferEntityFromID(funcID string) string {
	parts := strings.FieldsFunc(strings.ToLower(funcID), func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return ""
}

// EntityFunctionsByName returns functions for an entity by name (from entity index).
func (s *Service) EntityFunctionsByName(ctx context.Context, entityName string) (*EntityFunctionsResponse, error) {
	if entityName == "" {
		return nil, errorx.NewBadRequest("entity name is required")
	}

	entityName = strings.ToLower(strings.TrimSpace(entityName))
	items := make([]EntityFunction, 0)
	seen := map[string]bool{}

	// 1) Scan OpenAPI operations
	if s.svcCtx != nil && s.svcCtx.RegistryStore != nil {
		operations := s.svcCtx.RegistryStore.ListOpenAPIOperations()
		for funcID, op := range operations {
			if op == nil {
				continue
			}
			matched := matchesEntity(op.Extensions["x-entity"], entityName, entityName)
			if !matched {
				matched = strings.EqualFold(inferEntityFromID(funcID), entityName)
			}
			if !matched {
				continue
			}
			operation := "custom"
			if opType, ok := op.Extensions["x-operation"].(string); ok && strings.TrimSpace(opType) != "" {
				operation = opType
			}
			name := strings.TrimSpace(op.Summary)
			if name == "" {
				name = funcID
			}
			items = append(items, EntityFunction{ID: funcID, Operation: operation, Name: name})
			seen[funcID] = true
		}
	}

	// 2) Scan descriptors from logic layer
	descLogic := logicfunction.NewDescriptorsLogic(ctx, s.svcCtx)
	descs, err := descLogic.Descriptors(&logicfunction.DescriptorsRequest{})
	if err == nil {
		for _, d := range descs {
			fid, _ := d["id"].(string)
			if fid == "" || seen[fid] {
				continue
			}
			ent, _ := d["entity"].(string)
			if !strings.EqualFold(strings.TrimSpace(ent), entityName) &&
				!strings.EqualFold(inferEntityFromID(fid), entityName) {
				continue
			}
			operation, _ := d["operation"].(string)
			if operation == "" {
				operation = "custom"
			}
			name, _ := d["name"].(string)
			if name == "" {
				name = fid
			}
			items = append(items, EntityFunction{ID: fid, Operation: operation, Name: name})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})

	return &EntityFunctionsResponse{
		Items: items,
	}, nil
}

// GetDocument returns aggregated OpenAPI document
func (s *Service) GetDocument(ctx context.Context, req *GetDocumentRequest) (*GetDocumentResponse, error) {
	spec, err := s.svcCtx.RegistryStore.BuildOpenAPISpec()
	if err != nil {
		return nil, err
	}
	// Convert spec to map[string]interface{} for JSON response
	return &GetDocumentResponse{
		Spec: spec,
	}, nil
}

func (s *Service) BatchGetSpec(ctx context.Context, req *BatchGetSpecRequest) (BatchGetSpecResponse, error) {
	resp := make(BatchGetSpecResponse, len(req.FunctionIDs))
	for _, id := range req.FunctionIDs {
		functionID := strings.TrimSpace(id)
		if functionID == "" {
			continue
		}
		spec, err := s.svcCtx.RegistryStore.GetOpenAPI(functionID)
		if err != nil {
			if hasRegisteredFunction(s.svcCtx, functionID) {
				resp[functionID] = logicfunction.BuildFallbackOpenAPIOperation(functionID)
				continue
			}
			resp[functionID] = nil
			continue
		}
		resp[functionID] = spec
	}
	return resp, nil
}

func hasRegisteredFunction(svcCtx *svc.ServiceContext, functionID string) bool {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return false
	}
	if svcCtx == nil {
		return false
	}
	if svcCtx.RegistryStore != nil {
		svcCtx.RegistryStore.Mu().RLock()
		defer svcCtx.RegistryStore.Mu().RUnlock()
		for _, sess := range svcCtx.RegistryStore.AgentsUnsafe() {
			if sess == nil {
				continue
			}
			if _, ok := sess.Functions[functionID]; ok {
				return true
			}
		}
	}
	if svcCtx.FunctionModel != nil {
		if _, err := svcCtx.FunctionModel.FindByFunctionID(context.Background(), functionID); err == nil {
			return true
		}
	}
	return false
}

// normalizeOpenAPIDoc patches common non-critical gaps from external OpenAPI docs
// so import stays resilient (e.g. response description omitted by third-party generators).
func normalizeOpenAPIDoc(doc *openapi3.T) {
	if doc == nil || doc.Paths == nil {
		return
	}
	for _, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}
		operations := []*openapi3.Operation{
			pathItem.Get,
			pathItem.Post,
			pathItem.Put,
			pathItem.Patch,
			pathItem.Delete,
			pathItem.Options,
			pathItem.Head,
			pathItem.Trace,
		}
		for _, op := range operations {
			if op == nil || op.Responses == nil {
				continue
			}
			for statusCode, responseRef := range op.Responses.Map() {
				if responseRef == nil {
					continue
				}
				if responseRef.Value == nil {
					responseRef.Value = &openapi3.Response{}
				}
				if responseRef.Value.Description == nil || strings.TrimSpace(*responseRef.Value.Description) == "" {
					desc := fmt.Sprintf("Auto-generated response description for status %s", statusCode)
					responseRef.Value.Description = &desc
				}
			}
		}
	}
}
