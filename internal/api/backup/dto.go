package backup

// Backup represents a backup
type Backup struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// BackupsListRequest represents the request to list backups
type BackupsListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Type     string `form:"type"`
}

// BackupsListResponse represents the response with a list of backups
type BackupsListResponse struct {
	Items []Backup `json:"items"`
	Total int64    `json:"total"`
	Page  int      `json:"page"`
	Size  int      `json:"pageSize"`
}

// BackupCreateRequest represents the request to create a backup
type BackupCreateRequest struct {
	Name string `json:"name"`
	Type string `json:"type"` // full, incremental
}

// BackupCreateResponse represents the response after creating a backup
type BackupCreateResponse struct {
	Backup
}

// BackupDeleteRequest represents the request to delete a backup
type BackupDeleteRequest struct {
	ID string `uri:"id"`
}

// BackupDownloadRequest represents the request to download a backup
type BackupDownloadRequest struct {
	ID string `uri:"id"`
}

// BackupDetailResponse represents the response with backup details
type BackupDetailResponse struct {
	Backup
}

// DownloadPayload represents the download payload for a backup file
type DownloadPayload struct {
	Filename    string
	Size        int64
	Reader      interface{}
	RedirectURL string
}
