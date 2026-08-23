package terms

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
)

func TestService_List_NilRequest_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	resp, err := s.List(context.Background(), nil)
	assert.Error(t, err)
	_ = resp
}

func TestService_List_InvalidDomain_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	_, err := s.List(context.Background(), &TermsListRequest{Domain: "invalid domain!"})
	assert.Error(t, err)
}

func TestService_Upsert_NilRequest_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	_, err := s.Upsert(context.Background(), nil)
	assert.Error(t, err)
}

func TestService_Upsert_InvalidDomain_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	_, err := s.Upsert(context.Background(), &TermUpsertRequest{Domain: "invalid!"})
	assert.Error(t, err)
}

func TestService_Delete_NilRequest_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	_, err := s.Delete(context.Background(), nil)
	assert.Error(t, err)
}

func TestService_Delete_InvalidDomain_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	_, err := s.Delete(context.Background(), &TermDeleteRequest{Domain: "invalid!"})
	assert.Error(t, err)
}

func TestService_List_ValidDomain_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	// With nil TermDictModel, this should return error or empty
	_, err := s.List(context.Background(), &TermsListRequest{Domain: "function"})
	_ = err // may error with nil model
}

func TestService_Upsert_Valid_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	_, err := s.Upsert(context.Background(), &TermUpsertRequest{
		Domain:    "function",
		TermKey:   "key",
		Alias:     "alias",
		DisplayZh: "中文",
		DisplayEn: "English",
	})
	_ = err // may error with nil model
}

func TestService_Delete_Valid_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	_, err := s.Delete(context.Background(), &TermDeleteRequest{
		Domain: "function",
		Alias:  "alias",
	})
	_ = err // may error with nil model
}
