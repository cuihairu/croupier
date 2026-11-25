package approval

import (
	"context"
	"log/slog"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	notify "github.com/cuihairu/croupier/internal/service/notify"
)

// notifyApprovalEvent dispatches an approval event to the configured channels.
//
// Recipient resolution order:
//  1. in-app messages: admins with approval permission (role=admin/super_admin),
//     plus the requester
//  2. external channels (dingtalk/webhook): group-level push, not per person
//  3. email: skipped when the recipient is not an email address (recipients are
//     usernames)
//
// A notification failure is only logged; it must never block the approval main
// flow.
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
			"approver":   record.Approver,
		},
	})
}

// approvalRecipients lists the usernames of admins with approval permission
// (admin role).
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
