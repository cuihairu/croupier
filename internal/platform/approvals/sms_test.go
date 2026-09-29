package approvals

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// stubSMSProvider 记录发送参数的可编程 SMSProvider。
type stubSMSProvider struct {
	configured bool
	info       SMSProviderInfo
	sendErr    error

	mu   sync.Mutex
	sent []string
}

func (s *stubSMSProvider) Send(_ context.Context, recipient, template string, params map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, recipient+"|"+template)
	if s.sendErr != nil {
		return s.sendErr
	}
	return nil
}

func (s *stubSMSProvider) Configured() bool { return s.configured }
func (s *stubSMSProvider) Describe() SMSProviderInfo {
	return s.info
}

// TestSMSRegistryDefaultUnconfigured 未接入时 Status 不得假报「已开启」
// （BUG-016 的接入点契约：available 只认 provider 自报凭据）。
func TestSMSRegistryDefaultUnconfigured(t *testing.T) {
	r := NewSMSRegistry()
	if r.Provider().Configured() {
		t.Fatal("默认 registry 不应报告已配置")
	}
	status := r.Status()
	if status.Available {
		t.Fatal("未接入时 available 必须为 false（假已开启即 BUG-016）")
	}
	if status.Reason == "" {
		t.Fatal("未接入时必须带原因")
	}
	if err := r.Send(context.Background(), "+8613800000000", "tpl", nil); !errors.Is(err, ErrSMSNotConfigured) {
		t.Fatalf("未接入时 Send 应返回 ErrSMSNotConfigured，got %v", err)
	}
}

// TestSMSRegistrySendEmptyRecipient 空收件人在进入 provider 前被拒。
func TestSMSRegistrySendEmptyRecipient(t *testing.T) {
	r := NewSMSRegistry()
	p := &stubSMSProvider{configured: true}
	r.RegisterSMSProvider(p)
	if err := r.Send(context.Background(), "   ", "tpl", nil); err == nil || strings.Contains(err.Error(), "not configured") {
		t.Fatalf("空收件人应返回专用错误，got %v", err)
	}
	if len(p.sent) != 0 {
		t.Fatalf("provider 不应被调用，sent=%v", p.sent)
	}
}

// TestSMSRegistryRegisteredAvailable 注册已配置 provider 后 Status 可用、
// Send 透传（收件人两侧空白被归一）。
func TestSMSRegistryRegisteredAvailable(t *testing.T) {
	r := NewSMSRegistry()
	p := &stubSMSProvider{
		configured: true,
		info:       SMSProviderInfo{Provider: "acme", Signature: "Croupier", Template: "tpl-1"},
	}
	r.RegisterSMSProvider(p)

	status := r.Status()
	if !status.Available || status.Info == nil {
		t.Fatalf("已配置时 Status 应可用，got %+v", status)
	}
	if status.Info.Provider != "acme" || status.Info.Template != "tpl-1" {
		t.Fatalf("Describe 信息透传不完整，got %+v", status.Info)
	}

	if err := r.Send(context.Background(), "  +8613800000000  ", "tpl", map[string]string{"code": "1234"}); err != nil {
		t.Fatalf("已配置 provider 的 Send 不应失败，got %v", err)
	}
	if got := p.sent[0]; got != "+8613800000000|tpl" {
		t.Fatalf("收件人应被 TrimSpace 后透传，got %q", got)
	}
}

// TestSMSRegistryRegisterNilFallsBack 传 nil 回落为未接入。
func TestSMSRegistryRegisterNilFallsBack(t *testing.T) {
	r := NewSMSRegistry()
	r.RegisterSMSProvider(&stubSMSProvider{configured: true})
	r.RegisterSMSProvider(nil)
	if status := r.Status(); status.Available {
		t.Fatal("回落 nil 后 available 必须为 false")
	}
}

// TestUnconfiguredSMSProvider 默认实现的三个方法契约。
func TestUnconfiguredSMSProvider(t *testing.T) {
	p := unconfiguredSMSProvider{}
	if p.Configured() {
		t.Fatal("unconfigured 不应自称已配置")
	}
	if info := p.Describe(); info.Provider != "" {
		t.Fatalf("unconfigured Describe 应为零值，got %+v", info)
	}
	if err := p.Send(context.Background(), "x", "tpl", nil); !errors.Is(err, ErrSMSNotConfigured) {
		t.Fatalf("unconfigured Send 应返回 ErrSMSNotConfigured，got %v", err)
	}
}

// TestHTTPSMSProviderConfiguredMatrix endpoint/apiKey/template 三者齐备才算可用。
func TestHTTPSMSProviderConfiguredMatrix(t *testing.T) {
	cases := []struct {
		endpoint, apiKey, template string
		want                       bool
	}{
		{"https://sms", "key", "tpl", true},
		{"", "key", "tpl", false},
		{"https://sms", "", "tpl", false},
		{"https://sms", "key", "", false},
	}
	for i, c := range cases {
		p := &httpSMSProvider{endpoint: c.endpoint, apiKey: c.apiKey, template: c.template}
		if got := p.Configured(); got != c.want {
			t.Fatalf("case %d: Configured() = %v, want %v", i, got, c.want)
		}
	}
}

// TestHTTPSMSProviderSend 未配置返回稳定错误；已配置返回「未实现」错误且
// 空模板回落到默认模板（接入点骨架的现状行为）。
func TestHTTPSMSProviderSend(t *testing.T) {
	unset := &httpSMSProvider{}
	if err := unset.Send(context.Background(), "x", "tpl", nil); !errors.Is(err, ErrSMSNotConfigured) {
		t.Fatalf("未配置应返回 ErrSMSNotConfigured，got %v", err)
	}

	p := &httpSMSProvider{provider: "acme", endpoint: "https://sms", apiKey: "k", template: "tpl-default"}
	err := p.Send(context.Background(), "+8613800000000", "", map[string]string{"code": "1"})
	if err == nil || !strings.Contains(err.Error(), "not implemented yet") {
		t.Fatalf("参考骨架应返回未实现错误，got %v", err)
	}
	if !strings.Contains(err.Error(), "tpl-default") {
		t.Fatalf("空模板应回落默认模板，got %v", err)
	}
	if !strings.Contains(err.Error(), "+8613800000000") {
		t.Fatalf("错误信息应包含收件人便于排查，got %v", err)
	}
}

// TestHTTPSMSProviderDescribe 参考实现的 Describe 方法透传配置信息。
func TestHTTPSMSProviderDescribe(t *testing.T) {
	p := &httpSMSProvider{
		provider:  "acme",
		signature: "Croupier",
		template:  "tpl-1",
		endpoint:  "https://sms",
		apiKey:    "key",
	}
	info := p.Describe()
	if info.Provider != "acme" || info.Signature != "Croupier" || info.Template != "tpl-1" {
		t.Fatalf("Describe 应透传配置，got %+v", info)
	}

	// 零值时返回零值结构体
	var zero httpSMSProvider
	info = zero.Describe()
	if info.Provider != "" || info.Signature != "" || info.Template != "" {
		t.Fatalf("零值 Describe 应为零值，got %+v", info)
	}
}

// TestSMSRegistryConcurrentSwap 热替换与并发发送互不 panic（-race 下运行）。
func TestSMSRegistryConcurrentSwap(t *testing.T) {
	r := NewSMSRegistry()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			r.RegisterSMSProvider(&stubSMSProvider{configured: i%2 == 0})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = r.Send(context.Background(), "+8613800000000", "tpl", nil)
		}
	}()
	wg.Wait()
}
