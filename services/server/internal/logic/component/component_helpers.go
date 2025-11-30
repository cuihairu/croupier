package component

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/pack"
	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/svc"
)

type componentStatus string

const (
	componentStatusInstalled componentStatus = "installed"
	componentStatusDisabled  componentStatus = "disabled"
)

type componentDTO struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	Version      string                   `json:"version"`
	Description  string                   `json:"description"`
	Category     string                   `json:"category"`
	Dependencies []string                 `json:"dependencies"`
	Functions    []pack.ComponentFunction `json:"functions"`
	Author       string                   `json:"author"`
	License      string                   `json:"license"`
	Status       string                   `json:"status"`
	Path         string                   `json:"path"`
}

type componentEntry struct {
	Manifest *pack.ComponentManifest
	Status   componentStatus
}

func componentEntryToDTO(cfg config.Config, entry componentEntry) componentDTO {
	manifest := entry.Manifest
	if manifest == nil {
		manifest = &pack.ComponentManifest{}
	}
	dto := componentDTO{
		ID:           strings.TrimSpace(manifest.ID),
		Name:         manifest.Name,
		Version:      manifest.Version,
		Description:  manifest.Description,
		Category:     manifest.Category,
		Dependencies: append([]string(nil), manifest.Dependencies...),
		Functions:    append([]pack.ComponentFunction(nil), manifest.Functions...),
		Author:       manifest.Author,
		License:      manifest.License,
		Status:       string(entry.Status),
	}
	if path := componentDir(cfg, entry); path != "" {
		dto.Path = path
	}
	return dto
}

func componentDir(cfg config.Config, entry componentEntry) string {
	base := strings.TrimSpace(svc.ResolveComponentDataDir(cfg))
	if base == "" || entry.Manifest == nil {
		return ""
	}
	statusDir := "installed"
	if entry.Status == componentStatusDisabled {
		statusDir = "disabled"
	}
	category := strings.TrimSpace(entry.Manifest.Category)
	if category == "" {
		category = "default"
	}
	return filepath.Join(base, "components", statusDir, category, entry.Manifest.ID)
}

func withComponentManagerRead(ctx *svc.ServiceContext, fn func(*pack.ComponentManager) error) error {
	if ctx == nil || ctx.ComponentManager == nil {
		return errors.New("组件管理器未初始化")
	}
	if ctx.ComponentLock != nil {
		ctx.ComponentLock.RLock()
		defer ctx.ComponentLock.RUnlock()
	}
	return fn(ctx.ComponentManager)
}

func withComponentManagerWrite(ctx *svc.ServiceContext, fn func(*pack.ComponentManager) error) error {
	if ctx == nil || ctx.ComponentManager == nil {
		return errors.New("组件管理器未初始化")
	}
	if ctx.ComponentLock != nil {
		ctx.ComponentLock.Lock()
		defer ctx.ComponentLock.Unlock()
	}
	return fn(ctx.ComponentManager)
}

func buildComponentList(cfg config.Config, cm *pack.ComponentManager) ([]componentDTO, int, int, int) {
	total := len(cm.ListInstalled()) + len(cm.ListDisabled())
	items := make([]componentDTO, 0, total)
	installedCount := 0
	disabledCount := 0
	functionsCount := 0

	for _, manifest := range cm.ListInstalled() {
		entry := componentEntry{Manifest: manifest, Status: componentStatusInstalled}
		items = append(items, componentEntryToDTO(cfg, entry))
		installedCount++
		if manifest != nil {
			functionsCount += len(manifest.Functions)
		}
	}
	for _, manifest := range cm.ListDisabled() {
		entry := componentEntry{Manifest: manifest, Status: componentStatusDisabled}
		items = append(items, componentEntryToDTO(cfg, entry))
		disabledCount++
		if manifest != nil {
			functionsCount += len(manifest.Functions)
		}
	}

	statusOrder := map[string]int{
		string(componentStatusInstalled): 0,
		string(componentStatusDisabled):  1,
	}
	sort.Slice(items, func(i, j int) bool {
		left := statusOrder[items[i].Status]
		right := statusOrder[items[j].Status]
		if left == right {
			return items[i].ID < items[j].ID
		}
		return left < right
	})

	return items, installedCount, disabledCount, functionsCount
}

func findComponentEntry(cm *pack.ComponentManager, id string) (*componentEntry, error) {
	componentID := strings.TrimSpace(id)
	if componentID == "" {
		return nil, errors.New("组件ID不能为空")
	}
	if manifest, ok := cm.ListInstalled()[componentID]; ok {
		return &componentEntry{Manifest: manifest, Status: componentStatusInstalled}, nil
	}
	if manifest, ok := cm.ListDisabled()[componentID]; ok {
		return &componentEntry{Manifest: manifest, Status: componentStatusDisabled}, nil
	}
	return nil, fmt.Errorf("组件 %s 不存在", componentID)
}

