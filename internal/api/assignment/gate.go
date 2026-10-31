package assignment

import (
	"context"

	"github.com/cuihairu/croupier/internal/logic/assignment"
	"github.com/cuihairu/croupier/internal/svc"
)

// LoadScopeAssignments is a compatibility wrapper around the domain loader.
func LoadScopeAssignments(svcCtx *svc.ServiceContext) (map[string][]string, error) {
	return assignment.LoadScopeAssignments(svcCtx)
}

// EnsureFunctionAssigned is a compatibility wrapper around the domain assignment gate.
func EnsureFunctionAssigned(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string) error {
	return assignment.EnsureFunctionAssigned(ctx, svcCtx, functionID, gameID, env)
}
