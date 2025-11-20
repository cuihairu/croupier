package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthMeLogic struct {
	logx.Logger
	ctx context.Context
}

func NewAuthMeLogic(ctx context.Context) *AuthMeLogic {
	return &AuthMeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
	}
}

func (l *AuthMeLogic) AuthMe(username string, roles []string) (*types.AuthMeResponse, error) {
	return &types.AuthMeResponse{
		Username: username,
		Roles:    append([]string(nil), roles...),
	}, nil
}
