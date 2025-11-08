//go:build !pg && !sqlite

package gamesmeta

type PGStore struct{}

func NewPGStore(dsn string) (Store, error) { return nil, ErrDriverUnavailable("pg") }

type sqliteStub struct{}

func NewSQLiteStore(dsn string) (Store, error) { return nil, ErrDriverUnavailable("sqlite") }
