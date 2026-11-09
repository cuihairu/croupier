package agent

import (
	"context"
)

// ListServicesRequest requests a list of system services.
type ListServicesRequest struct {
	State       string `json:"state"`
	NamePattern string `json:"namePattern"`
	Limit       int32  `json:"limit"`
}

// ListServicesResponse contains the list of system services.
type ListServicesResponse struct {
	Services []*ServiceInfo `json:"services"`
	Total    int32          `json:"total"`
}

// GetServiceStatusRequest requests detailed status of a specific service.
type GetServiceStatusRequest struct {
	Name string `json:"name"`
}

// GetServiceStatusResponse contains detailed service status.
type GetServiceStatusResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
	StartType   string `json:"startType"`
	ProcessID   uint32 `json:"processId"`
	BinaryPath  string `json:"binaryPath"`
	Description string `json:"description"`
}

// ListCronJobsResponse contains the list of cron jobs.
type ListCronJobsResponse struct {
	Jobs  []*CronJob `json:"jobs"`
	Total int32      `json:"total"`
}

// ListServices returns system services.
func (s *OpsServer) ListServices(ctx context.Context, req *ListServicesRequest) (*ListServicesResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	services, err := ListServices(req.State, req.NamePattern, limit)
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	servicePtrs := make([]*ServiceInfo, len(services))
	for i := range services {
		servicePtrs[i] = &services[i]
	}

	return &ListServicesResponse{
		Services: servicePtrs,
		Total:    int32(len(services)),
	}, nil
}

// GetServiceStatus returns detailed service status.
func (s *OpsServer) GetServiceStatus(ctx context.Context, req *GetServiceStatusRequest) (*GetServiceStatusResponse, error) {
	status, err := GetServiceStatus(req.Name)
	if err != nil {
		return nil, err
	}

	return &GetServiceStatusResponse{
		Name:        status.Name,
		DisplayName: status.DisplayName,
		Status:      status.Status,
		StartType:   status.StartType,
		ProcessID:   status.ProcessID,
		BinaryPath:  status.BinaryPath,
		Description: status.Description,
	}, nil
}

// ListCronJobs returns cron jobs on Linux systems.
func (s *OpsServer) ListCronJobs(ctx context.Context) (*ListCronJobsResponse, error) {
	// 覆盖边界说明：同 ListCronJobsJSON——linux 平台 ListCronJobs 恒返回
	// nil error，err 分支仅 windows/stub 平台可达。
	jobs, err := ListCronJobs()
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	jobPtrs := make([]*CronJob, len(jobs))
	for i := range jobs {
		jobPtrs[i] = &jobs[i]
	}

	return &ListCronJobsResponse{
		Jobs:  jobPtrs,
		Total: int32(len(jobs)),
	}, nil
}
