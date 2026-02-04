//go:build windows

package agent

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// listServicesPlatform lists Windows services.
func listServicesPlatform(state, namePattern string, limit int) ([]ServiceInfo, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	services, err := m.ListServices()
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var result []ServiceInfo
	count := 0

	for _, name := range services {
		if count >= limit {
			break
		}

		// Apply name pattern filter
		if namePattern != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(namePattern)) {
			continue
		}

		// Open service to get details - need to open with query access
		s, err := m.OpenService(name)
		if err != nil {
			// Skip services we can't open (permission denied, etc.)
			continue
		}

		// Query service status
		status, err := s.Query()
		s.Close()
		if err != nil {
			continue
		}

		// Apply state filter
		if state != "" && !matchesState(status.State, state) {
			continue
		}

		// Open again with config access to get display name and config
		s, err = m.OpenService(name)
		if err != nil {
			continue
		}
		cfg, err := s.Config()
		s.Close()
		if err != nil {
			// Use minimal info if config fails
			info := ServiceInfo{
				Name:        name,
				DisplayName: name,
				Status:      stateToString(status.State),
				StartType:   "unknown",
			}
			result = append(result, info)
			count++
			continue
		}

		// Get process ID if running
		var pid uint32
		if status.State == svc.Running && status.ProcessId != 0 {
			pid = status.ProcessId
		}

		info := ServiceInfo{
			Name:        name,
			DisplayName: cfg.DisplayName,
			Status:      stateToString(status.State),
			StartType:   startTypeToString(cfg.StartType),
			ProcessID:   pid,
		}

		result = append(result, info)
		count++
	}

	return result, nil
}

// getServiceStatusPlatform gets detailed Windows service status.
func getServiceStatusPlatform(name string) (*ServiceStatusDetail, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return nil, fmt.Errorf("failed to open service %q: %w", name, err)
	}
	defer s.Close()

	// Query status
	status, err := s.Query()
	if err != nil {
		return nil, fmt.Errorf("failed to query service status: %w", err)
	}

	// Query config
	cfg, err := s.Config()
	if err != nil {
		return nil, fmt.Errorf("failed to query service config: %w", err)
	}

	var pid uint32
	if status.State == svc.Running && status.ProcessId != 0 {
		pid = status.ProcessId
	}

	return &ServiceStatusDetail{
		Name:        name,
		DisplayName: cfg.DisplayName,
		Status:      stateToString(status.State),
		StartType:   startTypeToString(cfg.StartType),
		ProcessID:   pid,
		BinaryPath:  cfg.BinaryPathName,
		Description: cfg.Description,
	}, nil
}

// listCronJobsPlatform is a stub for Windows.
func listCronJobsPlatform() ([]CronJob, error) {
	return nil, fmt.Errorf("cron jobs are not available on Windows")
}

// matchesState checks if the service state matches the filter.
func matchesState(state svc.State, filter string) bool {
	switch strings.ToLower(filter) {
	case "running":
		return state == svc.Running
	case "stopped":
		return state == svc.Stopped
	case "paused":
		return state == svc.Paused
	case "startpending":
		return state == svc.StartPending
	case "stoppending":
		return state == svc.StopPending
	default:
		return true
	}
}

// stateToString converts svc.State to string.
func stateToString(state svc.State) string {
	switch state {
	case svc.Running:
		return "running"
	case svc.Paused:
		return "paused"
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start_pending"
	case svc.StopPending:
		return "stop_pending"
	default:
		return "unknown"
	}
}

// startTypeToString converts service start type to string.
func startTypeToString(startType uint32) string {
	switch startType {
	case mgr.StartAutomatic:
		return "auto"
	case mgr.StartManual:
		return "manual"
	case mgr.StartDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}
