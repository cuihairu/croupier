package player

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List retrieves a paginated list of players
func (s *Service) List(ctx context.Context, req *PlayersListRequest) (*PlayersListResponse, error) {
	opts := model.ListPlayersOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		GameID:   strings.TrimSpace(req.GameId),
		Search:   strings.TrimSpace(req.Search),
	}
	if req.Status != 0 {
		status := req.Status
		opts.Status = &status
	}
	if req.Level != 0 {
		level := req.Level
		opts.Level = &level
	}
	if req.Vip != 0 {
		vip := req.Vip
		opts.VIP = &vip
	}

	players, total, err := s.svcCtx.PlayerModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]Player, 0, len(players))
	for i := range players {
		items = append(items, buildPlayer(&players[i]))
	}

	return &PlayersListResponse{
		Items: items,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}

// Create creates a new player
func (s *Service) Create(ctx context.Context, req *PlayerCreateRequest) (*PlayerCreateResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		return nil, errors.New("密码不能为空")
	}
	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errors.New("Game ID 不能为空")
	}

	player := &model.Player{
		Username: username,
		Nickname: strings.TrimSpace(req.Nickname),
		Email:    strings.TrimSpace(req.Email),
		Phone:    strings.TrimSpace(req.Phone),
		GameID:   gameID,
		Status:   model.PlayerStatusActive,
	}

	if err := s.svcCtx.PlayerModel.Create(ctx, player, password); err != nil {
		return nil, err
	}

	return &PlayerCreateResponse{
		Player: buildPlayer(player),
	}, nil
}

// Detail retrieves details of a specific player
func (s *Service) Detail(ctx context.Context, req *PlayerDetailRequest) (*PlayerDetailResponse, error) {
	id, err := utils.ParseUintID(req.ID, "玩家ID")
	if err != nil {
		return nil, err
	}

	player, err := s.svcCtx.PlayerModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	return &PlayerDetailResponse{
		Player: buildPlayer(player),
	}, nil
}

// Update updates an existing player
func (s *Service) Update(ctx context.Context, req *PlayerUpdateRequest) (*PlayerUpdateResponse, error) {
	id, err := utils.ParseUintID(req.ID, "玩家ID")
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Nickname); v != "" {
		updates["nickname"] = v
	}
	if v := strings.TrimSpace(req.Email); v != "" {
		updates["email"] = v
	}
	if v := strings.TrimSpace(req.Phone); v != "" {
		updates["phone"] = v
	}
	if req.Status != 0 {
		if req.Status != model.PlayerStatusActive &&
			req.Status != model.PlayerStatusBanned &&
			req.Status != model.PlayerStatusSuspended {
			return nil, errors.New("状态值无效")
		}
		updates["status"] = req.Status
	}
	if req.Level > 0 {
		updates["level"] = req.Level
	}
	if req.Vip >= 0 {
		updates["vip"] = req.Vip
	}

	if len(updates) == 0 {
		return nil, errors.New("至少提供一个更新字段")
	}

	if err := s.svcCtx.PlayerModel.Update(ctx, id, updates); err != nil {
		return nil, err
	}

	player, err := s.svcCtx.PlayerModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	return &PlayerUpdateResponse{
		Player: buildPlayer(player),
	}, nil
}

// Delete deletes a player
func (s *Service) Delete(ctx context.Context, req *PlayerDeleteRequest) error {
	id, err := utils.ParseUintID(req.ID, "玩家ID")
	if err != nil {
		return err
	}
	return s.svcCtx.PlayerModel.Delete(ctx, id)
}

// Balance adjusts a player's balance
func (s *Service) Balance(ctx context.Context, req *PlayerBalanceRequest) (*PlayerBalanceResponse, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}
	id, err := utils.ParseUintID(req.ID, "玩家ID")
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("调整原因不能为空")
	}
	player, err := s.svcCtx.PlayerModel.UpdateBalance(ctx, id, req.Amount, reason)
	if err != nil {
		return nil, err
	}

	return &PlayerBalanceResponse{
		Player: buildPlayer(player),
	}, nil
}

func buildPlayer(player *model.Player) Player {
	return Player{
		Id:        int64(player.ID),
		Username:  player.Username,
		Nickname:  player.Nickname,
		Email:     player.Email,
		Phone:     player.Phone,
		GameId:    player.GameID,
		Status:    player.Status,
		Balance:   player.Balance,
		Level:     player.Level,
		Vip:       player.VIP,
		CreatedAt: utils.FormatTimestamp(player.CreatedAt),
		UpdatedAt: utils.FormatTimestamp(player.UpdatedAt),
	}
}
