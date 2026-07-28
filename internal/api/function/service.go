package function

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// Function management methods

func (s *Service) FunctionsList(ctx context.Context, req *FunctionsListRequest) (*FunctionsListResponse, error) {
	return functionsList(ctx, s.svcCtx, req)
}

func (s *Service) FunctionsPending(ctx context.Context, req *FunctionsPendingRequest) (*FunctionsPendingResponse, error) {
	return functionsPending(ctx, s.svcCtx, req)
}

func (s *Service) FunctionDetail(ctx context.Context, req *FunctionDetailRequest) (*FunctionDetailResponse, error) {
	return functionDetail(ctx, s.svcCtx, req)
}

func (s *Service) FunctionAnalytics(ctx context.Context, req *FunctionAnalyticsRequest) (*FunctionAnalyticsResponse, error) {
	return functionAnalytics(ctx, s.svcCtx, req)
}

func (s *Service) FunctionCopy(ctx context.Context, req *FunctionCopyRequest) (*FunctionCopyResponse, error) {
	return functionCopy(ctx, s.svcCtx, req)
}

func (s *Service) FunctionDelete(ctx context.Context, req *FunctionDeleteRequest) error {
	return functionDelete(ctx, s.svcCtx, req)
}

func (s *Service) FunctionDisable(ctx context.Context, req *FunctionDisableRequest) error {
	return functionDisable(ctx, s.svcCtx, req)
}

func (s *Service) FunctionEnable(ctx context.Context, req *FunctionEnableRequest) error {
	return functionEnable(ctx, s.svcCtx, req)
}

func (s *Service) FunctionHistory(ctx context.Context, req *FunctionHistoryRequest) (*FunctionHistoryResponse, error) {
	return functionHistory(ctx, s.svcCtx, req)
}

func (s *Service) FunctionInvoke(ctx context.Context, req *FunctionInvokeRequest) (*FunctionInvokeResponse, error) {
	return functionInvoke(ctx, s.svcCtx, req)
}

func (s *Service) FunctionPublish(ctx context.Context, req *FunctionPublishRequest) (*FunctionPublishResponse, error) {
	return functionPublish(ctx, s.svcCtx, req)
}

// Instance management methods

func (s *Service) FunctionInstances(ctx context.Context, req *FunctionInstancesRequest) (*FunctionInstancesResponse, error) {
	return functionInstances(ctx, s.svcCtx, req)
}

func (s *Service) FunctionInstancesAll(ctx context.Context, req *FunctionInstancesAllRequest) (*FunctionInstancesAllResponse, error) {
	return functionInstancesAll(ctx, s.svcCtx, req)
}

// Permission management methods

func (s *Service) FunctionPermissions(ctx context.Context, req *FunctionPermissionsRequest) (*FunctionPermissionsResponse, error) {
	return functionPermissions(ctx, s.svcCtx, req)
}

func (s *Service) FunctionPermissionsUpdate(ctx context.Context, req *FunctionPermissionsUpdateRequest) error {
	return functionPermissionsUpdate(ctx, s.svcCtx, req)
}

// Function Form configuration methods

func (s *Service) FunctionForm(ctx context.Context, req *FunctionFormRequest) (*FunctionFormResponse, error) {
	return functionForm(ctx, s.svcCtx, req)
}

func (s *Service) FunctionFormUpdate(ctx context.Context, req *FunctionFormUpdateRequest) (*FunctionFormResponse, error) {
	return functionFormUpdate(ctx, s.svcCtx, req)
}

func (s *Service) FunctionFormHistory(ctx context.Context, req *FunctionFormHistoryRequest) (*FunctionFormHistoryResponse, error) {
	return functionFormHistory(ctx, s.svcCtx, req)
}

func (s *Service) FunctionFormRollback(ctx context.Context, req *FunctionFormRollbackRequest) (*FunctionFormRollbackResponse, error) {
	return functionFormRollback(ctx, s.svcCtx, req)
}

func (s *Service) FunctionWarnings(ctx context.Context, req *FunctionWarningsRequest) (*FunctionWarningsResponse, error) {
	return functionWarnings(ctx, s.svcCtx, req)
}

// Descriptors methods

func (s *Service) Descriptors(ctx context.Context, req *DescriptorsRequest) (*DescriptorsResponse, error) {
	return descriptors(ctx, s.svcCtx, req)
}

// Batch operations methods

func (s *Service) BatchCopyFunctions(ctx context.Context, req *BatchCopyFunctionsRequest) (*BatchCopyFunctionsResponse, error) {
	return batchCopyFunctions(ctx, s.svcCtx, req)
}

func (s *Service) BatchDeleteFunctions(ctx context.Context, req *BatchDeleteFunctionsRequest) (*BatchDeleteFunctionsResponse, error) {
	return batchDeleteFunctions(ctx, s.svcCtx, req)
}

func (s *Service) BatchUpdateFunctions(ctx context.Context, req *BatchUpdateFunctionsRequest) (*BatchUpdateFunctionsResponse, error) {
	return batchUpdateFunctions(ctx, s.svcCtx, req)
}

// Helper functions

func trimString(s string) string {
	return strings.TrimSpace(s)
}
