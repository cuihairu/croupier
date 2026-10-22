// Package mysql applies journaled CDC transactions to MySQL.
package mysql

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/recovery"
	_ "github.com/go-sql-driver/mysql"
)

const ledgerTable = "`croupier_recovery`.`applied_transactions`"

// Applier applies complete source transactions to MySQL atomically.
type Applier struct {
	db *sql.DB
}

// Open resolves the DSN from dsnEnv and prepares the idempotency ledger.
func Open(ctx context.Context, dsnEnv string) (*Applier, error) {
	if strings.TrimSpace(dsnEnv) == "" {
		return nil, errors.New("mysql recovery: dsnEnv is required")
	}
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		return nil, fmt.Errorf("mysql recovery: environment variable %s is not set", dsnEnv)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql recovery: open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql recovery: ping: %w", err)
	}
	if err := EnsureSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Applier{db: db}, nil
}

// NewApplier accepts an existing connection pool, normally in tests.
func NewApplier(ctx context.Context, db *sql.DB) (*Applier, error) {
	if db == nil {
		return nil, errors.New("mysql recovery: db is required")
	}
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	return &Applier{db: db}, nil
}

// EnsureSchema creates the transaction idempotency ledger.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("mysql recovery: db is required")
	}
	const statement = "CREATE DATABASE IF NOT EXISTS croupier_recovery CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci; CREATE TABLE IF NOT EXISTS " + ledgerTable + ` (
request_id varchar(191) NOT NULL,
transaction_id varchar(191) NOT NULL,
event_count bigint NOT NULL,
event_hash varbinary(32) NOT NULL,
applied_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
PRIMARY KEY (request_id, transaction_id)
) ENGINE=InnoDB`
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("mysql recovery: ensure ledger: %w", err)
	}
	return nil
}

// Close closes the connection pool.
func (a *Applier) Close() error {
	if a == nil || a.db == nil {
		return nil
	}
	return a.db.Close()
}

// Capabilities reports the supported row operations.
func (a *Applier) Capabilities(context.Context) (recovery.TargetCapabilities, error) {
	return recovery.TargetCapabilities{
		AtomicTransactions:     true,
		IdempotentTransactions: true,
		SupportedOperations:    []cdc.Operation{cdc.OpInsert, cdc.OpUpdate, cdc.OpDelete},
	}, nil
}

// ValidateTransaction checks mapped images and destination tables before writes.
func (a *Applier) ValidateTransaction(ctx context.Context, _ recovery.Plan, events []cdc.ChangeEvent) error {
	if len(events) == 0 {
		return errors.New("mysql recovery: empty transaction")
	}
	for _, event := range events {
		if !event.ColumnsMapped {
			return fmt.Errorf("mysql recovery: event %s columns are not mapped", event.ID)
		}
		var image map[string]json.RawMessage
		switch event.Operation {
		case cdc.OpInsert, cdc.OpUpdate:
			image = event.After
			if image == nil {
				return fmt.Errorf("mysql recovery: %s event %s has no after image", event.Operation, event.ID)
			}
		case cdc.OpDelete:
			image = event.Before
			if image == nil {
				return fmt.Errorf("mysql recovery: delete event %s has no before image", event.ID)
			}
		default:
			return fmt.Errorf("mysql recovery: unsupported operation %q", event.Operation)
		}
		if event.Database == "" || event.Table == "" {
			return fmt.Errorf("mysql recovery: event %s has no target schema and table", event.ID)
		}
		if err := a.columnsExist(ctx, event.Database, event.Table, rowColumnNames(image)); err != nil {
			return err
		}
		if err := validateKey(event, image); err != nil {
			return err
		}
	}
	return nil
}

func (a *Applier) columnsExist(ctx context.Context, schema, table string, columns []string) error {
	if len(columns) == 0 {
		return fmt.Errorf("mysql recovery: table %s.%s has no recoverable columns", schema, table)
	}
	var found int
	placeholders := make([]string, 0, len(columns))
	args := make([]any, 0, 3+len(columns))
	args = append(args, schema, table, "BASE TABLE")
	for i := range columns {
		placeholders = append(placeholders, "?")
		args = append(args, columns[i])
	}
	err := a.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=? AND table_name=? AND (? = 'BASE TABLE') AND column_name IN ("+strings.Join(placeholders, ", ")+")",
		args...,
	).Scan(&found)
	if err != nil {
		return fmt.Errorf("mysql recovery: inspect table %s.%s: %w", schema, table, err)
	}
	if found != len(columns) {
		return fmt.Errorf("mysql recovery: columns %v do not all exist in table %s.%s", columns, schema, table)
	}
	return nil
}

func validateKey(event cdc.ChangeEvent, image map[string]json.RawMessage) error {
	if event.Operation == cdc.OpInsert {
		return nil
	}
	if len(event.Key) == 0 {
		return fmt.Errorf("mysql recovery: event %s has no key columns", event.ID)
	}
	for _, column := range event.Key {
		if _, ok := image[column]; !ok {
			return fmt.Errorf("mysql recovery: event %s key column %q is missing", event.ID, column)
		}
	}
	return nil
}

