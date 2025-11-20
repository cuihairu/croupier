package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AnalyticsPaymentsTransactionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnalyticsPaymentsTransactionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnalyticsPaymentsTransactionsLogic {
	return &AnalyticsPaymentsTransactionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnalyticsPaymentsTransactionsLogic) AnalyticsPaymentsTransactions(req *types.AnalyticsPaymentsTransactionsQuery) (*types.AnalyticsPaymentsTransactionsResponse, error) {
	resp := &types.AnalyticsPaymentsTransactionsResponse{
		Transactions: []types.AnalyticsPaymentsTransaction{},
	}
	ch := l.svcCtx.ClickHouse()
	if ch == nil {
		return resp, nil
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.Size
	if size <= 0 || size > 200 {
		size = 20
	}
	offset := (page - 1) * size
	start, end := defaultRange(req.Start, req.End, 7)
	filters := map[string]string{
		"game_id":  strings.TrimSpace(req.GameId),
		"env":      strings.TrimSpace(req.Env),
		"channel":  strings.TrimSpace(req.Channel),
		"platform": strings.TrimSpace(req.Platform),
		"country":  strings.TrimSpace(req.Country),
		"region":   strings.TrimSpace(req.Region),
		"city":     strings.TrimSpace(req.City),
		"status":   strings.TrimSpace(req.Status),
	}
	where, args := buildPaymentsWhere(start, end, filters)
	if err := ch.QueryRow(l.ctx, "SELECT count() FROM analytics.payments"+where, args...).Scan(&resp.Total); err != nil {
		return resp, nil
	}
	qry := "SELECT time, order_id, user_id, amount_cents, status, channel FROM analytics.payments" + where + " ORDER BY time DESC LIMIT " + strconv.Itoa(size) + " OFFSET " + strconv.Itoa(offset)
	rows, err := ch.Query(l.ctx, qry, args...)
	if err != nil {
		return resp, nil
	}
	defer rows.Close()
	for rows.Next() {
		var ts time.Time
		var orderID, userID, status, channel string
		var amount uint64
		if err := rows.Scan(&ts, &orderID, &userID, &amount, &status, &channel); err != nil {
			continue
		}
		resp.Transactions = append(resp.Transactions, types.AnalyticsPaymentsTransaction{
			Time:        ts.Format(time.RFC3339),
			OrderId:     orderID,
			UserId:      userID,
			AmountCents: amount,
			Status:      status,
			Channel:     channel,
		})
	}
	return resp, nil
}
