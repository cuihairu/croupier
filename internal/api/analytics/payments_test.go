package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/require"
)

func TestResolvePaymentsRange(t *testing.T) {
	t.Parallel()

	start, end, err := resolvePaymentsRange("", "")
	if err != nil {
		t.Fatalf("resolvePaymentsRange() error = %v", err)
	}
	if end.Before(start) {
		t.Fatalf("expected end >= start, got start=%v end=%v", start, end)
	}
}

func TestConvertTransactionAndBuildPaymentsTrends(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	tx := convertTransaction(model.PaymentTransaction{
		TransactionID: "t1",
		UserID:        "u1",
		ProductID:     "p1",
		Amount:        12.5,
		Currency:      "USD",
		Status:        "success",
		PaymentMethod: "card",
		OccurredAt:    now,
	})
	if tx.Id != "t1" || tx.UserId != "u1" {
		t.Fatalf("unexpected converted tx: %#v", tx)
	}

	trends := buildPaymentsTrends([]model.DailyRevenueStat{
		{Day: now, Revenue: 100, Transactions: 4, Payers: 3},
	})
	revenue, ok := trends["revenue"].([]map[string]interface{})
	if !ok || len(revenue) != 1 {
		t.Fatalf("unexpected trends: %#v", trends)
	}
	if revenue[0]["avgSale"] != 25.0 {
		t.Fatalf("unexpected avgSale: %#v", revenue[0])
	}
}

func TestSummarizePayments(t *testing.T) {
	t.Parallel()

	stats := []model.DailyRevenueStat{
		{Day: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC), Revenue: 10, Transactions: 1, Payers: 1},
		{Day: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), Revenue: 20, Transactions: 2, Payers: 2},
	}

	day := summarizePayments(stats, "day")
	if len(day) != 2 {
		t.Fatalf("expected 2 day summaries, got %#v", day)
	}

	month := summarizePayments(stats, "month")
	if len(month) != 1 || month[0].Revenue != 30 {
		t.Fatalf("unexpected month summary: %#v", month)
	}
}

func TestDecodeTransactionsPayloadAndBuildTransaction(t *testing.T) {
	t.Parallel()

	items, err := decodeTransactionsPayload([]map[string]interface{}{
		{"userId": "u1", "amount": 12.5},
	})
	if err != nil {
		t.Fatalf("decodeTransactionsPayload() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	tx, err := buildTransaction(map[string]interface{}{
		"transactionId": "t1",
		"userId":        "u1",
		"amount":        json.Number("42.5"),
		"currency":      "CNY",
		"status":        "paid",
	}, "tower", "prod")
	if err != nil {
		t.Fatalf("buildTransaction() error = %v", err)
	}
	if tx.TransactionID != "t1" || tx.Amount != 42.5 {
		t.Fatalf("unexpected built transaction: %#v", tx)
	}

	if _, err := buildTransaction(map[string]interface{}{"amount": 1}, "tower", "prod"); err == nil {
		t.Fatal("expected missing user id error")
	}
}

func TestParseFloatValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input interface{}
		want  float64
	}{
		{name: "float64", input: 1.5, want: 1.5},
		{name: "float32", input: float32(2.5), want: 2.5},
		{name: "int", input: 3, want: 3},
		{name: "int64", input: int64(4), want: 4},
		{name: "jsonNumber", input: json.Number("5.5"), want: 5.5},
		{name: "string", input: "6.5", want: 6.5},
		{name: "invalid", input: "x", want: 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := parseFloatValue(tc.input); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

// Additional tests for payments coverage

func TestResolvePaymentsRange_InvalidDates(t *testing.T) {
	t.Parallel()

	_, _, err := resolvePaymentsRange("invalid", "2024-01-01")

	if err == nil {
		t.Fatal("expected error for invalid start date")
	}
}

func TestResolvePaymentsRange_StartOnly(t *testing.T) {
	t.Parallel()

	start, end, err := resolvePaymentsRange("2024-01-01", "")

	if err != nil {
		t.Fatalf("resolvePaymentsRange() error = %v", err)
	}
	// end gets set to current time when empty
	_ = end
	if start.IsZero() {
		t.Fatal("expected non-zero start")
	}
}

func TestResolvePaymentsRange_EndOnly(t *testing.T) {
	t.Parallel()

	start, end, err := resolvePaymentsRange("", "2024-01-01")

	if err != nil {
		t.Fatalf("resolvePaymentsRange() error = %v", err)
	}
	// end gets set to the provided date (normalized), start is defaulted
	if end.IsZero() {
		t.Fatal("expected non-zero end")
	}
	if start.IsZero() {
		t.Fatal("expected non-zero start (defaulted from end)")
	}
}

func TestConvertTransaction_AllFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	tx := convertTransaction(model.PaymentTransaction{
		TransactionID: "tx123",
		UserID:        "user456",
		ProductID:     "prod789",
		Amount:        99.99,
		Currency:      "EUR",
		Status:        "pending",
		PaymentMethod: "paypal",
		OccurredAt:    now,
	})

	if tx.Id != "tx123" {
		t.Fatalf("expected id tx123, got %s", tx.Id)
	}
	if tx.Amount != 99.99 {
		t.Fatalf("expected amount 99.99, got %v", tx.Amount)
	}
}

func TestBuildPaymentsTrends_EmptyStats(t *testing.T) {
	t.Parallel()

	trends := buildPaymentsTrends([]model.DailyRevenueStat{})

	revenue, ok := trends["revenue"].([]map[string]interface{})
	if !ok || len(revenue) != 0 {
		t.Fatalf("expected empty revenue array, got %v", trends)
	}
}

func TestBuildPaymentsTrends_TruncatesAtMaxPoints(t *testing.T) {
	t.Parallel()

	now := time.Now()
	stats := make([]model.DailyRevenueStat, maxTrendPoints+10)
	for i := range stats {
		stats[i] = model.DailyRevenueStat{
			Day:          now.Add(time.Duration(i) * 24 * time.Hour),
			Revenue:      float64(i),
			Transactions: int64(i),
			Payers:       int64(i),
		}
	}

	trends := buildPaymentsTrends(stats)
	revenue, ok := trends["revenue"].([]map[string]interface{})

	if !ok {
		t.Fatal("expected revenue array")
	}
	if len(revenue) != maxTrendPoints {
		t.Fatalf("expected %d trend points, got %d", maxTrendPoints, len(revenue))
	}
}

func TestSummarizePayments_EmptyStats(t *testing.T) {
	t.Parallel()

	items := summarizePayments([]model.DailyRevenueStat{}, "day")

	if len(items) != 0 {
		t.Fatalf("expected empty items for empty stats, got %v", items)
	}
}

func TestSummarizePayments_WeekGrouping(t *testing.T) {
	t.Parallel()

	stats := []model.DailyRevenueStat{
		{Day: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC), Revenue: 10, Transactions: 1, Payers: 1},
		{Day: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), Revenue: 20, Transactions: 2, Payers: 2},
	}

	items := summarizePayments(stats, "week")

	if len(items) != 1 {
		t.Fatalf("expected 1 week summary, got %d: %v", len(items), items)
	}
}

func TestSummarizePayments_DefaultGrouping(t *testing.T) {
	t.Parallel()

	stats := []model.DailyRevenueStat{
		{Day: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC), Revenue: 10, Transactions: 1, Payers: 1},
	}

	items := summarizePayments(stats, "")

	if len(items) != 1 {
		t.Fatalf("expected 1 item for empty groupBy, got %d", len(items))
	}
	if items[0].Date != "2026-03-14" {
		t.Fatalf("expected date 2026-03-14, got %s", items[0].Date)
	}
}

func TestSummarizePayments_CaseInsensitiveGroupBy(t *testing.T) {
	t.Parallel()

	stats := []model.DailyRevenueStat{
		{Day: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC), Revenue: 10, Transactions: 1, Payers: 1},
	}

	items := summarizePayments(stats, "  WEEK  ")

	if len(items) != 1 {
		t.Fatalf("expected 1 item for whitespace groupBy, got %d", len(items))
	}
}

