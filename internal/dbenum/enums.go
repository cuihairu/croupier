package dbenum

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Capability enumerates FunctionContract.capability. Wire form follows
// spec.CapabilityKind strings (collection_query|item_query|create|update|
// delete|action|task|report).
type Capability int

const (
	CapabilityUnknown Capability = iota
	CapabilityCollectionQuery
	CapabilityItemQuery
	CapabilityCreate
	CapabilityUpdate
	CapabilityDelete
	CapabilityAction
	CapabilityTask
	CapabilityReport
)

var capabilityNames = map[Capability]string{
	CapabilityUnknown:         "unknown",
	CapabilityCollectionQuery: "collection_query",
	CapabilityItemQuery:       "item_query",
	CapabilityCreate:          "create",
	CapabilityUpdate:          "update",
	CapabilityDelete:          "delete",
	CapabilityAction:          "action",
	CapabilityTask:            "task",
	CapabilityReport:          "report",
}

func (c Capability) String() string {
	if name, ok := capabilityNames[c]; ok {
		return name
	}
	return "unknown"
}

func (c Capability) Value() (driver.Value, error) { return ValueInt(int(c)) }

func (c *Capability) Scan(src any) error { return ScanInt((*int)(c), src) }

func (c Capability) MarshalJSON() ([]byte, error) { return json.Marshal(c.String()) }

func (c *Capability) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	parsed, err := ParseCapability(name)
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// ParseCapability converts a wire/domain capability name into the enum.
// The empty string parses to CapabilityUnknown (legacy rows).
func ParseCapability(name string) (Capability, error) {
	for cap, wire := range capabilityNames {
		if wire == name {
			return cap, nil
		}
	}
	return 0, fmt.Errorf("dbenum: unknown capability %q", name)
}

// ProposalStatus enumerates PageProposal.status (pending|accepted|rejected).
type ProposalStatus int

const (
	ProposalStatusPending ProposalStatus = iota
	ProposalStatusAccepted
	ProposalStatusRejected
)

var proposalStatusNames = map[ProposalStatus]string{
	ProposalStatusPending:  "pending",
	ProposalStatusAccepted: "accepted",
	ProposalStatusRejected: "rejected",
}

func (p ProposalStatus) String() string {
	if name, ok := proposalStatusNames[p]; ok {
		return name
	}
	return "pending"
}

func (p ProposalStatus) Value() (driver.Value, error) { return ValueInt(int(p)) }

func (p *ProposalStatus) Scan(src any) error { return ScanInt((*int)(p), src) }

func (p ProposalStatus) MarshalJSON() ([]byte, error) { return json.Marshal(p.String()) }

func (p *ProposalStatus) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	parsed, err := ParseProposalStatus(name)
	if err != nil {
		return err
	}
	*p = parsed
	return nil
}

// ParseProposalStatus converts a wire proposal status into the enum. The
// legacy "expired" value maps to pending (it was never written in practice).
func ParseProposalStatus(name string) (ProposalStatus, error) {
	switch name {
	case "", "pending", "expired":
		return ProposalStatusPending, nil
	case "accepted":
		return ProposalStatusAccepted, nil
	case "rejected":
		return ProposalStatusRejected, nil
	default:
		return 0, fmt.Errorf("dbenum: unknown proposal status %q", name)
	}
}

// TicketStatus enumerates Ticket.status (open|in_progress|resolved|closed).
type TicketStatus int

const (
	TicketStatusOpen TicketStatus = iota
	TicketStatusInProgress
	TicketStatusResolved
	TicketStatusClosed
)

var ticketStatusNames = map[TicketStatus]string{
	TicketStatusOpen:       "open",
	TicketStatusInProgress: "in_progress",
	TicketStatusResolved:   "resolved",
	TicketStatusClosed:     "closed",
}

func (t TicketStatus) String() string {
	if name, ok := ticketStatusNames[t]; ok {
		return name
	}
	return "open"
}

func (t TicketStatus) Value() (driver.Value, error) { return ValueInt(int(t)) }

func (t *TicketStatus) Scan(src any) error { return ScanInt((*int)(t), src) }

func (t TicketStatus) MarshalJSON() ([]byte, error) { return json.Marshal(t.String()) }

func (t *TicketStatus) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	parsed, err := ParseTicketStatus(name)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

// ParseTicketStatus converts a wire ticket status into the enum.
func ParseTicketStatus(name string) (TicketStatus, error) {
	for status, wire := range ticketStatusNames {
		if wire == name {
			return status, nil
		}
	}
	return 0, fmt.Errorf("dbenum: unknown ticket status %q", name)
}
