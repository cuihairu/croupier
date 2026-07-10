//go:build linux

package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

// systemdCmdTimeout caps how long a single systemctl invocation may take.
// systemctl normally returns quickly, but a wedged unit or D-Bus hang should
// not block the agent's sysinfo collection indefinitely.
const systemdCmdTimeout = 5 * time.Second

func init() {
	// On Linux we wire the systemd runner to a real exec.CommandContext call.
	// Other platforms keep the default stub declared in systemd_parse.go.
	systemdRunner = func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), systemdCmdTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "systemctl", args...)
		return cmd.CombinedOutput()
	}
}
