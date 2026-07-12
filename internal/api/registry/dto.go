package registry

// RegistryRequest registry request
type RegistryRequest struct{}

// RegistryResponse registry response
type RegistryResponse struct {
	Agents      []RegistryAgent     `json:"agents"`
	Functions   []RegistryFunction  `json:"functions"`
	Assignments map[string][]string `json:"assignments"`
	Coverage    []RegistryCoverage  `json:"coverage"`
}

// RegistryAgent registry agent info
type RegistryAgent struct {
	AgentID      string `json:"agentId"`
	GameID       string `json:"gameId"`
	Env          string `json:"env"`
	Addr         string `json:"addr"`
	Functions    int    `json:"functions"`
	Healthy      bool   `json:"healthy"`
	ExpiresInSec int    `json:"expiresInSec"`
}

// RegistryFunction registry function info
type RegistryFunction struct {
	GameID string   `json:"gameId"`
	ID     string   `json:"id"`
	Agents []string `json:"agents"`
}

// RegistryCoverage registry coverage info
type RegistryCoverage struct {
	GameEnv   string                          `json:"gameEnv"`
	Functions map[string]RegistryCoverageStat `json:"functions"`
	Uncovered []string                        `json:"uncovered"`
}

// RegistryCoverageStat registry coverage statistics
type RegistryCoverageStat struct {
	Total   int `json:"total"`
	Healthy int `json:"healthy"`
}
