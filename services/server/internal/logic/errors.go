package logic

import "errors"

var (
	ErrAgentNotFound   = errors.New("agent not found")
	ErrRateRuleInvalid = errors.New("invalid rate limit rule")
	ErrInvalidRequest  = errors.New("invalid request")
	ErrNotFound        = errors.New("not found")
	ErrUnavailable     = errors.New("service unavailable")
)
