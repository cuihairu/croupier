package approval

import (
	"context"
	"log/slog"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	notify "github.com/cuihairu/croupier/internal/service/notify"
)

// notifyApprovalEvent 把审批事件分发到已配置渠道。
//
// 接收人解析顺序：
//  1. 站内信：具有审批权限的管理员（role=admin/super_admin），叠加发起人
//  2. 外部渠道（钉钉/webhook）：群级推送，不按人
//  3. 邮件：接收人非邮箱时跳过（recipients 是用户名）
//
// 通知失败只记日志，绝不阻塞审批主流程。
func (s *Service) notifyApprovalEvent(ctx context.Context, event string, record *approvals.Approval, title, message string) {
	if s == nil || s.svcCtx == nil || s.svcCtx.NotifyService == nil || record == nil {
		return
	}
	recipients, err := s.approvalRecipients(ctx)
	if err != nil {
		slog.WarnContext(ctx, "approval notify: resolve recipients failed", "error", err)
		recipients = []string{}
	}
	if record.Actor != "" {
		recipients = append(recipients, record.Actor)
	}
	s.svcCtx.NotifyService.Dispatch(ctx, notify.Event{
		Type:       event,
		Title:      title,
		Message:    message,
		Recipients: dedupe(recipients),
		Priority:   "normal",
		Data: map[string]interface{}{
			"approvalId": record.ID,
			"functionId": record.FunctionID,
			"gameId":     record.GameID,
			"env":        record.Env,
			"state":      string(record.State),
			"actor":      record.Actor,
		},
	})
}

// approvalRecipients 列出有审批权限的管理员用户名（admin 角色）。
func (s *Service) approvalRecipients(ctx context.Context) ([]string, error) {
	if s.svcCtx.AdminModel == nil {
		return nil, nil
	}
	active := 1
	admins, _, err := s.svcCtx.AdminModel.List(ctx, model.ListAdminsOptions{Role: "admin", Status: &active, Page: 1, PageSize: 200})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(admins))
	for i := range admins {
		if admins[i].Username != "" {
			out = append(out, admins[i].Username)
		}
	}
	return out, nil
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
