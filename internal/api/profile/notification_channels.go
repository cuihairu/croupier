package profile

import (
	"context"

	"github.com/cuihairu/croupier/internal/platform/approvals"
	settings "github.com/cuihairu/croupier/internal/platform/settings"
)

// NotificationChannelState 描述一个通知通道对当前用户是否**真实可用**。
//
// 这份契约存在的唯一理由是消灭假状态：此前「登录通知」在用户填了手机号时
// 恒显示「已开启」，而仓内根本没有短信服务商（docs/BUGS.md BUG-016）。
// available 必须来自后端对「是否真的接入了可发送的服务商」的判断，
// 前端不得再根据「用户填了手机号」这类无关信号自行推断。
type NotificationChannelState struct {
	// Key 是通道标识：in_app / email / sms。
	Key string `json:"key"`
	// Available 为 true 仅当该通道已有可发送的实现且配置齐备。
	Available bool `json:"available"`
	// UserEnabled 反映用户是否开启了「希望收到该通道通知」的意愿。
	//
	// 与 available 是两个独立维度：通道可能整体不可用（available=false），
	// 而用户意愿仍是用户自己的选择，UI 应当如实呈现并在不可用时禁用开关。
	UserEnabled bool `json:"userEnabled"`
	// Reason 在 !available 时给出可展示的原因（已本地化或为稳定英文短语）。
	Reason string `json:"reason,omitempty"`
	// RequiresTarget 表明该通道需要用户填手机号/邮箱才能收得到。
	RequiresTarget bool `json:"requiresTarget"`
	// HasTarget 报告用户当前是否已填写该通道所需的目标（手机号/邮箱）。
	HasTarget bool `json:"hasTarget"`
	// Info 是服务商信息（仅在已接入时返回），用于展示「已接入：某服务商」。
	Info *approvals.SMSProviderInfo `json:"info,omitempty"`
}

// NotificationChannelsResponse 是 /api/v1/profile/notification-channels 的响应。
type NotificationChannelsResponse struct {
	Channels []NotificationChannelState `json:"channels"`
}

// NotificationBaseStatus 是平台侧（与短信无关）通道的事实来源。
//
// 抽成结构体 + 函数缝隙有两个原因：
//   - in_app / email 的真实状态在 platform settings 单例里，profile 包直接读
//     单例就没法在测试里构造「已配置 SMTP」这一状态；
//   - 让「可用性只来自后端判定」这条不变量有单一、可替换的取值点。
type NotificationBaseStatus struct {
	// InAppEnabled 站内信是否启用（默认 true，零配置即通）。
	InAppEnabled bool
	// EmailEnabled 用户是否开启邮件通知。
	EmailEnabled bool
	// SMTPConfigured SMTP 主机/端口是否齐备。EmailSender 未配置时是 no-op，
	// 此时声称邮件可用即为假状态。
	SMTPConfigured bool
}

// readBaseStatus 是生产实现：读平台设置单例。
func readBaseStatus() NotificationBaseStatus {
	layered := settings.Current()
	smtp := layered.NotifySMTP()
	return NotificationBaseStatus{
		InAppEnabled:   layered.GetBool(settings.KeyNotifyInAppEnabled, true),
		EmailEnabled:   layered.GetBool(settings.KeyNotifyEmailEnabled, false),
		SMTPConfigured: smtp.Host != "" && smtp.Port > 0,
	}
}

// GetNotificationChannels 报告各通知通道的真实可用性与用户意愿。
//
// 三条事实来源，都不是前端推断：
//   - in_app / email：platform settings 的 L3 覆盖层（站内信零配置即通；
//     邮件需要 SMTP 主机等配置齐备）；
//   - sms：SMSRegistry 里是否注册了具备凭据的服务商。
func (s *Service) GetNotificationChannels(ctx context.Context, username string) *NotificationChannelsResponse {
	admin, err := s.adminModel.FindByUsername(ctx, username)
	if err != nil || admin == nil {
		return &NotificationChannelsResponse{Channels: []NotificationChannelState{}}
	}

	base := s.baseStatus()

	// 站内信：消息模型就绪即可用，平台设置可关闭。
	inApp := NotificationChannelState{
		Key:         string(approvals.ChannelInApp),
		Available:   true,
		UserEnabled: base.InAppEnabled,
	}
	if !base.InAppEnabled {
		inApp.Reason = "in_app_disabled"
	}

	// 邮件：SMTP 未配置时 EmailSender 是 no-op，此时不得声称可用。
	email := NotificationChannelState{
		Key:            string(approvals.ChannelEmail),
		Available:      base.SMTPConfigured,
		UserEnabled:    base.EmailEnabled,
		RequiresTarget: true,
		HasTarget:      admin.Email != "",
	}
	if !base.SMTPConfigured {
		email.Reason = "smtp_not_configured"
	}

	// 短信：只看是否注册了可用服务商；没有就如实报未接入。
	smsStatus := s.smsRegistry().Status()
	sms := NotificationChannelState{
		Key:            string(approvals.ChannelSMS),
		Available:      smsStatus.Available,
		Reason:         smsStatus.Reason,
		Info:           smsStatus.Info,
		RequiresTarget: true,
		HasTarget:      admin.Phone != "",
	}

	return &NotificationChannelsResponse{
		Channels: []NotificationChannelState{inApp, email, sms},
	}
}

// smsRegistry 返回短信服务商注册表；未注入时构造一个「未接入」的实例，
// 保证 status 恒为 unavailable 而不是 panic。
func (s *Service) smsRegistry() *approvals.SMSRegistry {
	if s.sms != nil {
		return s.sms
	}
	return approvals.NewSMSRegistry()
}

// baseStatus 返回平台侧通道事实来源；未注入时读设置单例。
func (s *Service) baseStatus() NotificationBaseStatus {
	if s.notifyBase != nil {
		return s.notifyBase()
	}
	return readBaseStatus()
}
