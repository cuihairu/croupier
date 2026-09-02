package message

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dbenum"
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

// List returns the list of messages for the current user.
func (s *Service) List(ctx context.Context, username string, req *MessagesListRequest) (*MessagesListResponse, error) {
	if s.svcCtx.MessageModel == nil {
		return &MessagesListResponse{Items: []MessageItem{}, Total: 0, Page: req.Page, PageSize: req.PageSize}, nil
	}
	opts := model.ListMessagesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Type:     strings.TrimSpace(req.Type),
		Status:   parseMessageStatusFilter(strings.TrimSpace(req.Status)),
		To:       strings.TrimSpace(username),
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
		Items:    normalizeMessageItems(items),
		Total:    total,
		Page:     opts.Page,
		PageSize: opts.PageSize,
	}, nil
}

// Send sends a new message
func (s *Service) Send(ctx context.Context, req *MessageSendRequest) (*MessageSendResponse, error) {
	if s.svcCtx.MessageModel == nil {
		return nil, errors.New("消息服务未初始化")
	}
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
		Status:  dbenum.MessageStatusUnread,
	}

	if err := s.svcCtx.MessageModel.Create(ctx, msg); err != nil {
		return nil, err
	}

	return buildMessageItemResponse(msg), nil
}

// Detail returns the details of a message owned by the current user.
func (s *Service) Detail(ctx context.Context, username string, req *MessageDetailRequest) (*MessageDetailResponse, error) {
	if s.svcCtx.MessageModel == nil {
		return nil, errors.New("消息服务未初始化")
	}
	id, err := utils.ParseUintID(req.ID, "消息ID")
	if err != nil {
		return nil, err
	}

	msg, err := s.svcCtx.MessageModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(msg.To) != "" && strings.TrimSpace(msg.To) != strings.TrimSpace(username) {
		return nil, errors.New("无权查看此消息")
	}

	return buildMessageItemResponse(msg), nil
}

// Read marks a message as read, verifying ownership.
func (s *Service) Read(ctx context.Context, username string, req *MessageReadRequest) (*MessageReadResponse, error) {
	if s.svcCtx.MessageModel == nil {
		return nil, errors.New("消息服务未初始化")
	}
	id, err := utils.ParseUintID(req.ID, "消息ID")
	if err != nil {
		return nil, err
	}

	msg, err := s.svcCtx.MessageModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(msg.To) != "" && strings.TrimSpace(msg.To) != strings.TrimSpace(username) {
		return nil, errors.New("无权操作此消息")
	}

	if err := s.svcCtx.MessageModel.MarkRead(ctx, id); err != nil {
		return nil, err
	}

	msg, err = s.svcCtx.MessageModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	return buildMessageItemResponse(msg), nil
}

// UnreadCount returns the count of unread messages for the current user.
func (s *Service) UnreadCount(ctx context.Context, username string, req *MessagesUnreadCountRequest) (*MessagesUnreadCountResponse, error) {
	if s.svcCtx.MessageModel == nil {
		return &MessagesUnreadCountResponse{Count: 0}, nil
	}
	count, err := s.svcCtx.MessageModel.CountUnread(ctx, strings.TrimSpace(username))
	if err != nil {
		return nil, err
	}

	return &MessagesUnreadCountResponse{
		Count: count,
	}, nil
}

// Stream returns recent messages for the current user (SSE).
func (s *Service) Stream(ctx context.Context, username string, req *StreamMessagesRequest) (*StreamMessagesResponse, error) {
	if s.svcCtx.MessageModel == nil {
		return &StreamMessagesResponse{Items: []MessageItem{}}, nil
	}
	messages, err := s.svcCtx.MessageModel.Recent(ctx, 20, strings.TrimSpace(username))
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(messages))
	for i := range messages {
		items = append(items, utils.BuildMessageDTO(&messages[i]))
	}

	return &StreamMessagesResponse{
		Items: normalizeMessageItems(items),
	}, nil
}

func buildMessageItemResponse(msg *model.Message) *MessageItem {
	items := normalizeMessageItems([]map[string]interface{}{utils.BuildMessageDTO(msg)})
	if len(items) == 0 {
		return &MessageItem{}
	}
	return &items[0]
}

