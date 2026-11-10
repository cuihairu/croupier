package powerops

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeIPMITool writes a shell script that records argv (one line, \x1f
// separated) and prints a canned status line — command assembly is the
// contract under test, not a real BMC.
func fakeIPMITool(t *testing.T, stdout string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ipmitool")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " +
		filepath.Join(dir, "calls.log") + "\necho " + stdout + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func ipmiCalls(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(path), "calls.log"))
	if err != nil {
		t.Fatalf("read calls.log: %v", err)
	}
	var calls [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line != "" {
			calls = append(calls, strings.Fields(line))
		}
	}
	if len(calls) == 0 {
		t.Fatal("fake ipmitool recorded no calls")
	}
	return calls
}

func testMachine() Machine {
	return Machine{NodeID: "node-1", Provider: ProviderIPMI, BmcAddr: "10.0.0.7", BmcPort: 623}
}

func TestIPMICommandAssembly(t *testing.T) {
	t.Setenv("CROUPIER_TEST_BMC_PW", "s3cret")
	bin := fakeIPMITool(t, "Chassis Power is on")
	p, err := NewIPMIProvider(IPMIProvider{
		Binary: bin, User: "admin", PasswordEnv: "CROUPIER_TEST_BMC_PW",
		ExtraArgs: []string{"-I", "lanplus"},
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	ctx := context.Background()
	m := testMachine()
	if _, err = p.PowerOn(ctx, m); err != nil {
		t.Fatalf("power on: %v", err)
	}
	if _, err = p.PowerOff(ctx, m, false); err != nil {
		t.Fatalf("soft off: %v", err)
	}
	if _, err = p.PowerOff(ctx, m, true); err != nil {
		t.Fatalf("force off: %v", err)
	}
	if _, err = p.Reboot(ctx, m); err != nil {
		t.Fatalf("reboot: %v", err)
	}
	calls := ipmiCalls(t, bin)
	wantSubs := []string{"on", "soft", "off", "cycle"}
	for i, want := range wantSubs {
		last := calls[i]
		if got := last[len(last)-1]; got != want {
			t.Fatalf("call %d sub=%q want %q (args=%v)", i, got, want, last)
		}
		for _, secret := range []string{"-H", "10.0.0.7", "-U", "admin", "-P", "s3cret", "-I", "lanplus", "chassis", "power"} {
			if !contains(last, secret) {
				t.Fatalf("call %d missing %q: %v", i, secret, last)
			}
		}
		// Port must ride the -p flag.
		if !contains(last, "623") {
			t.Fatalf("call %d missing bmc port: %v", i, last)
		}
	}
}

func contains(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}

func TestIPMIStatusParsing(t *testing.T) {
	ctx := context.Background()
	m := testMachine()

	onBin := fakeIPMITool(t, "Chassis Power is on")
	pOn, _ := NewIPMIProvider(IPMIProvider{Binary: onBin})
	if st, err := pOn.Status(ctx, m); err != nil || st != "on" {
		t.Fatalf("status on=%q err=%v", st, err)
	}

	offBin := fakeIPMITool(t, "Chassis Power is off")
	pOff, _ := NewIPMIProvider(IPMIProvider{Binary: offBin})
	if st, err := pOff.Status(ctx, m); err != nil || st != "off" {
		t.Fatalf("status off=%q err=%v", st, err)
	}

	garbageBin := fakeIPMITool(t, "unexpected noise")
	pG, _ := NewIPMIProvider(IPMIProvider{Binary: garbageBin})
	st, err := pG.Status(ctx, m)
	if err == nil || st != "unknown" {
		t.Fatalf("garbage status=%q err=%v want unknown", st, err)
	}

	// Missing binary fails at construction, not at approve-then-fail time.
	if _, err := NewIPMIProvider(IPMIProvider{Binary: "/nonexistent/ipmitool-xyz"}); err == nil {
		t.Fatal("missing binary must fail construction")
	}
}

func TestWOLMagicPacketAndSend(t *testing.T) {
	pkt, err := MagicPacket("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("magic packet: %v", err)
	}
	if len(pkt) != 102 {
		t.Fatalf("packet len=%d want 102", len(pkt))
	}
	for i := 0; i < 6; i++ {
		if pkt[i] != 0xFF {
			t.Fatalf("sync byte %d=%x", i, pkt[i])
		}
	}
	for rep := 0; rep < 16; rep++ {
		off := 6 + rep*6
		if pkt[off] != 0xAA || pkt[off+5] != 0xFF {
			t.Fatalf("rep %d mac bytes wrong: %x", rep, pkt[off:off+6])
		}
	}
	if _, err := MagicPacket("not-a-mac"); err == nil {
		t.Fatal("malformed mac must be rejected")
	}

	// Real UDP loopback: a listener must receive exactly the packet.
	received := make(chan []byte, 1)
	addr := "127.0.0.1:0"
	if _, err := net.ResolveUDPAddr("udp", addr); err != nil {
		t.Skipf("udp unavailable: %v", err)
	}
	ln, err := net.ListenPacket("udp", addr)
	if err != nil {
		t.Skipf("udp listen unavailable: %v", err)
	}
	defer ln.Close()
	udp := ln.(*net.UDPConn)
	go func() {
		buf := make([]byte, 2048)
		n, _, err := udp.ReadFrom(buf)
		if err == nil {
			received <- buf[:n]
		}
		close(received)
	}()

	p := NewWOLProvider()
	m := Machine{NodeID: "node-2", Provider: ProviderWOL, MAC: "aa-bb-cc-dd-ee-ff", BmcAddr: udp.LocalAddr().String()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := p.PowerOn(ctx, m)
	if err != nil {
		t.Fatalf("wol power on: %v", err)
	}
	if !strings.Contains(out, udp.LocalAddr().String()) {
		t.Fatalf("output=%q misses target addr", out)
	}
	select {
	case got := <-received:
		if string(got) != string(pkt) {
			t.Fatalf("received %d bytes, want the magic packet", len(got))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no packet received on loopback")
	}

	// WOL is wake-only.
	if _, err := p.PowerOff(ctx, m, false); err != ErrUnsupportedAction {
		t.Fatalf("wol power off err=%v", err)
	}
	if _, err := p.PowerOff(ctx, m, true); err != ErrUnsupportedAction {
		t.Fatalf("wol force off err=%v", err)
	}
	if _, err := p.Reboot(ctx, m); err != ErrUnsupportedAction {
		t.Fatalf("wol reboot err=%v", err)
	}
	if st, err := p.Status(ctx, m); err == nil || st != "unknown" {
		t.Fatalf("wol status=%q err=%v", st, err)
	}
}

func TestCloudTemplateExecution(t *testing.T) {
	rec := &recordingRunner{}
	p := &CloudProvider{
		Templates: map[Action]string{
			ActionPowerOn:  "cloudctl start --id {instanceRef}",
			ActionPowerOff: "cloudctl stop --id {instanceRef}",
			ActionForceOff: "cloudctl force-off --id {instanceRef}",
			ActionReboot:   "cloudctl reboot --id {instanceRef}",
			ActionStatus:   "cloudctl describe --id {instanceRef}",
		},
		Runner: rec,
	}
	ctx := context.Background()
	m := Machine{NodeID: "cloud-1", Provider: ProviderCloud, CloudRef: "i-123"}
	for _, a := range []Action{ActionPowerOn, ActionReboot, ActionStatus} {
		// RunAction returns an ActionResult; errors land in res.Error.
		if res := RunAction(ctx, providersOf(p), m, a, time.Now()); !res.Success {
			t.Fatalf("action %s failed: %s", a, res.Error)
		}
	}
	// graceful off + force off
	res := RunAction(ctx, providersOf(p), m, ActionPowerOff, time.Now())
	if !res.Success {
		t.Fatalf("graceful off: %s", res.Error)
	}
	res = RunAction(ctx, providersOf(p), m, ActionForceOff, time.Now())
	if !res.Success {
		t.Fatalf("force off: %s", res.Error)
	}
	want := []string{
		"cloudctl start --id i-123",
		"cloudctl reboot --id i-123",
		"cloudctl describe --id i-123",
		"cloudctl stop --id i-123",
		"cloudctl force-off --id i-123",
	}
	if strings.Join(rec.commands, "|") != strings.Join(want, "|") {
		t.Fatalf("commands=%v want %v", rec.commands, want)
	}

	// Missing cloudRef is an honest failure, not a template run.
	bad := Machine{NodeID: "cloud-2", Provider: ProviderCloud}
	if res := RunAction(ctx, providersOf(p), bad, ActionPowerOn, time.Now()); res.Success || res.Error == "" {
		t.Fatalf("missing cloudRef res=%+v", res)
	}

	// Unparsable status output is unknown.
	rec.output = "some noise"
	st, err := p.Status(ctx, m)
	if err == nil || st != "unknown" {
		t.Fatalf("noise status=%q err=%v", st, err)
	}
}

func providersOf(p Provider) map[string]Provider {
	return map[string]Provider{p.Kind(): p}
}

type recordingRunner struct {
	commands []string
	output   string
}

func (r *recordingRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	r.commands = append(r.commands, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	if r.output != "" {
		return r.output, nil
	}
	return "running", nil
}

func TestAgentProviderInBand(t *testing.T) {
	var executed []string
	cmdr := &mockCommander{exec: func(agentID, command string, timeout int) (string, error) {
		executed = append(executed, command)
		return "ok", nil
	}}
	p := &AgentProvider{Commander: cmdr}
	ctx := context.Background()
	m := Machine{NodeID: "agent-1", Provider: ProviderAgent, AgentID: "ag-9"}

	if _, err := p.PowerOn(ctx, m); err != ErrUnsupportedAction {
		t.Fatalf("agent power on err=%v", err)
	}
	// Force off is refused by design.
	if _, err := p.PowerOff(ctx, m, true); err == nil {
		t.Fatal("agent force off must be refused")
	}
	if _, err := p.PowerOff(ctx, m, false); err != nil {
		t.Fatalf("graceful off: %v", err)
	}
	if _, err := p.Reboot(ctx, m); err != nil {
		t.Fatalf("reboot: %v", err)
	}
	want := []string{"shutdown -h now", "systemctl reboot"}
	if strings.Join(executed, "|") != strings.Join(want, "|") {
		t.Fatalf("commands=%v want %v", executed, want)
	}
	// Liveness probe: reachable → on.
	if st, err := p.Status(ctx, m); err != nil || st != "on" {
		t.Fatalf("status=%q err=%v", st, err)
	}
	// Unreachable → off (verify loop's reconnect check).
	down := &mockCommander{exec: func(agentID, command string, timeout int) (string, error) {
		return "", context.DeadlineExceeded
	}}
	pDown := &AgentProvider{Commander: down}
	if st, err := pDown.Status(ctx, m); err != nil || st != "off" {
		t.Fatalf("down status=%q err=%v", st, err)
	}
}

type mockCommander struct {
	exec func(agentID, command string, timeout int) (string, error)
}

func (m *mockCommander) Execute(ctx context.Context, agentID, command string, timeoutSeconds int) (string, error) {
	return m.exec(agentID, command, timeoutSeconds)
}

func TestRunActionDispatchAndHonestFailures(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	m := Machine{NodeID: "n1", Provider: "nosuch"}

	// Missing provider → honest failure row, no panic.
	res := RunAction(ctx, map[string]Provider{}, m, ActionPowerOn, now)
	if res.Success || res.Error == "" || res.NodeID != "n1" {
		t.Fatalf("missing provider res=%+v", res)
	}

	// Unsupported action → honest failure row.
	wol := NewWOLProvider()
	res = RunAction(ctx, providersOf(wol), Machine{NodeID: "n2", Provider: ProviderWOL, MAC: "aa:bb:cc:dd:ee:01"}, ActionReboot, now)
	if res.Success || !strings.Contains(res.Error, ErrUnsupportedAction.Error()) {
		t.Fatalf("unsupported action res=%+v", res)
	}

	// Supported action routes to the right provider.
	rec := &recordingRunner{}
	cloud := &CloudProvider{Templates: map[Action]string{ActionPowerOn: "cloudctl start {instanceRef}"}, Runner: rec}
	res = RunAction(ctx, providersOf(cloud), Machine{NodeID: "n3", Provider: ProviderCloud, CloudRef: "i-9"}, ActionPowerOn, now)
	if !res.Success || res.At.IsZero() {
		t.Fatalf("cloud power on res=%+v", res)
	}
	if len(rec.commands) != 1 || !strings.Contains(rec.commands[0], "i-9") {
		t.Fatalf("runner commands=%v", rec.commands)
	}
}

func TestActionMetadata(t *testing.T) {
	if ActionForceOff.RiskWord() != "high" {
		t.Fatalf("force_off risk=%s", ActionForceOff.RiskWord())
	}
	if ActionPowerOn.RiskWord() != "low" {
		t.Fatalf("power_on risk=%s", ActionPowerOn.RiskWord())
	}
	if got := ActionReboot.FunctionID(); got != "ops.power.reboot" {
		t.Fatalf("functionID=%s", got)
	}
}
