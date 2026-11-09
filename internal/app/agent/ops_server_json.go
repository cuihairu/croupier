package agent

import (
	"context"
	"encoding/json"
)

// ListServicesJSON handles ListServicesRequest via JSON
func (s *OpsServer) ListServicesJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	var req ListServicesRequest
	if len(jsonReq) > 0 {
		if err := json.Unmarshal(jsonReq, &req); err != nil {
			return nil, err
		}
	}

	services, err := ListServices(req.State, req.NamePattern, int(req.Limit))
	if err != nil {
		return nil, err
	}

	// Return JSON response
	resp := struct {
		Services []*ServiceInfo `json:"services"`
		Total    int32          `json:"total"`
	}{
		Services: make([]*ServiceInfo, len(services)),
		Total:    int32(len(services)),
	}
	for i := range services {
		resp.Services[i] = &services[i]
	}

	return json.Marshal(resp)
}

// GetServiceStatusJSON handles GetServiceStatusRequest via JSON
func (s *OpsServer) GetServiceStatusJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	var req GetServiceStatusRequest
	if err := json.Unmarshal(jsonReq, &req); err != nil {
		return nil, err
	}

	status, err := GetServiceStatus(req.Name)
	if err != nil {
		return nil, err
	}

	// Return JSON response
	resp := &ServiceStatusDetail{
		Name:        status.Name,
		DisplayName: status.DisplayName,
		Status:      status.Status,
		StartType:   status.StartType,
		ProcessID:   status.ProcessID,
		BinaryPath:  status.BinaryPath,
		Description: status.Description,
	}

	return json.Marshal(resp)
}

// ListCronJobsJSON handles ListCronJobsRequest via JSON
func (s *OpsServer) ListCronJobsJSON(ctx context.Context) ([]byte, error) {
	// 覆盖边界说明：linux 的 listCronJobsPlatform 恒返回 nil error（所有
	// 目录读取失败均静默跳过），此 err 分支仅 windows/stub 平台可达，
	// linux 测试构建不可覆盖。
	jobs, err := ListCronJobs()
	if err != nil {
		return nil, err
	}

	// Return JSON response
	resp := struct {
		Jobs  []*CronJob `json:"jobs"`
		Total int32      `json:"total"`
	}{
		Jobs:  make([]*CronJob, len(jobs)),
		Total: int32(len(jobs)),
	}
	for i := range jobs {
		resp.Jobs[i] = &jobs[i]
	}

	return json.Marshal(resp)
}

// ========== System Services ==========
