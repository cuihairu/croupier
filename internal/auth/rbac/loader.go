package rbac

import (
    "encoding/json"
    "os"
)

// LoadPolicy reads a simple JSON policy file: {"allow": {"user": ["permission", "*"]}}
func LoadPolicy(path string) (*Policy, error) {
    b, err := os.ReadFile(path)
    if err != nil { return nil, err }
    var raw struct{
        Allow map[string][]string `json:"allow"`
    }
    if err := json.Unmarshal(b, &raw); err != nil { return nil, err }
    p := NewPolicy()
    for user, perms := range raw.Allow {
        for _, perm := range perms { p.Grant(user, perm) }
    }
    return p, nil
}

