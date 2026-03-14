package analytics

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
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
