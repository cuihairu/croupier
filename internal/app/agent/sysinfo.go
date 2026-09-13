// Package agent provides system information collection capabilities.
package agent

import (
	"os"
	"runtime"
)

// ServiceInfo contains information about a system service.
type ServiceInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`    // running, stopped, paused
	StartType   string `json:"startType"` // auto, manual, disabled
	ProcessID   uint32 `json:"processId,omitempty"`
}

// ServiceStatusDetail contains detailed service status.
type ServiceStatusDetail struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
	StartType   string `json:"startType"`
	ProcessID   uint32 `json:"processId,omitempty"`
	BinaryPath  string `json:"binaryPath,omitempty"`
	Description string `json:"description,omitempty"`
}

// CronJob represents a cron job entry.
type CronJob struct {
	Schedule   string `json:"schedule"`   // cron expression
	Command    string `json:"command"`    // command to execute
	User       string `json:"user"`       // user who runs the job
	SourceFile string `json:"sourceFile"` // file where this job is defined
	Enabled    bool   `json:"enabled"`    // whether the job is active
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
	// 覆盖边界说明：windows 分支由 runtime.GOOS 编译期常量决定，linux 测试
	// 构建下不可达（windows 构建由 sysinfo_windows_test.go 覆盖其余路径）。
	if runtime.GOOS == "windows" {
		info["service_manager"] = "Windows Service Manager (SCM)"
	} else if runtime.GOOS == "linux" {
		info["service_manager"] = detectLinuxServiceManager()
	}

	return info
}

// systemdDetectPath 是 detectLinuxServiceManager 探测的 systemd 运行目录，
// 提为包级变量以便测试注入不存在的路径覆盖非 systemd 分支。
var systemdDetectPath = "/run/systemd"

// detectLinuxServiceManager detects which service manager is in use.
func detectLinuxServiceManager() string {
	// Simple heuristic: check for systemd directory
	if _, err := os.Stat(systemdDetectPath); err == nil {
		return "systemd"
	}
	// Default to unknown for other init systems
	return "unknown"
}
