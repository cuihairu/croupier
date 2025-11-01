//go:build !pg

package approvals

import "fmt"

// NewPGStore is implemented in pgstore_pg.go behind build tag 'pg'.
// The stub returns an error when not built with the 'pg' tag.
func NewPGStore(dsn string) (Store, error) {
    return nil, fmt.Errorf("postgres approval store not enabled (build with -tags pg)")
}
