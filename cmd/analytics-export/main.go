package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

type Any = any

type Spec struct {
	Version   string         `json:"version"`
	Events    map[string]Any `json:"events"`
	Metrics   map[string]Any `json:"metrics"`
	GameTypes map[string]Any `json:"gameTypes"`
	Taxonomy  map[string]Any `json:"taxonomy"`
	Derived   map[string]Any `json:"derived,omitempty"`
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

// indexById 从 yaml 顶层 section（形如 `events:` 列表）提取 id→条目 索引；
// section 缺失或形态不符返回 nil，调用方按原语义跳过派生键。
func indexById(root map[string]Any, section string) map[string]Any {
	items, ok := root[section].([]any)
	if !ok {
		return nil
	}
	m := map[string]Any{}
	for _, it := range items {
		if im, ok := it.(map[string]Any); ok {
			if id, ok := im["id"].(string); ok {
				m[id] = im
			}
		}
	}
	return m
}

// runExport 读取 dir 下三份必需 yaml（taxonomy 可缺省），派生 id 索引后
// 序列化为 JSON 写入 out。错误统一返回给 main 的 panic 语义。
func runExport(dir, out string) error {
	events, err := readYAML(filepath.Join(dir, "events.yaml"))
	if err != nil {
		return fmt.Errorf("read events: %w", err)
	}
	metrics, err := readYAML(filepath.Join(dir, "metrics.yaml"))
	if err != nil {
		return fmt.Errorf("read metrics: %w", err)
	}
	gameTypes, err := readYAML(filepath.Join(dir, "game_types.yaml"))
	if err != nil {
		return fmt.Errorf("read game_types: %w", err)
	}
	taxonomy, _ := readYAML(filepath.Join(dir, "taxonomy.yaml"))

	derived := map[string]Any{}
	if m := indexById(events, "events"); m != nil {
		derived["eventsById"] = m
	}
	if m := indexById(metrics, "metrics"); m != nil {
		derived["metricsById"] = m
	}
	if m := indexById(gameTypes, "game_types"); m != nil {
		derived["gameTypesById"] = m
	}

	spec := Spec{
		Version:   "1",
		Events:    events,
		Metrics:   metrics,
		GameTypes: gameTypes,
		Taxonomy:  taxonomy,
		Derived:   derived,
	}

	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}

	if err := ensureDir(filepath.Dir(out)); err != nil {
		return err
	}
	if err := os.WriteFile(out, b, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (size=%d)\n", out, len(b))
	return nil
}

func main() {
	var (
		dir = flag.String("configs", "configs/analytics", "path to analytics configs dir")
		out = flag.String("out", "dashboard/public/analytics-spec.json", "output JSON file")
	)
	flag.Parse()

	if err := runExport(*dir, *out); err != nil {
		panic(err)
	}
}
