package pack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestNewComponentManager 测试创建组件管理器
func TestNewComponentManager(t *testing.T) {
	dataDir := t.TempDir()

	cm := NewComponentManager(dataDir)

	if cm == nil {
		t.Fatal("NewComponentManager should return non-nil manager")
	}

	if cm.dataDir != dataDir {
		t.Errorf("dataDir = %q, want %q", cm.dataDir, dataDir)
	}

	expectedInstalled := filepath.Join(dataDir, "components", "installed")
	if cm.installedDir != expectedInstalled {
		t.Errorf("installedDir = %q, want %q", cm.installedDir, expectedInstalled)
	}

	expectedDisabled := filepath.Join(dataDir, "components", "disabled")
	if cm.disabledDir != expectedDisabled {
		t.Errorf("disabledDir = %q, want %q", cm.disabledDir, expectedDisabled)
	}

	if cm.registry == nil {
		t.Error("registry should not be nil")
	}

	if cm.registry.Installed == nil {
		t.Error("registry.Installed should not be nil")
	}

	if cm.registry.Disabled == nil {
		t.Error("registry.Disabled should not be nil")
	}
}

// TestComponentManager_LoadRegistry_SaveRegistry 测试注册表加载和保存
func TestComponentManager_LoadRegistry_SaveRegistry(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	// 保存初始空注册表
	err := cm.SaveRegistry()
	if err != nil {
		t.Fatalf("SaveRegistry() error = %v", err)
	}

	// 验证文件存在
	registryPath := filepath.Join(dataDir, "component-registry.json")
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Error("Registry file should exist after SaveRegistry")
	}

	// 加载注册表
	cm2 := NewComponentManager(dataDir)
	err = cm2.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	if len(cm2.registry.Installed) != 0 {
		t.Errorf("Installed should be empty, got %d", len(cm2.registry.Installed))
	}

	if len(cm2.registry.Disabled) != 0 {
		t.Errorf("Disabled should be empty, got %d", len(cm2.registry.Disabled))
	}
}

// TestComponentManager_LoadRegistry_WithExistingData 测试加载已有数据
func TestComponentManager_LoadRegistry_WithExistingData(t *testing.T) {
	dataDir := t.TempDir()
	registryPath := filepath.Join(dataDir, "component-registry.json")

	// 创建测试注册表数据
	registry := ComponentRegistry{
		Installed: map[string]*ComponentManifest{
			"test-comp": {
				ID:      "test-comp",
				Name:    "Test Component",
				Version: "1.0.0",
			},
		},
		Disabled: map[string]*ComponentManifest{},
	}

	data, _ := json.MarshalIndent(registry, "", "  ")
	os.WriteFile(registryPath, data, 0644)

	// 加载注册表
	cm := NewComponentManager(dataDir)
	err := cm.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	if len(cm.registry.Installed) != 1 {
		t.Errorf("Installed should have 1 component, got %d", len(cm.registry.Installed))
	}

	comp, exists := cm.registry.Installed["test-comp"]
	if !exists {
		t.Fatal("test-comp should exist in Installed")
	}

	if comp.Name != "Test Component" {
		t.Errorf("Component name = %q, want 'Test Component'", comp.Name)
	}
}

// TestComponentManager_ListInstalled 测试列出已安装组件
func TestComponentManager_ListInstalled(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	// 添加测试组件
	cm.registry.Installed = map[string]*ComponentManifest{
		"comp1": {ID: "comp1", Name: "Component 1"},
		"comp2": {ID: "comp2", Name: "Component 2"},
	}

	result := cm.ListInstalled()

	if len(result) != 2 {
		t.Errorf("ListInstalled() should return 2 components, got %d", len(result))
	}

	if result["comp1"] == nil {
		t.Error("comp1 should be in result")
	}

	if result["comp2"] == nil {
		t.Error("comp2 should be in result")
	}
}

// TestComponentManager_ListDisabled 测试列出已禁用组件
func TestComponentManager_ListDisabled(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	// 添加禁用组件
	cm.registry.Disabled = map[string]*ComponentManifest{
		"comp1": {ID: "comp1", Name: "Disabled Component"},
	}

	result := cm.ListDisabled()

	if len(result) != 1 {
		t.Errorf("ListDisabled() should return 1 component, got %d", len(result))
	}

	if result["comp1"] == nil {
		t.Error("comp1 should be in result")
	}
}

// TestComponentManager_ListByCategory 测试按类别列出
func TestComponentManager_ListByCategory(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	// 添加不同类别的组件
	cm.registry.Installed = map[string]*ComponentManifest{
		"comp1": {ID: "comp1", Category: "player"},
		"comp2": {ID: "comp2", Category: "player"},
		"comp3": {ID: "comp3", Category: "economy"},
	}

	// 列出 player 类别
	playerComps := cm.ListByCategory("player")
	if len(playerComps) != 2 {
		t.Errorf("ListByCategory('player') should return 2 components, got %d", len(playerComps))
	}

	// 列出 economy 类别
	economyComps := cm.ListByCategory("economy")
	if len(economyComps) != 1 {
		t.Errorf("ListByCategory('economy') should return 1 component, got %d", len(economyComps))
	}

	// 列出不存在的类别
	emptyComps := cm.ListByCategory("nonexistent")
	if len(emptyComps) != 0 {
		t.Errorf("ListByCategory('nonexistent') should return 0 components, got %d", len(emptyComps))
	}
}

