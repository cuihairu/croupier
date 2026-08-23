package platform

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
)

func TestStringInSlice_V7(t *testing.T) {
	tests := []struct {
		name   string
		list   []string
		target string
		want   bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b"}, "c", false},
		{"case insensitive", []string{"Hello", "World"}, "hello", true},
		{"empty list", nil, "a", false},
		{"whitespace", []string{"  hello  "}, "hello", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stringInSlice(tt.list, tt.target))
		})
	}
}

func TestResolveMethodsSource_V7(t *testing.T) {
	assert.Equal(t, "extension", resolveMethodsSource(true))
	assert.Equal(t, "", resolveMethodsSource(false))
}

func TestBuildExternalFunctionID_V7(t *testing.T) {
	id := buildExternalFunctionID("platform", "method")
	assert.NotEmpty(t, id)
}

func TestNewService_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	assert.NotNil(t, s)
	assert.Equal(t, svcCtx, s.svcCtx)
}

func TestService_ListPlatforms_NilCtx_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	resp, err := s.ListPlatforms(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestService_ListMethods_EmptyPlatform_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	resp, err := s.ListMethods(context.Background(), "")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 400, resp.Code)
	assert.Empty(t, resp.Methods)
}

func TestService_ListMethods_UnknownPlatform_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	resp, err := s.ListMethods(context.Background(), "unknown_platform")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Methods)
}

func TestService_Call_EmptyPlatform_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	resp, err := s.Call(context.Background(), &CallPlatformRequest{Platform: ""})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 400, resp.Code)
}

func TestService_Call_EmptyMethod_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	resp, err := s.Call(context.Background(), &CallPlatformRequest{Platform: "test", Method: ""})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 400, resp.Code)
}

func TestService_Call_InvalidRequest_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	// A nil request dereferences req.Platform and panics; documented behaviour.
	assert.Panics(t, func() {
		_, _ = s.Call(context.Background(), nil)
	})
}

func TestDiscoverExternalPlatforms_NilCtx_V7(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	result := s.discoverExternalPlatforms(context.Background())
	assert.NotNil(t, result)
}

func TestExtractPlatformMethodsFromBindings_Empty_V7(t *testing.T) {
	result := extractPlatformMethodsFromBindings(nil)
	assert.NotNil(t, result)
	assert.Empty(t, result)

	result = extractPlatformMethodsFromBindings([]model.ExtensionRuntimeBinding{})
	assert.Empty(t, result)
}
