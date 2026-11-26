package model

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// BehaviorModel handles behavior analytics persistence.
type BehaviorModel struct {
	db *gorm.DB
}

// NewBehaviorModel creates a new behavior model helper.
func NewBehaviorModel(db *gorm.DB) *BehaviorModel {
	return &BehaviorModel{db: db}
}

// BehaviorEventOptions controls event queries.
type BehaviorEventOptions struct {
	PaginationOptions
	GameID    string
	Env       string
	ServerID  string
	EventType string
	StartTime time.Time
	EndTime   time.Time
}

// RecordEvent stores a new behavior event.
func (m *BehaviorModel) RecordEvent(ctx context.Context, event *BehaviorEvent) error {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = NowUTC()
	}
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(event).Error
}

// ListEvents returns paginated behavior events.
func (m *BehaviorModel) ListEvents(ctx context.Context, opts BehaviorEventOptions) ([]BehaviorEvent, int64, error) {
	opts.Normalize()

	var (
		items []BehaviorEvent
		total int64
	)

	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&BehaviorEvent{})

	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		query = query.Where("env = ?", opts.Env)
	}
	if opts.ServerID != "" {
		query = query.Where("server_id = ?", opts.ServerID)
	}
	if opts.EventType != "" {
		query = query.Where("event_type = ?", opts.EventType)
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

// UpsertFeatureAdoption stores aggregated adoption data.
func (m *BehaviorModel) UpsertFeatureAdoption(ctx context.Context, adoption *FeatureAdoption) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(adoption).Error
}

