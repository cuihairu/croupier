package ops

import (
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/svc"
)

var errOpsStateUnavailable = errors.New("ops state store unavailable")

func snapshotOpsState(ctx *svc.ServiceContext) svc.OpsState {
	if ctx != nil && ctx.OpsStateStore != nil {
		return ctx.OpsStateStore.Snapshot()
	}
	return svc.OpsState{}
}

func updateOpsState(ctx *svc.ServiceContext, fn func(*svc.OpsState)) (svc.OpsState, error) {
	if ctx == nil || ctx.OpsStateStore == nil {
		return svc.OpsState{}, errOpsStateUnavailable
	}
	return ctx.OpsStateStore.Update(fn)
}
