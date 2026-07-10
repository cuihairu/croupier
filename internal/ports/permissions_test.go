package ports

import (
	"testing"

	"github.com/cuihairu/croupier/internal/service/permission"
)

// TestPermissionServiceSatisfiesPort is a compile-time + runtime contract:
// the concrete *svc.PermissionService must structurally satisfy the
// ports.Permissions domain port. This guards the ServiceContext split: as long
// as the adapter satisfies the port, consumers can depend on the narrow
// interface and ServiceContext remains the composition root.
func TestPermissionServiceSatisfiesPort(t *testing.T) {
	// Compile-time assertion: *permission.PermissionService implements
	// ports.Permissions. If this compiles, the adapter is valid.
	var _ Permissions = (*permission.PermissionService)(nil)

	// A nil adapter must still satisfy the interface type (callers guard nil).
	var p Permissions //nolint:unused // documents the satisfied contract
	p = (*permission.PermissionService)(nil)
	_ = p
}
