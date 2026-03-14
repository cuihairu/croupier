package migrate

// Migration related types

type MigrationResult struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	AppliedBy string `json:"appliedBy"`
}
