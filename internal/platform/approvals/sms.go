package approvals

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// 短信通道（SMS）接入点
// ---------------------------------------------------------------------------
//
// 背景：本仓库此前把「用户有手机号」当作「登录短信通知已开启」展示给用户，
// 但仓内**没有任何短信服务商实现**（`ChannelSMS` 只是一个没人使用的枚举常量，
// 没有 sender、没有配置项、没有 UI 入口）。界面因此显示了一个永远为真的
// 「已开启」，属于禁止的假状态（docs/BUGS.md BUG-016）。
//
// 本文件把接入点固定下来：
//
//   - `SMSProvider` 是唯一需要实现的接口（3 个方法）；
//   - `ErrSMSNotConfigured` 是「未接入」的稳定错误，供上层判定状态；
//   - 默认实例 `unconfiguredSMSProvider` 恒返回该错误，因此未接入时任何发送
//     尝试都会明确失败，而不会静默成功（静默成功正是「假已开启」的根源）。
//
// 接入步骤（任选一家 HTTP API 即可，无需引入 SDK）：
//  1. 实现 SMSProvider.Send：调用服务商 HTTP 接口投递；
//  2. 在 internal/service/notify 的装配处（见 NotifyChannelsResolved）读取
//     notification.sms* 配置并 RegisterSMSProvider；
//  3. 前端 /api/v1/me/notification-channels 会自动把 sms.available 置为 true。
//
// 已内置的真实可发送实现见同包的 EmailSender / DingTalkSender /
// FeishuSender / WecomSender / WebhookSender——SMS 接入应与它们同构。

// ErrSMSNotConfigured 表示短信通道未接入服务商。
var ErrSMSNotConfigured = errors.New("sms: notification provider is not configured")

// SMSProvider 是短信通道的接入接口。
//
// 与 NotificationSender 的区别：短信需要**模板**与**签名**（各服务商强制要求，
// 不能把 body 原样当消息体），且发送结果需要区分「未接入」与「发送失败」。
type SMSProvider interface {
	// Send 投递一条短信。recipient 是手机号（E.164 或服务商要求的本地格式，
	// 具体归一由实现负责）。未接入时必须返回 ErrSMSNotConfigured。
	Send(ctx context.Context, recipient, template string, params map[string]string) error

	// Configured 报告该实现是否已具备可发送的凭据。
	Configured() bool

	// Describe 返回接入信息（服务商名/模板 ID 等），仅用于展示与审计。
	Describe() SMSProviderInfo
}

// SMSProviderInfo 描述已接入的短信服务商。
type SMSProviderInfo struct {
	Provider  string `json:"provider"`
	Signature string `json:"signature,omitempty"`
	Template  string `json:"template,omitempty"`
}

// SMSRegistry 持有当前生效的短信服务商。
//
// 并发安全：设置页保存短信配置后需要热替换 provider，而发送路径可能同时在跑。
type SMSRegistry struct {
	mu       sync.RWMutex
	provider SMSProvider
}

// NewSMSRegistry 建一个未接入的注册表。
func NewSMSRegistry() *SMSRegistry { return &SMSRegistry{} }

// RegisterSMSProvider 装入（或替换）服务商；传 nil 表示回落为「未接入」。
func (r *SMSRegistry) RegisterSMSProvider(p SMSProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.provider = p
}

// Provider 返回当前服务商，未接入时返回 unconfiguredSMSProvider。
func (r *SMSRegistry) Provider() SMSProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.provider == nil {
		return unconfiguredSMSProvider{}
	}
	return r.provider
}

// Status 报告短信通道当前是否真正可用。
type SMSStatus struct {
	Available bool             `json:"available"`
	Reason    string           `json:"reason"`
	Info      *SMSProviderInfo `json:"info,omitempty"`
}

// Status 返回可直接下发给前端的真实状态。
//
// available 只在「已注册 provider 且 provider 自称凭据齐备」时为 true——
// 既不依据「用户填了手机号」这类无关信号，也不写死。
func (r *SMSRegistry) Status() SMSStatus {
	p := r.Provider()
	if !p.Configured() {
		return SMSStatus{
			Available: false,
			Reason:    "sms provider not configured",
		}
	}
	info := p.Describe()
	return SMSStatus{Available: true, Info: &info}
}

// Send 经当前服务商投递。
func (r *SMSRegistry) Send(ctx context.Context, recipient, template string, params map[string]string) error {
	rec := strings.TrimSpace(recipient)
	if rec == "" {
		return errors.New("sms: recipient is empty")
	}
	return r.Provider().Send(ctx, rec, template, params)
}

// unconfiguredSMSProvider 是默认实现：恒为「未接入」。
type unconfiguredSMSProvider struct{}

func (unconfiguredSMSProvider) Send(context.Context, string, string, map[string]string) error {
	return ErrSMSNotConfigured
}
func (unconfiguredSMSProvider) Configured() bool { return false }
func (unconfiguredSMSProvider) Describe() SMSProviderInfo {
	return SMSProviderInfo{}
}

var _ SMSProvider = unconfiguredSMSProvider{}

// httpSMSProvider 是接入点的参考实现骨架：把「凭据齐备」与「实际发送」两处
// 留空 TODO，便于接入方按服务商文档补齐。生产不会构造它（NewSMSRegistry
// 默认返回未接入实现），保留它是为了让接入点有可直接对照的形状。
type httpSMSProvider struct {
	provider  string
	signature string
	template  string
	endpoint  string
	apiKey    string
}

func (p *httpSMSProvider) Configured() bool {
	return p.endpoint != "" && p.apiKey != "" && p.template != ""
}

func (p *httpSMSProvider) Describe() SMSProviderInfo {
	return SMSProviderInfo{Provider: p.provider, Signature: p.signature, Template: p.template}
}

func (p *httpSMSProvider) Send(_ context.Context, recipient, template string, params map[string]string) error {
	if !p.Configured() {
		return ErrSMSNotConfigured
	}
	if template == "" {
		template = p.template
	}
	// TODO(接入点): 按所选服务商的 HTTP 协议组装请求并投递
	// （阿里云 dysmsapi / 腾讯云 sms / Twilio 均为 HTTPS + JSON 或表单）。
	// 务必在此处理服务商返回的逐条状态码，不要只看 HTTP 200。
	return fmt.Errorf("sms: %s sender not implemented yet (recipient=%s template=%s params=%v)",
		p.provider, recipient, template, params)
}

var _ SMSProvider = (*httpSMSProvider)(nil)
