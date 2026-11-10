package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	funcopenapi "github.com/cuihairu/croupier/internal/function/openapi"
	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	"github.com/cuihairu/croupier/internal/model"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"
)

const maxOpenAPISourceBytes = 2 << 20

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
		if hasRegisteredFunction(ctx, s.svcCtx, req.ID) {
			spec = logicfunction.BuildFallbackOpenAPIOperation(req.ID)
		} else {
			return nil, err
		}
	}
	return &GetSpecResponse{Spec: mustMarshalRaw(spec)}, nil
}

func (s *Service) CreateSource(ctx context.Context, req *OpenAPISourceCreateRequest) (*OpenAPISourceGetResponse, error) {
	if err := s.requireSourceWrite(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	ctx, finishSpan := s.startSourceSpan(ctx, "openapi.source.create", gameID, env,
		attribute.String("openapi_source.name", strings.TrimSpace(req.Name)),
	)
	var spanErr error
	defer func() { finishSpan(spanErr) }()
	name := strings.TrimSpace(req.Name)
	parsed, format, err := s.parseValidSource(req.Spec)
	if err != nil {
		spanErr = err
		return nil, err
	}
	if name == "" {
		name = firstNonEmpty(parsed.InfoTitle, "OpenAPI Source")
	}
	modelSource := &model.OpenAPISource{
		GameID:         gameID,
		Env:            env,
		SourceID:       uuid.NewString(),
		Name:           name,
		Revision:       1,
		Format:         format,
		OpenAPIVersion: parsed.OpenAPIVersion,
		InfoTitle:      parsed.InfoTitle,
		InfoVersion:    parsed.InfoVersion,
		ContentHash:    sha256Hex(parsed.Spec),
	}
	modelSource.SetSpec(parsed.Spec)
	// 设计债清理：parsed.Operations（[]OpenAPISourceOperation）与
	// parsed.Diagnostics（[]spec.Diagnostic）均为纯 string/bool 字段的
	// JSON 可序列化结构，SetOperations/SetDiagnostics 内部的 json.Marshal
	// 恒成功，原 err 分支不可达已删除（UpdateSource 同）。
	_ = modelSource.SetOperations(parsed.Operations)
	_ = modelSource.SetDiagnostics(parsed.Diagnostics)
	// T4/D4：源落库与 unbound 契约生成同事务——上传即成页的契约侧入口，
	// 无运行时函数的 operation 直接成为物料（不再要求前置注册/绑定）。
	// T5：事务前启动摘要基线快照，事务提交后收尾模板/提案联动并产出摘要。
	tracker, err := s.startUploadPipelineTracker(ctx, gameID, env, parsed)
	if err != nil {
		spanErr = err
		return nil, err
	}
	var created []dashboardservice.FunctionMetaInput
	if err := s.scopedTransaction(ctx, func(txCtx context.Context) error {
		if err := s.svcCtx.OpenAPISourceModel.Create(txCtx, modelSource); err != nil {
			return err
		}
		var err error
		created, err = s.createUnboundContractsForSource(txCtx, gameID, env, modelSource, parsed.Operations, nil)
		return err
	}); err != nil {
		spanErr = err
		return nil, err
	}
	summary, err := s.finishUploadPipeline(ctx, gameID, env, modelSource.SourceID, tracker, created)
	if err != nil {
		spanErr = err
		return nil, err
	}
	finishSpan(nil,
		attribute.String("openapi_source.id", modelSource.SourceID),
		attribute.Int("openapi_source.revision", modelSource.Revision),
		attribute.String("openapi_source.format", modelSource.Format),
		attribute.Int("openapi_source.operation_count", len(parsed.Operations)),
		attribute.Int("openapi_source.diagnostic_count", len(parsed.Diagnostics)),
		attribute.Int("openapi_source.contracts_created", summary.ContractsCreated),
		attribute.Int("openapi_source.templates_updated", summary.TemplatesUpdated),
		attribute.Int("openapi_source.proposals_created", summary.ProposalsCreated),
	)
	finishSpan = func(error, ...attribute.KeyValue) {}
	s.auditSourceEvent(ctx, audit.EventOpenAPISourceCreate, gameID, env, modelSource.SourceID, modelSource.Name, map[string]interface{}{
		"revision":          modelSource.Revision,
		"format":            modelSource.Format,
		"openapi_version":   modelSource.OpenAPIVersion,
		"operation_count":   len(parsed.Operations),
		"diagnostic_count":  len(parsed.Diagnostics),
		"content_hash":      modelSource.ContentHash,
		"contracts_created": summary.ContractsCreated,
		"templates_updated": summary.TemplatesUpdated,
		"proposals_created": summary.ProposalsCreated,
	})
	return &OpenAPISourceGetResponse{Source: sourceDetailFromModel(modelSource, nil), Summary: summary}, nil
}

func (s *Service) UpdateSource(ctx context.Context, req *OpenAPISourceUpdateRequest) (*OpenAPISourceGetResponse, error) {
	if err := s.requireSourceWrite(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	source, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, strings.TrimSpace(req.SourceID))
	if err != nil {
		return nil, errorx.NewNotFound("OpenAPI source not found")
	}
	ctx, finishSpan := s.startSourceSpan(ctx, "openapi.source.update", gameID, env,
		attribute.String("openapi_source.id", source.SourceID),
		attribute.Int("openapi_source.previous_revision", source.Revision),
	)
	var spanErr error
	defer func() { finishSpan(spanErr) }()
	parsed, format, err := s.parseValidSource(req.Spec)
	if err != nil {
		spanErr = err
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = firstNonEmpty(parsed.InfoTitle, source.Name, "OpenAPI Source")
	}
	previousRevision := source.Revision
	source.Name = name
	source.Revision++
	source.Format = format
	source.OpenAPIVersion = parsed.OpenAPIVersion
	source.InfoTitle = parsed.InfoTitle
	source.InfoVersion = parsed.InfoVersion
	source.ContentHash = sha256Hex(parsed.Spec)
	source.SetSpec(parsed.Spec)
	// 设计债清理：见 CreateSource 同注，Marshal 恒成功。
	_ = source.SetOperations(parsed.Operations)
	_ = source.SetDiagnostics(parsed.Diagnostics)
	bindings, err := s.svcCtx.OpenAPISourceBindingModel.ListBySource(ctx, gameID, env, source.SourceID)
	if err != nil {
		spanErr = err
		return nil, err
	}
	// T4/D4：源更新与 unbound 契约生成同事务（既有 provider 绑定的重建
	// 路径保持原时序，在事务提交后执行）。T5：事务前启动摘要基线快照，
	// 事务提交后收尾模板/提案联动并产出摘要（快照覆盖 unbound 与 bound
	// 两条路径的全部产出）。
	tracker, err := s.startUploadPipelineTracker(ctx, gameID, env, parsed)
	if err != nil {
		spanErr = err
		return nil, err
	}
	var created []dashboardservice.FunctionMetaInput
	if err := s.scopedTransaction(ctx, func(txCtx context.Context) error {
		if err := s.svcCtx.OpenAPISourceModel.Update(txCtx, source); err != nil {
			return err
		}
		var err error
		created, err = s.createUnboundContractsForSource(txCtx, gameID, env, source, parsed.Operations, bindings)
		return err
	}); err != nil {
		spanErr = err
		return nil, err
	}
	if err := s.rebuildContractsForSourceBindings(ctx, gameID, env, source, parsed.Operations, bindings); err != nil {
		spanErr = err
		return nil, err
	}
	summary, err := s.finishUploadPipeline(ctx, gameID, env, source.SourceID, tracker, created)
	if err != nil {
		spanErr = err
		return nil, err
	}
	finishSpan(nil,
		attribute.Int("openapi_source.revision", source.Revision),
		attribute.String("openapi_source.format", source.Format),
		attribute.Int("openapi_source.operation_count", len(parsed.Operations)),
		attribute.Int("openapi_source.diagnostic_count", len(parsed.Diagnostics)),
		attribute.Int("openapi_source.contracts_created", summary.ContractsCreated),
		attribute.Int("openapi_source.templates_updated", summary.TemplatesUpdated),
		attribute.Int("openapi_source.proposals_created", summary.ProposalsCreated),
	)
	finishSpan = func(error, ...attribute.KeyValue) {}
	s.auditSourceEvent(ctx, audit.EventOpenAPISourceUpdate, gameID, env, source.SourceID, source.Name, map[string]interface{}{
		"previous_revision": previousRevision,
		"revision":          source.Revision,
		"format":            source.Format,
		"openapi_version":   source.OpenAPIVersion,
		"operation_count":   len(parsed.Operations),
		"diagnostic_count":  len(parsed.Diagnostics),
		"content_hash":      source.ContentHash,
		"contracts_created": summary.ContractsCreated,
		"templates_updated": summary.TemplatesUpdated,
		"proposals_created": summary.ProposalsCreated,
	})
	return &OpenAPISourceGetResponse{Source: sourceDetailFromModel(source, bindings), Summary: summary}, nil
}

func (s *Service) CreateSourceFromMultipart(ctx context.Context, name string, file multipart.File) (*OpenAPISourceGetResponse, error) {
	if file == nil {
		return nil, errorx.NewBadRequest("file is required")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxOpenAPISourceBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxOpenAPISourceBytes {
		return nil, errorx.NewBadRequest("OpenAPI source exceeds 2 MiB limit")
	}
	return s.CreateSource(ctx, &OpenAPISourceCreateRequest{
		Name: name,
		Spec: json.RawMessage(data),
	})
}

func (s *Service) ListSources(ctx context.Context, req *OpenAPISourceListRequest) (*OpenAPISourceListResponse, error) {
	if err := s.requireSourceRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.svcCtx.OpenAPISourceModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	resp := make([]OpenAPISourceSummary, 0, len(items))
	for i := range items {
		resp = append(resp, sourceSummaryFromModel(&items[i]))
	}
	return &OpenAPISourceListResponse{Items: resp}, nil
}

// RuntimeSources 列出当前 scope 下的运行时导入：每个 Agent 侧 openapi
// provider 会话一条，标注来源 Agent、函数清单与最近心跳。前端
// OpenAPI Sources 页的「运行时导入」区块与绑定弹窗的运行时函数候选
// 都以此为准（agentID 标注来源，避免与控制台上传的 Source 混淆）。
func (s *Service) RuntimeSources(ctx context.Context, _ *RuntimeSourcesListRequest) (*RuntimeSourcesListResponse, error) {
	if err := s.requireSourceRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]RuntimeProviderItem, 0, 4)
	if s.svcCtx != nil && s.svcCtx.RegistryStore != nil {
		for _, snapshot := range s.svcCtx.RegistryStore.ProviderSessionSnapshots() {
			if strings.TrimSpace(snapshot.GameID) != gameID || strings.TrimSpace(snapshot.Env) != env {
				continue
			}
			name := strings.TrimPrefix(snapshot.ProviderID, "provider:")
			if name == snapshot.ProviderID {
				// 只展示 openapi provider 会话，普通 agent 不在此列
				continue
			}
			functions := append([]string(nil), snapshot.FunctionIDs...)
			sort.Strings(functions)
			// #27②：导入时间/最新版本做服务端归一，前端无需再判零值/空串
			// （内存快照的历史字段在会话过期/重启后自然缺失）。
			firstSeen := snapshot.FirstSeenUnix
			if firstSeen <= 0 {
				firstSeen = snapshot.LastSeenUnix
			}
			latest := snapshot.VersionHWM
			if latest == "" {
				latest = snapshot.Version
			}
			items = append(items, RuntimeProviderItem{
				ProviderID:    snapshot.ProviderID,
				Name:          name,
				AgentID:       snapshot.AgentID,
				GameID:        snapshot.GameID,
				Env:           snapshot.Env,
				Version:       snapshot.Version,
				Metadata:      snapshot.Metadata,
				ServiceAddr:   snapshot.Addr,
				FirstSeenUnix: firstSeen,
				LatestVersion: latest,
				FunctionCount: len(functions),
				Functions:     functions,
				LastSeenUnix:  snapshot.LastSeenUnix,
			})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ProviderID < items[j].ProviderID })
	return &RuntimeSourcesListResponse{Items: items, Total: len(items)}, nil
}