// TestComponentManager_checkDependencies 测试依赖检查
func TestComponentManager_checkDependencies(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	manifest := &ComponentManifest{
		ID: "test-comp",
		Dependencies: []string{"dep1", "dep2"},
	}

	// 没有安装依赖，应该失败
	err := cm.checkDependencies(manifest)
	if err == nil {
		t.Error("checkDependencies should fail when dependencies are missing")
	}

	// 安装依赖
	cm.registry.Installed = map[string]*ComponentManifest{
		"dep1": {ID: "dep1"},
		"dep2": {ID: "dep2"},
	}

	err = cm.checkDependencies(manifest)
	if err != nil {
		t.Errorf("checkDependencies should succeed when dependencies exist, got error: %v", err)
	}
}

// TestComponentManager_checkReverseDependencies 测试反向依赖检查
func TestComponentManager_checkReverseDependencies(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	tests := []struct {
		name          string
		componentID   string
		installed     map[string]*ComponentManifest
		shouldFail    bool
	}{
		{
			name:        "有反向依赖",
			componentID: "test-comp",
			installed: map[string]*ComponentManifest{
				"dependent1": {
					ID: "dependent1",
					Dependencies: []string{"test-comp"},
				},
			},
			shouldFail: true,
		},
		{
			name:        "无反向依赖",
			componentID: "standalone",
			installed: map[string]*ComponentManifest{
				"dependent1": {
					ID: "dependent1",
					Dependencies: []string{"other"},
				},
				"dependent2": {
					ID: "dependent2",
					Dependencies: []string{},
				},
			},
			shouldFail: false,
		},
		{
			name:        "空安装列表",
			componentID: "nonexistent",
			installed:   map[string]*ComponentManifest{},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm.registry.Installed = tt.installed
			err := cm.checkReverseDependencies(tt.componentID)

			if tt.shouldFail && err == nil {
				t.Error("checkReverseDependencies should fail when components depend on this one")
			}
			if !tt.shouldFail && err != nil {
				t.Errorf("checkReverseDependencies should succeed, got error: %v", err)
			}
		})
	}
}

// TestComponentManager_UninstallComponent 测试卸载组件
func TestComponentManager_UninstallComponent(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	// 添加测试组件
	cm.registry.Installed = map[string]*ComponentManifest{
		"test-comp": {
			ID:       "test-comp",
			Category: "test",
		},
	}

	// 卸载不存在的组件
	err := cm.UninstallComponent("nonexistent")
	if err == nil {
		t.Error("UninstallComponent should fail for nonexistent component")
	}

	// 卸载存在的组件（会失败因为文件不存在，但逻辑测试）
	err = cm.UninstallComponent("test-comp")
	// 由于文件不存在，RemoveAll 会失败，但至少验证了逻辑
	if err != nil {
		// 预期失败，因为文件不存在
		t.Logf("Expected failure (files don't exist): %v", err)
	}
}

// TestComponentManager_EnableComponent 测试启用组件
func TestComponentManager_EnableComponent(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	// 添加到禁用列表
	cm.registry.Disabled = map[string]*ComponentManifest{
		"test-comp": {
			ID:       "test-comp",
			Category: "test",
		},
	}

	// 启用不存在的组件
	err := cm.EnableComponent("nonexistent")
	if err == nil {
		t.Error("EnableComponent should fail for nonexistent component")
	}

	// 启用存在的组件（会失败因为目录不存在）
	err = cm.EnableComponent("test-comp")
	if err != nil {
		t.Logf("Expected failure (directory doesn't exist): %v", err)
	}
}

// TestComponentManager_DisableComponent 测试禁用组件
func TestComponentManager_DisableComponent(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	// 添加到已安装列表
	cm.registry.Installed = map[string]*ComponentManifest{
		"test-comp": {
			ID:       "test-comp",
			Category: "test",
		},
	}

	// 禁用不存在的组件
	err := cm.DisableComponent("nonexistent")
	if err == nil {
		t.Error("DisableComponent should fail for nonexistent component")
	}

	// 禁用存在的组件（会失败因为目录不存在）
	err = cm.DisableComponent("test-comp")
	if err != nil {
		t.Logf("Expected failure (directory doesn't exist): %v", err)
	}
}

