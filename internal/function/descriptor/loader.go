package descriptor

import (
    "encoding/json"
    "io/fs"
    "os"
    "path/filepath"
)

// Descriptor is a simplified function descriptor model for UI/validation.
type Descriptor struct {
    ID      string `json:"id"`
    Version string `json:"version"`
    Category string `json:"category"`
    Risk    string `json:"risk"`
    Auth    map[string]any `json:"auth"`
    Params  map[string]any `json:"params"`
    Semantics map[string]any `json:"semantics"`
    Transport map[string]any `json:"transport"`
    Outputs   map[string]any `json:"outputs"`
    UI        map[string]any `json:"ui"`
}

func LoadAll(dir string) ([]*Descriptor, error) {
    var out []*Descriptor
    err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        if d.IsDir() { return nil }
        if filepath.Ext(path) != ".json" { return nil }
        // Skip UI schemas and other non-descriptor JSON files
        // We only keep files that (a) are not under a "ui" subdirectory, and (b) contain a non-empty string field "id".
        if base := filepath.Base(filepath.Dir(path)); base == "ui" {
            return nil
        }
        b, err := os.ReadFile(path)
        if err != nil { return err }
        // quick preflight: ensure there is an "id" field present and non-empty
        var probe map[string]any
        if err := json.Unmarshal(b, &probe); err != nil { return err }
        if v, ok := probe["id"]; !ok || v == nil {
            return nil
        } else if s, ok := v.(string); !ok || s == "" {
            return nil
        }
        var desc Descriptor
        if err := json.Unmarshal(b, &desc); err != nil { return err }
        // defensive: must have ID
        if desc.ID == "" { return nil }
        out = append(out, &desc)
        return nil
    })
    return out, err
}
