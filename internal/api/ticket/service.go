package ticket

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/api/bug"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

// auditTicket 是工单域敏感写路径的统一留痕（AUD-049）：创建/更新/删除/
// 状态流转/评论/bug 关联共用事件类型，成功/失败/denied 差异只在 outcome。
func (s *Service) auditTicket(ctx context.Context, event audit.AuditEventType, state, action, resourceID string, extra map[string]interface{}) {
	details := map[string]interface{}{"action": action}
	for k, v := range extra {
		details[k] = v
	}
	actor := ""
	if name, err := utils.CurrentUsername(ctx); err == nil {
		actor = name
	}
	audit.LogWrite(s.svcCtx.AuditService, ctx, event, state, actor, "ticket", resourceID, details)
}

// ticketActorID 是写留痕的资源 ID 形态（工单 id → 字符串）。
func ticketActorID(id uint) string { return fmt.Sprint(id) }

// withTicketTransaction 在当前作用域连接（dbctx.Resolve）上开事务并把 tx
// 注入 ctx：包内 model 方法经 dbctx.Resolve(ctx, m.db) 自动落到事务连接
// （与 page 域 withPageTransaction 同范式，AUD-050 多步写包事务）。
func (s *Service) withTicketTransaction(ctx context.Context, fn func(context.Context) error) error {
	if s.svcCtx == nil || s.svcCtx.DB == nil {
		// 测试装配常只给 model 不给 DB：退化为直连双写（无原子性兜底）；
		// 生产装配恒带 DB，多步写走真事务。
		return fn(ctx)
	}
	db := dbctx.Resolve(ctx, s.svcCtx.DB)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(dbctx.WithDB(ctx, tx))
	})
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// requireTicketScope 单条工单行级作用域校验（对齐 task 的 requireTaskScope）：
// 列表与创建已按请求 scope 过滤/归属（#21），但按 id 的单条读/写/评论/关联
// 此前只认 id，可跨 game/env 越权。错误不回显目标行作用域。
func requireTicketScope(ctx context.Context, ticket *model.Ticket) error {
	scope := svc.GameScopeFromContext(ctx)
	if ticket == nil || ((scope.GameID != "" || scope.Env != "") && !svc.ScopeMatches(ctx, ticket.GameID, ticket.Env)) {
		return utils.NewPermissionDenied("无权访问该工单（当前游戏/环境与工单作用域不一致）", nil)
	}
	return nil
}

// ticketForScope 取单条工单并做行级作用域校验，按 id 的单条操作共用。
func (s *Service) ticketForScope(ctx context.Context, id uint) (*model.Ticket, error) {
	ticket, err := s.svcCtx.TicketModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := requireTicketScope(ctx, ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

// List returns a list of tickets
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	opts := model.TicketQueryOptions{
		PaginationOptions: model.NewPagination(req.Page, req.PageSize),
		Query:             strings.TrimSpace(req.Query),
		Status:            parseTicketStatusFilter(strings.TrimSpace(req.Status)),
		Category:          strings.TrimSpace(req.Category),
		Priority:          strings.TrimSpace(req.Priority),
		Assignee:          strings.TrimSpace(req.Assignee),
		// #21：scope 由 GameDBMiddleware 按 X-Game-ID/X-Env（顶栏）解析，
		// 请求参数里的 gameId/env 不再作为查询依据（页面已移除游戏/环境过滤）。
		GameID: svc.ResolveGameID(ctx, strings.TrimSpace(req.GameID)),
		Env:    svc.ResolveEnv(ctx, strings.TrimSpace(req.Env)),
	}

	items, total, err := s.svcCtx.TicketModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	dto := make([]Ticket, 0, len(items))
	for i := range items {
		dto = append(dto, buildTicketDTO(&items[i]))
	}

	return &ListResponse{
		Items: dto,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}

// Create creates a new ticket
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error) {
	ticket, err := sanitizeTicketFields(req)
	if err != nil {
		return nil, err
	}

	// #21：工单归属跟随请求 scope（顶栏）；body 里的 gameId/env 仅在无 scope
	// 的直调方（玩家侧 SDK 走独立路由）场景下作为兜底。
	ticket.GameID = svc.ResolveGameID(ctx, ticket.GameID)
	ticket.Env = svc.ResolveEnv(ctx, ticket.Env)

	assignee, err := s.validateAssignee(ctx, ticket.Assignee)
	if err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	ticket.Assignee = assignee

	if err := s.svcCtx.TicketModel.Create(ctx, ticket); err != nil {
		return nil, err
	}

	s.auditTicket(ctx, audit.EventTicketCreate, audit.StateSuccess, "create", ticketActorID(ticket.ID),
		map[string]interface{}{"title": ticket.Title, "gameId": ticket.GameID, "env": ticket.Env, "assignee": ticket.Assignee})
	s.notifyAssignee(ctx, ticket, ticket.Assignee, commentAuthor(ctx))

	return &CreateResponse{
		Ticket: buildTicketDTO(ticket),
	}, nil
}

// Get returns ticket details with comments
func (s *Service) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return nil, err
	}

	ticket, err := s.ticketForScope(ctx, id)
	if err != nil {
		return nil, err
	}

	comments, err := s.svcCtx.TicketModel.ListComments(ctx, id)
	if err != nil {
		return nil, err
	}

	return &GetResponse{
		Ticket:   buildTicketDTO(ticket),
		Comments: buildCommentsDTO(comments),
	}, nil
}

