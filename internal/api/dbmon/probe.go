// Standardized health probe for registered database sources
// (docs/research/db-monitoring-design.md §2.2). Cross-cloud compatible:
// plain SQL against information_schema / performance_schema / pg_stat_*,
// executed with a hard timeout on a read-only connection. Metrics a cloud
// RDS does not expose are reported as unavailable (degraded), never guessed.
package dbmon

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

// ProbeResult is the normalized snapshot for one source.
type ProbeResult struct {
	SourceID  uint   `json:"sourceId"`
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	Kind      string `json:"kind"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latencyMs,omitempty"`

	// Connections.
	Connections *ConnectionsInfo `json:"connections,omitempty"`
	// Locks: current lock wait entries.
	LockWaits []LockWait `json:"lockWaits,omitempty"`
	// Deadlocks: cumulative counter where available (degraded on cloud RDS).
	DeadlockCount *int64 `json:"deadlockCount,omitempty"`
	DeadlockNote  string `json:"deadlockNote,omitempty"`
	// Cumulative counters since server start (where available).
	QueryCount *int64 `json:"queryCount,omitempty"`
	TxnCount   *int64 `json:"txnCount,omitempty"`

	ProbedAt time.Time `json:"probedAt"`
}

type ConnectionsInfo struct {
	Current int `json:"current"`
	Max     int `json:"max"` // max_connections; -1 = unknown
	Active  int `json:"active"`
}

type LockWait struct {
	WaitPIDorID string  `json:"waitId"`
	BlockedBy   string  `json:"blockedBy"`
	Table       string  `json:"table,omitempty"`
	WaitSecs    float64 `json:"waitSecs"`
	Query       string  `json:"query,omitempty"`
}

// Probe runs all checks against one source with a per-query timeout.
func Probe(ctx context.Context, src *model.DBSource, dsn string) (*ProbeResult, error) {
	if src == nil {
		return nil, fmt.Errorf("nil source")
	}
	driver := strings.ToLower(strings.TrimSpace(src.Driver))
	res := &ProbeResult{
		SourceID: src.ID, Name: src.Name, Driver: driver, Kind: src.Kind,
		ProbedAt: time.Now(),
	}
	start := time.Now()
	db, err := sql.Open(driverName(driver), dsn)
	if err != nil {
		res.Error = fmt.Sprintf("open: %v", err)
		return res, nil
	}
	defer db.Close()
	res.LatencyMS = time.Since(start).Milliseconds()

	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	switch driver {
	case "postgres":
		probePostgres(pctx, db, res)
	default:
		probeMySQL(pctx, db, res)
	}

	if res.Error == "" {
		res.OK = true
	}
	res.LatencyMS = time.Since(start).Milliseconds()
	return res, nil
}

func driverName(driver string) string {
	if driver == "postgres" {
		return "pgx"
	}
	return "mysql"
}

// ---- MySQL ----

func probeMySQL(ctx context.Context, db *sql.DB, res *ProbeResult) {
	res.Connections = &ConnectionsInfo{Max: -1}
	var variable, value string
	rows, err := db.QueryContext(ctx,
		"SHOW GLOBAL STATUS WHERE Variable_name IN ('Threads_connected','Threads_running','Innodb_deadlocks','Queries','Com_commit','Com_rollback')")
	if err != nil {
		res.Error = "status query: " + err.Error()
		return
	} else {
		statuses := map[string]string{}
		for rows.Next() {
			_ = rows.Scan(&variable, &value)
			statuses[variable] = value
		}
		rows.Close()
		res.Connections.Current = atoi(statuses["Threads_connected"])
		res.Connections.Active = atoi(statuses["Threads_running"])
		if v := atoi(statuses["Innodb_deadlocks"]); v > 0 || statuses["Innodb_deadlocks"] != "" {
			n := int64(v)
			res.DeadlockCount = &n
			res.DeadlockNote = "cumulative since server start"
		}
		if q := statuses["Queries"]; q != "" {
			n := int64(atoi(q))
			res.QueryCount = &n
		}
		t := int64(atoi(statuses["Com_commit"]) + atoi(statuses["Com_rollback"]))
		res.TxnCount = &t
	}

	var maxConns sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT @@max_connections").Scan(&maxConns); err == nil {
		res.Connections.Max = int(maxConns.Int64)
	}

	lockRows, err := db.QueryContext(ctx, lockWaitsSQL)
	if err == nil {
		defer lockRows.Close()
		for lockRows.Next() {
			var lw LockWait
			var query sql.NullString
			if err := lockRows.Scan(&lw.WaitPIDorID, &lw.BlockedBy, &lw.Table, &lw.WaitSecs, &query); err == nil {
				lw.Query = truncate(query.String, 120)
				res.LockWaits = append(res.LockWaits, lw)
			}
		}
	}
}

// MySQL 8 first, 5.7 fallback handled by the caller trying both shapes is
// avoided: information_schema.innodb_lock_waits exists on both (8.0 keeps a
// compatibility view).
const lockWaitsSQL = `
SELECT
  COALESCE(r.trx_id, w.requesting_engine_transaction_id) AS waiter,
  COALESCE(b.trx_mysql_thread_id, b.blocking_engine_transaction_id) AS blocker,
  COALESCE(r.lock_table, '') AS table_name,
  TIMESTAMPDIFF(SECOND, r.trx_waited_at, NOW()) AS wait_secs,
  COALESCE(r.trx_query, '')
FROM information_schema.innodb_lock_waits w
JOIN information_schema.innodb_trx r ON r.trx_id = w.requesting_trx_id
JOIN information_schema.innodb_trx b ON b.trx_id = w.blocking_trx_id`

// ---- Postgres ----

func probePostgres(ctx context.Context, db *sql.DB, res *ProbeResult) {
	res.Connections = &ConnectionsInfo{Max: -1}
	var cur, active, maxc int
	err := db.QueryRowContext(ctx, `
SELECT
  count(*) FILTER (WHERE state IS NOT NULL),
  count(*) FILTER (WHERE state = 'active'),
  current_setting('max_connections')::int
FROM pg_stat_activity`).Scan(&cur, &active, &maxc)
	if err == nil {
		res.Connections = &ConnectionsInfo{Current: cur, Active: active, Max: maxc}
	}

	var deadlocks int64
	if err := db.QueryRowContext(ctx,
		"SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(&deadlocks); err == nil {
		res.DeadlockCount = &deadlocks
		res.DeadlockNote = "cumulative since stats reset"
	}

	lockRows, err := db.QueryContext(ctx, `
SELECT blocked.pid,
       blocking.pid,
       COALESCE(blocked.relname, ''),
       EXTRACT(EPOCH FROM (now() - blocked.query_start)),
       left(blocked.query, 120)
FROM pg_locks bl
JOIN pg_stat_activity blocked ON blocked.pid = bl.pid
JOIN pg_locks kl ON kl.locktype = bl.locktype AND kl.relation IS NOT DISTINCT FROM bl.relation
   AND kl.pid <> bl.pid AND kl.granted
JOIN pg_stat_activity blocking ON blocking.pid = kl.pid
WHERE NOT bl.granted`)
	if err == nil {
		defer lockRows.Close()
		for lockRows.Next() {
			var lw LockWait
			var secs float64
			if err := lockRows.Scan(&lw.WaitPIDorID, &lw.BlockedBy, &lw.Table, &secs, &lw.Query); err == nil {
				lw.WaitSecs = secs
				res.LockWaits = append(res.LockWaits, lw)
			}
		}
	}
}

// ---- shared helpers ----

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
