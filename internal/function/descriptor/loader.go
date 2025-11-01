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
}

func LoadAll(dir string) ([]*Descriptor, error) {
    var out []*Descriptor
    err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        if d.IsDir() { return nil }
        if filepath.Ext(path) != ".json" { return nil }
        b, err := os.ReadFile(path)
        if err != nil { return err }
        var desc Descriptor
        if err := json.Unmarshal(b, &desc); err != nil { return err }
        out = append(out, &desc)
        return nil
    })
    return out, err
}