// Update updates a ticket
func (s *Service) Update(ctx context.Context, req *UpdateRequest) (*UpdateResponse, error) {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Title); v != "" {
		updates["title"] = v
	}
	if v := strings.TrimSpace(req.Content); v != "" {
		updates["content"] = v
	}
	if v := strings.TrimSpace(req.Category); v != "" {
		updates["category"] = v
	}
	if v := strings.TrimSpace(req.Priority); v != "" {
		updates["priority"] = sanitizePriority(v)
	}
	if v := strings.TrimSpace(req.Assignee); v != "" {
		assignee, err := s.validateAssignee(ctx, v)
		if err != nil {
			return nil, errorx.NewBadRequest(err.Error())
		}
		updates["assignee"] = assignee
	}
	if req.Tags != nil {
		updates["tags"] = encodeTicketTags(req.Tags)
	}

	if len(updates) == 0 {
		return nil, errorx.NewBadRequest("请提供需要更新的字段")
	}

	before, err := s.ticketForScope(ctx, id)
	if err != nil {
		s.auditTicket(ctx, audit.EventTicketUpdate, audit.StateDenied, "update", ticketActorID(id), nil)
		return nil, err
	}
	prevAssignee := before.Assignee

	if err := s.svcCtx.TicketModel.Update(ctx, id, updates); err != nil {
		return nil, err
	}

	ticket, err := s.svcCtx.TicketModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	if newAssignee, _ := updates["assignee"].(string); newAssignee != prevAssignee {
		s.notifyAssignee(ctx, ticket, newAssignee, commentAuthor(ctx))
	}
	s.auditTicket(ctx, audit.EventTicketUpdate, audit.StateSuccess, "update", ticketActorID(id), nil)

	return &UpdateResponse{
		Ticket: buildTicketDTO(ticket),
	}, nil
}

// Delete deletes a ticket
func (s *Service) Delete(ctx context.Context, req *DeleteRequest) error {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return err
	}
	// 删除保持幂等契约（不存在=成功，handler 层钉住 200）；行存在但跨
	// scope 才拒绝（AUD-COV-001 P1 行级越权补齐）。
	ticket, err := s.svcCtx.TicketModel.FindOne(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := requireTicketScope(ctx, ticket); err != nil {
		s.auditTicket(ctx, audit.EventTicketDelete, audit.StateDenied, "delete", ticketActorID(id), nil)
		return err
	}
	if err := s.svcCtx.TicketModel.Delete(ctx, id); err != nil {
		return err
	}
	s.auditTicket(ctx, audit.EventTicketDelete, audit.StateSuccess, "delete", ticketActorID(id), nil)
	return nil
}

