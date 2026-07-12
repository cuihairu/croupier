package feedback

import (
	"context"
	"errors"
	"strconv"
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

// List retrieves a paginated list of feedbacks
func (s *Service) List(ctx context.Context, req *FeedbackListRequest) (*FeedbackListResponse, error) {
	if s.svcCtx.FeedbackModel == nil {
		return nil, errors.New("反馈模型未初始化")
	}
	if req == nil {
		req = &FeedbackListRequest{}
	}

	opts := model.ListFeedbackOptions{
		PaginationOptions: model.NewPagination(req.Page, req.PageSize),
		Status:   strings.TrimSpace(req.Status),
		Category: strings.TrimSpace(req.Category),
		GameID:   strings.TrimSpace(req.GameId),
	}

	entries, total, err := s.svcCtx.FeedbackModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]Feedback, 0, len(entries))
	for i := range entries {
		items = append(items, buildFeedback(&entries[i]))
	}

	return &FeedbackListResponse{
		Items: items,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}

// Create creates a new feedback
func (s *Service) Create(ctx context.Context, req *FeedbackCreateRequest) (*FeedbackCreateResponse, error) {
	if s.svcCtx.FeedbackModel == nil {
		return nil, errors.New("反馈模型未初始化")
	}
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}

	contact := strings.TrimSpace(req.Contact)
	if contact == "" {
		return nil, errors.New("联系方式不能为空")
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errors.New("反馈内容不能为空")
	}

	category := strings.TrimSpace(req.Category)
	if category == "" {
		return nil, errors.New("反馈分类不能为空")
	}

	feedback := &model.Feedback{
		PlayerID: strings.TrimSpace(req.PlayerId),
		Contact:  contact,
		Content:  content,
		Category: category,
		Priority: "normal",
		Status:   "open",
		Rating:   utils.NormalizeFeedbackRating(req.Rating),
		Attach:   strings.TrimSpace(req.Attach),
		GameID:   strings.TrimSpace(req.GameId),
		Env:      strings.TrimSpace(req.Env),
	}

	if err := s.svcCtx.FeedbackModel.Create(ctx, feedback); err != nil {
		return nil, err
	}

	return &FeedbackCreateResponse{
		Feedback: buildFeedback(feedback),
	}, nil
}

// Update updates an existing feedback
func (s *Service) Update(ctx context.Context, req *FeedbackUpdateRequest) (*FeedbackUpdateResponse, error) {
	if s.svcCtx.FeedbackModel == nil {
		return nil, errors.New("反馈模型未初始化")
	}
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}

	id, err := strconv.ParseUint(req.ID, 10, 64)
	if err != nil {
		return nil, errors.New("反馈ID格式不正确")
	}

	updates := map[string]interface{}{}
	if status := strings.TrimSpace(req.Status); status != "" {
		updates["status"] = status
	}
	if priority := strings.TrimSpace(req.Priority); priority != "" {
		updates["priority"] = priority
	}
	if reply := strings.TrimSpace(req.Reply); reply != "" {
		updates["reply"] = reply
	} else if req.Reply != "" {
		updates["reply"] = ""
	}

	if len(updates) == 0 {
		return nil, errors.New("没有需要更新的字段")
	}

	if err := s.svcCtx.FeedbackModel.Update(ctx, uint(id), updates); err != nil {
		return nil, err
	}

	updated, err := s.svcCtx.FeedbackModel.FindByID(ctx, uint(id))
	if err != nil {
		return nil, err
	}

	return &FeedbackUpdateResponse{
		Feedback: buildFeedback(updated),
	}, nil
}

// Delete deletes a feedback
func (s *Service) Delete(ctx context.Context, req *FeedbackDeleteRequest) error {
	if s.svcCtx.FeedbackModel == nil {
		return errors.New("反馈模型未初始化")
	}
	if req == nil {
		return errors.New("请求体不能为空")
	}

	id, err := strconv.ParseUint(req.ID, 10, 64)
	if err != nil {
		return errors.New("反馈ID格式不正确")
	}

	if _, err := s.svcCtx.FeedbackModel.FindByID(ctx, uint(id)); err != nil {
		return err
	}

	return s.svcCtx.FeedbackModel.Delete(ctx, uint(id))
}

// Stats retrieves feedback statistics
func (s *Service) Stats(ctx context.Context, req *FeedbackStatsRequest) (*FeedbackStatsResponse, error) {
	if s.svcCtx.FeedbackModel == nil {
		return nil, errors.New("反馈模型未初始化")
	}
	if req == nil {
		req = &FeedbackStatsRequest{}
	}

	days := req.Days
	if days <= 0 {
		days = 7
	}

	stats, err := s.svcCtx.FeedbackModel.Stats(ctx, model.FeedbackStatsOptions{
		GameID: strings.TrimSpace(req.GameId),
		Days:   days,
	})
	if err != nil {
		return nil, err
	}

	byCategory := make(map[string]int, len(stats.ByCategory))
	for k, v := range stats.ByCategory {
		byCategory[k] = int(v)
	}
	byStatus := make(map[string]int, len(stats.ByStatus))
	for k, v := range stats.ByStatus {
		byStatus[k] = int(v)
	}

	response := FeedbackStatsResponse{
		FeedbackStats: FeedbackStats{
			Total:        int(stats.Total),
			ByCategory:   byCategory,
			ByStatus:     byStatus,
			AvgRating:    stats.AvgRating,
			ResponseRate: 0,
		},
	}
	if stats.Total > 0 {
		response.FeedbackStats.ResponseRate = float64(stats.Responded) / float64(stats.Total)
	}

	return &response, nil
}

func buildFeedback(record *model.Feedback) Feedback {
	if record == nil {
		return Feedback{}
	}
	return Feedback{
		Id:        int64(record.ID),
		PlayerId:  record.PlayerID,
		Contact:   record.Contact,
		Content:   record.Content,
		Category:  record.Category,
		Priority:  record.Priority,
		Status:    record.Status,
		Rating:    record.Rating,
		Attach:    record.Attach,
		GameId:    record.GameID,
		Env:       record.Env,
		Reply:     record.Reply,
		CreatedAt: utils.FormatTimestamp(record.CreatedAt),
		UpdatedAt: utils.FormatTimestamp(record.UpdatedAt),
	}
}
