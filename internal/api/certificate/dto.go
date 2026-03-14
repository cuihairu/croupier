package certificate

// Certificate represents a certificate entity
type Certificate struct {
	ID            uint   `json:"id"`
	Domain        string `json:"domain"`
	Issuer        string `json:"issuer"`
	ExpiresAt     string `json:"expiresAt"`
	Status        string `json:"status"`
	LastCheckedAt string `json:"lastCheckedAt"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
	CreatedAt     string `json:"createdAt"`
}

// CertificateItem represents a certificate in the list
type CertificateItem struct {
	ID            uint   `json:"id"`
	Domain        string `json:"domain"`
	Issuer        string `json:"issuer"`
	ExpiresAt     string `json:"expiresAt"`
	Status        string `json:"status"`
	LastCheckedAt string `json:"lastCheckedAt"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
	CreatedAt     string `json:"createdAt"`
}

// CertificateAddRequest is the request to add a certificate
type CertificateAddRequest struct {
	Domain      string `json:"domain"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"privateKey"`
}

// CertificateAddResponse is the response from adding a certificate
type CertificateAddResponse struct {
	Certificate CertificateItem `json:"certificate"`
}

// CertificateAlertAddRequest is the request to add a certificate alert
type CertificateAlertAddRequest struct {
	Domain    string `json:"domain"`
	Threshold int    `json:"threshold"`
}

// CertificateAlertAddResponse is the response from adding a certificate alert
type CertificateAlertAddResponse struct {
	ID            uint   `json:"id"`
	Domain        string `json:"domain"`
	ThresholdDays int    `json:"thresholdDays"`
}

// CertificateAlertsListRequest is the request to list certificate alerts
type CertificateAlertsListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

// CertificateAlertsListResponse is the response with certificate alerts list
type CertificateAlertsListResponse struct {
	Items []CertificateAlertItem `json:"items"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Size  int                    `json:"size"`
}

// CertificateAlertItem represents a certificate alert
type CertificateAlertItem struct {
	ID              uint   `json:"id"`
	Domain          string `json:"domain"`
	ThresholdDays   int    `json:"thresholdDays"`
	Active          bool   `json:"active"`
	LastTriggeredAt string `json:"lastTriggeredAt,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

// CertificateCheckAllRequest is the request to check all certificates
type CertificateCheckAllRequest struct {
}

// CertificateCheckAllResponse is the response from checking all certificates
type CertificateCheckAllResponse struct {
	Checked int `json:"checked"`
	Failed  int `json:"failed"`
	Total   int `json:"total"`
}

// CertificateCheckRequest is the request to check a certificate
type CertificateCheckRequest struct {
	ID string `uri:"id"`
}

// CertificateCheckResponse is the response from checking a certificate
type CertificateCheckResponse struct {
	Certificate CertificateItem `json:"certificate"`
}

// CertificateDeleteRequest is the request to delete a certificate
type CertificateDeleteRequest struct {
	ID string `uri:"id"`
}

// CertificateDeleteResponse is the response from deleting a certificate
type CertificateDeleteResponse struct {
	Message string `json:"message"`
}

// CertificateDetailRequest is the request to get certificate details
type CertificateDetailRequest struct {
	ID string `uri:"id"`
}

// CertificateDetailResponse is the response with certificate details
type CertificateDetailResponse struct {
	Certificate CertificateItem `json:"certificate"`
}

// CertificateDomainInfoRequest is the request to get domain certificate info
type CertificateDomainInfoRequest struct {
	Domain string `form:"domain"`
}

// CertificateDomainInfoResponse is the response with domain certificate info
type CertificateDomainInfoResponse struct {
	Certificate CertificateItem `json:"certificate"`
}

// CertificateExpiringRequest is the request to get expiring certificates
type CertificateExpiringRequest struct {
	Days int `form:"days"`
}

// CertificateExpiringResponse is the response with expiring certificates
type CertificateExpiringResponse struct {
	Items []CertificateItem `json:"items"`
	Days  int               `json:"days"`
}

// CertificateStatsRequest is the request to get certificate statistics
type CertificateStatsRequest struct {
}

// CertificateStatsResponse is the response with certificate statistics
type CertificateStatsResponse struct {
	Total   int64 `json:"total"`
	Valid   int64 `json:"valid"`
	Expiring int64 `json:"expiring"`
	Expired int64 `json:"expired"`
	Invalid int64 `json:"invalid"`
}

// CertificatesListRequest is the request to list certificates
type CertificatesListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Status   string `form:"status"`
}

// CertificatesListResponse is the response with certificates list
type CertificatesListResponse struct {
	Items []CertificateItem `json:"items"`
	Total int64             `json:"total"`
	Page  int               `json:"page"`
	Size  int               `json:"size"`
}

// Type aliases for backward compatibility
type ListRequest = CertificatesListRequest
type ListResponse = CertificatesListResponse
type AddRequest = CertificateAddRequest
type AddResponse = CertificateAddResponse
type GetRequest = CertificateDetailRequest
type GetResponse = CertificateDetailResponse
type DeleteRequest = CertificateDeleteRequest
type CheckRequest = CertificateCheckRequest
type CheckResponse = CertificateCheckResponse
type StatsRequest = CertificateStatsRequest
type StatsResponse = CertificateStatsResponse
type AddAlertRequest = CertificateAlertAddRequest
type AddAlertResponse = CertificateAlertAddResponse
type ListAlertsRequest = CertificateAlertsListRequest
type ListAlertsResponse = CertificateAlertsListResponse
type AlertItem = CertificateAlertItem
type DomainInfoRequest = CertificateDomainInfoRequest
type DomainInfoResponse = CertificateDomainInfoResponse
type ExpiringRequest = CertificateExpiringRequest
type ExpiringResponse = CertificateExpiringResponse
type CheckAllRequest = CertificateCheckAllRequest
type CheckAllResponse = CertificateCheckAllResponse
