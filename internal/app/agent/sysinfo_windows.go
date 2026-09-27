//go:build windows

package agent

import (
	"fmt"
	"os/exec"
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

// schtasksQuery 是 Windows 计划任务采集的包级接缝：生产为真实 exec 调用，
// windows 测试可注入固定输出驱动解析/过滤分支（与 listCronJobs 接缝同法）。
var schtasksQuery = runSchtasksQuery

func runSchtasksQuery() ([]byte, error) {
	out, err := exec.Command("schtasks", "/query", "/fo", "csv", "/v").Output()
	if err != nil {
		return nil, fmt.Errorf("schtasks 执行失败: %w", err)
	}
	return out, nil
}

// listCronJobsPlatform 采集 Windows 计划任务（schtasks /query /fo csv /v）。
// #24：此前 Windows 恒返回 not available，宿主机任务只覆盖 linux crontab；
// 现经 ParseSchtasksCSV 解析后按来源字段映射 CronJob（TaskName→SourceFile、
// Task To Run→Command、Run As User→User、Scheduled Task State→Enabled）。
// \Microsoft\ 内置任务（数百条、非 GM 关注点）在采集端剔除。
// 已知边界：非英文 locale 下 schtasks 列名为译文且输出为 OEM 代码页，
// 结构解析走固定下标回退仍正确，但中文内容可能显示乱码（不引入编码转换依赖）。
func listCronJobsPlatform() ([]CronJob, error) {
	out, err := schtasksQuery()
	if err != nil {
		return nil, err
	}
	return FilterSchtasksSystemJobs(ParseSchtasksCSV(out)), nil
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
