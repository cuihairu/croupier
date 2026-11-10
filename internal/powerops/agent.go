package powerops

import (
	"context"
	"fmt"
	"time"
)

// AgentCommander executes one in-band command on an agent host. The API
// wiring adapts this to the shared agent-ops client (GetAgentOpsClient);
// the package itself stays free of that dependency.
type AgentCommander interface {
	Execute(ctx context.Context, agentID string, command string, timeoutSeconds int) (string, error)
}

// Default agent command timeouts (seconds).
const (
	agentRebootTimeout  = 30
	agentOfflineTimeout = 5
)

// AgentProvider is the in-band path for machines whose agent session is
// alive (planned graceful reboot / shutdown only — design §3). Power-on via
// agent is impossible (the agent is off exactly when we need it) and
// force-off is refused by design: the channel dies with the host, so a
// "forced" in-band cut is neither forced nor confirmable — deployments use
// the out-of-band provider registered for the machine.
type AgentProvider struct {
	Commander AgentCommander
	// RebootCommand is the host reboot command (default "systemctl reboot").
	RebootCommand string
	// ShutdownCommand is the graceful shutdown (default "shutdown -h now").
	ShutdownCommand string
}

func (p *AgentProvider) Kind() string { return ProviderAgent }

func (p *AgentProvider) rebootCmd() string {
	if p.RebootCommand != "" {
		return p.RebootCommand
	}
	return "systemctl reboot"
}

func (p *AgentProvider) shutdownCmd() string {
	if p.ShutdownCommand != "" {
		return p.ShutdownCommand
	}
	return "shutdown -h now"
}

// PowerOn is not expressible in-band.
func (p *AgentProvider) PowerOn(ctx context.Context, m Machine) (string, error) {
	return "", ErrUnsupportedAction
}

// PowerOff sends the graceful shutdown command; force is refused (design §9).
func (p *AgentProvider) PowerOff(ctx context.Context, m Machine, force bool) (string, error) {
	if force {
		return "", fmt.Errorf("powerops: force off via in-band agent refused (use the out-of-band provider)")
	}
	if m.AgentID == "" {
		return "", fmt.Errorf("powerops: machine %s missing agentId", m.NodeID)
	}
	_, err := p.Commander.Execute(ctx, m.AgentID, p.shutdownCmd(), agentRebootTimeout)
	if err != nil {
		return "", fmt.Errorf("powerops: agent shutdown %s: %w", m.AgentID, err)
	}
	return "shutdown issued via agent " + m.AgentID, nil
}

// Reboot sends the reboot command. The command may outlive the session —
// send errors are reported honestly and the verify loop confirms the host
// actually went down and came back.
func (p *AgentProvider) Reboot(ctx context.Context, m Machine) (string, error) {
	if m.AgentID == "" {
		return "", fmt.Errorf("powerops: machine %s missing agentId", m.NodeID)
	}
	_, err := p.Commander.Execute(ctx, m.AgentID, p.rebootCmd(), agentRebootTimeout)
	if err != nil {
		return "", fmt.Errorf("powerops: agent reboot %s: %w", m.AgentID, err)
	}
	return "reboot issued via agent " + m.AgentID, nil
}

// Status probes session liveness with a short command: reachable = on,
// unreachable = off. This doubles as the verify-loop reconnect check.
func (p *AgentProvider) Status(ctx context.Context, m Machine) (string, error) {
	if m.AgentID == "" {
		return "unknown", ErrUnknownMachineState
	}
	probeCtx, cancel := context.WithTimeout(ctx, agentOfflineTimeout*time.Second)
	defer cancel()
	if _, err := p.Commander.Execute(probeCtx, m.AgentID, "true", agentOfflineTimeout); err != nil {
		return "off", nil
	}
	return "on", nil
}