// Transition transitions a ticket to a new status
func (s *Service) Transition(ctx context.Context, req *TransitionRequest) (*TransitionResponse, error) {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return nil, err
	}

	if _, err := s.ticketForScope(ctx, id); err != nil {
		s.auditTicket(ctx, audit.EventTicketTransition, audit.StateDenied, "transition", ticketActorID(id), map[string]interface{}{"to": req.Status})
		return nil, err
	}

	status, err := sanitizeTicketStatus(req.Status)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"status": status,
	}
	// Reopening clears the CSAT rating: a stale score from the previous
	// resolution must not leak into satisfaction reports.
	if status == dbenum.TicketStatusOpen {
		current, findErr := s.svcCtx.TicketModel.FindOne(ctx, id)
		if findErr == nil && current.Status != dbenum.TicketStatusOpen {
			updates["rating"] = 0
			updates["rated_by"] = ""
			updates["rated_at"] = nil
		}
	}
	// AUD-050：状态备注与状态更新同事务——备注落库后状态更新失败会留下
	// 「有备注无状态变更」的半提交中间态，回滚保证两写同生同灭。
	if err := s.withTicketTransaction(ctx, func(txCtx context.Context) error {
		if note := strings.TrimSpace(req.Note); note != "" {
			comment := addComment(commentAuthor(txCtx), fmt.Sprintf("[状态变更] %s", note), id)
			if err := s.svcCtx.TicketModel.CreateComment(txCtx, comment); err != nil {
				return err
			}
		}
		return s.svcCtx.TicketModel.Update(txCtx, id, updates)
	}); err != nil {
		return nil, err
	}

	ticket, err := s.svcCtx.TicketModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	actor := commentAuthor(ctx)
	s.notifyTicketEvent(ctx, ticket, actor,
		fmt.Sprintf("工单 #%d 状态变更为 %s", ticket.ID, ticket.Status),
		fmt.Sprintf("【%s】%s", ticket.Category, ticket.Title))

	s.auditTicket(ctx, audit.EventTicketTransition, audit.StateSuccess, "transition", ticketActorID(id), map[string]interface{}{"to": ticket.Status})

	comments, err := s.svcCtx.TicketModel.ListComments(ctx, id)
	if err != nil {
		return nil, err
	}

	return &TransitionResponse{
		Ticket:   buildTicketDTO(ticket),
		Comments: buildCommentsDTO(comments),
	}, nil
}

// GetComments returns comments for a ticket
func (s *Service) GetComments(ctx context.Context, req *GetCommentsRequest) (*GetCommentsResponse, error) {
	id, err := parseTicketID(req.TicketID)
	if err != nil {
		return nil, err
	}

	if _, err := s.ticketForScope(ctx, id); err != nil {
		return nil, err
	}

	comments, err := s.svcCtx.TicketModel.ListComments(ctx, id)
	if err != nil {
		return nil, err
	}

	return &GetCommentsResponse{
		Items: buildCommentsDTO(comments),
	}, nil
}

// CreateComment creates a new comment on a ticket
func (s *Service) CreateComment(ctx context.Context, req *CreateCommentRequest) (*CreateCommentResponse, error) {
	id, err := parseTicketID(req.TicketID)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errorx.NewBadRequest("评论内容不能为空")
	}

	if _, err := s.ticketForScope(ctx, id); err != nil {
		s.auditTicket(ctx, audit.EventTicketComment, audit.StateDenied, "comment", ticketActorID(id), nil)
		return nil, err
	}

	actor := commentAuthor(ctx)
	comment := addComment(actor, content, id)
	if err := s.svcCtx.TicketModel.CreateComment(ctx, comment); err != nil {
		return nil, err
	}

	ticket, err := s.svcCtx.TicketModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	s.notifyTicketEvent(ctx, ticket, actor,
		fmt.Sprintf("工单 #%d 有新回复", ticket.ID),
		fmt.Sprintf("%s: %s", actor, truncateContent(content, 120)))

	comments, err := s.svcCtx.TicketModel.ListComments(ctx, id)
	if err != nil {
		return nil, err
	}

	s.auditTicket(ctx, audit.EventTicketComment, audit.StateSuccess, "comment", ticketActorID(id), nil)
	return &CreateCommentResponse{
		Items: buildCommentsDTO(comments),
	}, nil
}

