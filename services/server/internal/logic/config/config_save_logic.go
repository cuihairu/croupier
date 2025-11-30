package config

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type ConfigSaveRequest struct {
	GameID      string `json:"game_id"`
	Env         string `json:"env"`
	Format      string `json:"format"`
	Content     string `json:"content"`
	Message     string `json:"message,optional"`
	BaseVersion int    `json:"base_version,optional"`
}

func NewConfigSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigSaveLogic {
	return &ConfigSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigSaveLogic) ConfigSave(id string, req *ConfigSaveRequest) (map[string]interface{}, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("配置ID不能为空")
	}
	payload := model.ConfigVersionPayload{
		Key:         id,
		Content:     req.Content,
		Format:      req.Format,
		GameID:      req.GameID,
		Env:         req.Env,
		Message:     req.Message,
		BaseVersion: req.BaseVersion,
	}
	record, err := l.svcCtx.ConfigVersionModel.CreateWithMeta(l.ctx, payload, configAuthor(l.ctx))
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":      record.Key,
		"version": record.Version,
	}, nil
}