func normalizeMessageItems(items []map[string]interface{}) []MessageItem {
	out := make([]MessageItem, 0, len(items))
	for _, item := range items {
		out = append(out, MessageItem{
			ID:        item["id"],
			To:        stringValue(item["to"]),
			Type:      stringValue(item["type"]),
			Title:     stringValue(item["title"]),
			Content:   stringValue(item["content"]),
			Data:      item["data"],
			Status:    stringValue(item["status"]),
			ReadAt:    stringValue(item["readAt"]),
			CreatedAt: stringValue(item["createdAt"]),
			UpdatedAt: stringValue(item["updatedAt"]),
		})
	}
	return out
}

func stringValue(value interface{}) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

// parseMessageStatusFilter converts a wire status filter into the enum.
// Unknown values map to -1 (no rows match by accident semantics keep).
func parseMessageStatusFilter(value string) dbenum.MessageStatus {
	if value == "" {
		return -1
	}
	parsed, err := dbenum.ParseMessageStatus(strings.ToLower(value))
	if err != nil {
		return -1
	}
	return parsed
}

// Broadcast 管理员群发站内信：展开受众（all=全员 / role=按角色 /
// users=指定用户名列表）后批量落库，复用单发的校验规则。当前后台用户
// 量级（几十~几百）同步展开即可；量大再异步化。
func (s *Service) Broadcast(ctx context.Context, req *BroadcastRequest) (*BroadcastResponse, error) {
	if s.svcCtx.MessageModel == nil || s.svcCtx.AdminModel == nil {
		return nil, errors.New("消息服务未初始化")
	}
	messageType, err := utils.ValidateMessageType(strings.TrimSpace(req.Type))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("消息标题不能为空")
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, errors.New("消息内容不能为空")
	}

	recipients, err := s.resolveRecipients(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(recipients) == 0 {
		return nil, errors.New("收件人列表为空")
	}
	dataJSON, err := model.EncodeData(req.Data)
	if err != nil {
		return nil, errorx.NewBadRequest("序列化消息数据失败")
	}
	for _, to := range recipients {
		msg := &model.Message{
			To:      to,
			Type:    messageType,
			Title:   req.Title,
			Content: req.Content,
			Data:    dataJSON,
		}
		if err := s.svcCtx.MessageModel.Create(ctx, msg); err != nil {
			return nil, err
		}
	}
	return &BroadcastResponse{Sent: len(recipients), Recipients: recipients}, nil
}

func (s *Service) resolveRecipients(ctx context.Context, req *BroadcastRequest) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(req.Audience)) {
	case "", "all":
		return s.allUsernames(ctx)
	case "role":
		role := strings.TrimSpace(req.Role)
		if role == "" {
			return nil, errors.New("audience=role 时必须指定 role")
		}
		return s.usernamesByRole(ctx, role)
	case "users":
		if len(req.Usernames) == 0 {
			return nil, errors.New("audience=users 时必须提供 usernames")
		}
		seen := map[string]bool{}
		out := make([]string, 0, len(req.Usernames))
		for _, name := range req.Usernames {
			name = strings.TrimSpace(name)
			if name == "" || seen[name] {
				continue
			}
			if _, err := s.svcCtx.AdminModel.FindByUsername(ctx, name); err != nil {
				return nil, fmt.Errorf("用户 %s 不存在", name)
			}
			seen[name] = true
			out = append(out, name)
		}
		return out, nil
	default:
		return nil, errors.New("audience 必须是 all、role 或 users")
	}
}

func (s *Service) allUsernames(ctx context.Context) ([]string, error) {
	var out []string
	page := 1
	for {
		admins, total, err := s.svcCtx.AdminModel.List(ctx, model.ListAdminsOptions{Page: page, PageSize: 500, Status: statusActivePtr()})
		if err != nil {
			return nil, err
		}
		for _, a := range admins {
			out = append(out, a.Username)
		}
		if int64(page*500) >= total {
			break
		}
		page++
	}
	return out, nil
}

func (s *Service) usernamesByRole(ctx context.Context, role string) ([]string, error) {
	var out []string
	page := 1
	for {
		admins, total, err := s.svcCtx.AdminModel.List(ctx, model.ListAdminsOptions{Page: page, PageSize: 500, Role: role, Status: statusActivePtr()})
		if err != nil {
			return nil, err
		}
		for _, a := range admins {
			out = append(out, a.Username)
		}
		if int64(page*500) >= total {
			break
		}
		page++
	}
	return out, nil
}

func statusActivePtr() *int {
	v := 1
	return &v
}
