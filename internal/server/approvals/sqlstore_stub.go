//go:build !sqlite

package approvals

import "fmt"

// NewSQLiteStore is available when built with -tags sqlite.
func NewSQLiteStore(dsn string) (Store, error) {
    return nil, fmt.Errorf("sqlite approval store not enabled (build with -tags sqlite)")
}

