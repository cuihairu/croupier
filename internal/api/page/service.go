// Package page provides canonical PageSpec management API.
package page

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

const rendererSchemaVersion = "page-spec:1"

// 测试注入缝隙：默认值与生产行为完全一致，仅用于在测试中注入故障以
// 覆盖真实输入下不可达的防御分支（对齐 internal/policy/manager.go 惯例）。
var (
	currentUsername             = logicutils.CurrentUsername
	pageJSONMarshal             = json.Marshal
	validateSelectorFn          = spec.ValidateSelector
	validateOutputAssignmentsFn = spec.ValidateOutputAssignments
	buildPageSpecJSONFn         = buildPageSpecJSON
	pageSpecFromModelFn         = pageSpecFromModel
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}
func requireScope(ctx context.Context) (string, string, error) {
	scope := svc.GameScopeFromContext(ctx)
	gameID := strings.TrimSpace(scope.GameID)
	env := strings.TrimSpace(scope.Env)
	if gameID == "" {
		return "", "", errorx.NewBadRequest("X-Game-ID is required")
	}
	if env == "" {
		return "", "", errorx.NewBadRequest("X-Env is required")
	}
	return gameID, env, nil
}
func (s *Service) withPageTransaction(
	ctx context.Context,
	fn func(context.Context, *model.PageSpecModel, *model.PublishedPageSpecModel, *model.PageVersionModel) error,
) error {
	if s.svcCtx == nil || s.svcCtx.DB == nil {
		return errorx.NewInternalError("page service database is not initialized")
	}
	db := dbctx.Resolve(ctx, s.svcCtx.DB)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := dbctx.WithDB(ctx, tx)
		return fn(txCtx, model.NewPageSpecModel(tx), model.NewPublishedPageSpecModel(tx), model.NewPageVersionModel(tx))
	})
}
func (s *Service) normalizedFunctions(ctx context.Context) map[string]spec.FunctionSpec {
	gameID, env, err := requireScope(ctx)
	if err != nil || s == nil || s.svcCtx == nil || s.svcCtx.DB == nil {
		return map[string]spec.FunctionSpec{}
	}
	functions, err := contractsvc.FunctionSpecsByScope(ctx, model.NewFunctionContractModel(s.svcCtx.DB), gameID, env)
	if err != nil {
		return map[string]spec.FunctionSpec{}
	}
	return functions
}
func (s *Service) functionsForScope(ctx context.Context, gameID, env string) map[string]spec.FunctionSpec {
	if s == nil || s.svcCtx == nil || s.svcCtx.DB == nil {
		return map[string]spec.FunctionSpec{}
	}
	functions, err := contractsvc.FunctionSpecsByScope(ctx, model.NewFunctionContractModel(s.svcCtx.DB), gameID, env)
	if err != nil {
		return map[string]spec.FunctionSpec{}
	}
	return functions
}
func (s *Service) requirePageRead(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权查看页面", "admin:all", "pages:read", "pages:edit", "pages:publish", "pages:rollback")
	return err
}
func (s *Service) requirePageEdit(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权编辑页面", "admin:all", "pages:edit")
	return err
}
func (s *Service) requirePagePublish(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权发布页面", "admin:all", "pages:publish")
	return err
}
func (s *Service) requirePageRollback(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权回滚页面", "admin:all", "pages:rollback")
	return err
}
func (s *Service) auditPageEvent(ctx context.Context, eventType audit.AuditEventType, gameID, env, pageKey string, details map[string]interface{}) {
	if s == nil || s.svcCtx == nil || s.svcCtx.AuditService == nil {
		return
	}
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		actor = "unknown"
	}
	if details == nil {
		details = map[string]interface{}{}
	}
	details["game_id"] = gameID
	details["env"] = env
	details["page_key"] = pageKey

	_, err = s.svcCtx.AuditService.Log(ctx, eventType,
		audit.WithActorID(actor, "user", actor),
		audit.WithResource(audit.ResourceInfo{
			Type:        "page",
			ID:          pageKey,
			Name:        pageKey,
			GameID:      gameID,
			Environment: env,
		}),
		audit.WithDetails(details),
		audit.WithOutcome("success", ""),
	)
	if err != nil {
		s.svcCtx.AuditService.ReportWriteGap(ctx, eventType, err)
	}
}