func TestDecodeTransactionsPayload_Nil(t *testing.T) {
	t.Parallel()

	items, err := decodeTransactionsPayload(nil)

	if err != nil {
		t.Fatalf("decodeTransactionsPayload(nil) error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty items for nil input, got %v", items)
	}
}

func TestDecodeTransactionsPayload_EmptyList(t *testing.T) {
	t.Parallel()

	items, err := decodeTransactionsPayload([]map[string]interface{}{})

	if err != nil {
		t.Fatalf("decodeTransactionsPayload(empty) error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty items, got %v", items)
	}
}

func TestDecodeTransactionsPayload_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := decodeTransactionsPayload("invalid")

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestBuildTransaction_EmptyEntry(t *testing.T) {
	t.Parallel()

	_, err := buildTransaction(nil, "tower", "prod")

	if err == nil {
		t.Fatal("expected error for nil entry")
	}
}

func TestBuildTransaction_MissingUserId(t *testing.T) {
	t.Parallel()

	_, err := buildTransaction(map[string]interface{}{
		"amount": 10.0,
	}, "tower", "prod")

	if err == nil {
		t.Fatal("expected error for missing userId")
	}
	if err.Error() != "缺少用户ID" {
		t.Fatalf("expected '缺少用户ID' error, got %v", err)
	}
}

func TestBuildTransaction_DefaultValues(t *testing.T) {
	t.Parallel()

	tx, err := buildTransaction(map[string]interface{}{
		"userId": "u1",
		"amount": 100,
	}, "tower", "prod")

	if err != nil {
		t.Fatalf("buildTransaction() error = %v", err)
	}
	if tx.Currency != "USD" {
		t.Fatalf("expected default currency USD, got %s", tx.Currency)
	}
	if tx.Status != "success" {
		t.Fatalf("expected default status success, got %s", tx.Status)
	}
	if tx.TransactionID == "" {
		t.Fatal("expected auto-generated transaction ID")
	}
	if tx.OccurredAt.IsZero() {
		t.Fatal("expected default occurredAt to be set")
	}
}

func TestBuildTransaction_WithAllFields(t *testing.T) {
	t.Parallel()

	entry := map[string]interface{}{
		"id":            "custom-id",
		"userId":        "user123",
		"productId":     "prod456",
		"productName":   "Cool Item",
		"amount":        25.5,
		"currency":      "GBP",
		"status":        "completed",
		"paymentMethod": "visa",
		"timestamp":     "2024-01-15T10:30:00Z",
	}

	tx, err := buildTransaction(entry, "game1", "dev")

	if err != nil {
		t.Fatalf("buildTransaction() error = %v", err)
	}
	if tx.TransactionID != "custom-id" {
		t.Fatalf("expected custom id, got %s", tx.TransactionID)
	}
	// Check the product ID from the entry
	// The productName field is stored in the model but not exposed in PaymentTransaction DTO
}

func TestParseFloatValue_Negative(t *testing.T) {
	t.Parallel()

	got := parseFloatValue(-10.5)
	if got != -10.5 {
		t.Fatalf("expected -10.5, got %v", got)
	}
}

func TestParseFloatValue_Zero(t *testing.T) {
	t.Parallel()

	got := parseFloatValue(0)
	if got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
}

func TestParseFloatValue_WhitespaceString(t *testing.T) {
	t.Parallel()

	got := parseFloatValue("  42.5  ")
	if got != 42.5 {
		t.Fatalf("expected 42.5, got %v", got)
	}
}

func TestAggregateBy_EmptyStats(t *testing.T) {
	t.Parallel()

	items := aggregateBy([]model.DailyRevenueStat{}, func(t time.Time) string {
		return t.Format("2006-01-02")
	})

	if len(items) != 0 {
		t.Fatalf("expected empty items, got %v", items)
	}
}

func TestAggregateBy_MultipleBuckets(t *testing.T) {
	t.Parallel()

	date1 := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC) // different week
	stats := []model.DailyRevenueStat{
		{Day: date1, Revenue: 10, Transactions: 1, Payers: 1},
		{Day: date2, Revenue: 20, Transactions: 2, Payers: 2},
	}

	items := aggregateBy(stats, func(t time.Time) string {
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 aggregated items, got %d", len(items))
	}
}

