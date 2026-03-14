package faq

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/datatypes"

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

// List retrieves a paginated list of FAQs
func (s *Service) List(ctx context.Context, req *FAQListRequest) (*FAQListResponse, error) {
	opts := model.ListFAQOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Category: strings.TrimSpace(req.Category),
		Keyword:  strings.TrimSpace(req.Keyword),
		Visible:  req.Visible,
	}

	items, total, err := s.svcCtx.FAQModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	faqs := make([]FAQ, 0, len(items))
	for i := range items {
		faqs = append(faqs, buildFAQResponse(&items[i]))
	}

	return &FAQListResponse{
		Items: faqs,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}

// Create creates a new FAQ
func (s *Service) Create(ctx context.Context, req *FAQCreateRequest) (*FAQCreateResponse, error) {
	question, answer, category, err := sanitizeFAQInput(req.Question, req.Answer, req.Category)
	if err != nil {
		return nil, err
	}

	faq := &model.FAQ{
		Question: question,
		Answer:   answer,
		Category: category,
		Tags:     encodeTags(req.Tags),
		Visible:  req.Visible,
		Sort:     req.Sort,
	}

	if err := s.svcCtx.FAQModel.Create(ctx, faq); err != nil {
		return nil, err
	}

	return &FAQCreateResponse{
		FAQ: buildFAQResponse(faq),
	}, nil
}

// Update updates an existing FAQ
func (s *Service) Update(ctx context.Context, req *FAQUpdateRequest) (*FAQUpdateResponse, error) {
	id, err := utils.ParseUintID(req.ID, "FAQ ID")
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})
	if v := strings.TrimSpace(req.Question); v != "" {
		updates["question"] = v
	}
	if v := strings.TrimSpace(req.Answer); v != "" {
		updates["answer"] = v
	}
	if v := strings.TrimSpace(req.Category); v != "" {
		updates["category"] = v
	}
	if req.Tags != nil {
		updates["tags"] = encodeTags(req.Tags)
	}
	if req.Visible != nil {
		updates["visible"] = *req.Visible
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}

	if len(updates) == 0 {
		return nil, errorx.NewBadRequest("请提供需要更新的字段")
	}

	if err := s.svcCtx.FAQModel.Update(ctx, id, updates); err != nil {
		return nil, err
	}

	faq, err := s.svcCtx.FAQModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	return &FAQUpdateResponse{
		FAQ: buildFAQResponse(faq),
	}, nil
}

// Delete deletes an FAQ
func (s *Service) Delete(ctx context.Context, req *FAQDeleteRequest) error {
	id, err := utils.ParseUintID(req.ID, "FAQ ID")
	if err != nil {
		return err
	}
	return s.svcCtx.FAQModel.Delete(ctx, id)
}

// Categories retrieves FAQ categories
func (s *Service) Categories(ctx context.Context, req *FAQCategoriesRequest) (*FAQCategoriesResponse, error) {
	cats, err := s.svcCtx.FAQModel.ListCategories(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]FAQCategory, 0, len(cats))
	for i := range cats {
		items = append(items, FAQCategory{
			Name:  cats[i].Name,
			Count: cats[i].Count,
		})
	}

	return &FAQCategoriesResponse{
		Items: items,
	}, nil
}

// Helper functions

func buildFAQResponse(faq *model.FAQ) FAQ {
	return FAQ{
		Id:        int64(faq.ID),
		Question:  faq.Question,
		Answer:    faq.Answer,
		Category:  faq.Category,
		Tags:      decodeTags(faq.Tags),
		Visible:   faq.Visible,
		Sort:      faq.Sort,
		Views:     faq.Views,
		CreatedAt: utils.FormatTimestamp(faq.CreatedAt),
		UpdatedAt: utils.FormatTimestamp(faq.UpdatedAt),
	}
}

func decodeTags(data datatypes.JSON) []string {
	if len(data) == 0 {
		return nil
	}
	var tags []string
	if err := json.Unmarshal(data, &tags); err != nil {
		return nil
	}
	return tags
}

func encodeTags(tags []string) datatypes.JSON {
	if len(tags) == 0 {
		return datatypes.JSON{}
	}
	norm := normalizeTags(tags)
	bytes, _ := json.Marshal(norm)
	return datatypes.JSON(bytes)
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		lower := strings.ToLower(tag)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func sanitizeFAQInput(question, answer, category string) (string, string, string, error) {
	q := strings.TrimSpace(question)
	if q == "" {
		return "", "", "", errorx.NewBadRequest("问题不能为空")
	}
	a := strings.TrimSpace(answer)
	if a == "" {
		return "", "", "", errorx.NewBadRequest("答案不能为空")
	}
	c := strings.TrimSpace(category)
	if c == "" {
		return "", "", "", errorx.NewBadRequest("分类不能为空")
	}
	return q, a, c, nil
}
