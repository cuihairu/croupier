package analytics_payments

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

const (
	defaultPaymentsWindowDays = 7
	maxTrendPoints            = 90
)

func resolvePaymentsRange(startRaw, endRaw string) (time.Time, time.Time, error) {
	start, end, err := utils.NormalizeDateRange(startRaw, endRaw)
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

func safeDivide(num float64, denom float64) float64 {
	if denom == 0 {
		return 0
	}
	return num / denom
}

func convertTransaction(tx model.PaymentTransaction) types.PaymentTransaction {
	return types.PaymentTransaction{
		Id:            tx.TransactionID,
		UserId:        tx.UserID,
		ProductId:     tx.ProductID,
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		Status:        tx.Status,
		PaymentMethod: tx.PaymentMethod,
		CreatedAt:     utils.FormatTimestamp(tx.OccurredAt),
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

func summarizePayments(stats []model.DailyRevenueStat, groupBy string) []types.PaymentsSummary {
	if len(stats) == 0 {
		return []types.PaymentsSummary{}
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
		items := make([]types.PaymentsSummary, 0, len(stats))
		for _, stat := range stats {
			items = append(items, types.PaymentsSummary{
				Date:         stat.Day.Format("2006-01-02"),
				Revenue:      stat.Revenue,
				Transactions: int(stat.Transactions),
				Users:        int(stat.Payers),
			})
		}
		return items
	}
}

func aggregateBy(stats []model.DailyRevenueStat, keyFn func(time.Time) string) []types.PaymentsSummary {
	buckets := make(map[string]*types.PaymentsSummary)
	order := []string{}
	for _, stat := range stats {
		key := keyFn(stat.Day)
		if key == "" {
			key = stat.Day.Format("2006-01-02")
		}
		if _, ok := buckets[key]; !ok {
			buckets[key] = &types.PaymentsSummary{
				Date: key,
			}
			order = append(order, key)
		}
		entry := buckets[key]
		entry.Revenue += stat.Revenue
		entry.Transactions += int(stat.Transactions)
		entry.Users += int(stat.Payers)
	}

	items := make([]types.PaymentsSummary, 0, len(order))
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
		txID = uuid.NewString()
	}
	userID := pickString(entry, "userId", "playerId", "player_id")
	if userID == "" {
		return nil, errors.New("缺少用户ID")
	}

	amount := parseFloat(entry["amount"])
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
	occurredAt, err := utils.ParseDate(timestampStr)
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

func pickString(entry map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := entry[key]; ok {
			switch v := val.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			default:
				str := fmt.Sprintf("%v", v)
				if strings.TrimSpace(str) != "" {
					return strings.TrimSpace(str)
				}
			}
		}
	}
	return ""
}

func parseFloat(value interface{}) float64 {
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