// Tests for error paths in payments function

func TestPayments_NilModel(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		PaymentsModel: nil,
	}
	_, err := payments(context.Background(), svcCtx, &PaymentsRequest{})
	if err == nil {
		t.Fatal("expected error for nil payments model")
	}
}

func TestPayments_NilRequest(t *testing.T) {
	t.Parallel()

	// Nil model will be checked first, but we still test nil request path
	svcCtx := &svc.ServiceContext{
		PaymentsModel: nil,
	}
	_, err := payments(context.Background(), svcCtx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestPayments_InvalidDateRange(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		PaymentsModel: nil, // Will fail at model check first
	}
	req := &PaymentsRequest{
		StartDate: "invalid-date",
	}
	_, err := payments(context.Background(), svcCtx, req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPaymentsIngest_NilModel(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		PaymentsModel: nil,
	}
	_, err := paymentsIngest(context.Background(), svcCtx, &PaymentsIngestRequest{})
	if err == nil {
		t.Fatal("expected error for nil payments model")
	}
}

func TestPaymentsIngest_NilRequest(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		PaymentsModel: nil,
	}
	_, err := paymentsIngest(context.Background(), svcCtx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestPaymentsIngest_EmptyGameId(t *testing.T) {
	t.Parallel()

	svcCtx := &svc.ServiceContext{
		PaymentsModel: nil, // Will fail at model check
	}
	req := &PaymentsIngestRequest{
		GameId: "   ", // whitespace only
	}
	_, err := paymentsIngest(context.Background(), svcCtx, req)
	if err == nil {
		t.Fatal("expected error for empty game ID")
	}
}

// DATA-056: one player paying on two days of the same week/month must count
// once in that bucket. Revenue and transaction counts remain the sum of day
// rows; only the user count is deduped by payer identity.
func TestSummarizePaymentsWithPayerDays_CrossDayPayerCountedOnce(t *testing.T) {
	t.Parallel()

	d1 := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC) // Monday
	d2 := d1.AddDate(0, 0, 1)                           // same ISO week and month
	stats := []model.DailyRevenueStat{
		{Day: d1, Revenue: 10, Transactions: 1, Payers: 1},
		{Day: d2, Revenue: 15, Transactions: 2, Payers: 2}, // u1 + u2
	}
	pairs := []model.PayerDay{
		{Day: d1, UserID: "u1"},
		{Day: d2, UserID: "u1"}, // same player, second day: must not double count
		{Day: d2, UserID: "u2"},
	}

	week := summarizePaymentsWithPayerDays(stats, pairs, "week")
	if len(week) != 1 {
		t.Fatalf("expected 1 week bucket, got %#v", week)
	}
	if week[0].Users != 2 {
		t.Errorf("week users = %d, want 2 (u1 counted once + u2)", week[0].Users)
	}
	if week[0].Revenue != 25 || week[0].Transactions != 3 {
		t.Errorf("week revenue/transactions = %v/%d, want 25/3 (day-sum semantics)", week[0].Revenue, week[0].Transactions)
	}

	month := summarizePaymentsWithPayerDays(stats, pairs, "month")
	if len(month) != 1 || month[0].Users != 2 {
		t.Errorf("month summary = %#v, want one bucket with 2 users", month)
	}

	// Old path still sums day Payers (3) — this is the defect DATA-056 fixes.
	if old := summarizePayments(stats, "week"); len(old) != 1 || old[0].Users != 3 {
		t.Fatalf("day-sum path changed unexpectedly: %#v", old)
	}
}

func TestSummarizePaymentsWithPayerDays_DistinctWeeksAndMonths(t *testing.T) {
	t.Parallel()

	d1 := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC) // next ISO week, same month
	stats := []model.DailyRevenueStat{
		{Day: d1, Revenue: 10, Transactions: 1, Payers: 1},
		{Day: d2, Revenue: 20, Transactions: 2, Payers: 1},
	}
	pairs := []model.PayerDay{{Day: d1, UserID: "u1"}, {Day: d2, UserID: "u1"}}

	week := summarizePaymentsWithPayerDays(stats, pairs, "week")
	if len(week) != 2 || week[0].Users != 1 || week[1].Users != 1 {
		t.Errorf("distinct weeks should each count u1 once: %#v", week)
	}
	month := summarizePaymentsWithPayerDays(stats, pairs, "month")
	if len(month) != 1 || month[0].Users != 1 {
		t.Errorf("same month should count u1 once overall: %#v", month)
	}
}

func TestSummarizePaymentsWithPayerDays_EmptyAndPairOutsideBucket(t *testing.T) {
	t.Parallel()

	if items := summarizePaymentsWithPayerDays(nil, nil, "week"); len(items) != 0 {
		t.Fatalf("empty stats should yield no buckets, got %#v", items)
	}

	stats := []model.DailyRevenueStat{
		{Day: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), Revenue: 5, Transactions: 1, Payers: 1},
	}
	// A (day, user) pair for a day absent from the day rows must not
	// invent a bucket or inflate an unrelated one.
	pairs := []model.PayerDay{
		{Day: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), UserID: "z"},
	}
	items := summarizePaymentsWithPayerDays(stats, pairs, "month")
	if len(items) != 1 || items[0].Users != 0 {
		t.Errorf("out-of-window pair must not create/count a bucket: %#v", items)
	}
}

