package agent

import (
	"errors"
	"fmt"
	"strings"
)

// systemdRunner is the function used to invoke systemctl. The default value
// returns a "not supported" error so non-Linux builds compile and tests that
// do not override it fail loudly. On Linux, an init() in sysinfo_linux.go
// replaces it with a real exec.Command implementation.
var systemdRunner = func(args ...string) ([]byte, error) {
	return nil, errors.New("systemctl invocation is not supported on this platform")
}

// runSystemdCmd invokes systemctl with the given arguments and returns its
// combined stdout/stderr output. Errors are wrapped with both the argument
// list and the trimmed process output to make debugging straightforward.
func runSystemdCmd(args ...string) (string, error) {
	output, err := systemdRunner(args...)
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return "", fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, trimmed)
	}
	return string(output), nil
}

// parseSystemdList parses the output of `systemctl list-units --type=service --all --no-pager`.
// Each row has the format: UNIT LOAD ACTIVE SUB DESCRIPTION.
//
// The function lives in this platform-neutral file so the parsing logic can be
// unit-tested without a real systemctl binary.
func parseSystemdList(output, state, namePattern string, limit int) ([]ServiceInfo, error) {
	if limit <= 0 {
		return nil, nil
	}

	var result []ServiceInfo
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if len(result) >= limit {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "UNIT") {
			continue
		}
		// Skip the summary footer that systemctl prints at the bottom
		// (e.g. "N loaded units listed.").
		if strings.HasSuffix(line, "loaded units listed.") {
			break
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		// Skip the footer legend lines such as "LOAD   = Reflects ..." and
		// the plural header row. They have the same columnar shape as real
		// units but the first field is one of {LOAD, ACTIVE, SUB, DESCRIPTION}.
		first := strings.ToUpper(strings.TrimSuffix(fields[0], "="))
		if first == "LOAD" || first == "ACTIVE" || first == "SUB" || first == "DESCRIPTION" {
			continue
		}

		name := strings.TrimSpace(fields[0])
		activeState := fields[2]
		_ = fields[3] // subState - reserved for future use

		if namePattern != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(namePattern)) {
			continue
		}

		status := "unknown"
		switch activeState {
		case "active":
			status = "running"
		case "inactive":
			status = "stopped"
		case "failed":
			status = "stopped"
		}

		if state != "" && status != state {
			continue
		}

		result = append(result, ServiceInfo{
			Name:        name,
			DisplayName: name,
			Status:      status,
			StartType:   "unknown",
		})
	}

	return result, nil
}

// parseSystemdStatus parses the output of `systemctl show <name> --no-pager`,
// which is a series of "Key=Value" lines.
func parseSystemdStatus(name, output string) (*ServiceStatusDetail, error) {
	detail := &ServiceStatusDetail{
		Name:        name,
		DisplayName: name,
		Status:      "unknown",
		StartType:   "unknown",
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Both "systemctl show" (Key=Value) and "systemctl status"
		// (Loaded:/Active:) formats are accepted to remain robust across
		// invocation modes.
		switch {
		case strings.HasPrefix(line, "LoadState="):
			detail.StartType = mapStartType(strings.TrimPrefix(line, "LoadState="))
		case strings.HasPrefix(line, "Loaded="):
			detail.StartType = mapStartType(strings.TrimPrefix(line, "Loaded="))
		case strings.HasPrefix(line, "Loaded:"):
			// systemctl status format: "Loaded: loaded (/path; enabled; ...)"
			// Pass everything after the colon so mapStartType can find the
			// enabled/disabled keyword.
			detail.StartType = mapStartType(afterColon(line))
		case strings.HasPrefix(line, "UnitFileState="):
			// systemctl show emits UnitFileState=enabled|disabled; only apply
			// when LoadState did not already give us a more specific value.
			if detail.StartType == "unknown" {
				detail.StartType = mapStartType(strings.TrimPrefix(line, "UnitFileState="))
			}
		case strings.HasPrefix(line, "ActiveState="):
			detail.Status = mapActiveState(strings.TrimPrefix(line, "ActiveState="))
		case strings.HasPrefix(line, "SubState="):
			// SubState=running|dead refines the status from ActiveState.
			if sub := mapSubState(strings.TrimPrefix(line, "SubState=")); sub != "" {
				detail.Status = sub
			}
		case strings.HasPrefix(line, "Active="):
			detail.Status = mapActiveState(strings.TrimPrefix(line, "Active="))
		case strings.HasPrefix(line, "Active:"):
			detail.Status = mapActiveState(afterColon(line))
		case strings.HasPrefix(line, "MainPID="):
			pidStr := strings.TrimPrefix(line, "MainPID=")
			if pidStr != "0" && pidStr != "" {
				var pid int
				if _, err := fmt.Sscanf(pidStr, "%d", &pid); err == nil && pid > 0 {
					detail.ProcessID = uint32(pid)
				}
			}
		case strings.HasPrefix(line, "ExecStart="):
			detail.BinaryPath = extractBinaryPath(line)
		case strings.HasPrefix(line, "Description="):
			detail.Description = strings.Trim(strings.TrimPrefix(line, "Description="), "\"'")
		}
	}

	return detail, nil
}

func mapStartType(loadedState string) string {
	switch {
	case strings.Contains(loadedState, "enabled"):
		return "auto"
	case strings.Contains(loadedState, "disabled"):
		return "manual"
	default:
		return "unknown"
	}
}

func mapActiveState(activeState string) string {
	switch {
	case strings.Contains(activeState, "(running)"):
		return "running"
	case strings.HasPrefix(activeState, "active"):
		return "running"
	case strings.Contains(activeState, "inactive"), strings.Contains(activeState, "(dead)"), strings.HasPrefix(activeState, "inactive"):
		return "stopped"
	case strings.Contains(activeState, "failed"):
		return "stopped"
	default:
		return "unknown"
	}
}

// mapSubState refines an ActiveState-derived status using SubState values
// (running, dead, failed, ...). Returns "" when the value does not change
// anything meaningful.
func mapSubState(sub string) string {
	switch sub {
	case "running":
		return "running"
	case "dead", "failed", "exited", "stop-sigterm":
		return "stopped"
	default:
		return ""
	}
}

// afterColon returns the substring after the first ":", trimmed of leading
// whitespace. Used to parse "Key: value" lines emitted by `systemctl status`.
func afterColon(line string) string {
	if idx := strings.Index(line, ":"); idx >= 0 {
		return strings.TrimSpace(line[idx+1:])
	}
	return ""
}

func extractBinaryPath(line string) string {
	if !strings.Contains(line, "path=") {
		return ""
	}
	start := strings.Index(line, "path=")
	if start < 0 {
		return ""
	}
	start += len("path=")
	rest := line[start:]
	end := strings.IndexAny(rest, " \t")
	if end > 0 {
		return rest[:end]
	}
	return strings.Trim(rest, "{}")
}
