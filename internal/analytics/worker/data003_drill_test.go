package worker

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestDrill_DATA003_AggregateReplacement verifies the corrected aggregate
// engines against a real ClickHouse: repeated snapshots for the same key keep
// only the newest version under FINAL, rather than summing stale duplicates.
func TestDrill_DATA003_AggregateReplacement(t *testing.T) {
	dsn := os.Getenv("DATA003_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("DATA-003: DATA003_CLICKHOUSE_DSN not set (needs an empty controllable ClickHouse database)")
	}
	opts, err := clickhouse.ParseDSN(dsn)
	require.NoError(t, err)
	conn, err := clickhouse.Open(opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, conn.Ping(ctx))

	database := strings.ToLower("data003_" + strings.ReplaceAll(uuid.NewString(), "-", "_"))
	require.NoError(t, conn.Exec(ctx, "CREATE DATABASE "+database))
	t.Cleanup(func() { require.NoError(t, conn.Exec(context.Background(), "DROP DATABASE "+database)) })

	ddl := []string{
		`CREATE TABLE ` + database + `.minute_online (
			m DateTime, game_id LowCardinality(String), env LowCardinality(String),
			online UInt32, version UInt64
		) ENGINE = ReplacingMergeTree(version) ORDER BY (game_id, env, m)`,
		`CREATE TABLE ` + database + `.daily_users (
			d Date, game_id LowCardinality(String), env LowCardinality(String),
			dau UInt64, new_users UInt64, version UInt64
		) ENGINE = ReplacingMergeTree(version) ORDER BY (game_id, env, d)`,
		`CREATE TABLE ` + database + `.daily_revenue (
			d Date, game_id LowCardinality(String), env LowCardinality(String),
			revenue_cents UInt64, refunds_cents UInt64, failed UInt64, version UInt64
		) ENGINE = ReplacingMergeTree(version) ORDER BY (game_id, env, d)`,
	}
	for _, query := range ddl {
		require.NoError(t, conn.Exec(ctx, query))
	}

	inserts := []string{
		`INSERT INTO ` + database + `.minute_online VALUES
		('2026-11-01 10:00:00', 'g1', 'prod', 80, 100),
		('2026-11-01 10:00:00', 'g1', 'prod', 95, 200)`,
		`INSERT INTO ` + database + `.daily_users VALUES
		('2026-11-01', 'g1', 'prod', 40, 5, 100),
		('2026-11-01', 'g1', 'prod', 55, 6, 200)`,
		`INSERT INTO ` + database + `.daily_revenue VALUES
		('2026-11-01', 'g1', 'prod', 1000, 10, 1, 100),
		('2026-11-01', 'g1', 'prod', 1200, 20, 2, 200)`,
	}
	for _, query := range inserts {
		require.NoError(t, conn.Exec(ctx, query))
	}

	var online uint64
	var dau, newUsers uint64
	var revenue, refunds, failed uint64
	require.NoError(t, conn.QueryRow(ctx, `
		SELECT sum(online) FROM `+database+`.minute_online FINAL
	`).Scan(&online))
	require.NoError(t, conn.QueryRow(ctx, `
		SELECT dau, new_users FROM `+database+`.daily_users FINAL
	`).Scan(&dau, &newUsers))
	require.NoError(t, conn.QueryRow(ctx, `
		SELECT revenue_cents, refunds_cents, failed FROM `+database+`.daily_revenue FINAL
	`).Scan(&revenue, &refunds, &failed))

	require.Equal(t, uint64(95), online, "minute snapshot must replace, not sum")
	require.Equal(t, uint64(55), dau)
	require.Equal(t, uint64(6), newUsers)
	require.Equal(t, uint64(1200), revenue)
	require.Equal(t, uint64(20), refunds)
	require.Equal(t, uint64(2), failed)
}
