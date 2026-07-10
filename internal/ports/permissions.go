// Package ports defines domain-facing narrow interfaces (ports) that decouple
// API/logic layers from the concrete *svc.ServiceContext service locator.
//
// These ports are the target shape for gradually splitting ServiceContext:
// services and logic depend on the smallest interface they need, and
// ServiceContext (the composition root) adapts its concrete components to
// satisfy them. New code should depend on the port, not on ServiceContext.
package ports

import "context"

// Permissions is the authorization port used by scope checks and RBAC gates.
// *svc.PermissionService satisfies this structurally; logic should depend on
// the interface so it can be tested with a fake and so ServiceContext can be
// narrowed one consumer at a time rather than in a single rewrite.
type Permissions interface {
	CheckPermission(ctx context.Context, adminID uint, resource, action string) (bool, error)
	CheckGameScope(ctx context.Context, adminID uint, gameID uint) (bool, error)
	CheckGameEnvScope(ctx context.Context, adminID uint, gameID uint, env string) (bool, error)
}
