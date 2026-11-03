// Package notify 把审批/告警事件分发到已配置的通知渠道。
//
// 渠道矩阵（docs/architecture/config-layering.md notification.*）：
//   - 站内信（默认开启）：写 messages 表 + SSE 推送，复用 message 模块
//   - 钉钉群机器人：notification.dingtalkUrl/Secret 配置后启用
//   - 企业微信群机器人：notification.wecomUrl 配置后启用（无加签，key 在 URL）
//   - 飞书群机器人：notification.feishuUrl/Secret 配置后启用（加签可选）
//   - 通用 webhook：notification.webhookUrl/Secret 配置后启用
//   - 邮件：notification.emailEnabled + smtp.* 配置后启用
//
// 所有渠道未配置时静默跳过（no-op），通知失败只记日志不阻塞业务——
// 审批/调用主流程的可用性优先于通知送达。
package notify

import (
	"context"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/settings"
)

// Service 分发通知事件。
type Service struct {
	layered      *settings.Layered
	messageModel *model.MessageModel
}

// New creates a notify service. layered/messageModel 均可为 nil（测试或
// 未初始化环境），此时仅外部渠道可用/全部跳过。
func New(layered *settings.Layered, messageModel *model.MessageModel) *Service {
	return &Service{layered: layered, messageModel: messageModel}
}

// Event 是通知事件的平台级形态（approvals.NotificationEvent 的简化入口）。
type Event struct {
	Type    string
	Title   string
	Message string
	// Recipients 是站内信接收人（用户名列表）；外部渠道不按人分发。
	Recipients []string
	Priority   string
	Data       map[string]interface{}
	// Channels 是调用方指定的事件→渠道路由白名单（渠道标识，见
	// channelAllowed）。为空表示不限制（发往所有已启用渠道）；非空时仅
	// 发往名单内且已配置启用的渠道。用于预算告警等按事件挑选渠道。
	Channels []string
}

// Dispatch 把事件发往所有已启用渠道。错误只记日志。
func (s *Service) Dispatch(ctx context.Context, ev Event) {
	if s == nil {
		return
	}
	s.dispatchInApp(ctx, ev)
	s.dispatchExternal(ctx, ev)
}

// channelAllowed 判断渠道标识是否在事件名单内。空名单=不限制。
// 支持别名：in_app/站内 → 站内信；dingtalk/ding → 钉钉；wecom/企业微信；
// feishu/飞书；webhook；email/邮件。
func channelAllowed(channels []string, names ...string) bool {
	if len(channels) == 0 {
		return true
	}
	for _, want := range channels {
		w := strings.ToLower(strings.TrimSpace(want))
		for _, n := range names {
			if w == n {
				return true
			}
		}
	}
	return false
}

func (s *Service) dispatchInApp(ctx context.Context, ev Event) {
	if s.messageModel == nil {
		return
	}
	if !channelAllowed(ev.Channels, "in_app", "站内") {
		return
	}
	if s.layered != nil && !s.layered.GetBool(settings.KeyNotifyInAppEnabled, true) {
		return
	}
	sent := make(map[string]struct{}, len(ev.Recipients))
	for _, to := range ev.Recipients {
		if to == "" {
			continue
		}
		if _, ok := sent[to]; ok {
			continue
		}
		sent[to] = struct{}{}
		dataJSON, err := model.EncodeData(ev.Data)
		if err != nil {
			dataJSON = nil
		}
		msg := &model.Message{
			To:      to,
			Type:    msgType(ev),
			Title:   ev.Title,
			Content: ev.Message,
			Data:    dataJSON,
			Status:  dbenum.MessageStatusUnread,
		}
		if err := s.messageModel.Create(ctx, msg); err != nil {
			slog.WarnContext(ctx, "notify: in-app message create failed", "to", to, "error", err)
		}
	}
}

func (s *Service) dispatchExternal(ctx context.Context, ev Event) {
	if s.layered == nil {
		return
	}
	ch := s.layered.NotifyChannels()
	approvalEvent := approvals.NotificationEvent{
		Type:     ev.Type,
		Title:    ev.Title,
		Message:  ev.Message,
		Priority: ev.Priority,
		Data:     ev.Data,
	}

	if ch.DingtalkURL != "" && channelAllowed(ev.Channels, "dingtalk", "ding") {
		sender := approvals.NewDingTalkSender(ch.DingtalkURL, ch.DingtalkSecret)
		if err := sender.Send(ctx, "", approvalEvent); err != nil {
			slog.WarnContext(ctx, "notify: dingtalk send failed", "error", err)
		}
	}
	if ch.WecomURL != "" && channelAllowed(ev.Channels, "wecom", "企业微信") {
		sender := approvals.NewWecomSender(ch.WecomURL)
		if err := sender.Send(ctx, "", approvalEvent); err != nil {
			slog.WarnContext(ctx, "notify: wecom send failed", "error", err)
		}
	}
	if ch.FeishuURL != "" && channelAllowed(ev.Channels, "feishu", "飞书") {
		sender := approvals.NewFeishuSender(ch.FeishuURL, ch.FeishuSecret)
		if err := sender.Send(ctx, "", approvalEvent); err != nil {
			slog.WarnContext(ctx, "notify: feishu send failed", "error", err)
		}
	}
	if ch.WebhookURL != "" && channelAllowed(ev.Channels, "webhook") {
		sender := approvals.NewWebhookSender(ch.WebhookURL, ch.WebhookSecret, "croupier")
		if err := sender.Send(ctx, joinRecipients(ev.Recipients), approvalEvent); err != nil {
			slog.WarnContext(ctx, "notify: webhook send failed", "error", err)
		}
	}
	if ch.EmailEnabled && channelAllowed(ev.Channels, "email", "邮件") {
		if smtp := s.layered.NotifySMTP(); smtp.Host != "" {
			sender := approvals.NewEmailSender(smtp.Host, smtp.Port, smtp.User, smtp.Password, smtp.From).
				WithTransport(smtp.Encryption, smtp.AuthType, smtp.InsecureSkipVerify)
			for _, to := range ev.Recipients {
				// 邮件按人发送（recipients 携带邮箱时）。
				if err := sender.Send(ctx, to, approvalEvent); err != nil {
					slog.WarnContext(ctx, "notify: email send failed", "to", to, "error", err)
				}
			}
		}
	}
}

// msgType 规范化事件类型为消息类型。
func msgType(ev Event) string {
	if ev.Type != "" {
		return ev.Type
	}
	return "system"
}

func joinRecipients(rs []string) string {
	out := ""
	for i, r := range rs {
		if i > 0 {
			out += ","
		}
		out += r
	}
	return out
}
