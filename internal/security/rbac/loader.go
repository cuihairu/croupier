package rbac

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// LoadPolicy reads a simple JSON policy file: {"allow": {"user": ["permission", "*"]}}
func LoadPolicy(path string) (*Policy, error) {
	log.Printf("[RBAC] Loading legacy policy from: %s", path)
	b, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[RBAC] Failed to read policy file: %v", err)
		return nil, err
	}
	var raw struct {
		Allow map[string][]string `json:"allow"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		log.Printf("[RBAC] Failed to parse policy JSON: %v", err)
		return nil, err
	}
	p := NewPolicy()
	for user, perms := range raw.Allow {
		for _, perm := range perms {
			p.Grant(user, perm)
			log.Printf("[RBAC] Granted: %s -> %s", user, perm)
		}
	}
	log.Printf("[RBAC] Legacy policy loaded successfully with %d users/roles", len(raw.Allow))
	return p, nil
}

// LoadCasbinPolicy loads Casbin policy with model and policy files
func LoadCasbinPolicy(configPath string) (PolicyInterface, error) {
	// Try to find Casbin config files
	dir := filepath.Dir(configPath)
	modelPath := filepath.Join(dir, "rbac_model.conf")
	policyPath := filepath.Join(dir, "rbac_policy.csv")

	// Check if Casbin files exist
	if _, err := os.Stat(modelPath); err == nil {
		if _, err := os.Stat(policyPath); err == nil {
			log.Printf("[RBAC] Found Casbin config files, loading Casbin policy")
			return NewCasbinPolicy(modelPath, policyPath)
		}
	}

	// Fallback to legacy policy
	log.Printf("[RBAC] Casbin files not found, falling back to legacy policy")
	return LoadPolicy(configPath)
}

// LoadPolicyAuto automatically detects and loads the appropriate policy type
func LoadPolicyAuto(configPath string) (PolicyInterface, error) {
	// If the config path is JSON, it's legacy
	if strings.HasSuffix(configPath, ".json") {
		log.Printf("[RBAC] JSON config detected, loading legacy policy")
		return LoadPolicy(configPath)
	}

	// Otherwise try Casbin first
	return LoadCasbinPolicy(configPath)
}