func (s *Service) GetSource(ctx context.Context, req *OpenAPISourceGetRequest) (*OpenAPISourceGetResponse, error) {
	if err := s.requireSourceRead(ctx); err != nil {
		return nil, err
	}
	source, bindings, err := s.loadSourceWithBindings(ctx, req.SourceID)
	if err != nil {
		return nil, err
	}
	return &OpenAPISourceGetResponse{Source: sourceDetailFromModel(source, bindings)}, nil
}

func (s *Service) SourceDiagnostics(ctx context.Context, req *OpenAPISourceGetRequest) (*OpenAPISourceDiagnosticsResponse, error) {
	if err := s.requireSourceRead(ctx); err != nil {
		return nil, err
	}
	source, _, err := s.loadSourceWithBindings(ctx, req.SourceID)
	if err != nil {
		return nil, err
	}
	var diags []spec.Diagnostic
	if err := source.GetDiagnostics(&diags); err != nil {
		return nil, err
	}
	return &OpenAPISourceDiagnosticsResponse{
		SourceID:    source.SourceID,
		Diagnostics: diags,
	}, nil
}

func (s *Service) CreateBinding(ctx context.Context, req *OpenAPISourceBindingCreateRequest) (*OpenAPISourceBindingResponse, error) {
	if err := s.requireSourceWrite(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	ctx, finishSpan := s.startSourceSpan(ctx, "openapi.source.binding.create", gameID, env,
		attribute.String("openapi_source.id", strings.TrimSpace(req.SourceID)),
		attribute.String("openapi_source.operation_id", strings.TrimSpace(req.OperationID)),
		attribute.String("openapi_source.binding_id", strings.TrimSpace(req.BindingID)),
		attribute.String("function.id", strings.TrimSpace(req.FunctionID)),
		attribute.String("provider.id", strings.TrimSpace(req.ProviderID)),
	)
	var spanErr error
	defer func() { finishSpan(spanErr) }()
	source, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, strings.TrimSpace(req.SourceID))
	if err != nil {
		spanErr = err
		return nil, errorx.NewNotFound("OpenAPI source not found")
	}
	var operations []OpenAPISourceOperation
	if err := source.GetOperations(&operations); err != nil {
		spanErr = err
		return nil, err
	}
	operation, ok := findSourceOperation(operations, req.OperationID)
	if !ok {
		err := errorx.NewBadRequest("operationId is not part of this OpenAPI source")
		spanErr = err
		return nil, err
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		err := errorx.NewBadRequest("kind is required")
		spanErr = err
		return nil, err
	}
	if kind != "provider" {
		err := errorx.NewBadRequest("only provider execution binding is enabled; httpConnector requires allowlist and SecretRef policy")
		spanErr = err
		return nil, err
	}
	functionID := strings.TrimSpace(req.FunctionID)
	if functionID == "" {
		err := errorx.NewBadRequest("functionId is required for provider binding")
		spanErr = err
		return nil, err
	}
	runtimeMeta, ok := registeredFunctionMetaInScope(ctx, s.svcCtx, gameID, env, functionID)
	if !ok {
		err := errorx.NewBadRequest("functionId is not registered in current game/env runtime")
		spanErr = err
		return nil, err
	}
	bindingID := strings.TrimSpace(req.BindingID)
	if bindingID == "" {
		bindingID = sanitizeBindingID(req.OperationID)
	}
	binding := &model.OpenAPISourceBinding{
		GameID:      gameID,
		Env:         env,
		SourceID:    source.SourceID,
		BindingID:   bindingID,
		OperationID: strings.TrimSpace(req.OperationID),
		Kind:        kind,
		FunctionID:  functionID,
		ProviderID:  strings.TrimSpace(req.ProviderID),
	}
	if err := s.scopedTransaction(ctx, func(txCtx context.Context) error {
		if err := s.svcCtx.OpenAPISourceBindingModel.Upsert(txCtx, binding); err != nil {
			return err
		}
		// 不同名绑定取代 operationId 名下的 unbound 物料：绑定时即清理
		// （与上传重放的 removeSupersededUnboundContract 对称），否则资源
		// 语义槽位出现同源双候选 → unresolved conflict → proposal 被
		// 降级 needs_review。同名绑定走 T6 原地翻转，原行保留不删。
		supersededID := unboundFunctionID(funcopenapi.DeriveFunctionID(
			openAPIOperationFromSource(source, req.OperationID), operation.Path))
		if supersededID != "" && supersededID != functionID {
			if err := s.removeSupersededUnboundContract(
				txCtx, dashboardservice.NewContractService(s.svcCtx.DB), gameID, env, supersededID); err != nil {
				return err
			}
		}
		return s.rebuildContractForSourceBinding(txCtx, gameID, env, source, operation, binding, runtimeMeta)
	}); err != nil {
		spanErr = err
		return nil, err
	}
	// T2 收口：绑定契约重建在事务内完成，模板联动在提交后单次执行
	// （事务内经全局连接写模板表在文件型 sqlite 下会与事务写锁互等待
	// 自死锁，见 service.RegenerateContractTemplates 注）。失败仅告警。
	if err := dashboardservice.RegenerateContractTemplates(ctx, gameID, env); err != nil {
		slog.WarnContext(ctx, "auto regenerate component templates after binding create failed",
			"game_id", gameID, "env", env, "function_id", functionID, "error", err)
	}
	finishSpan(nil,
		attribute.String("openapi_source.id", source.SourceID),
		attribute.Int("openapi_source.revision", source.Revision),
		attribute.String("openapi_source.binding_id", binding.BindingID),
	)
	finishSpan = func(error, ...attribute.KeyValue) {}
	s.auditSourceEvent(ctx, audit.EventOpenAPISourceBindingCreate, gameID, env, source.SourceID, source.Name, map[string]interface{}{
		"binding_id":   binding.BindingID,
		"operation_id": binding.OperationID,
		"kind":         binding.Kind,
		"function_id":  binding.FunctionID,
		"provider_id":  binding.ProviderID,
		"revision":     source.Revision,
	})
	return &OpenAPISourceBindingResponse{
		Binding:  bindingDTOFromModel(*binding),
		Proposal: s.generatedProposalForBinding(ctx, gameID, env, functionID),
	}, nil
}

