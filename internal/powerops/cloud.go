package powerops

import (
	"context"
	"fmt"
	"strings"
)

// CloudProvider drives instance power through config-declared command
// templates (design §9 — command-template layer, not deep provider SDK
// integration; a future SDK integration replaces this). Templates may embed
// "{instanceRef}" which is replaced with the machine's CloudRef.
//
// Cloud credentials live in the template's own environment (server config /
// CLI config files), never in API payloads.
type CloudProvider struct {
	// Templates maps action → command. The first token is the binary, the
	// rest are args (split on whitespace; quoting is not supported — keep
	// templates simple).
	Templates map[Action]string
	Runner    CommandRunner
}

func (p *CloudProvider) Kind() string { return ProviderCloud }

func (p *CloudProvider) template(a Action) (string, bool) {
	t, ok := p.Templates[a]
	if !ok || strings.TrimSpace(t) == "" {
		return "", false
	}
	return t, true
}

// run executes the configured template for an action against one machine.
func (p *CloudProvider) run(ctx context.Context, a Action, m Machine) (string, error) {
	tmpl, ok := p.template(a)
	if !ok {
		return "", ErrUnsupportedAction
	}
	if m.CloudRef == "" {
		return "", fmt.Errorf("powerops: machine %s missing cloudRef", m.NodeID)
	}
	fields := strings.Fields(strings.ReplaceAll(tmpl, "{instanceRef}", m.CloudRef))
	if len(fields) == 0 {
		return "", fmt.Errorf("powerops: empty cloud template for %s", a)
	}
	return p.Runner.Run(ctx, fields[0], fields[1:]...)
}

// PowerOn runs the power_on template (e.g. provider CLI start-instance).
func (p *CloudProvider) PowerOn(ctx context.Context, m Machine) (string, error) {
	return p.run(ctx, ActionPowerOn, m)
}

// PowerOff runs the power_off template; force uses the force_off template
// when configured, falling back to power_off (provider semantics vary — the
// provider's own result is reported honestly).
func (p *CloudProvider) PowerOff(ctx context.Context, m Machine, force bool) (string, error) {
	if force {
		if _, ok := p.template(ActionForceOff); ok {
			return p.run(ctx, ActionForceOff, m)
		}
	}
	return p.run(ctx, ActionPowerOff, m)
}

// Reboot runs the reboot template.
func (p *CloudProvider) Reboot(ctx context.Context, m Machine) (string, error) {
	return p.run(ctx, ActionReboot, m)
}

// Status runs the status template; the output must contain "on"/"off" to be
// conclusive — anything else is honest "unknown".
func (p *CloudProvider) Status(ctx context.Context, m Machine) (string, error) {
	out, err := p.run(ctx, ActionStatus, m)
	if err != nil {
		return "unknown", err
	}
	lower := strings.ToLower(out)
	switch {
	case strings.Contains(lower, "running"), strings.Contains(lower, "on"):
		return "on", nil
	case strings.Contains(lower, "stopped"), strings.Contains(lower, "off"):
		return "off", nil
	default:
		return "unknown", ErrUnknownMachineState
	}
}
