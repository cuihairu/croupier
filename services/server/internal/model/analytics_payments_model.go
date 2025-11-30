package model

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// PaymentsModel handles payment analytics persistence.
type PaymentsModel struct {
	db *gorm.DB
}

// NewPaymentsModel creates a new payments model helper.
func NewPaymentsModel(db *gorm.DB) *PaymentsModel {
	return &PaymentsModel{db: db}
}

// PaymentQueryOptions controls transaction queries.
type PaymentQueryOptions struct {
	PaginationOptions
	GameID    string
	Env       string
	Status    string
	StartTime time.Time
	EndTime   time.Time
}

// CreateTransaction records a new transaction.
func (m *PaymentsModel) CreateTransaction(ctx context.Context, tx *PaymentTransaction) error {
	if tx.OccurredAt.IsZero() {
		tx.OccurredAt = NowUTC()
	}
	return m.db.WithContext(ctx).Create(tx).Error
}

// ListTransactions returns paginated transactions.
func (m *PaymentsModel) ListTransactions(ctx context.Context, opts PaymentQueryOptions) ([]PaymentTransaction, int64, error) {
	opts.PaginationOptions.Normalize()

	var (
		items []PaymentTransaction
		total int64
	)

	query := m.db.WithContext(ctx).Model(&PaymentTransaction{})

	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		query = query.Where("env = ?", opts.Env)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if !opts.StartTime.IsZero() {
		query = query.Where("occurred_at >= ?", opts.StartTime)
	}
	if !opts.EndTime.IsZero() {
		query = query.Where("occurred_at <= ?", opts.EndTime)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Order("occurred_at DESC").
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// UpsertProductTrend stores aggregated product trend data.
func (m *PaymentsModel) UpsertProductTrend(ctx context.Context, trend *ProductTrend) error {
	if trend.WindowStart.IsZero() {
		trend.WindowStart = NowUTC().Add(-24 * time.Hour)
	}
	if trend.WindowEnd.IsZero() {
		trend.WindowEnd = NowUTC()
	}

	return m.db.WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(trend).Error
}

// ListProductTrends fetches trend snapshots.
func (m *PaymentsModel) ListProductTrends(ctx context.Context, gameID, env string) ([]ProductTrend, error) {
	query := m.db.WithContext(ctx).Model(&ProductTrend{})
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}

	var items []ProductTrend
	if err := query.Order("revenue DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// RevenueAggregate summarizes revenue data for a period.
type RevenueAggregate struct {
	Revenue      float64
	Payers       int64
	Transactions int64
}

// DailyRevenueStat tracks revenue per day.
type DailyRevenueStat struct {
	Day          time.Time
	Revenue      float64
	Transactions int64
	Payers       int64
}

// AggregateRevenue returns revenue, paying users, and transaction counts for the window.
func (m *PaymentsModel) AggregateRevenue(ctx context.Context, gameID, env string, start, end time.Time) (RevenueAggregate, error) {
	query := m.scopedTransactions(ctx, gameID, env, start, end)

	var row struct {
		Revenue      float64
		Payers       int64
		Transactions int64
	}

	if err := query.
		Select("COALESCE(SUM(amount),0) AS revenue, COUNT(DISTINCT user_id) AS payers, COUNT(*) AS transactions").
		Scan(&row).Error; err != nil {
		return RevenueAggregate{}, err
	}

	return RevenueAggregate{
		Revenue:      row.Revenue,
		Payers:       row.Payers,
		Transactions: row.Transactions,
	}, nil
}

// DailyRevenue aggregates revenue per day within the range.
func (m *PaymentsModel) DailyRevenue(ctx context.Context, gameID, env string, start, end time.Time) ([]DailyRevenueStat, error) {
	type row struct {
		Day          string
		Revenue      float64
		Transactions int64
		Payers       int64
	}

	query := m.scopedTransactions(ctx, gameID, env, start, end).
		Select("DATE(occurred_at) AS day, COALESCE(SUM(amount),0) AS revenue, COUNT(*) AS transactions, COUNT(DISTINCT user_id) AS payers").
		Group("day").
		Order("day ASC")

	var rows []row
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	stats := make([]DailyRevenueStat, 0, len(rows))
	for _, r := range rows {
		day, err := time.Parse("2006-01-02", r.Day)
		if err != nil {
			day = time.Time{}
		}
		stats = append(stats, DailyRevenueStat{
			Day:          day,
			Revenue:      r.Revenue,
			Transactions: r.Transactions,
			Payers:       r.Payers,
		})
	}
	return stats, nil
}

func (m *PaymentsModel) scopedTransactions(ctx context.Context, gameID, env string, start, end time.Time) *gorm.DB {
	query := m.db.WithContext(ctx).
		Model(&PaymentTransaction{}).
		Where("status = ?", "success")
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}
	if !start.IsZero() {
		query = query.Where("occurred_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("occurred_at <= ?", end)
	}
	return query
}
