package terms

import (
	"context"

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

// List retrieves terms for a given domain
func (s *Service) List(ctx context.Context, req *TermsListRequest) (*TermsListResponse, error) {
	if req == nil {
		req = &TermsListRequest{}
	}
	if _, err := model.NormalizeTermDictionaryDomain(req.Domain); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	terms, err := s.svcCtx.TermDictModel.List(ctx, req.Domain)
	if err != nil {
		return nil, err
	}

	items := make([]TermItem, 0, len(terms))
	for _, term := range terms {
		items = append(items, TermItem{
			Id:        int64(term.ID),
			Domain:    term.Domain,
			TermKey:   term.TermKey,
			Alias:     term.Alias,
			DisplayZh: term.DisplayZh,
			DisplayEn: term.DisplayEn,
			Order:     int64(term.SortOrder),
		})
	}

	return &TermsListResponse{
		Items: items,
	}, nil
}

// Upsert creates or updates a term
func (s *Service) Upsert(ctx context.Context, req *TermUpsertRequest) (*TermUpsertResponse, error) {
	if req == nil {
		return nil, errorx.NewBadRequest("term request is required")
	}
	if _, err := model.NormalizeTermDictionaryDomain(req.Domain); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	term := &model.TermDictionary{
		Domain:    req.Domain,
		TermKey:   req.TermKey,
		Alias:     req.Alias,
		DisplayZh: req.DisplayZh,
		DisplayEn: req.DisplayEn,
		SortOrder: int(req.Order),
	}

	err := s.svcCtx.TermDictModel.Upsert(ctx, term)
	if err != nil {
		return nil, err
	}

	return &TermUpsertResponse{
		Ok: true,
	}, nil
}

// Delete removes a term by domain and alias
func (s *Service) Delete(ctx context.Context, req *TermDeleteRequest) (*TermDeleteResponse, error) {
	if req == nil {
		return nil, errorx.NewBadRequest("term delete request is required")
	}
	if _, err := model.NormalizeTermDictionaryDomain(req.Domain); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	err := s.svcCtx.TermDictModel.DeleteByAlias(ctx, req.Domain, req.Alias)
	if err != nil {
		return nil, err
	}

	return &TermDeleteResponse{
		Ok: true,
	}, nil
}
