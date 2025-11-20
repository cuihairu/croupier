package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

var (
	ErrAuthDisabled   = errors.New("auth disabled")
	ErrLoginRateLimit = errors.New("login rate limited")
	ErrUnauthorized   = errors.New("unauthorized")
)

type AuthLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthLoginLogic {
	return &AuthLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AuthLoginLogic) AuthLogin(req *types.AuthLoginRequest, ip, userAgent string) (*types.AuthLoginResponse, error) {
	repo := l.svcCtx.UserRepository()
	jwtMgr := l.svcCtx.JWTManager()
	if repo == nil || jwtMgr == nil {
		return nil, ErrAuthDisabled
	}
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		return nil, ErrInvalidRequest
	}
	if !l.svcCtx.AllowLogin(ip, username) {
		return nil, ErrLoginRateLimit
	}
	user, err := repo.Verify(l.ctx, username, password)
	if err != nil {
		return nil, ErrUnauthorized
	}
	roles, err := repo.ListUserRoles(l.ctx, user.ID)
	if err != nil {
		return nil, err
	}
	token, err := jwtMgr.Sign(user.Username, roles, 8*time.Hour)
	if err != nil {
		return nil, err
	}
	resp := &types.AuthLoginResponse{
		Token: token,
		User: types.AuthUserInfo{
			Username: user.Username,
			Roles:    roles,
		},
	}
	return resp, nil
}
