package profile

import (
	"context"
	"errors"
	"strconv"

	"github.com/cuihairu/croupier/services/server/internal/model"
)

type Service struct {
	adminModel *model.AdminModel
	gameModel  *model.GameModel
}

func NewService(adminModel *model.AdminModel, gameModel *model.GameModel) *Service {
	return &Service{
		adminModel: adminModel,
		gameModel:  gameModel,
	}
}

// GetProfile 获取个人资料
func (s *Service) GetProfile(ctx context.Context, username string) (*ProfileGetResponse, error) {
	// 查询管理员信息
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 获取角色
	roleModels, err := s.adminModel.GetAdminRoles(ctx, admin.ID)
	if err != nil {
		return nil, errors.New("获取用户角色失败")
	}

	roles := make([]string, 0, len(roleModels))
	for _, role := range roleModels {
		roles = append(roles, role.Name)
	}

	return &ProfileGetResponse{
		Username: admin.Username,
		Nickname: admin.Nickname,
		Email:    admin.Email,
		Phone:    admin.Phone,
		Roles:    roles,
	}, nil
}

// GetUserGames 获取用户的游戏列表
func (s *Service) GetUserGames(ctx context.Context, username string) (*ProfileGamesResponse, error) {
	// 查询管理员信息
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 获取管理员的游戏权限
	adminGames, err := s.adminModel.GetAdminGames(ctx, admin.ID)
	if err != nil {
		return nil, errors.New("获取游戏列表失败")
	}

	games := make([]GameInfo, 0, len(adminGames))
	for _, ag := range adminGames {
		// 查询游戏详情
		game, err := s.gameModel.FindByGameID(ctx, ag.GameID)
		if err != nil {
			continue
		}
		games = append(games, GameInfo{
			GameID:   strconv.FormatUint(uint64(game.ID), 10),
			GameName: game.Name,
		})
	}

	return &ProfileGamesResponse{
		Games: games,
	}, nil
}

// UpdateProfile 更新个人资料
func (s *Service) UpdateProfile(ctx context.Context, username string, req *ProfileUpdateRequest) (*ProfileUpdateResponse, error) {
	// 查询管理员信息
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 准备更新字段
	updates := map[string]interface{}{
		"nickname": req.Nickname,
		"email":    req.Email,
		"phone":    req.Phone,
	}

	// 保存更新
	if err := s.adminModel.Update(ctx, admin.ID, updates); err != nil {
		return nil, errors.New("更新失败")
	}

	return &ProfileUpdateResponse{Ok: true}, nil
}

// ChangePassword 修改密码
func (s *Service) ChangePassword(ctx context.Context, username string, req *ChangePasswordRequest) (*ChangePasswordResponse, error) {
	// 验证旧密码
	_, err := s.adminModel.ValidatePassword(ctx, username, req.OldPassword)
	if err != nil {
		return nil, errors.New("旧密码错误")
	}

	// 查询管理员信息
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 更新密码
	if err := s.adminModel.UpdatePassword(ctx, admin.ID, req.NewPassword); err != nil {
		return nil, errors.New("修改密码失败")
	}

	return &ChangePasswordResponse{Ok: true}, nil
}
