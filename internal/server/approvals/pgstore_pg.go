//go:build pg

package approvals

import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
    "fmt"
    "strings"
)

type PGStore struct { db *sql.DB }

func NewPGStore(dsn string) (Store, error) {
    db, err := sql.Open("pgx", dsn)
    if err != nil { return nil, err }
    if err := db.Ping(); err != nil { return nil, err }
    return &PGStore{db: db}, nil
}

func (s *PGStore) Create(a *Approval) error {
    q := `INSERT INTO approvals (id, created_at, actor, function_id, payload, idempotency_key, route, target_service_id, hash_key, game_id, env, state, mode, reason)
          VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
    _, err := s.db.Exec(q, a.ID, a.CreatedAt, a.Actor, a.FunctionID, a.Payload, a.IdempotencyKey, a.Route, a.TargetServiceID, a.HashKey, a.GameID, a.Env, a.State, a.Mode, a.Reason)
    return err
}

func (s *PGStore) Get(id string) (*Approval, error) {
    q := `SELECT id, created_at, actor, function_id, payload, idempotency_key, route, target_service_id, hash_key, game_id, env, state, mode, reason FROM approvals WHERE id=$1`
    row := s.db.QueryRow(q, id)
    var a Approval
    if err := row.Scan(&a.ID, &a.CreatedAt, &a.Actor, &a.FunctionID, &a.Payload, &a.IdempotencyKey, &a.Route, &a.TargetServiceID, &a.HashKey, &a.GameID, &a.Env, &a.State, &a.Mode, &a.Reason); err != nil {
        return nil, err
    }
    return &a, nil
}

func (s *PGStore) Approve(id string) (*Approval, error) {
    // state transition guard
    q := `UPDATE approvals SET state='approved', reason=NULL, updated_at=NOW() WHERE id=$1 AND state='pending' RETURNING id, created_at, actor, function_id, payload, idempotency_key, route, target_service_id, hash_key, game_id, env, state, mode, reason`
    var a Approval
    if err := s.db.QueryRow(q, id).Scan(&a.ID, &a.CreatedAt, &a.Actor, &a.FunctionID, &a.Payload, &a.IdempotencyKey, &a.Route, &a.TargetServiceID, &a.HashKey, &a.GameID, &a.Env, &a.State, &a.Mode, &a.Reason); err != nil {
        return nil, err
    }
    return &a, nil
}

func (s *PGStore) Reject(id string, reason string) (*Approval, error) {
    q := `UPDATE approvals SET state='rejected', reason=$2, updated_at=NOW() WHERE id=$1 AND state='pending' RETURNING id, created_at, actor, function_id, payload, idempotency_key, route, target_service_id, hash_key, game_id, env, state, mode, reason`
    var a Approval
    if err := s.db.QueryRow(q, id, reason).Scan(&a.ID, &a.CreatedAt, &a.Actor, &a.FunctionID, &a.Payload, &a.IdempotencyKey, &a.Route, &a.TargetServiceID, &a.HashKey, &a.GameID, &a.Env, &a.State, &a.Mode, &a.Reason); err != nil {
        return nil, err
    }
    return &a, nil
}

func (s *PGStore) List(f Filter, p Page) (items []*Approval, total int, err error) {
    if p.Size <= 0 { p.Size = 20 }
    if p.Page <= 0 { p.Page = 1 }
    order := "DESC"
    if strings.ToLower(p.Sort) == "created_at_asc" { order = "ASC" }
    // build where dynamically
    where := []string{"1=1"}
    args := []any{}
    idx := 1
    add := func(cond string, v any) {
        where = append(where, fmt.Sprintf(cond, idx))
        args = append(args, v)
        idx++
    }
    if f.State != "" { add("state=$%d", f.State) }
    if f.FunctionID != "" { add("function_id=$%d", f.FunctionID) }
    if f.GameID != "" { add("game_id=$%d", f.GameID) }
    if f.Env != "" { add("env=$%d", f.Env) }
    if f.Actor != "" { add("actor=$%d", f.Actor) }
    if f.Mode != "" { add("mode=$%d", f.Mode) }
    w := strings.Join(where, " AND ")
    // count
    cq := "SELECT COUNT(*) FROM approvals WHERE " + w
    if err = s.db.QueryRow(cq, args...).Scan(&total); err != nil { return }
    // list
    q := "SELECT id, created_at, actor, function_id, payload, idempotency_key, route, target_service_id, hash_key, game_id, env, state, mode, reason FROM approvals WHERE " + w + " ORDER BY created_at " + order + " LIMIT $%d OFFSET $%d"
    q = fmt.Sprintf(q, idx, idx+1)
    args = append(args, p.Size, (p.Page-1)*p.Size)
    rows, err := s.db.Query(q, args...)
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

