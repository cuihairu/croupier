package provider

import (
	"context"
	"errors"
	"testing"
)

// closeErrProvider：Close 返回错误，覆盖告警与 firstErr 汇总分支。
type closeErrProvider struct {
	mockProvider
	closeErr error
}

func (m *closeErrProvider) Close() error { return m.closeErr }

func TestRegisterInitError(t *testing.T) {
	r := NewRegistry(nil)
	p := &initErrProvider{err: errors.New("init boom")}
	err := r.Register(context.Background(), p, ProviderConfig{})
	if err == nil {
		t.Fatal("Register should fail when Init errors")
	}
}

type initErrProvider struct{ err error }

func (p *initErrProvider) Name() string                                          { return "init-err" }
func (p *initErrProvider) Init(ctx context.Context, config ProviderConfig) error { return p.err }
func (p *initErrProvider) IsEnabled() bool                                       { return true }
func (p *initErrProvider) SupportedMethods() []string                            { return nil }
func (p *initErrProvider) Call(ctx context.Context, method string, request []byte) ([]byte, error) {
	return nil, nil
}
func (p *initErrProvider) Close() error { return nil }

func TestUnregisterCloseErrorLogs(t *testing.T) {
	r := NewRegistry(nil)
	p := &closeErrProvider{mockProvider: mockProvider{name: "boom"}, closeErr: errors.New("close boom")}
	if err := r.Register(context.Background(), p, ProviderConfig{}); err != nil {
		t.Fatal(err)
	}
	// Close 失败只告警，不阻断注销
	if err := r.Unregister(context.Background(), "boom"); err != nil {
		t.Fatalf("Unregister should not fail on provider close error: %v", err)
	}
}

func TestRegistryCloseAggregatesErrors(t *testing.T) {
	r := NewRegistry(nil)
	p := &closeErrProvider{mockProvider: mockProvider{name: "boom"}, closeErr: errors.New("close boom")}
	if err := r.Register(context.Background(), p, ProviderConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err == nil {
		t.Fatal("Close should surface the first provider close error")
	}
}
