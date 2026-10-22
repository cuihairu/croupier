package message

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns the list of messages for the current user.
// 读侧可见范围（incident-reports §6 + 插件机制 §5.1）：管理员全量；普通
// 用户可见 = 直收（含定向给本人的 leader 分片）∪ 兜底组无归属广播（scope
// 无 categories，不落空）∪ 自己可见类别（leader 或 audience）的 scope 通
// 知。scope 过滤在 Go 侧做（JSON 列无跨方言查询先例），故受众路径整窗拉
// 取后再分页，total 以过滤后为准。Detail/Read 沿既有归属校验不动。
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

	isAdmin, visibleSlugs := s.resolveVisibility(ctx, username)
	if isAdmin {
		opts.To = "" // 管理员全量
	} else if username != "" {
		// 非管理员：整窗拉回（直收 + 兜底组 + 通知源类别消息），Go 过滤后
		// 再分页——JSON scope 无法跨方言下推 SQL。username 为空沿用既有
		// 不过滤行为（测试直连场景；生产路由经鉴权中间件必有登录名）。
		opts.To = ""
		opts.Recipients = []string{username, outlet.DefaultStationRecipient}
		opts.IncludeSource = "incident_report"
		opts.Page = 1
		opts.PageSize = visibilityFetchCap
	}

	messages, total, err := s.svcCtx.MessageModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	if !isAdmin && username != "" {
		// scope 受众过滤后重算分页与总数；SQL 已按 id DESC，序保持。
		// 分页参数用请求原值（opts 已被整窗拉取覆盖）。
		filtered := make([]model.Message, 0, len(messages))
		for i := range messages {
			if messageVisibleTo(&messages[i], username, visibleSlugs) {
				filtered = append(filtered, messages[i])
			}
		}
		total = int64(len(filtered))
		page, size := req.Page, req.PageSize
		if page <= 0 {
			page = 1
		}
		if size <= 0 {
			size = 20
		}
		start := (page - 1) * size
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + size
		if end > len(filtered) {
			end = len(filtered)
		}
		messages = filtered[start:end]
		opts.Page, opts.PageSize = req.Page, req.PageSize // 响应回显保持请求原值
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

// visibilityFetchCap 受众路径整窗拉取上限（GM 站内信量级很小；超过上限的
// 尾部行不可见属已知边界，量级上来再改存储侧过滤）。
const visibilityFetchCap = 1000

// resolveVisibility 判定当前用户是否管理员与其可见类别 slug 集（leader ∪
// audience.users ∪ audience.roles∩用户角色）。依赖缺失（无 AdminModel/无
// DB/无登录态）一律降级为「非管理员 + 空类别集」，行为等同既有按收件人
// 过滤。
func (s *Service) resolveVisibility(ctx context.Context, username string) (bool, map[string]bool) {
	roleNames := map[string]bool{}
	if s.svcCtx.AdminModel != nil {
		_, roles, err := utils.LoadCurrentAdmin(ctx, s.svcCtx)
		if err == nil {
			for _, r := range roles {
				roleNames[r.Name] = true
			}
		}
	}
	if roleNames["admin"] {
		return true, nil
	}
	slugs := map[string]bool{}
	if s.svcCtx.DB != nil {
		cats, err := model.NewIncidentCategoryModel(s.svcCtx.DB).List(ctx, false)
		if err == nil {
			for _, cat := range cats {
				if categoryVisibleToUser(cat, username, roleNames) {
					slugs[cat.Slug] = true
				}
			}
		}
	}
	return false, slugs
}

// categoryVisibleToUser 类别对用户是否可见：leader 或 audience 命中（users
// 直配或 roles 与用户角色相交）。
func categoryVisibleToUser(cat model.IncidentCategory, username string, roleNames map[string]bool) bool {
	if strings.TrimSpace(cat.Leader) != "" && cat.Leader == username {
		return true
	}
	if len(cat.Audience) == 0 {
		return false
	}
	var aud struct {
		Roles []string `json:"roles"`
		Users []string `json:"users"`
	}
	if err := json.Unmarshal(cat.Audience, &aud); err != nil {
		return false
	}
	for _, u := range aud.Users {
		if u == username {
			return true
		}
	}
	for _, r := range aud.Roles {
		if roleNames[r] {
			return true
		}
	}
	return false
}

// messageVisibleTo 单条消息对非管理员用户是否可见：直收恒可见；scope 无
// categories 的兜底组广播可见（不落空）；scope 有 categories 的按受众集
// 相交判定（其他收件人的无 scope 行不可见，点对点消息保持私密）。
func messageVisibleTo(msg *model.Message, username string, slugs map[string]bool) bool {
	if msg.To == username {
		return true
	}
	cats := messageScopeCategories(msg)
	if len(cats) == 0 {
		return msg.To == outlet.DefaultStationRecipient
	}
	for _, c := range cats {
		if slugs[c] {
			return true
		}
	}
	return false
}

// messageScopeCategories 解析消息 scope 的 categories（解析失败视为无）。
func messageScopeCategories(msg *model.Message) []string {
	if len(msg.Scope) == 0 {
		return nil
	}
	var sc struct {
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal(msg.Scope, &sc); err != nil {
		return nil
	}
	return sc.Categories
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
	// 设计债清理：normalizeMessageItems 对每个输入元素无条件 append 一项，
	// 输出长度恒等于输入长度；这里传入单元素 slice，items 恒有且仅有 1 项，
	// 原 len(items)==0 分支不可达已删除。
	items := normalizeMessageItems([]map[string]interface{}{utils.BuildMessageDTO(msg)})
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
