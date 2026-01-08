// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package audit

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// maxPageSize limits the number of audit entries returned per page to
	// avoid overflow and excessive memory allocations.
	maxPageSize = 1000
	// maxPage is a very large upper bound on the page number to ensure that
	// (page-1)*size cannot overflow an int when size is bounded by maxPageSize.
	maxPage = 1_000_000_000
)

type AuditLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取审计日志
func NewAuditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuditLogic {
	return &AuditLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AuditLogic) Audit(req *types.AuditRequest) (resp *types.AuditResponse, err error) {
	if l.svcCtx == nil || l.svcCtx.OpsStateStore == nil {
		return nil, errors.New("audit store unavailable")
	}
	if req == nil {
		req = &types.AuditRequest{}
	}

	page := req.Page
	if page <= 0 {
		page = 1
	} else if page > maxPage {
		page = maxPage
	}
	size := req.PageSize
	if size <= 0 {
		size = 20
	}
	if size > maxPageSize {
		size = maxPageSize
	}

	actionFilter := strings.TrimSpace(req.Action)
	userFilter := strings.TrimSpace(req.UserID)

	state := l.svcCtx.OpsStateStore.Snapshot()
	entries := state.Audit.Entries

	filtered := make([]svc.OpsAuditEntry, 0, len(entries))
	for _, entry := range entries {
		if actionFilter != "" && !strings.EqualFold(entry.Action, actionFilter) {
			continue
		}
		if userFilter != "" && !strings.EqualFold(entry.UserID, userFilter) {
			continue
		}
		filtered = append(filtered, entry)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}

	items := make([]map[string]interface{}, 0, end-start)
	for _, entry := range filtered[start:end] {
		items = append(items, map[string]interface{}{
			"id":        entry.ID,
			"action":    entry.Action,
			"userId":    entry.UserID,
			"gameId":    entry.GameID,
			"env":       entry.Env,
			"target":    entry.Target,
			"result":    entry.Result,
			"traceId":   entry.TraceID,
			"metadata":  entry.Metadata,
			"createdAt": utils.FormatTimestamp(entry.CreatedAt),
		})
	}

	return &types.AuditResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"total": total,
			"page":  page,
			"size":  size,
		},
	}, nil
}
