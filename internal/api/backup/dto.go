package backup

// Backup represents a backup
type Backup struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	// ErrorMessage carries the failure reason for failed rows (AUD-082: the
	// executor's failure text and the orphan-pending convergence reason land
	// here, so a failed backup is explainable without reading the database).
	ErrorMessage string `json:"errorMessage,omitempty"`
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
	// IdempotencyKey is the client idempotency key (AUD-053): when a retry or
	// double-click carries the same key, a pending/succeeded backup with that
	// key is reused instead of stacking duplicate backups; after a failure
	// (failed), retrying with the same key creates a new backup. The handler
	// side also accepts the Idempotency-Key header.
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
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
