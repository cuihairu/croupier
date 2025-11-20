// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AssignmentsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssignmentsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignmentsUpdateLogic {
	return &AssignmentsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AssignmentsUpdateLogic) AssignmentsUpdate(req *types.AssignmentsUpdateRequest) (resp *types.AssignmentsUpdateResponse, err error) {
	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errors.New("game_id required")
	}
	env := strings.TrimSpace(req.Env)

	valid := make([]string, 0, len(req.Functions))
	unknown := make([]string, 0)
	seen := map[string]struct{}{}
	for _, fid := range req.Functions {
		id := strings.TrimSpace(fid)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if l.svcCtx.HasFunction(id) {
			valid = append(valid, id)
		} else {
			unknown = append(unknown, id)
		}
	}
	if err := l.svcCtx.UpdateAssignments(gameID, env, valid); err != nil {
		return nil, err
	}
	return &types.AssignmentsUpdateResponse{
		Ok:      true,
		Unknown: unknown,
	}, nil
}
