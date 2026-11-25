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
// Receiving-side visibility scope (incident-reports §6 + plugin mechanism
// §5.1): admins see everything; ordinary users can see = direct receipts
// (including leader slices addressed to them) ∪ fallback-group broadcasts
// without ownership (scope without categories, never a void) ∪ scope
// notifications of their visible categories (leader or audience). The scope
// filter runs on the Go side (no cross-dialect query precedent for JSON
// columns), so the audience path pulls the whole window first and pages after;
// total is counted post-filter. Detail/Read keep the existing ownership
// validation unchanged.
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
		opts.To = "" // admin sees everything
	} else if username != "" {
		// Non-admin: pull the whole window back (direct + fallback group +
		// notification-source-category messages), filter in Go, then page —
		// JSON scope cannot be pushed down to SQL across dialects. An empty
		// username keeps the existing no-filter behavior (test direct-connect
		// scenario; production routes go through the auth middleware, so a
		// login name is always present).
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
		// Recompute pagination and totals after the scope-audience filter; SQL
		// already ordered by id DESC, so the order is preserved. Pagination
		// uses the request's original values (opts was overwritten by the
		// whole-window fetch).
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
		opts.Page, opts.PageSize = req.Page, req.PageSize // echo back the request's original values
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

// visibilityFetchCap is the whole-window fetch cap for the audience path (GM
// in-app message volume is tiny; tail rows beyond the cap being invisible is a
// known boundary — move filtering to the storage side if the volume grows).
const visibilityFetchCap = 1000

// resolveVisibility determines whether the current user is an admin and their
// set of visible category slugs (leader ∪ audience.users ∪ audience.roles∩
// user roles). Missing dependencies (no AdminModel / no DB / no login state)
// always degrade to "non-admin + empty category set", equivalent to the
// existing filter-by-recipient behavior.
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

// categoryVisibleToUser reports whether a category is visible to a user: the
// leader or an audience hit (users directly listed, or roles intersecting the
// user's roles).
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

// messageVisibleTo reports whether one message is visible to a non-admin
// user: direct receipts are always visible; fallback-group broadcasts whose
// scope has no categories are visible (never a void); when the scope has
// categories, visibility is decided by intersection with the audience set
// (rows without scope addressed to other recipients stay invisible, so
// point-to-point messages remain private).
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

// messageScopeCategories parses the categories of a message's scope (a parse
// failure is treated as none).
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
	// Design-debt cleanup: normalizeMessageItems unconditionally appends one
	// item per input element, so output length always equals input length;
	// we pass a single-element slice, so items always has exactly 1 item and
	// the original len(items)==0 branch was unreachable and has been removed.
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

// Broadcast: admins send in-app messages in bulk — expand the audience
// (all = everyone / role = by role / users = a list of named usernames) and
// land the rows in batches, reusing the single-send validation rules. At the
// current back-office user scale (tens to a few hundreds), synchronous
// expansion is enough; go async when the volume grows.
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
