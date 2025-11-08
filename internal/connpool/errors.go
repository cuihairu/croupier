package connpool

import "errors"

var (
	// ErrPoolClosed is returned when the pool is closed
	ErrPoolClosed = errors.New("connection pool is closed")

	// ErrTooManyConnections is returned when the maximum number of connections is reached
	ErrTooManyConnections = errors.New("too many connections for target")

	// ErrConnectionUnhealthy is returned when a connection is unhealthy
	ErrConnectionUnhealthy = errors.New("connection is unhealthy")

	// ErrDialTimeout is returned when connection dial times out
	ErrDialTimeout = errors.New("connection dial timeout")
)
