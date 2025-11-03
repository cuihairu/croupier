package pack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ComponentManager manages function components only
// Backend core functions (auth, users, roles, audit) are not managed here
type ComponentManager struct {
	dataDir      string
	installedDir string
	disabledDir  string
	registry     *ComponentRegistry
}

type ComponentRegistry struct {
	Installed map[string]*ComponentManifest `json:"installed"`
	Disabled  map[string]*ComponentManifest `json:"disabled"`
}

type ComponentManifest struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Version      string               `json:"version"`
	Description  string               `json:"description"`
	Category     string               `json:"category"` // player, item, economy, social, etc.
	Dependencies []string             `json:"dependencies,omitempty"`
	Functions    []ComponentFunction  `json:"functions"`
	Author       string               `json:"author,omitempty"`
	License      string               `json:"license,omitempty"`
}

type ComponentFunction struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description,omitempty"`
}

func NewComponentManager(dataDir string) *ComponentManager {
	return &ComponentManager{
		dataDir:      dataDir,
		installedDir: filepath.Join(dataDir, "components", "installed"),
		disabledDir:  filepath.Join(dataDir, "components", "disabled"),
		registry:     &ComponentRegistry{
			Installed: make(map[string]*ComponentManifest),
			Disabled:  make(map[string]*ComponentManifest),
		},
	}
}

func (cm *ComponentManager) LoadRegistry() error {
	registryPath := filepath.Join(cm.dataDir, "component-registry.json")

	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		return cm.SaveRegistry()
	}

	data, err := os.ReadFile(registryPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, cm.registry)
}

func (cm *ComponentManager) SaveRegistry() error {
	registryPath := filepath.Join(cm.dataDir, "component-registry.json")

	data, err := json.MarshalIndent(cm.registry, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(registryPath, data, 0644)
}

func (cm *ComponentManager) InstallComponent(componentPath string) error {
	manifest, err := cm.loadManifest(componentPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Check dependencies
	if err := cm.checkDependencies(manifest); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Copy to installed directory
	destDir := filepath.Join(cm.installedDir, manifest.Category, manifest.ID)
	if err := cm.copyComponent(componentPath, destDir); err != nil {
		return fmt.Errorf("failed to copy component: %w", err)
	}

	// Update registry
	cm.registry.Installed[manifest.ID] = manifest

	return cm.SaveRegistry()
}

func (cm *ComponentManager) UninstallComponent(componentID string) error {
	manifest, exists := cm.registry.Installed[componentID]
	if !exists {
		return fmt.Errorf("component %s not found", componentID)
	}

	// Check reverse dependencies
	if err := cm.checkReverseDependencies(componentID); err != nil {
		return fmt.Errorf("cannot uninstall: %w", err)
	}

	// Remove files
	componentDir := filepath.Join(cm.installedDir, manifest.Category, componentID)
	if err := os.RemoveAll(componentDir); err != nil {
		return fmt.Errorf("failed to remove component files: %w", err)
	}

	// Update registry
	delete(cm.registry.Installed, componentID)

	return cm.SaveRegistry()
}

func (cm *ComponentManager) EnableComponent(componentID string) error {
	manifest, exists := cm.registry.Disabled[componentID]
	if !exists {
		return fmt.Errorf("component %s not found in disabled components", componentID)
	}

	// Move to enabled directory
	srcDir := filepath.Join(cm.disabledDir, manifest.Category, componentID)
	destDir := filepath.Join(cm.installedDir, manifest.Category, componentID)

	if err := os.Rename(srcDir, destDir); err != nil {
		return fmt.Errorf("failed to enable component: %w", err)
	}

	// Update registry
	cm.registry.Installed[componentID] = manifest
	delete(cm.registry.Disabled, componentID)

	return cm.SaveRegistry()
}

func (cm *ComponentManager) DisableComponent(componentID string) error {
	manifest, exists := cm.registry.Installed[componentID]
	if !exists {
		return fmt.Errorf("component %s not found", componentID)
	}

	// Move to disabled directory
	srcDir := filepath.Join(cm.installedDir, manifest.Category, componentID)
	destDir := filepath.Join(cm.disabledDir, manifest.Category, componentID)

	os.MkdirAll(filepath.Dir(destDir), 0755)
	if err := os.Rename(srcDir, destDir); err != nil {
		return fmt.Errorf("failed to disable component: %w", err)
	}

	// Update registry
	cm.registry.Disabled[componentID] = manifest
	delete(cm.registry.Installed, componentID)

	return cm.SaveRegistry()
}

func (cm *ComponentManager) ListInstalled() map[string]*ComponentManifest {
	return cm.registry.Installed
}

func (cm *ComponentManager) ListDisabled() map[string]*ComponentManifest {
	return cm.registry.Disabled
}

func (cm *ComponentManager) ListByCategory(category string) []*ComponentManifest {
	var result []*ComponentManifest

	for _, manifest := range cm.registry.Installed {
		if manifest.Category == category {
			result = append(result, manifest)
		}
	}

	return result
}

func (cm *ComponentManager) loadManifest(componentPath string) (*ComponentManifest, error) {
	manifestPath := filepath.Join(componentPath, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest ComponentManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func (cm *ComponentManager) checkDependencies(manifest *ComponentManifest) error {
	for _, dep := range manifest.Dependencies {
		if _, exists := cm.registry.Installed[dep]; !exists {
			return fmt.Errorf("missing dependency: %s", dep)
		}
	}
	return nil
}

func (cm *ComponentManager) checkReverseDependencies(componentID string) error {
	for id, manifest := range cm.registry.Installed {
		for _, dep := range manifest.Dependencies {
			if dep == componentID {
				return fmt.Errorf("component %s depends on %s", id, componentID)
			}
		}
	}
	return nil
}

func (cm *ComponentManager) copyComponent(src, dest string) error {
	return copyDir(src, dest)
}

func copyDir(src, dest string) error {
	// 简单的目录复制实现
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, info.Mode())
	})
}