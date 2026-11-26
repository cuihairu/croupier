package platform

import (
	"github.com/cuihairu/croupier/internal/svc"
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

func TestNewService(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	service := NewService(svcCtx)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.svcCtx != svcCtx {
		t.Fatal("expected svcCtx to be set")
	}
}
