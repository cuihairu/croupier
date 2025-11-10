package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

type Any = any

type Spec struct {
	Version   string                 `json:"version"`
	Events    map[string]Any         `json:"events"`
	Metrics   map[string]Any         `json:"metrics"`
	GameTypes map[string]Any         `json:"game_types"`
	Taxonomy  map[string]Any         `json:"taxonomy"`
	Derived   map[string]Any         `json:"derived,omitempty"`
}

func readYAML(path string) (map[string]Any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out map[string]Any
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func ensureDir(p string) error {
	return os.MkdirAll(p, 0o755)
}

func main() {
	var (
		dir = flag.String("configs", "configs/analytics", "path to analytics configs dir")
		out = flag.String("out", "web/public/analytics-spec.json", "output JSON file")
	)
	flag.Parse()

	events, err := readYAML(filepath.Join(*dir, "events.yaml"))
	if err != nil { panic(fmt.Errorf("read events: %w", err)) }
	metrics, err := readYAML(filepath.Join(*dir, "metrics.yaml"))
	if err != nil { panic(fmt.Errorf("read metrics: %w", err)) }
	gameTypes, err := readYAML(filepath.Join(*dir, "game_types.yaml"))
	if err != nil { panic(fmt.Errorf("read game_types: %w", err)) }
	taxonomy, _ := readYAML(filepath.Join(*dir, "taxonomy.yaml"))

	spec := Spec{
		Version:   "1",
		Events:    events,
		Metrics:   metrics,
		GameTypes: gameTypes,
		Taxonomy:  taxonomy,
		Derived:   map[string]Any{},
	}

	// derive quick lookup maps if possible
	if evs, ok := events["events"].([]any); ok {
		m := map[string]Any{}
		for _, e := range evs {
			if em, ok := e.(map[string]Any); ok {
				if id, ok := em["id"].(string); ok {
					m[id] = em
				}
			}
		}
		spec.Derived["events_by_id"] = m
	}
	if mets, ok := metrics["metrics"].([]any); ok {
		m := map[string]Any{}
		for _, me := range mets {
			if mm, ok := me.(map[string]Any); ok {
				if id, ok := mm["id"].(string); ok {
					m[id] = mm
				}
			}
		}
		spec.Derived["metrics_by_id"] = m
	}
	if gts, ok := gameTypes["game_types"].([]any); ok {
		m := map[string]Any{}
		for _, gt := range gts {
			if gm, ok := gt.(map[string]Any); ok {
				if id, ok := gm["id"].(string); ok {
					m[id] = gm
				}
			}
		}
		spec.Derived["game_types_by_id"] = m
	}

	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil { panic(err) }

	if err := ensureDir(filepath.Dir(*out)); err != nil { panic(err) }
	if err := ioutil.WriteFile(*out, b, 0o644); err != nil { panic(err) }
	fmt.Printf("wrote %s (size=%d)\n", *out, len(b))
}