func (s *Service) DeleteBinding(ctx context.Context, req *OpenAPISourceBindingDeleteRequest) (*OpenAPISourceBindingResponse, error) {
	if err := s.requireSourceWrite(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	ctx, finishSpan := s.startSourceSpan(ctx, "openapi.source.binding.delete", gameID, env,
		attribute.String("openapi_source.id", strings.TrimSpace(req.SourceID)),
		attribute.String("openapi_source.binding_id", strings.TrimSpace(req.BindingID)),
	)
	var spanErr error
	defer func() { finishSpan(spanErr) }()
	source, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, strings.TrimSpace(req.SourceID))
	if err != nil {
		spanErr = err
		return nil, errorx.NewNotFound("OpenAPI source not found")
	}
	var binding *model.OpenAPISourceBinding
	if err := s.scopedTransaction(ctx, func(txCtx context.Context) error {
		var err error
		binding, err = s.svcCtx.OpenAPISourceBindingModel.FindByScopeSourceAndBindingID(txCtx, gameID, env, source.SourceID, strings.TrimSpace(req.BindingID))
		if err != nil {
			return err
		}
		if err := s.svcCtx.OpenAPISourceBindingModel.Delete(txCtx, gameID, env, source.SourceID, binding.BindingID); err != nil {
			return err
		}
		return s.reconcileContractAfterBindingDelete(txCtx, gameID, env, binding)
	}); err != nil {
		spanErr = err
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("OpenAPI source binding not found")
		}
		return nil, err
	}
	// T2 收口：解绑的契约重建/删除在事务内完成，模板联动在提交后单次执行
	// （同 CreateBinding 注：事务内写模板表会与事务写锁互等待自死锁）。
	if err := dashboardservice.RegenerateContractTemplates(ctx, gameID, env); err != nil {
		slog.WarnContext(ctx, "auto regenerate component templates after binding delete failed",
			"game_id", gameID, "env", env, "function_id", binding.FunctionID, "error", err)
	}
	finishSpan(nil, attribute.Int("openapi_source.revision", source.Revision))
	finishSpan = func(error, ...attribute.KeyValue) {}
	s.auditSourceEvent(ctx, audit.EventOpenAPISourceBindingDelete, gameID, env, source.SourceID, source.Name, map[string]interface{}{
		"binding_id": binding.BindingID,
		"revision":   source.Revision,
	})
	return &OpenAPISourceBindingResponse{Binding: OpenAPISourceBindingDTO{
		BindingID: binding.BindingID,
	}}, nil
}

