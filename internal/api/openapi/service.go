package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
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

	// TODO: Get entity functions from Entity Manager
	// Currently return empty list, will implement after EntityManager integration
	slog.InfoContext(ctx, "Getting functions for entity", "id", req.ID)

	// Temporary implementation: Get all operations from registry store, then filter
	operations := s.svcCtx.RegistryStore.ListOpenAPIOperations()

	items := []EntityFunction{}
	for funcID, op := range operations {
		// Check operation extensions
		if op.Extensions != nil {
			if entityID, ok := op.Extensions["x-entity"].(string); ok {
				if entityID == req.ID {
					// Extract operation type
					operation := "custom"
					if opType, ok := op.Extensions["x-operation"].(string); ok {
						operation = opType
					}

					items = append(items, EntityFunction{
						ID:        funcID,
						Operation: operation,
						Name:      op.Summary,
					})
				}
			}
		}
	}

	slog.InfoContext(ctx, "Found functions for entity", "id", req.ID, "count", len(items))

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
