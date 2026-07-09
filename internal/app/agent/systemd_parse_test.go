package agent

import (
	"errors"
	"strings"
	"testing"
)

func TestParseSystemdList(t *testing.T) {
	t.Parallel()

	output := `  UNIT                                           LOAD   ACTIVE SUB     DESCRIPTION
  ssh.service                                    loaded active running OpenSSH server daemon
  cron.service                                   loaded active running Regular background program processing daemon
  nginx.service                                  loaded inactive dead    nginx high performance web server
  docker.service                                 loaded failed failed  Docker Application Container Engine

LOAD = Reflects whether the unit definition was properly loaded.
4 loaded units listed.`
	tests := []struct {
		name        string
		state       string
		namePattern string
		limit       int
		wantCount   int
		wantNames   []string
	}{
		{name: "all", limit: 10, wantCount: 4, wantNames: []string{"ssh.service", "cron.service", "nginx.service", "docker.service"}},
		{name: "running only", state: "running", limit: 10, wantCount: 2, wantNames: []string{"ssh.service", "cron.service"}},
		{name: "stopped only", state: "stopped", limit: 10, wantCount: 2, wantNames: []string{"nginx.service", "docker.service"}},
		{name: "name filter", namePattern: "sh", limit: 10, wantCount: 1, wantNames: []string{"ssh.service"}},
		{name: "limit", limit: 1, wantCount: 1, wantNames: []string{"ssh.service"}},
		{name: "limit zero", limit: 0, wantCount: 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			services, err := parseSystemdList(output, tc.state, tc.namePattern, tc.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(services) != tc.wantCount {
				t.Fatalf("got %d services, want %d: %+v", len(services), tc.wantCount, services)
			}
			if len(tc.wantNames) == 0 {
				return
			}
			for i, want := range tc.wantNames {
				if services[i].Name != want {
					t.Errorf("services[%d].Name = %q, want %q", i, services[i].Name, want)
				}
			}
		})
	}
}

func TestParseSystemdListStopsAtFooter(t *testing.T) {
	t.Parallel()

	output := `UNIT                       LOAD   ACTIVE SUB
foo.service                loaded active running
bar.service                loaded active running

2 loaded units listed.`
	services, err := parseSystemdList(output, "", "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
}

func TestParseSystemdStatus(t *testing.T) {
	t.Parallel()

	// `systemctl show` Key=Value format
	output := `Id=ssh.service
Description=OpenBSD Secure Shell server
LoadState=loaded
ActiveState=active
SubState=running
UnitFileState=enabled
MainPID=1234
ExecStart={ path=/usr/sbin/sshd ; argv[]=/usr/sbin/sshd -D }
`
	detail, err := parseSystemdStatus("ssh.service", output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Name != "ssh.service" {
		t.Errorf("Name = %q", detail.Name)
	}
	if detail.Status != "running" {
		t.Errorf("Status = %q, want running", detail.Status)
	}
	if detail.StartType != "auto" {
		t.Errorf("StartType = %q, want auto", detail.StartType)
	}
	if detail.ProcessID != 1234 {
		t.Errorf("ProcessID = %d, want 1234", detail.ProcessID)
	}
	if detail.BinaryPath != "/usr/sbin/sshd" {
		t.Errorf("BinaryPath = %q, want /usr/sbin/sshd", detail.BinaryPath)
	}
	if detail.Description != "OpenBSD Secure Shell server" {
		t.Errorf("Description = %q", detail.Description)
	}
}

func TestParseSystemdStatusLegacyFormat(t *testing.T) {
	t.Parallel()

	// `systemctl status` human-readable format
	output := `● ssh.service - OpenSSH server daemon
     Loaded: loaded (/lib/systemd/system/ssh.service; enabled; vendor preset: enabled)
     Active: active (running) since Thu 2025-01-01 00:00:00 UTC; 1h ago
   Main PID: 1234 (sshd)`
	detail, err := parseSystemdStatus("ssh.service", output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Status != "running" {
		t.Errorf("Status = %q, want running", detail.Status)
	}
	if detail.StartType != "auto" {
		t.Errorf("StartType = %q, want auto", detail.StartType)
	}
}

func TestParseSystemdStatusMainPIDZeroIgnored(t *testing.T) {
	t.Parallel()

	output := `MainPID=0
ActiveState=inactive
`
	detail, err := parseSystemdStatus("foo.service", output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.ProcessID != 0 {
		t.Errorf("ProcessID = %d, want 0", detail.ProcessID)
	}
	if detail.Status != "stopped" {
		t.Errorf("Status = %q, want stopped", detail.Status)
	}
}

func TestExtractBinaryPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{in: "ExecStart={ path=/usr/sbin/sshd ; argv[]=/usr/sbin/sshd -D }", want: "/usr/sbin/sshd"},
		{in: "ExecStart={ argv[]=/bin/foo }", want: ""},
		{in: "no path here", want: ""},
	}
	for _, c := range cases {
		if got := extractBinaryPath(c.in); got != c.want {
			t.Errorf("extractBinaryPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestRunSystemdCmdFailurePath verifies that runSystemdCmd wraps subprocess
// failures with the offending arguments for easier debugging. Runs on all
// platforms via the systemdRunner indirection.
func TestRunSystemdCmdFailurePath(t *testing.T) {
	t.Parallel()

	original := systemdRunner
	t.Cleanup(func() { systemdRunner = original })

	systemdRunner = func(args ...string) ([]byte, error) {
		return []byte("Unit nonexistent.service could not be found."), errors.New("exit status 1")
	}

	_, err := runSystemdCmd("show", "nonexistent.service")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "systemctl show") {
		t.Errorf("error should mention invocation: %v", err)
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("error should wrap underlying: %v", err)
	}
}

func TestRunSystemdCmdSuccess(t *testing.T) {
	t.Parallel()

	original := systemdRunner
	t.Cleanup(func() { systemdRunner = original })

	systemdRunner = func(args ...string) ([]byte, error) {
		return []byte("LIST_OUTPUT"), nil
	}

	out, err := runSystemdCmd("list-units", "--type=service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "LIST_OUTPUT" {
		t.Errorf("got %q, want LIST_OUTPUT", out)
	}
}