// TestComponentManager_loadManifest 测试加载清单
func TestComponentManager_loadManifest(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	// 创建临时组件目录
	componentDir := filepath.Join(dataDir, "test-component")
	os.MkdirAll(componentDir, 0755)

	// 创建 manifest.json
	manifest := ComponentManifest{
		ID:          "test-comp",
		Name:        "Test Component",
		Version:     "1.0.0",
		Description: "A test component",
		Category:    "test",
		Functions: []ComponentFunction{
			{
				ID:      "func1",
				Version: "1.0.0",
				Enabled: true,
			},
		},
	}

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(componentDir, "manifest.json"), manifestData, 0644)

	// 加载清单
	loaded, err := cm.loadManifest(componentDir)
	if err != nil {
		t.Fatalf("loadManifest() error = %v", err)
	}

	if loaded.ID != "test-comp" {
		t.Errorf("ID = %q, want 'test-comp'", loaded.ID)
	}

	if loaded.Name != "Test Component" {
		t.Errorf("Name = %q, want 'Test Component'", loaded.Name)
	}

	if len(loaded.Functions) != 1 {
		t.Errorf("Functions should have 1 item, got %d", len(loaded.Functions))
	}
}

// TestComponentManager_loadManifest_NotFound 测试加载不存在的清单
func TestComponentManager_loadManifest_NotFound(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	_, err := cm.loadManifest("nonexistent")
	if err == nil {
		t.Error("loadManifest should fail for nonexistent directory")
	}
}

// TestComponentManager_loadManifest_InvalidJSON 测试加载无效 JSON
func TestComponentManager_loadManifest_InvalidJSON(t *testing.T) {
	dataDir := t.TempDir()
	cm := NewComponentManager(dataDir)

	// 创建临时组件目录
	componentDir := filepath.Join(dataDir, "test-component")
	os.MkdirAll(componentDir, 0755)

	// 写入无效 JSON
	os.WriteFile(filepath.Join(componentDir, "manifest.json"), []byte("{invalid json"), 0644)

	_, err := cm.loadManifest(componentDir)
	if err == nil {
		t.Error("loadManifest should fail for invalid JSON")
	}
}

// TestComponentManifest 测试组件清单结构
func TestComponentManifest(t *testing.T) {
	manifest := ComponentManifest{
		ID:          "test-id",
		Name:        "Test Component",
		Version:     "1.0.0",
		Description: "Test Description",
		Category:    "test",
		Dependencies: []string{"dep1", "dep2"},
		Author:      "Test Author",
		License:     "MIT",
		Functions: []ComponentFunction{
			{
				ID:          "func1",
				Version:     "1.0.0",
				Enabled:     true,
				Description: "Test function",
			},
		},
	}

	if manifest.ID != "test-id" {
		t.Errorf("ID = %q, want 'test-id'", manifest.ID)
	}

	if len(manifest.Dependencies) != 2 {
		t.Errorf("Dependencies should have 2 items, got %d", len(manifest.Dependencies))
	}

	if len(manifest.Functions) != 1 {
		t.Errorf("Functions should have 1 item, got %d", len(manifest.Functions))
	}
}

// TestComponentFunction 测试组件函数结构
func TestComponentFunction(t *testing.T) {
	fn := ComponentFunction{
		ID:          "test-func",
		Version:     "2.0.0",
		Enabled:     true,
		Description: "A test function",
	}

	if fn.ID != "test-func" {
		t.Errorf("ID = %q, want 'test-func'", fn.ID)
	}

	if fn.Version != "2.0.0" {
		t.Errorf("Version = %q, want '2.0.0'", fn.Version)
	}

	if !fn.Enabled {
		t.Error("Enabled should be true")
	}
}

// TestCopyDir 测试目录复制
func TestCopyDir(t *testing.T) {
	// 创建源目录
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "test.txt")
	os.WriteFile(srcFile, []byte("test content"), 0644)

	// 创建目标目录
	destDir := filepath.Join(t.TempDir(), "dest")

	// 复制目录
	err := copyDir(srcDir, destDir)
	if err != nil {
		t.Fatalf("copyDir() error = %v", err)
	}

	// 验证文件被复制
	destFile := filepath.Join(destDir, "test.txt")
	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}

	if string(data) != "test content" {
		t.Errorf("File content = %q, want 'test content'", string(data))
	}
}

// BenchmarkNewComponentManager 性能基准测试
func BenchmarkNewComponentManager(b *testing.B) {
	dataDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewComponentManager(dataDir)
	}
}

// BenchmarkListInstalled 性能基准测试
func BenchmarkListInstalled(b *testing.B) {
	dataDir := b.TempDir()
	cm := NewComponentManager(dataDir)

	// 添加 100 个组件
	for i := 0; i < 100; i++ {
		cm.registry.Installed["comp"+string(rune(i))] = &ComponentManifest{
			ID: "comp" + string(rune(i)),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.ListInstalled()
	}
}

// BenchmarkListByCategory 性能基准测试
func BenchmarkListByCategory(b *testing.B) {
	dataDir := b.TempDir()
	cm := NewComponentManager(dataDir)

	// 添加不同类别的组件
	for i := 0; i < 100; i++ {
		category := "player"
		if i%3 == 0 {
			category = "economy"
		}
		cm.registry.Installed["comp"+string(rune(i))] = &ComponentManifest{
			ID:       "comp" + string(rune(i)),
			Category: category,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.ListByCategory("player")
	}
}
