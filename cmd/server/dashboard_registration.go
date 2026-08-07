package main

import (
	"context"
	"fmt"
	"strings"

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

func (p *registrationContractPipeline) RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta interface{}) error {
	contractSvc, err := p.contractService(ctx, gameID, env)
	if err != nil {
		return err
	}
	return contractSvc.RebuildContractFromFunctionMeta(ctx, gameID, env, source, meta)
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
