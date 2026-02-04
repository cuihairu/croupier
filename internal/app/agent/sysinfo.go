// Package agent provides system information collection capabilities.
package agent

import (
	"os"
	"runtime"
)

// ServiceInfo contains information about a system service.
type ServiceInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`     // running, stopped, paused
	StartType   string `json:"start_type"` // auto, manual, disabled
	ProcessID   uint32 `json:"process_id,omitempty"`
}

// ServiceStatusDetail contains detailed service status.
type ServiceStatusDetail struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	StartType   string `json:"start_type"`
	ProcessID   uint32 `json:"process_id,omitempty"`
	BinaryPath  string `json:"binary_path,omitempty"`
	Description string `json:"description,omitempty"`
}

// CronJob represents a cron job entry.
type CronJob struct {
	Schedule   string `json:"schedule"`    // cron expression
	Command    string `json:"command"`     // command to execute
	User       string `json:"user"`        // user who runs the job
	SourceFile string `json:"source_file"` // file where this job is defined
	Enabled    bool   `json:"enabled"`     // whether the job is active
}

// ListServices returns system services based on the platform.
func ListServices(state, namePattern string, limit int) ([]ServiceInfo, error) {
	return listServicesPlatform(state, namePattern, limit)
}

// GetServiceStatus returns detailed status of a specific service.
func GetServiceStatus(name string) (*ServiceStatusDetail, error) {
	return getServiceStatusPlatform(name)
}

// ListCronJobs returns cron jobs on Linux systems.
func ListCronJobs() ([]CronJob, error) {
	return listCronJobsPlatform()
}

// GetPlatformInfo returns platform-specific system information.
func GetPlatformInfo() map[string]interface{} {
	info := make(map[string]interface{})
	info["os"] = runtime.GOOS
	info["arch"] = runtime.GOARCH

	// Add platform-specific info
	if runtime.GOOS == "windows" {
		info["service_manager"] = "Windows Service Manager (SCM)"
	} else if runtime.GOOS == "linux" {
		info["service_manager"] = detectLinuxServiceManager()
	}

	return info
}

// detectLinuxServiceManager detects which service manager is in use.
func detectLinuxServiceManager() string {
	// Simple heuristic: check for systemd directory
	if _, err := os.Stat("/run/systemd"); err == nil {
		return "systemd"
	}
	// Default to unknown for other init systems
	return "unknown"
}