// FilterOptions returns server-aggregated filter options for the ticket list
// (OPEN-ISSUES #21): category/assignee options are the distinct full set
// under the resolved scope — never derived from the current (filtered) page.
func (s *Service) FilterOptions(ctx context.Context) (*FilterOptionsResponse, error) {
	gameID := svc.ResolveGameID(ctx, "")
	env := svc.ResolveEnv(ctx, "")

	cats, err := s.svcCtx.TicketModel.ListCategories(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	assignees, err := s.svcCtx.TicketModel.ListAssignees(ctx, gameID, env)
	if err != nil {
		return nil, err
	}

	resp := &FilterOptionsResponse{
		Categories: make([]TicketFilterOption, 0, len(cats)),
		Assignees:  make([]TicketFilterOption, 0, len(assignees)),
	}
	for _, c := range cats {
		resp.Categories = append(resp.Categories, TicketFilterOption{Name: c.Name, Count: int64(c.Count)})
	}
	for _, a := range assignees {
		resp.Assignees = append(resp.Assignees, TicketFilterOption{Name: a.Name, Count: int64(a.Count)})
	}
	return resp, nil
}

// parseTicketStatusFilter converts a wire status filter into the enum.
// Unknown values map to -1 so no rows match by accident.
func parseTicketStatusFilter(value string) dbenum.TicketStatus {
	if value == "" {
		return -1
	}
	parsed, err := dbenum.ParseTicketStatus(strings.ToLower(value))
	if err != nil {
		return -1
	}
	return parsed
}

// ListBugs returns the bugs linked to the ticket (#25).
func (s *Service) ListBugs(ctx context.Context, ticketID uint) (*TicketBugsResponse, error) {
	if _, err := s.ticketForScope(ctx, ticketID); err != nil {
		return nil, err
	}
	items, err := s.svcCtx.BugModel.ListBugsByTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	return &TicketBugsResponse{Items: items}, nil
}

// LinkBug associates a bug with the ticket (#25).
func (s *Service) LinkBug(ctx context.Context, ticketID uint, req *TicketBugLinkRequest) error {
	if req == nil || req.BugID == 0 {
		return errorx.NewBadRequest("bugId 不能为空")
	}
	if _, err := s.ticketForScope(ctx, ticketID); err != nil {
		s.auditTicket(ctx, audit.EventTicketBugLink, audit.StateDenied, "bug_link", ticketActorID(ticketID), map[string]interface{}{"bugId": req.BugID})
		return err
	}
	createdBy, _ := utils.CurrentUsername(ctx)
	if err := s.svcCtx.BugModel.LinkBugTicket(ctx, req.BugID, ticketID, createdBy); err != nil {
		return bug.TranslateBugLinkError(err)
	}
	s.auditTicket(ctx, audit.EventTicketBugLink, audit.StateSuccess, "bug_link", ticketActorID(ticketID), map[string]interface{}{"bugId": req.BugID})
	return nil
}

// UnlinkBug removes the ticket↔bug association (#25).
func (s *Service) UnlinkBug(ctx context.Context, ticketID, bugID uint) error {
	if _, err := s.ticketForScope(ctx, ticketID); err != nil {
		s.auditTicket(ctx, audit.EventTicketBugUnlink, audit.StateDenied, "bug_unlink", ticketActorID(ticketID), map[string]interface{}{"bugId": bugID})
		return err
	}
	if err := s.svcCtx.BugModel.UnlinkBugTicket(ctx, bugID, ticketID); err != nil {
		return err
	}
	s.auditTicket(ctx, audit.EventTicketBugUnlink, audit.StateSuccess, "bug_unlink", ticketActorID(ticketID), map[string]interface{}{"bugId": bugID})
	return nil
}