func locateComponentSource(cfg config.Config, name, version string) (string, error) {
	componentName := strings.TrimSpace(name)
	if componentName == "" {
		return "", errors.New("组件名称不能为空")
	}
	version = strings.TrimSpace(version)

	var candidates []string
	addCandidates := func(base string) {
		if base == "" {
			return
		}
		paths := []string{componentName}
		if version != "" {
			paths = append(paths,
				fmt.Sprintf("%s-%s", componentName, version),
				fmt.Sprintf("%s_%s", componentName, version),
				filepath.Join(componentName, version),
			)
		}
		for _, p := range paths {
			candidates = append(candidates, filepath.Join(base, p))
		}
	}

	addCandidates(svc.ResolveComponentStagingDir(cfg))

	if filepath.IsAbs(componentName) || strings.Contains(componentName, "/") || strings.Contains(componentName, "\\") {
		candidates = append(candidates, componentName)
	}

	repoCandidates := []string{
		filepath.Join("components", componentName),
	}
	if version != "" {
		repoCandidates = append(repoCandidates,
			filepath.Join("components", fmt.Sprintf("%s-%s", componentName, version)),
			filepath.Join("components", componentName, version),
		)
	}
	candidates = append(candidates, repoCandidates...)

	seen := make(map[string]struct{})
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		path := cand
		if !filepath.IsAbs(path) {
			if abs, err := filepath.Abs(path); err == nil {
				path = abs
			}
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(path, "manifest.json")); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("未找到组件 %s 的安装源", componentName)
}

func readComponentManifest(dir string) (*pack.ComponentManifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var manifest pack.ComponentManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func normalizePatchMap(in interface{}) (map[string]interface{}, error) {
	if in == nil {
		return map[string]interface{}{}, nil
	}
	if m, ok := in.(map[string]interface{}); ok {
		return m, nil
	}
	bytes, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(bytes, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func applyComponentPatch(manifest *pack.ComponentManifest, patch map[string]interface{}) (bool, error) {
	if manifest == nil {
		return false, errors.New("组件定义为空")
	}
	oldCategory := strings.TrimSpace(manifest.Category)
	for key, value := range patch {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "id":
			return false, errors.New("不支持修改组件ID")
		case "name":
			if str, err := toString(value); err == nil {
				manifest.Name = str
			} else {
				return false, fmt.Errorf("name 无效: %w", err)
			}
		case "description":
			if str, err := toString(value); err == nil {
				manifest.Description = str
			} else {
				return false, fmt.Errorf("description 无效: %w", err)
			}
		case "version":
			if str, err := toString(value); err == nil {
				manifest.Version = str
			} else {
				return false, fmt.Errorf("version 无效: %w", err)
			}
		case "category":
			if str, err := toString(value); err == nil {
				manifest.Category = str
			} else {
				return false, fmt.Errorf("category 无效: %w", err)
			}
		case "dependencies":
			deps, err := toStringSlice(value)
			if err != nil {
				return false, fmt.Errorf("dependencies 无效: %w", err)
			}
			manifest.Dependencies = deps
		case "functions":
			funcs, err := toComponentFunctions(value)
			if err != nil {
				return false, fmt.Errorf("functions 无效: %w", err)
			}
			manifest.Functions = funcs
		case "author":
			if str, err := toString(value); err == nil {
				manifest.Author = str
			} else {
				return false, fmt.Errorf("author 无效: %w", err)
			}
		case "license":
			if str, err := toString(value); err == nil {
				manifest.License = str
			} else {
				return false, fmt.Errorf("license 无效: %w", err)
			}
		default:
			// ignore unknown keys
		}
	}
	newCategory := strings.TrimSpace(manifest.Category)
	return oldCategory != newCategory, nil
}

func toString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v), nil
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		var out string
		if err := json.Unmarshal(bytes, &out); err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}
}

func toStringSlice(value interface{}) ([]string, error) {
	switch v := value.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out, nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			str, err := toString(item)
			if err != nil {
				return nil, err
			}
			if str != "" {
				out = append(out, str)
			}
		}
		return out, nil
	default:
		if str, err := toString(value); err == nil && str != "" {
			return []string{str}, nil
		}
		return []string{}, nil
	}
}

func toComponentFunctions(value interface{}) ([]pack.ComponentFunction, error) {
	if value == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var functions []pack.ComponentFunction
	if err := json.Unmarshal(bytes, &functions); err != nil {
		return nil, err
	}
	return functions, nil
}

func writeComponentManifest(path string, manifest *pack.ComponentManifest) error {
	if manifest == nil {
		return errors.New("manifest 为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func moveComponentCategory(cfg config.Config, entry componentEntry, oldCategory string) error {
	if entry.Manifest == nil {
		return errors.New("组件定义为空")
	}
	oldCat := strings.TrimSpace(oldCategory)
	if oldCat == "" {
		oldCat = "default"
	}
	newCat := strings.TrimSpace(entry.Manifest.Category)
	if newCat == "" {
		newCat = "default"
	}
	if oldCat == newCat {
		return nil
	}
	base := strings.TrimSpace(svc.ResolveComponentDataDir(cfg))
	if base == "" {
		return fmt.Errorf("组件目录未配置")
	}
	statusDir := "installed"
	if entry.Status == componentStatusDisabled {
		statusDir = "disabled"
	}
	oldPath := filepath.Join(base, "components", statusDir, oldCat, entry.Manifest.ID)
	newPath := filepath.Join(base, "components", statusDir, newCat, entry.Manifest.ID)
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(oldPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Rename(oldPath, newPath)
}

func removeDisabledComponent(cm *pack.ComponentManager, cfg config.Config, entry componentEntry) error {
	if entry.Manifest == nil {
		return fmt.Errorf("组件定义不存在")
	}
	path := componentDir(cfg, entry)
	if path != "" {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	delete(cm.ListDisabled(), entry.Manifest.ID)
	return cm.SaveRegistry()
}
