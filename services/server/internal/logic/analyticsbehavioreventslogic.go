package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsBehaviorEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsBehaviorEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsBehaviorEventsLogic {
	return &AnalyticsBehaviorEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsBehaviorEventsLogic) AnalyticsBehaviorEvents(req *types.AnalyticsBehaviorEventsQuery) (*types.AnalyticsBehaviorEventsResponse, error) {
	resp := &types.AnalyticsBehaviorEventsResponse{Events: []types.AnalyticsBehaviorEvent{}}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	start := strings.TrimSpace(req.Start)
	end := strings.TrimSpace(req.End)
	if start == "" || end == "" {
		t2 := time.Now()
		t1 := t2.Add(-24 * time.Hour)
		start = t1.Format(time.RFC3339)
		end = t2.Format(time.RFC3339)
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.Size
	if size <= 0 || size > 500 {
		size = 50
	}
	offset := (page - 1) * size
	where := " WHERE event_time>=toDateTime(?) AND event_time<=toDateTime(?)"
	args := []any{start, end}
	game := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	if game != "" {
		where += " AND game_id=?"
		args = append(args, game)
	}
	if env != "" {
		where += " AND env=?"
		args = append(args, env)
	}
	if ev := strings.TrimSpace(req.Event); ev != "" {
		where += " AND event=?"
		args = append(args, ev)
	}
	propKey := strings.TrimSpace(req.PropKey)
	propVal := strings.TrimSpace(req.PropVal)
	if propKey != "" && propVal != "" && validAnalyticsPropKey(propKey) {
		where += " AND JSON_VALUE(props_json, '$." + propKey + "') = ?"
		args = append(args, propVal)
	}
	if err := ch.QueryRow(l.ctx, "SELECT count() FROM analytics.events"+where, args...).Scan(&resp.Total); err != nil {
		return resp, nil
	}
	qry := fmt.Sprintf("SELECT event_time, event, user_id FROM analytics.events%s ORDER BY event_time DESC LIMIT %d OFFSET %d", where, size, offset)
	rows, err := ch.Query(l.ctx, qry, args...)
	if err != nil {
		return resp, nil
	}
	defer rows.Close()
	for rows.Next() {
		var ts time.Time
		var ev, uid string
		if err := rows.Scan(&ts, &ev, &uid); err != nil {
			continue
		}
		resp.Events = append(resp.Events, types.AnalyticsBehaviorEvent{
			Time:   ts.Format(time.RFC3339),
			Event:  ev,
			UserId: uid,
		})
	}
	return resp, nil
}
