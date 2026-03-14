package message

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
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

// List returns the list of messages
func (s *Service) List(ctx context.Context, req *MessagesListRequest) (*MessagesListResponse, error) {
	opts := model.ListMessagesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Type:     strings.TrimSpace(req.Type),
		Status:   strings.TrimSpace(req.Status),
	}

	messages, total, err := s.svcCtx.MessageModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(messages))
	for i := range messages {
		items = append(items, utils.BuildMessageDTO(&messages[i]))
	}

	return &MessagesListResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"total": total,
			"page":  opts.Page,
			"size":  opts.PageSize,
		},
	}, nil
}

// Send sends a new message
func (s *Service) Send(ctx context.Context, req *MessageSendRequest) (*MessageSendResponse, error) {
	to := strings.TrimSpace(req.To)
	if to == "" {
		return nil, errors.New("消息接收者不能为空")
	}

	messageType, err := utils.ValidateMessageType(strings.TrimSpace(req.Type))
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errors.New("消息内容不能为空")
	}

	dataJSON, err := model.EncodeData(req.Data)
	if err != nil {
		return nil, errorx.NewBadRequest("序列化消息数据失败")
	}

	msg := &model.Message{
		To:      to,
		Type:    messageType,
		Title:   strings.TrimSpace(req.Title),
		Content: content,
		Data:    dataJSON,
		Status:  "unread",
	}

	if err := s.svcCtx.MessageModel.Create(ctx, msg); err != nil {
		return nil, err
	}

	return &MessageSendResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildMessageDTO(msg),
	}, nil
}

// Detail returns the details of a message
func (s *Service) Detail(ctx context.Context, req *MessageDetailRequest) (*MessageDetailResponse, error) {
	id, err := utils.ParseUintID(req.ID, "消息ID")
	if err != nil {
		return nil, err
	}

	msg, err := s.svcCtx.MessageModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	return &MessageDetailResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildMessageDTO(msg),
	}, nil
}

// Read marks a message as read
func (s *Service) Read(ctx context.Context, req *MessageReadRequest) (*MessageReadResponse, error) {
	id, err := utils.ParseUintID(req.ID, "消息ID")
	if err != nil {
		return nil, err
	}

	if err := s.svcCtx.MessageModel.MarkRead(ctx, id); err != nil {
		return nil, err
	}

	msg, err := s.svcCtx.MessageModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	return &MessageReadResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildMessageDTO(msg),
	}, nil
}

// UnreadCount returns the count of unread messages
func (s *Service) UnreadCount(ctx context.Context, req *MessagesUnreadCountRequest) (*MessagesUnreadCountResponse, error) {
	count, err := s.svcCtx.MessageModel.CountUnread(ctx, "")
	if err != nil {
		return nil, err
	}

	return &MessagesUnreadCountResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"count": count,
		},
	}, nil
}

// Stream returns recent messages for streaming
func (s *Service) Stream(ctx context.Context, req *StreamMessagesRequest) (*StreamMessagesResponse, error) {
	messages, err := s.svcCtx.MessageModel.Recent(ctx, 20, "")
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(messages))
	for i := range messages {
		items = append(items, utils.BuildMessageDTO(&messages[i]))
	}

	return &StreamMessagesResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
		},
	}, nil
}
