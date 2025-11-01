//go:build sqlite

package approvals

import (
    "database/sql"
    _ "modernc.org/sqlite"
    "fmt"
    "strings"
)

type SQLiteStore struct { db *sql.DB }

func NewSQLiteStore(dsn string) (Store, error) {
    // Accept dsn forms: sqlite:///path/to.db, file:path.db?cache=shared, :memory:
    drv := "sqlite"
    // normalize sqlite:/// to file:
    if strings.HasPrefix(dsn, "sqlite:///") {
        dsn = "file:" + strings.TrimPrefix(dsn, "sqlite:///")
    }
    db, err := sql.Open(drv, dsn)
    if err != nil { return nil, err }
    if err := db.Ping(); err != nil { return nil, err }
    s := &SQLiteStore{db: db}
    if err := s.init(); err != nil { return nil, err }
    return s, nil
}

func (s *SQLiteStore) init() error {
    stmts := []string{
        `CREATE TABLE IF NOT EXISTS approvals (
            id TEXT PRIMARY KEY,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            actor TEXT NOT NULL,
            function_id TEXT NOT NULL,
            payload BLOB,
            idempotency_key TEXT,
            route TEXT,
            target_service_id TEXT,
            hash_key TEXT,
            game_id TEXT,
            env TEXT,
            state TEXT DEFAULT 'pending',
            mode TEXT NOT NULL,
            reason TEXT
        )`,
        `CREATE INDEX IF NOT EXISTS idx_approvals_state ON approvals(state)`,
        `CREATE INDEX IF NOT EXISTS idx_approvals_function ON approvals(function_id)`,
        `CREATE INDEX IF NOT EXISTS idx_approvals_game_env ON approvals(game_id, env)`,
        `CREATE INDEX IF NOT EXISTS idx_approvals_actor ON approvals(actor)`,
        `CREATE INDEX IF NOT EXISTS idx_approvals_created_at ON approvals(created_at)`,
    }
    for _, q := range stmts {
        if _, err := s.db.Exec(q); err != nil { return fmt.Errorf("sqlite init: %w", err) }
    }
    return nil
}

func (s *SQLiteStore) Create(a *Approval) error {
    q := `INSERT INTO approvals (id, created_at, actor, function_id, payload, idempotency_key, route, target_service_id, hash_key, game_id, env, state, mode, reason)
          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
    _, err := s.db.Exec(q, a.ID, a.CreatedAt, a.Actor, a.FunctionID, a.Payload, a.IdempotencyKey, a.Route, a.TargetServiceID, a.HashKey, a.GameID, a.Env, a.State, a.Mode, a.Reason)
    return err
}

func (s *SQLiteStore) Get(id string) (*Approval, error) {
    q := `SELECT id, created_at, actor, function_id, payload, idempotency_key, route, target_service_id, hash_key, game_id, env, state, mode, reason FROM approvals WHERE id=?`
    row := s.db.QueryRow(q, id)
    var a Approval
    if err := row.Scan(&a.ID, &a.CreatedAt, &a.Actor, &a.FunctionID, &a.Payload, &a.IdempotencyKey, &a.Route, &a.TargetServiceID, &a.HashKey, &a.GameID, &a.Env, &a.State, &a.Mode, &a.Reason); err != nil {
        return nil, err
    }
    return &a, nil
}

func (s *SQLiteStore) Approve(id string) (*Approval, error) {
    // state transition guard
    tx, err := s.db.Begin()
    if err != nil { return nil, err }
    defer func(){ _ = tx.Rollback() }()
    // check & update
    res, err := tx.Exec(`UPDATE approvals SET state='approved', reason=NULL, updated_at=CURRENT_TIMESTAMP WHERE id=? AND state='pending'`, id)
    if err != nil { return nil, err }
    n, _ := res.RowsAffected()
    if n == 0 { return nil, fmt.Errorf("not pending") }
    var a Approval
    row := tx.QueryRow(`SELECT id, created_at, actor, function_id, payload, idempotency_key, route, target_service_id, hash_key, game_id, env, state, mode, reason FROM approvals WHERE id=?`, id)
    if err := row.Scan(&a.ID, &a.CreatedAt, &a.Actor, &a.FunctionID, &a.Payload, &a.IdempotencyKey, &a.Route, &a.TargetServiceID, &a.HashKey, &a.GameID, &a.Env, &a.State, &a.Mode, &a.Reason); err != nil {
        return nil, err
    }
    if err := tx.Commit(); err != nil { return nil, err }
    return &a, nil
}

func (s *SQLiteStore) Reject(id string, reason string) (*Approval, error) {
    tx, err := s.db.Begin()
    if err != nil { return nil, err }
    defer func(){ _ = tx.Rollback() }()
    res, err := tx.Exec(`UPDATE approvals SET state='rejected', reason=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND state='pending'`, reason, id)
    if err != nil { return nil, err }
    n, _ := res.RowsAffected()
    if n == 0 { return nil, fmt.Errorf("not pending") }
    var a Approval
    row := tx.QueryRow(`SELECT id, created_at, actor, function_id, payload, idempotency_key, route, target_service_id, hash_key, game_id, env, state, mode, reason FROM approvals WHERE id=?`, id)
    if err := row.Scan(&a.ID, &a.CreatedAt, &a.Actor, &a.FunctionID, &a.Payload, &a.IdempotencyKey, &a.Route, &a.TargetServiceID, &a.HashKey, &a.GameID, &a.Env, &a.State, &a.Mode, &a.Reason); err != nil {
        return nil, err
    }
    if err := tx.Commit(); err != nil { return nil, err }
    return &a, nil
}

func (s *SQLiteStore) List(f Filter, p Page) (items []*Approval, total int, err error) {
    if p.Size <= 0 { p.Size = 20 }
    if p.Page <= 0 { p.Page = 1 }
    order := "DESC"
    if strings.ToLower(p.Sort) == "created_at_asc" { order = "ASC" }
    where := []string{"1=1"}
    args := []any{}
    add := func(cond string, v any) { where = append(where, cond); args = append(args, v) }
    if f.State != "" { add("state=?", f.State) }
    if f.FunctionID != "" { add("function_id=?", f.FunctionID) }
    if f.GameID != "" { add("game_id=?", f.GameID) }
    if f.Env != "" { add("env=?", f.Env) }
    if f.Actor != "" { add("actor=?", f.Actor) }
    if f.Mode != "" { add("mode=?", f.Mode) }
    w := strings.Join(where, " AND ")
    // count
    cq := "SELECT COUNT(*) FROM approvals WHERE " + w
    if err = s.db.QueryRow(cq, args...).Scan(&total); err != nil { return }
    // list
    q := fmt.Sprintf("SELECT id, created_at, actor, function_id, payload, idempotency_key, route, target_service_id, hash_key, game_id, env, state, mode, reason FROM approvals WHERE %s ORDER BY created_at %s LIMIT ? OFFSET ?", w, order)
    args2 := append(append([]any{}, args...), p.Size, (p.Page-1)*p.Size)
    rows, err := s.db.Query(q, args2...)
    if err != nil { return }
    defer rows.Close()
    for rows.Next() {
        var a Approval
        if err = rows.Scan(&a.ID, &a.CreatedAt, &a.Actor, &a.FunctionID, &a.Payload, &a.IdempotencyKey, &a.Route, &a.TargetServiceID, &a.HashKey, &a.GameID, &a.Env, &a.State, &a.Mode, &a.Reason); err != nil { return }
        aa := a
        items = append(items, &aa)
    }
    return
}

