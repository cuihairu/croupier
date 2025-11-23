package auth
import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

var (
	ErrAuthDisabled   = errors.New("auth disabled")
	ErrLoginRateLimit = errors.New("login rate limited")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrInvalidRequest = errors.New("invalid request")
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

func (l *AuthLoginLogic) AuthLogin(req *types.LoginRequest) (resp *types.LoginResponse, err error) {
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
	// TODO: Get IP from context or request headers
	// if !l.svcCtx.AllowLogin(ip, username) {
	// 	return nil, ErrLoginRateLimit
	// }
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
	resp = &types.LoginResponse{
		Code:    0,
		Message: "success",
		Data: types.LoginData{
			Token: token,
			User: types.UserInfo{
				Username: user.Username,
				Roles:    roles,
			},
		},
	}
	return resp, nil
}
