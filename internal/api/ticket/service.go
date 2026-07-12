package ticket

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns a list of tickets
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	opts := model.TicketQueryOptions{
		PaginationOptions: model.NewPagination(req.Page, req.PageSize),
		Status:            strings.TrimSpace(req.Status),
		Category:          strings.TrimSpace(req.Category),
		Priority:          strings.TrimSpace(req.Priority),
		Assignee:          strings.TrimSpace(req.Assignee),
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

	if err := s.svcCtx.TicketModel.Create(ctx, ticket); err != nil {
		return nil, err
	}

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

	ticket, err := s.svcCtx.TicketModel.FindOne(ctx, id)
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
		updates["assignee"] = v
	}
	if req.Tags != nil {
		updates["tags"] = encodeTicketTags(req.Tags)
	}

	if len(updates) == 0 {
		return nil, errorx.NewBadRequest("请提供需要更新的字段")
	}

	if err := s.svcCtx.TicketModel.Update(ctx, id, updates); err != nil {
		return nil, err
	}

	ticket, err := s.svcCtx.TicketModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

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
	return s.svcCtx.TicketModel.Delete(ctx, id)
}

// Transition transitions a ticket to a new status
func (s *Service) Transition(ctx context.Context, req *TransitionRequest) (*TransitionResponse, error) {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return nil, err
	}

	status, err := sanitizeTicketStatus(req.Status)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"status": status,
	}
	if note := strings.TrimSpace(req.Note); note != "" {
		comment := addComment(commentAuthor(ctx), fmt.Sprintf("[状态变更] %s", note), id)
		if err := s.svcCtx.TicketModel.CreateComment(ctx, comment); err != nil {
			return nil, err
		}
	}

	if err := s.svcCtx.TicketModel.Update(ctx, id, updates); err != nil {
		return nil, err
	}

	ticket, err := s.svcCtx.TicketModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

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

	if _, err := s.svcCtx.TicketModel.FindOne(ctx, id); err != nil {
		return nil, err
	}

	comment := addComment(commentAuthor(ctx), content, id)
	if err := s.svcCtx.TicketModel.CreateComment(ctx, comment); err != nil {
		return nil, err
	}

	comments, err := s.svcCtx.TicketModel.ListComments(ctx, id)
	if err != nil {
		return nil, err
	}

	return &CreateCommentResponse{
		Items: buildCommentsDTO(comments),
	}, nil
}
