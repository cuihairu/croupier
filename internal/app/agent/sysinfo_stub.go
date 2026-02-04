//go:build !windows && !linux

package agent

import (
	"fmt"
)

// listServicesPlatform is a stub for unsupported platforms.
func listServicesPlatform(state, namePattern string, limit int) ([]ServiceInfo, error) {
	return nil, fmt.Errorf("service listing is not supported on this platform")
}

// getServiceStatusPlatform is a stub for unsupported platforms.
func getServiceStatusPlatform(name string) (*ServiceStatusDetail, error) {
	return nil, fmt.Errorf("service status query is not supported on this platform")
}

// listCronJobsPlatform is a stub for unsupported platforms.
func listCronJobsPlatform() ([]CronJob, error) {
	return nil, fmt.Errorf("cron jobs are not available on this platform")
}
