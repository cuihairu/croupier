package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

func wireDashboardRegistrationPipeline(svcCtx *svc.ServiceContext) {
	if svcCtx == nil || svcCtx.RegistryStore == nil || svcCtx.DB == nil {
		return
	}
	svcCtx.RegistryStore.SetContractService(&registrationContractPipeline{svcCtx: svcCtx})
}

type registrationContractPipeline struct {
	svcCtx *svc.ServiceContext
}

func (p *registrationContractPipeline) RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta spec.FunctionContractInput) error {
	contractSvc, err := p.contractService(ctx, gameID, env)
	if err != nil {
		return err
	}
	return contractSvc.RebuildContractFromFunctionMeta(ctx, gameID, env, source, meta)
}

func (p *registrationContractPipeline) RemoveFunctionContract(ctx context.Context, gameID, env, functionID string) (string, error) {
	contractSvc, err := p.contractService(ctx, gameID, env)
	if err != nil {
		return "", err
	}
	return contractSvc.RemoveFunctionContract(ctx, gameID, env, functionID)
}

func (p *registrationContractPipeline) RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error {
	contractSvc, err := p.contractService(ctx, gameID, env)
	if err != nil {
		return err
	}
	return contractSvc.RebuildResourceCapability(ctx, gameID, env, resourceKey)
}

func (p *registrationContractPipeline) RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error {
	contractSvc, err := p.contractService(ctx, gameID, env)
	if err != nil {
		return err
	}
	return contractSvc.RebuildProposalsForResource(ctx, gameID, env, resourceKey)
}

func (p *registrationContractPipeline) RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error {
	contractSvc, err := p.contractService(ctx, gameID, env)
	if err != nil {
		return err
	}
	return contractSvc.RebuildProposalForFunction(ctx, gameID, env, functionID)
}

// RegenerateContractTemplates 注册事务提交后的组件模板收口（T2）。模板表
// 在全局/meta 库，走进程级注入闭包（与组件 handler regenerate 同一实现），
// 不经 per-game 契约服务解析。
func (p *registrationContractPipeline) RegenerateContractTemplates(ctx context.Context, gameID, env string) error {
	return dashboardservice.RegenerateContractTemplates(ctx, gameID, env)
}

func (p *registrationContractPipeline) contractService(ctx context.Context, gameID, env string) (*dashboardservice.ContractService, error) {
	db, err := p.scopedDB(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	return dashboardservice.NewContractService(db), nil
}

func (p *registrationContractPipeline) scopedDB(ctx context.Context, gameID, env string) (*gorm.DB, error) {
	if p == nil || p.svcCtx == nil || p.svcCtx.DB == nil {
		return nil, fmt.Errorf("dashboard registration pipeline is not initialized")
	}
	// Store materialization may already be inside a scoped game transaction.
	// Preserve that transaction instead of re-resolving the base game DB.
	if scoped := dbctx.Get(ctx); scoped != nil {
		return scoped, nil
	}
	if p.svcCtx.Router == nil {
		return p.svcCtx.DB, nil
	}
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if gameID == "" || env == "" {
		return nil, fmt.Errorf("game_id and env are required for dashboard registration in database-per-game mode")
	}
	if p.svcCtx.GameModel != nil {
		dbName, err := p.svcCtx.GameModel.LookupDatabaseName(ctx, gameID, env)
		if err != nil {
			return nil, fmt.Errorf("lookup game database binding: %w", err)
		}
		if dbName == "" {
			return nil, fmt.Errorf("game scope not found: game_id=%s env=%s", gameID, env)
		}
	}
	db, err := p.svcCtx.Router.GameDB(ctx, gameID, env)
	if err != nil {
		return nil, fmt.Errorf("resolve game database: %w", err)
	}
	return db, nil
}