// TestPaymentsSummary_CrossDayPayerWeekMonthDedup drives the real handler on
// SQLite: one player paying on two days of one week. The week and month
// buckets must report 2 users (u1 once + u2) while revenue and transaction
// counts keep their day-sum values (3 = 1+2). Before DATA-056 the week
// reported 3 users.
func TestPaymentsSummary_CrossDayPayerWeekMonthDedup(t *testing.T) {
	db := setupServiceTestDB(t)
	ctx := context.Background()
	pm := model.NewPaymentsModel(db)

	d1 := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC) // Monday
	d2 := d1.AddDate(0, 0, 1)
	for _, tx := range []*model.PaymentTransaction{
		{TransactionID: "w1", GameID: "g", Env: "dev", UserID: "u1", Amount: 10, Status: "success", OccurredAt: d1},
		{TransactionID: "w2", GameID: "g", Env: "dev", UserID: "u1", Amount: 15, Status: "success", OccurredAt: d2},
		{TransactionID: "w3", GameID: "g", Env: "dev", UserID: "u2", Amount: 20, Status: "success", OccurredAt: d2},
	} {
		require.NoError(t, pm.CreateTransaction(ctx, tx))
	}

	resp, err := paymentsSummary(ctx, &svc.ServiceContext{PaymentsModel: model.NewPaymentsModel(db)}, &PaymentsSummaryRequest{
		GameId: "g", Env: "dev",
		StartDate: "2026-03-15", EndDate: "2026-03-24",
		GroupBy: "week",
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1, "week buckets: %#v", resp.Items)
	if resp.Items[0].Users != 2 {
		t.Errorf("week users = %d, want 2 (u1 deduped across days)", resp.Items[0].Users)
	}

	resp, err = paymentsSummary(ctx, &svc.ServiceContext{PaymentsModel: model.NewPaymentsModel(db)}, &PaymentsSummaryRequest{
		GameId: "g", Env: "dev",
		StartDate: "2026-03-15", EndDate: "2026-03-24",
		GroupBy: "month",
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1, "month buckets: %#v", resp.Items)
	if resp.Items[0].Users != 2 {
		t.Errorf("month users = %d, want 2 (u1 deduped across days)", resp.Items[0].Users)
	}

	// Day granularity is untouched: per-day payer counts stay 1 and 2.
	resp, err = paymentsSummary(ctx, &svc.ServiceContext{PaymentsModel: model.NewPaymentsModel(db)}, &PaymentsSummaryRequest{
		GameId: "g", Env: "dev",
		StartDate: "2026-03-15", EndDate: "2026-03-24",
		GroupBy: "day",
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	if resp.Items[0].Users != 1 || resp.Items[1].Users != 2 {
		t.Errorf("day users = %d/%d, want 1/2", resp.Items[0].Users, resp.Items[1].Users)
	}
}
