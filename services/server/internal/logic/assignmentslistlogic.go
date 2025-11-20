// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AssignmentsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAssignmentsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignmentsListLogic {
	return &AssignmentsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AssignmentsListLogic) AssignmentsList(req *types.AssignmentsQuery) (resp *types.AssignmentsResponse, err error) {
	assignments := l.svcCtx.AssignmentsSnapshot()
	out := make(map[string][]string)
	for key, fns := range assignments {
		if req.GameId != "" || req.Env != "" {
			game := ""
			env := ""
			parts := strings.SplitN(key, "|", 2)
			if len(parts) > 0 {
				game = parts[0]
			}
			if len(parts) > 1 {
				env = parts[1]
			}
			if req.GameId != "" && game != req.GameId {
				continue
			}
			if req.Env != "" && env != req.Env {
				continue
			}
		}
		out[key] = append([]string{}, fns...)
	}
	return &types.AssignmentsResponse{Assignments: out}, nil
}
