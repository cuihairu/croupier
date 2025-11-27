package types

// MigrateUpRequest represents the request payload for triggering an up migration.
type MigrateUpRequest struct {
	Step   int  `json:"step,optional"`
	DryRun bool `json:"dryRun,optional"`
}

// MigrationResult captures the outcome of a single migration execution.
type MigrationResult struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
	Duration string `json:"duration,omitempty"`
}

// MigrateUpResponse returns a high level summary for the migration run.
type MigrateUpResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Results []MigrationResult `json:"results,omitempty"`
}
