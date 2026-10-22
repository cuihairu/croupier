package probe

import (
	"fmt"
)

type CapabilityError struct {
	Capability string
	Err        error
}

func (e *CapabilityError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("capability: %s is required for ICMP", e.Capability)
	}
	return fmt.Sprintf("capability: %s is required for ICMP: %v", e.Capability, e.Err)
}

func (e *CapabilityError) Unwrap() error { return e.Err }