func (s *Service) scopedTransaction(ctx context.Context, work func(context.Context) error) error {
	if s == nil || s.svcCtx == nil || s.svcCtx.DB == nil {
		return errorx.NewInternalError("OpenAPI source database is not initialized")
	}
	db := dbctx.Resolve(ctx, s.svcCtx.DB)
	return db.Transaction(func(tx *gorm.DB) error {
		return work(dbctx.WithDB(ctx, tx))
	})
}

func (s *Service) loadSourceWithBindings(ctx context.Context, sourceID string) (*model.OpenAPISource, []model.OpenAPISourceBinding, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, nil, err
	}
	source, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, strings.TrimSpace(sourceID))
	if err != nil {
		return nil, nil, errorx.NewNotFound("OpenAPI source not found")
	}
	bindings, err := s.svcCtx.OpenAPISourceBindingModel.ListBySource(ctx, gameID, env, source.SourceID)
	if err != nil {
		return nil, nil, err
	}
	return source, bindings, nil
}

// GetDocument returns aggregated OpenAPI document
// 设计债清理：RegistryStore 为具体 *reg.Store，其 BuildOpenAPISpec 是纯
// 组装（唯一出口无出错路径），GetDocument 无 error 出路，error 返回值已随
// 签名收紧删除（handler 侧 err 分支随之简化）。
func (s *Service) GetDocument(ctx context.Context, req *GetDocumentRequest) *GetDocumentResponse {
	// BuildOpenAPISpec（registry.Store）为纯组装，唯一出口 return doc, nil，
	// 恒不报错；其签名在本包外暂保留 error，此处显式丢弃。
	spec, _ := s.svcCtx.RegistryStore.BuildOpenAPISpec()
	return &GetDocumentResponse{
		Spec: mustMarshalRaw(spec),
	}
}

// BatchGetSpec 批量取函数 OpenAPI spec。
// 设计债清理：失败以「未注册则 nil / 已注册则回退 operation」承载，无 error
// 出路，error 返回值已随签名收紧删除。
func (s *Service) BatchGetSpec(ctx context.Context, req *BatchGetSpecRequest) BatchGetSpecResponse {
	resp := make(BatchGetSpecResponse, len(req.FunctionIDs))
	for _, id := range req.FunctionIDs {
		functionID := strings.TrimSpace(id)
		if functionID == "" {
			continue
		}
		spec, err := s.svcCtx.RegistryStore.GetOpenAPI(functionID)
		if err != nil {
			if hasRegisteredFunction(ctx, s.svcCtx, functionID) {
				resp[functionID] = mustMarshalRaw(logicfunction.BuildFallbackOpenAPIOperation(functionID))
				continue
			}
			resp[functionID] = nil
			continue
		}
		resp[functionID] = mustMarshalRaw(spec)
	}
	return resp
}
