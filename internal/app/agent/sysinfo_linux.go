//go:build linux

package agent

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// listServicesPlatform lists Linux systemd services.
func listServicesPlatform(state, namePattern string, limit int) ([]ServiceInfo, error) {
	// Try systemd first
	services, err := listSystemdServices(state, namePattern, limit)
	if err == nil && len(services) > 0 {
		return services, nil
	}

	// Fallback to empty list with explanation
	return nil, fmt.Errorf("no service manager found (systemd not available)")
}

// getServiceStatusPlatform gets detailed Linux service status.
func getServiceStatusPlatform(name string) (*ServiceStatusDetail, error) {
	// Try systemd first
	status, err := getSystemdServiceStatus(name)
	if err == nil {
		return status, nil
	}

	return nil, fmt.Errorf("service %q not found or service manager unavailable", name)
}

// listCronJobsPlatform lists Linux cron jobs.
func listCronJobsPlatform() ([]CronJob, error) {
	var jobs []CronJob

	// List user crontabs
	cronDirs := []string{
		"/var/spool/cron/crontabs",
		"/var/spool/cron",
	}

	// List system crontabs
	systemCronFiles := []string{
		"/etc/crontab",
		"/etc/cron.d/",
	}

	// Also check for cron.{daily,weekly,monthly,hourly} directories
	cronFreqDirs := []string{
		"/etc/cron.daily",
		"/etc/cron.weekly",
		"/etc/cron.monthly",
		"/etc/cron.hourly",
	}

	for _, dir := range cronFreqDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				jobs = append(jobs, CronJob{
					Schedule:   "periodic",
					Command:    filepath.Join(dir, entry.Name()),
					User:       "root",
					SourceFile: filepath.Join(dir, entry.Name()),
					Enabled:    true,
				})
			}
		}
	}

	// Read system crontab
	for _, cronPath := range systemCronFiles {
		if info, err := os.Stat(cronPath); err == nil {
			if info.IsDir() {
				// Read all files in cron.d
				entries, _ := os.ReadDir(cronPath)
				for _, entry := range entries {
					fileJobs := parseCronFile(filepath.Join(cronPath, entry.Name()), "root")
					jobs = append(jobs, fileJobs...)
				}
			} else {
				// Read /etc/crontab
				fileJobs := parseCronFile(cronPath, "root")
				jobs = append(jobs, fileJobs...)
			}
		}
	}

	// Read user crontabs
	for _, dir := range cronDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				userJobs := parseCronFile(filepath.Join(dir, entry.Name()), entry.Name())
				jobs = append(jobs, userJobs...)
			}
		}
	}

	return jobs, nil
}

// parseCronFile parses a crontab file.
func parseCronFile(filePath, user string) []CronJob {
	var jobs []CronJob

	file, err := os.Open(filePath)
	if err != nil {
		return jobs
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse cron line: schedule user command (system crontab)
		// or schedule command (user crontab)
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		var schedule, command string
		if len(fields) >= 6 && looksLikeCronSchedule(fields[0:5]) {
			// System crontab format: schedule user command
			schedule = strings.Join(fields[0:5], " ")
			command = strings.Join(fields[6:], " ")
		} else if len(fields) >= 5 && looksLikeCronSchedule(fields[0:5]) {
			// User crontab format: schedule command
			schedule = strings.Join(fields[0:5], " ")
			command = strings.Join(fields[5:], " ")
		} else {
			continue
		}

		// Handle @hourly, @daily, etc.
		if strings.HasPrefix(fields[0], "@") {
			schedule = fields[0]
			if len(fields) > 1 {
				command = strings.Join(fields[1:], " ")
			}
		}

		if command != "" {
			jobs = append(jobs, CronJob{
				Schedule:   schedule,
				Command:    command,
				User:       user,
				SourceFile: filePath,
				Enabled:    true,
			})
		}
	}

	return jobs
}

