package powerops

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandRunner abstracts external command execution so tests can assemble
// commands against a fake ipmitool binary without touching real BMCs.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// ExecCommandRunner runs real binaries (server host must have them; a missing
// binary surfaces at provider construction — design §8).
type ExecCommandRunner struct{}

// Run executes the command and returns combined output.
func (ExecCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("powerops: %s: %w: %s", name, err, strings.TrimSpace(buf.String()))
	}
	return buf.String(), nil
}

// IPMIProvider wraps the ipmitool CLI over the BMC network (out-of-band).
// Password travels as an argv flag — it never appears in API payloads or
// persisted records (design §8); deployments that need more should put a
// wrapper binary in BinaryPath.
type IPMIProvider struct {
	// Binary is the ipmitool path (default "ipmitool").
	Binary string
	User   string
	// PasswordEnv names the environment variable holding the BMC password
	// (provision bootstrap-token pattern; committed config carries the name,
	// never the secret). Empty password is allowed (some BMCs are unauthed
	// on management LANs).
	PasswordEnv string
	// ExtraArgs precede the chassis subcommand (e.g. -I lanplus, -p port).
	ExtraArgs []string
	Runner    CommandRunner
}

// NewIPMIProvider builds the provider; missing binary fails here, not at
// approve-then-fail time.
func NewIPMIProvider(cfg IPMIProvider) (*IPMIProvider, error) {
	if cfg.Runner == nil {
		cfg.Runner = ExecCommandRunner{}
	}
	bin := cfg.Binary
	if bin == "" {
		bin = "ipmitool"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return nil, fmt.Errorf("powerops: ipmi binary %q not found: %w", bin, err)
	}
	cfg.Binary = bin
	return &cfg, nil
}

func (p *IPMIProvider) Kind() string { return ProviderIPMI }

// chassis runs `ipmitool ... chassis power <sub>` with assembled args.
func (p *IPMIProvider) chassis(ctx context.Context, m Machine, sub string) (string, error) {
	if m.BmcAddr == "" {
		return "", fmt.Errorf("powerops: machine %s missing bmcAddr", m.NodeID)
	}
	args := []string{"-H", m.BmcAddr}
	if m.BmcPort > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", m.BmcPort))
	}
	if p.User != "" {
		args = append(args, "-U", p.User)
	}
	if pw := p.password(); pw != "" {
		args = append(args, "-P", pw)
	}
	args = append(args, p.ExtraArgs...)
	args = append(args, "chassis", "power", sub)
	return p.Runner.Run(ctx, p.Binary, args...)
}

func (p *IPMIProvider) password() string {
	if p.PasswordEnv == "" {
		return ""
	}
	return os.Getenv(p.PasswordEnv)
}

// PowerOn sends `chassis power on`.
func (p *IPMIProvider) PowerOn(ctx context.Context, m Machine) (string, error) {
	return p.chassis(ctx, m, "on")
}

// PowerOff sends `chassis power soft` (graceful) or `chassis power off`
// (force, i.e. power cut).
func (p *IPMIProvider) PowerOff(ctx context.Context, m Machine, force bool) (string, error) {
	if force {
		return p.chassis(ctx, m, "off")
	}
	return p.chassis(ctx, m, "soft")
}

// Reboot sends `chassis power cycle` (hard power cycle).
func (p *IPMIProvider) Reboot(ctx context.Context, m Machine) (string, error) {
	return p.chassis(ctx, m, "cycle")
}

// Status parses `chassis power status` output into on/off; unparsable output
// is unknown (honest).
func (p *IPMIProvider) Status(ctx context.Context, m Machine) (string, error) {
	out, err := p.chassis(ctx, m, "status")
	if err != nil {
		return "unknown", err
	}
	switch {
	case strings.Contains(strings.ToLower(out), "on"):
		return "on", nil
	case strings.Contains(strings.ToLower(out), "off"):
		return "off", nil
	default:
		return "unknown", ErrUnknownMachineState
	}
}