// ApplyTransaction commits all events or rolls the transaction back completely.
func (a *Applier) ApplyTransaction(ctx context.Context, requestID, transactionID string, events []cdc.ChangeEvent) error {
	if requestID == "" || transactionID == "" || len(events) == 0 {
		return errors.New("mysql recovery: request, transaction and events are required")
	}
	if err := a.ValidateTransaction(ctx, recovery.Plan{}, events); err != nil {
		return err
	}
	eventHash, err := hashEvents(events)
	if err != nil {
		return err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql recovery: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx,
		"INSERT IGNORE INTO "+ledgerTable+" (request_id, transaction_id, event_count, event_hash) VALUES (?, ?, ?, ?)",
		requestID, transactionID, int64(len(events)), eventHash,
	)
	if err != nil {
		return fmt.Errorf("mysql recovery: reserve transaction: %w", err)
	}
	if result == nil {
		return errors.New("mysql recovery: reserve transaction returned no result")
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql recovery: read reserve result: %w", err)
	}
	if inserted == 0 {
		var existingCount int64
		var existingHash []byte
		err = tx.QueryRowContext(ctx,
			"SELECT event_count, event_hash FROM "+ledgerTable+" WHERE request_id=? AND transaction_id=?",
			requestID, transactionID,
		).Scan(&existingCount, &existingHash)
		if err != nil {
			return fmt.Errorf("mysql recovery: read applied transaction: %w", err)
		}
		if existingCount != int64(len(events)) || !bytes.Equal(existingHash, eventHash) {
			return fmt.Errorf("mysql recovery: transaction %s already applied with different content for request %s", transactionID, requestID)
		}
		return tx.Commit()
	}
	for _, event := range events {
		if err := applyEvent(ctx, tx, event); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql recovery: commit: %w", err)
	}
	return nil
}

func hashEvents(events []cdc.ChangeEvent) ([]byte, error) {
	encoded, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("mysql recovery: encode transaction hash: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return sum[:], nil
}

func applyEvent(ctx context.Context, tx *sql.Tx, event cdc.ChangeEvent) error {
	switch event.Operation {
	case cdc.OpInsert:
		columns, values, err := rowColumns(event.After)
		if err != nil {
			return err
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(columns)), ", ")
		query := "INSERT INTO " + qualifiedTable(event.Database, event.Table) + " (" + quoteColumns(columns) + ") VALUES (" + placeholders + ")"
		_, err = tx.ExecContext(ctx, query, values...)
		return err
	case cdc.OpUpdate:
		key, err := eventKey(event)
		if err != nil {
			return err
		}
		columns, values, err := rowColumns(event.After)
		if err != nil {
			return err
		}
		sets := make([]string, len(columns))
		args := make([]any, 0, len(values)+len(key))
		for i, column := range columns {
			sets[i] = quoteIdentifier(column) + "=?"
			args = append(args, values[i])
		}
		keyValues, err := keyValues(event.After, key)
		if err != nil {
			return err
		}
		args = append(args, keyValues...)
		query := "UPDATE " + qualifiedTable(event.Database, event.Table) + " SET " + strings.Join(sets, ", ") + " WHERE " + whereColumns(key)
		_, err = tx.ExecContext(ctx, query, args...)
		return err
	case cdc.OpDelete:
		key, err := eventKey(event)
		if err != nil {
			return err
		}
		values, err := keyValues(event.Before, key)
		if err != nil {
			return err
		}
		query := "DELETE FROM " + qualifiedTable(event.Database, event.Table) + " WHERE " + whereColumns(key)
		_, err = tx.ExecContext(ctx, query, values...)
		return err
	default:
		return fmt.Errorf("mysql recovery: unsupported operation %q", event.Operation)
	}
}

func qualifiedTable(schema, table string) string {
	return quoteIdentifier(schema) + "." + quoteIdentifier(table)
}

func eventKey(event cdc.ChangeEvent) ([]string, error) {
	if len(event.Key) == 0 {
		return nil, fmt.Errorf("mysql recovery: event %s has no key columns", event.ID)
	}
	image := event.After
	if event.Operation == cdc.OpDelete {
		image = event.Before
	}
	for _, column := range event.Key {
		if _, ok := image[column]; !ok {
			return nil, fmt.Errorf("mysql recovery: event %s key column %q is missing", event.ID, column)
		}
	}
	return append([]string(nil), event.Key...), nil
}

func rowColumnNames(image map[string]json.RawMessage) []string {
	names := make([]string, 0, len(image))
	for name := range image {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func keyValues(image map[string]json.RawMessage, key []string) ([]any, error) {
	args := make([]any, 0, len(key))
	for _, column := range key {
		value, err := rawValue(image[column])
		if err != nil {
			return nil, err
		}
		args = append(args, value)
	}
	return args, nil
}

func whereColumns(columns []string) string {
	conditions := make([]string, len(columns))
	for i := range columns {
		conditions[i] = quoteIdentifier(columns[i]) + "=?"
	}
	return strings.Join(conditions, " AND ")
}

func rowColumns(image map[string]json.RawMessage) ([]string, []any, error) {
	columns := make([]string, 0, len(image))
	for column := range image {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	values := make([]any, 0, len(columns))
	for _, column := range columns {
		value, err := rawValue(image[column])
		if err != nil {
			return nil, nil, err
		}
		values = append(values, value)
	}
	return columns, values, nil
}

func quoteColumns(columns []string) string {
	quoted := make([]string, len(columns))
	for i := range columns {
		quoted[i] = quoteIdentifier(columns[i])
	}
	return strings.Join(quoted, ", ")
}

func quoteIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func rawValue(raw json.RawMessage) (any, error) {
	if raw == nil {
		return nil, nil
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("mysql recovery: decode row value: %w", err)
	}
	switch typed := value.(type) {
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer, nil
		}
		floating, err := typed.Float64()
		if err != nil {
			return nil, fmt.Errorf("mysql recovery: decode numeric %q: %w", typed.String(), err)
		}
		return floating, nil
	case map[string]any, []any:
		return string(raw), nil
	default:
		return typed, nil
	}
}