// looksLikeCronSchedule checks if fields look like a cron schedule.
func looksLikeCronSchedule(fields []string) bool {
	if len(fields) != 5 {
		return false
	}

	// Check if first 5 fields look like cron schedule
	// minute hour day month weekday
	for _, f := range fields {
		if f == "*" || f == "?" {
			continue
		}
		// Check for numeric ranges
		if strings.ContainsAny(f, "0123456789,-*/") {
			continue
		}
		return false
	}

	return true
}

// Systemd-specific functions

func listSystemdServices(state, namePattern string, limit int) ([]ServiceInfo, error) {
	services, err := runSystemdCmd("list-units", "--type=service", "--all", "--no-pager")
	if err != nil {
		return nil, err
	}

	return parseSystemdList(services, state, namePattern, limit)
}

func getSystemdServiceStatus(name string) (*ServiceStatusDetail, error) {
	output, err := runSystemdCmd("show", name, "--no-pager")
	if err != nil {
		return nil, err
	}

	return parseSystemdStatus(name, output)
}

func runSystemdCmd(args ...string) (string, error) {
	// This is a simplified implementation
	// In production, use exec.Command or dbus interface
	return "", fmt.Errorf("systemd command execution not implemented")
}

func parseSystemdList(output, state, namePattern string, limit int) ([]ServiceInfo, error) {
	// Parse systemctl list-units output
	// Format: UNIT LOAD ACTIVE SUB DESCRIPTION
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

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		name := strings.TrimSpace(fields[0])
		activeState := fields[2]
		subState := fields[3]

		// Apply name pattern filter
		if namePattern != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(namePattern)) {
			continue
		}

		// Map systemd states to our status
		status := "unknown"
		if activeState == "active" {
			status = "running"
		} else if activeState == "inactive" {
			status = "stopped"
		}

		// Apply state filter
		if state != "" && status != state {
			continue
		}

		result = append(result, ServiceInfo{
			Name:        name,
			DisplayName: name, // systemd doesn't separate display name
			Status:      status,
			StartType:   "unknown", // Would need additional query
		})
	}

	return result, nil
}

func parseSystemdStatus(name, output string) (*ServiceStatusDetail, error) {
	// Parse systemctl show output
	lines := strings.Split(output, "\n")

	detail := &ServiceStatusDetail{
		Name:        name,
		DisplayName: name,
		Status:      "unknown",
		StartType:   "unknown",
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "Loaded:") {
			// Parse: Loaded: loaded (/lib/systemd/system/ssh.service; enabled; vendor preset: enabled)
			if strings.Contains(line, "enabled") {
				detail.StartType = "auto"
			} else if strings.Contains(line, "disabled") {
				detail.StartType = "manual"
			}
			// Extract description
			if idx := strings.Index(line, ";"); idx > 0 {
				parts := strings.Split(line[idx+1:], "-")
				if len(parts) > 1 {
					detail.Description = strings.TrimSpace(parts[1])
				}
			}
		} else if strings.HasPrefix(line, "Active:") {
			// Parse: Active: active (running) since ...
			if strings.Contains(line, "active (running)") {
				detail.Status = "running"
			} else if strings.Contains(line, "inactive (dead)") {
				detail.Status = "stopped"
			}
		} else if strings.HasPrefix(line, "MainPID=") {
			// Parse: MainPID=1234
			pidStr := strings.TrimPrefix(line, "MainPID=")
			if pidStr != "0" {
				fmt.Sscanf(pidStr, "%d", &detail.ProcessID)
			}
		} else if strings.HasPrefix(line, "ExecStart=") {
			// Parse: ExecStart={ path=/usr/sbin/sshd ... }
			detail.BinaryPath = extractBinaryPath(line)
		}
	}

	return detail, nil
}

func extractBinaryPath(line string) string {
	// Extract path from ExecStart line
	if strings.Contains(line, "path=") {
		start := strings.Index(line, "path=")
		if start >= 0 {
			start += 5
			end := strings.Index(line[start:], " ")
			if end > 0 {
				return line[start : start+end]
			}
			return line[start:]
		}
	}
	return ""
}
