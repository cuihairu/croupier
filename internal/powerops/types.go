// Package powerops implements the remote machine power lifecycle (OPS power
// management, docs/design/ops-power-management.md): power on / off / reboot /
// status across four provider kinds (ipmi, wol, cloud, agent), plus the
// approval-gated request state machine (pending → executing → verifying →
// done/partial/failed, rejected on approval reject).
//
// The HTTP layer stays a DTO shell; orchestration lives here (provision
// layering precedent). Provider actions are protocol wrappers only — no
// provider ever mutates request state.
package powerops

import (
	"context"
	"errors"
	"time"
)

// Action is a power lifecycle operation. The wire spelling is lower snake
// (machine identifier vocabulary, not contract keys).
type Action string

const (
	ActionPowerOn  Action = "power_on"
	ActionPowerOff Action = "power_off"
	ActionForceOff Action = "force_off"
	ActionReboot   Action = "reboot"
	ActionStatus   Action = "status"
)

// Provider kinds. ipmi/wol/cloud are out-of-band (work with the target's
// agent down — the auto power-on scenario); agent is in-band (dies with the
// host, used for planned graceful reboot/shutdown only).
const (
	ProviderIPMI  = "ipmi"
	ProviderWOL   = "wol"
	ProviderCloud = "cloud"
	ProviderAgent = "agent"
)

// ErrUnsupportedAction is returned when a provider cannot express an action
// (e.g. WOL wake-only). Callers map it to an honest per-machine failure, not
// a fake success.
var ErrUnsupportedAction = errors.New("powerops: action not supported by provider")

// ErrUnknownMachineState is the Status sentinel when a provider cannot tell.
var ErrUnknownMachineState = errors.New("powerops: power state unknown")

// RiskWord maps an action to the approval risk vocabulary (low/medium/high).
// It is metadata for the approval record and the x-risk OpenAPI declarations;
// the approve-time OTP gate resolves risk from the FunctionID through the
// shared ResolvePolicyRisk seam, never from this word directly.
func (a Action) RiskWord() string {
	switch a {
	case ActionPowerOn, ActionStatus:
		return "low"
	case ActionPowerOff, ActionReboot:
		return "medium"
	case ActionForceOff:
		return "high"
	default:
		return "medium"
	}
}

// FunctionID is the governance function id the approval record and risk
// resolution key on (ops.power.<action>, e.g. ops.power.force_off).
func (a Action) FunctionID() string {
	return "ops.power." + string(a)
}

// Machine is one power target, resolved from the machine inventory
// (nodes.meta.power) at submit time and snapshotted into the request so the
// approved scope cannot silently change afterwards.
type Machine struct {
	NodeID   string `json:"nodeId"`
	Provider string `json:"provider"`
	// BmcAddr is the BMC host for ipmi, broadcast address for wol (with
	// BcastPort), unused otherwise.
	BmcAddr string `json:"bmcAddr,omitempty"`
	BmcPort int    `json:"bmcPort,omitempty"`
	// MAC is the wake target for wol.
	MAC string `json:"mac,omitempty"`
	// CloudRef is the provider-side instance reference for cloud machines.
	CloudRef string `json:"cloudRef,omitempty"`
	// AgentID is the in-band session id for agent machines.
	AgentID string `json:"agentId,omitempty"`
}

// ActionResult is one machine's outcome inside a power request.
type ActionResult struct {
	NodeID   string    `json:"nodeId"`
	Success  bool      `json:"success"`
	State    string    `json:"state,omitempty"` // on/off/unknown
	Output   string    `json:"output,omitempty"`
	Error    string    `json:"error,omitempty"`
	Verified bool      `json:"verified"`
	At       time.Time `json:"at"`
}

// Provider executes power actions for one machine kind. Implementations are
// protocol wrappers; verification polling reuses Status.
type Provider interface {
	Kind() string
	PowerOn(ctx context.Context, m Machine) (string, error)
	PowerOff(ctx context.Context, m Machine, force bool) (string, error)
	Reboot(ctx context.Context, m Machine) (string, error)
	// Status returns "on", "off" or "unknown" for the machine.
	Status(ctx context.Context, m Machine) (string, error)
}

// Supports reports whether a provider kind can express an action. power_on
// and status work everywhere except agent (which cannot power on a host it
// is not running on); force_off is refused on the agent kind by design — the
// channel dies with the host (see design §9).
func Supports(provider string, a Action) bool {
	switch a {
	case ActionPowerOn:
		return provider == ProviderIPMI || provider == ProviderWOL || provider == ProviderCloud
	case ActionPowerOff:
		return provider == ProviderIPMI || provider == ProviderCloud
	case ActionForceOff:
		return provider == ProviderIPMI || provider == ProviderCloud
	case ActionReboot:
		return provider == ProviderIPMI || provider == ProviderCloud || provider == ProviderAgent
	case ActionStatus:
		return provider == ProviderIPMI || provider == ProviderCloud || provider == ProviderAgent
	default:
		return false
	}
}

// RunAction dispatches one action against the right provider and wraps the
// outcome in an ActionResult (never panics on provider absence — missing
// provider is an honest failure row).
func RunAction(ctx context.Context, providers map[string]Provider, m Machine, a Action, at time.Time) ActionResult {
	res := ActionResult{NodeID: m.NodeID, At: at}
	p := providers[m.Provider]
	if p == nil {
		res.Error = "powerops: no provider registered for kind " + m.Provider
		return res
	}
	if !Supports(m.Provider, a) {
		res.Error = ErrUnsupportedAction.Error()
		return res
	}
	var (
		out string
		err error
	)
	switch a {
	case ActionPowerOn:
		out, err = p.PowerOn(ctx, m)
	case ActionPowerOff:
		out, err = p.PowerOff(ctx, m, false)
	case ActionForceOff:
		out, err = p.PowerOff(ctx, m, true)
	case ActionReboot:
		out, err = p.Reboot(ctx, m)
	case ActionStatus:
		out, err = p.Status(ctx, m)
	}
	if out != "" {
		res.Output = out
	}
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.Success = true
	if a == ActionStatus {
		res.State = out
	}
	return res
}