// ListFeatureAdoptions fetches adoption snapshots.
func (m *BehaviorModel) ListFeatureAdoptions(ctx context.Context, gameID, env string) ([]FeatureAdoption, error) {
	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&FeatureAdoption{})
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}

	var items []FeatureAdoption
	if err := query.Order("feature ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// BehaviorDailyStat describes per-day activity metrics.
type BehaviorDailyStat struct {
	Day         time.Time
	ActiveUsers int64
	Events      int64
}

// EventTypeCount represents aggregated counts per event type.
type EventTypeCount struct {
	EventType string
	Total     int64
}

// CountDistinctUsers returns the number of unique users within the provided window.
func (m *BehaviorModel) CountDistinctUsers(ctx context.Context, gameID, env string, start, end time.Time) (int64, error) {
	query := m.scopedEvents(ctx, gameID, env, start, end)
	var count int64
	if err := query.Distinct("user_id").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountEvents returns total events within the provided window.
func (m *BehaviorModel) CountEvents(ctx context.Context, gameID, env string, start, end time.Time) (int64, error) {
	query := m.scopedEvents(ctx, gameID, env, start, end)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// EventTypeCounts aggregates events by type, ordered by volume.
func (m *BehaviorModel) EventTypeCounts(ctx context.Context, gameID, env string, start, end time.Time, limit int) ([]EventTypeCount, error) {
	query := m.scopedEvents(ctx, gameID, env, start, end).
		Select("event_type, COUNT(*) AS total").
		Group("event_type").
		Order("total DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	var rows []EventTypeCount
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// eventTypeAliases expands each canonical event type name with its dotted
// OTel spelling ("session_start" -> "session.start") so both name layers are
// counted in one query.
func eventTypeAliases(eventTypes []string) []string {
	aliases := make([]string, 0, len(eventTypes)*2)
	for _, raw := range eventTypes {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		aliases = append(aliases, name)
		if dotted := strings.ReplaceAll(name, "_", "."); dotted != name {
			aliases = append(aliases, dotted)
		}
	}
	return aliases
}

// CountEventsByType returns the number of matching events of the given types
// in the window. Names match case-insensitively and include both canonical
// underscore and dotted OTel spellings.
func (m *BehaviorModel) CountEventsByType(ctx context.Context, gameID, env string, start, end time.Time, eventTypes ...string) (int64, error) {
	query := m.scopedEvents(ctx, gameID, env, start, end)
	if aliases := eventTypeAliases(eventTypes); len(aliases) > 0 {
		query = query.Where("LOWER(event_type) IN ?", aliases)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountDistinctUsersByEventTypes counts distinct users that emitted any of
// the given event types in the window. Multiple events by one user (for
// example repeated heartbeats) contribute exactly one to the result.
func (m *BehaviorModel) CountDistinctUsersByEventTypes(ctx context.Context, gameID, env string, start, end time.Time, eventTypes []string) (int64, error) {
	query := m.scopedEvents(ctx, gameID, env, start, end)
	if aliases := eventTypeAliases(eventTypes); len(aliases) > 0 {
		query = query.Where("LOWER(event_type) IN ?", aliases)
	} else {
		// An empty type list must match nothing, not every event.
		query = query.Where("1 = 0")
	}
	var count int64
	if err := query.Distinct("user_id").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountEventsAnyType is CountEventsByType with an explicit multi-type name.
func (m *BehaviorModel) CountEventsAnyType(ctx context.Context, gameID, env string, start, end time.Time, eventTypes []string) (int64, error) {
	return m.CountEventsByType(ctx, gameID, env, start, end, eventTypes...)
}

// RealtimeBucketRow is the minimal projection used by the realtime series
// bucketer. Keeping the row narrow lets the whole window be fetched in one
// bounded query instead of two queries per bucket.
type RealtimeBucketRow struct {
	OccurredAt time.Time
	UserID     string
	EventType  string
}

// RetentionPairRow is one distinct (UTC day, user) activity pair, the raw
// input of N-day retention computation.
type RetentionPairRow struct {
	Day    string
	UserID string
}

// DailyActivePairs returns the distinct (day, user) activity pairs in the
// window. limit is a hard row cap; a non-positive limit means no cap.
func (m *BehaviorModel) DailyActivePairs(ctx context.Context, gameID, env string, start, end time.Time, limit int) ([]RetentionPairRow, error) {
	query := m.scopedEvents(ctx, gameID, env, start, end).
		Select("DATE(occurred_at) AS day, user_id").
		Group("day, user_id").
		Order("day ASC, user_id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []RetentionPairRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// RealtimeBucketEvents returns events in [start, end] ordered by occurrence.
// limit is a hard row cap; a non-positive limit means no cap (callers always
// pass the realtime bound). The window itself is encoded in the WHERE clause.
func (m *BehaviorModel) RealtimeBucketEvents(ctx context.Context, gameID, env string, start, end time.Time, limit int) ([]RealtimeBucketRow, error) {
	query := m.scopedEvents(ctx, gameID, env, start, end).
		Select("occurred_at", "user_id", "event_type").
		Order("occurred_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []RealtimeBucketRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DailyActivity returns per-day active users and event counts between the provided range.
func (m *BehaviorModel) DailyActivity(ctx context.Context, gameID, env string, start, end time.Time) ([]BehaviorDailyStat, error) {
	type row struct {
		Day         string
		ActiveUsers int64
		Events      int64
	}

	query := m.scopedEvents(ctx, gameID, env, start, end).
		Select("DATE(occurred_at) AS day, COUNT(DISTINCT user_id) AS active_users, COUNT(*) AS events").
		Group("day").
		Order("day ASC")

	var rows []row
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	stats := make([]BehaviorDailyStat, 0, len(rows))
	for _, r := range rows {
		day, err := time.Parse("2006-01-02", r.Day)
		if err != nil {
			day = time.Time{}
		}
		stats = append(stats, BehaviorDailyStat{
			Day:         day,
			ActiveUsers: r.ActiveUsers,
			Events:      r.Events,
		})
	}
	return stats, nil
}

func (m *BehaviorModel) scopedEvents(ctx context.Context, gameID, env string, start, end time.Time) *gorm.DB {
	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&BehaviorEvent{})
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
