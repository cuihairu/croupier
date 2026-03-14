package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

const (
	defaultPaymentsWindowDays = 7
	maxTrendPoints            = 90
)

func payments(ctx context.Context, svcCtx *svc.ServiceContext, req *PaymentsRequest) (*PaymentsResponse, error) {
	if svcCtx.PaymentsModel == nil {
		return nil, errors.New("payments model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolvePaymentsRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)

	agg, err := svcCtx.PaymentsModel.AggregateRevenue(ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}

	activeUsers := agg.Payers
	if svcCtx.BehaviorModel != nil {
		if count, countErr := svcCtx.BehaviorModel.CountDistinctUsers(ctx, gameID, env, start, end); countErr == nil && count > 0 {
			activeUsers = count
		}
	}

	arpuBase := float64(activeUsers)
	metrics := PaymentsMetrics{
		Revenue:      agg.Revenue,
		Transactions: int(agg.Transactions),
		PayingUsers:  int(agg.Payers),
		ARPU:         safeDivide(agg.Revenue, arpuBase),
		ARPPU:        safeDivide(agg.Revenue, float64(agg.Payers)),
	}
	if arpuBase > 0 {
		metrics.ConversionRate = safeDivide(float64(agg.Payers), arpuBase)
	}

	revenueStats, err := svcCtx.PaymentsModel.DailyRevenue(ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}

	trends := buildPaymentsTrends(revenueStats)

	return &PaymentsResponse{
		Metrics: metrics,
		Trends:  trends,
	}, nil
}

func paymentsIngest(ctx context.Context, svcCtx *svc.ServiceContext, req *PaymentsIngestRequest) (*PaymentsIngestResponse, error) {
	if svcCtx.PaymentsModel == nil {
		return nil, errors.New("payments model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}
	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errors.New("gameId 不能为空")
	}

	env := strings.TrimSpace(req.Env)

	rawEntries, err := decodeTransactionsPayload(req.Transactions)
	if err != nil {
		return nil, err
	}

	var accepted, rejected int
	for _, entry := range rawEntries {
		tx, buildErr := buildTransaction(entry, gameID, env)
		if buildErr != nil {
			rejected++
			continue
		}
		if err := svcCtx.PaymentsModel.CreateTransaction(ctx, tx); err != nil {
			rejected++
			continue
		}
		accepted++
	}

	return &PaymentsIngestResponse{
		Accepted: accepted,
		Rejected: rejected,
		BatchId:  uuid.New().String(),
	}, nil
}

func paymentsProductTrend(ctx context.Context, svcCtx *svc.ServiceContext, req *PaymentsProductTrendRequest) (*PaymentsProductTrendResponse, error) {
	if svcCtx.PaymentsModel == nil {
		return nil, errors.New("payments model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := helper.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	items, err := svcCtx.PaymentsModel.ListProductTrends(ctx, strings.TrimSpace(req.GameId), strings.TrimSpace(req.Env))
	if err != nil {
		return nil, err
	}

	respItems := make([]ProductTrend, 0, len(items))
	for _, item := range items {
		if !start.IsZero() && item.WindowEnd.Before(start) {
			continue
		}
		if !end.IsZero() && item.WindowStart.After(end) {
			continue
		}
		respItems = append(respItems, ProductTrend{
			ProductId:   item.ProductID,
			ProductName: item.ProductName,
			Revenue:     item.Revenue,
			Sales:       item.Sales,
			Growth:      item.Growth,
		})
		if len(respItems) >= limit {
			break
		}
	}

	return &PaymentsProductTrendResponse{
		Items: respItems,
	}, nil
}

func paymentsSummary(ctx context.Context, svcCtx *svc.ServiceContext, req *PaymentsSummaryRequest) (*PaymentsSummaryResponse, error) {
	if svcCtx.PaymentsModel == nil {
		return nil, errors.New("payments model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	start, end, err := resolvePaymentsRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)

	stats, err := svcCtx.PaymentsModel.DailyRevenue(ctx, gameID, env, start, end)
	if err != nil {
		return nil, err
	}

	items := summarizePayments(stats, req.GroupBy)
	return &PaymentsSummaryResponse{
		Items: items,
	}, nil
}

func paymentsTransactions(ctx context.Context, svcCtx *svc.ServiceContext, req *PaymentsTransactionsRequest) (*PaymentsTransactionsResponse, error) {
	if svcCtx.PaymentsModel == nil {
		return nil, errors.New("payments model unavailable")
	}
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.PageSize
	if size <= 0 {
		size = 20
	}

	start, end, err := helper.NormalizeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	opts := model.PaymentQueryOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     page,
			PageSize: size,
		},
		GameID:    strings.TrimSpace(req.GameId),
		Env:       strings.TrimSpace(req.Env),
		Status:    strings.TrimSpace(req.Status),
		StartTime: start,
		EndTime:   end,
	}

	items, total, err := svcCtx.PaymentsModel.ListTransactions(ctx, opts)
	if err != nil {
		return nil, err
	}

	respItems := make([]PaymentTransaction, 0, len(items))
	for _, tx := range items {
		respItems = append(respItems, convertTransaction(tx))
	}

	return &PaymentsTransactionsResponse{
		Items: respItems,
		Total: int(total),
		Page:  page,
		Size:  size,
	}, nil
}

// Helper functions for payments analytics

func resolvePaymentsRange(startRaw, endRaw string) (time.Time, time.Time, error) {
	start, end, err := helper.NormalizeDateRange(startRaw, endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	now := time.Now().UTC()
	if end.IsZero() {
		end = now
	}
	if start.IsZero() {
		start = end.Add(-defaultPaymentsWindowDays * 24 * time.Hour)
	}
	return start, end, nil
}

func convertTransaction(tx model.PaymentTransaction) PaymentTransaction {
	return PaymentTransaction{
		Id:            tx.TransactionID,
		UserId:        tx.UserID,
		ProductId:     tx.ProductID,
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		Status:        tx.Status,
		PaymentMethod: tx.PaymentMethod,
		CreatedAt:     helper.FormatTimestamp(tx.OccurredAt),
	}
}

func buildPaymentsTrends(stats []model.DailyRevenueStat) map[string]interface{} {
	if len(stats) > maxTrendPoints {
		stats = stats[len(stats)-maxTrendPoints:]
	}
	revenue := make([]map[string]interface{}, 0, len(stats))
	transactions := make([]map[string]interface{}, 0, len(stats))
	payers := make([]map[string]interface{}, 0, len(stats))

	for _, stat := range stats {
		date := stat.Day.Format("2006-01-02")
		revenue = append(revenue, map[string]interface{}{
			"date":    date,
			"value":   stat.Revenue,
			"payers":  stat.Payers,
			"orders":  stat.Transactions,
			"avgSale": safeDivide(stat.Revenue, float64(stat.Transactions)),
		})
		transactions = append(transactions, map[string]interface{}{
			"date":  date,
			"value": stat.Transactions,
		})
		payers = append(payers, map[string]interface{}{
			"date":  date,
			"value": stat.Payers,
		})
	}

	return map[string]interface{}{
		"revenue":      revenue,
		"transactions": transactions,
		"payers":       payers,
	}
}

func summarizePayments(stats []model.DailyRevenueStat, groupBy string) []PaymentsSummary {
	if len(stats) == 0 {
		return []PaymentsSummary{}
	}
	groupBy = strings.ToLower(strings.TrimSpace(groupBy))
	if groupBy == "" {
		groupBy = "day"
	}

	switch groupBy {
	case "week":
		return aggregateBy(stats, func(t time.Time) string {
			year, week := t.ISOWeek()
			return fmt.Sprintf("%d-W%02d", year, week)
		})
	case "month":
		return aggregateBy(stats, func(t time.Time) string {
			return t.Format("2006-01")
		})
	default:
		items := make([]PaymentsSummary, 0, len(stats))
		for _, stat := range stats {
			items = append(items, PaymentsSummary{
				Date:         stat.Day.Format("2006-01-02"),
				Revenue:      stat.Revenue,
				Transactions: int(stat.Transactions),
				Users:        int(stat.Payers),
			})
		}
		return items
	}
}

func aggregateBy(stats []model.DailyRevenueStat, keyFn func(time.Time) string) []PaymentsSummary {
	buckets := make(map[string]*PaymentsSummary)
	order := []string{}
	for _, stat := range stats {
		key := keyFn(stat.Day)
		if key == "" {
			key = stat.Day.Format("2006-01-02")
		}
		if _, ok := buckets[key]; !ok {
			buckets[key] = &PaymentsSummary{
				Date: key,
			}
			order = append(order, key)
		}
		entry := buckets[key]
		entry.Revenue += stat.Revenue
		entry.Transactions += int(stat.Transactions)
		entry.Users += int(stat.Payers)
	}

	items := make([]PaymentsSummary, 0, len(order))
	for _, key := range order {
		items = append(items, *buckets[key])
	}
	return items
}

func decodeTransactionsPayload(raw interface{}) ([]map[string]interface{}, error) {
	if raw == nil {
		return []map[string]interface{}{}, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func buildTransaction(entry map[string]interface{}, gameID, env string) (*model.PaymentTransaction, error) {
	if entry == nil {
		return nil, errors.New("empty transaction entry")
	}

	txID := pickString(entry, "id", "transactionId", "transaction_id")
	if txID == "" {
		txID = uuid.New().String()
	}
	userID := pickString(entry, "userId", "playerId", "player_id")
	if userID == "" {
		return nil, errors.New("缺少用户ID")
	}

	amount := parseFloatValue(entry["amount"])
	currency := pickString(entry, "currency")
	if currency == "" {
		currency = "USD"
	}

	status := pickString(entry, "status")
	if status == "" {
		status = "success"
	}

	paymentMethod := pickString(entry, "paymentMethod", "method")
	productID := pickString(entry, "productId", "product_id")
	productName := pickString(entry, "productName", "product_name")
	timestampStr := pickString(entry, "timestamp", "occurredAt", "createdAt")
	occurredAt, err := helper.ParseDate(timestampStr)
	if err != nil || occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	return &model.PaymentTransaction{
		TransactionID: txID,
		GameID:        gameID,
		Env:           env,
		UserID:        userID,
		ProductID:     productID,
		ProductName:   productName,
		Amount:        amount,
		Currency:      currency,
		Status:        status,
		PaymentMethod: paymentMethod,
		OccurredAt:    occurredAt,
	}, nil
}

func parseFloatValue(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return 0
}
