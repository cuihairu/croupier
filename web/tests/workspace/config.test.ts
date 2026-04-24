/**
 * Workspace 配置服务测试
 */

import {
  loadWorkspaceConfig,
  saveWorkspaceConfig,
  listWorkspaceConfigs,
  deleteWorkspaceConfig,
  validateWorkspaceConfig,
  cloneWorkspaceConfig,
  exportWorkspaceConfig,
  importWorkspaceConfig,
} from '@/services/workspaceConfig';
import type { WorkspaceConfig } from '@/types/workspace';

describe('Workspace Config Service', () => {
  const mockConfig: WorkspaceConfig = {
    objectKey: 'test',
    title: '测试 Workspace',
    layout: {
      type: 'tabs',
      tabs: [
        {
          key: 'tab1',
          title: 'Tab 1',
          functions: ['test.list'],
          layout: {
            type: 'list',
            listFunction: 'test.list',
            columns: [
              { key: 'id', title: 'ID', width: 100 },
              { key: 'name', title: '名称', width: 150 },
            ],
          },
        },
      ],
    },
  };

  describe('loadWorkspaceConfig', () => {
    it('should load config successfully', async () => {
      const config = await loadWorkspaceConfig('player');
      expect(config).toBeDefined();
      expect(config?.objectKey).toBe('player');
    });

    it('should return null for non-existent config', async () => {
      const config = await loadWorkspaceConfig('non-existent');
      expect(config).toBeNull();
    });

    it('should use cache on second load', async () => {
      const config1 = await loadWorkspaceConfig('player');
      const config2 = await loadWorkspaceConfig('player');
      expect(config1).toEqual(config2);
    });

    it('should force refresh when specified', async () => {
      const config = await loadWorkspaceConfig('player', { forceRefresh: true });
      expect(config).toBeDefined();
    });
  });

  describe('saveWorkspaceConfig', () => {
    it('should save config successfully', async () => {
      const savedConfig = await saveWorkspaceConfig(mockConfig);
      expect(savedConfig).toBeDefined();
      expect(savedConfig.objectKey).toBe('test');
    });

    it('should update cache after save', async () => {
      await saveWorkspaceConfig(mockConfig);
      const config = await loadWorkspaceConfig('test');
      expect(config).toEqual(mockConfig);
    });
  });

  describe('listWorkspaceConfigs', () => {
    it('should list all configs', async () => {
      const configs = await listWorkspaceConfigs();
      expect(Array.isArray(configs)).toBe(true);
      expect(configs.length).toBeGreaterThan(0);
    });

    it('should update cache for all configs', async () => {
      const configs = await listWorkspaceConfigs();
      for (const config of configs) {
        const cached = await loadWorkspaceConfig(config.objectKey);
        expect(cached).toEqual(config);
      }
    });
  });

  describe('deleteWorkspaceConfig', () => {
    it('should delete config successfully', async () => {
      await deleteWorkspaceConfig('test');
      const config = await loadWorkspaceConfig('test');
      expect(config).toBeNull();
    });

    it('should clear cache after delete', async () => {
      await saveWorkspaceConfig(mockConfig);
      await deleteWorkspaceConfig('test');
      const config = await loadWorkspaceConfig('test', { useCache: true });
      expect(config).toBeNull();
    });
  });

  describe('validateWorkspaceConfig', () => {
    it('should validate valid config', () => {
      const result = validateWorkspaceConfig(mockConfig);
      expect(result.valid).toBe(true);
      expect(result.errors).toHaveLength(0);
    });

    it('should reject config without objectKey', () => {
      const invalidConfig = { ...mockConfig, objectKey: '' };
      const result = validateWorkspaceConfig(invalidConfig as any);
      expect(result.valid).toBe(false);
      expect(result.errors).toContain('objectKey 不能为空');
    });

    it('should reject config without title', () => {
      const invalidConfig = { ...mockConfig, title: '' };
      const result = validateWorkspaceConfig(invalidConfig as any);
      expect(result.valid).toBe(false);
      expect(result.errors).toContain('title 不能为空');
    });

    it('should reject tabs layout without tabs', () => {
      const invalidConfig = {
        ...mockConfig,
        layout: { type: 'tabs', tabs: [] },
      };
      const result = validateWorkspaceConfig(invalidConfig as any);
      expect(result.valid).toBe(false);
      expect(result.errors).toContain('tabs 布局至少需要一个 tab');
    });
  });

  describe('cloneWorkspaceConfig', () => {
    it('should clone config successfully', async () => {
      await saveWorkspaceConfig(mockConfig);
      const cloned = await cloneWorkspaceConfig('test', 'test-clone', '测试克隆');
      expect(cloned.objectKey).toBe('test-clone');
      expect(cloned.title).toBe('测试克隆');
      expect(cloned.layout).toEqual(mockConfig.layout);
    });

    it('should throw error for non-existent source', async () => {
      await expect(cloneWorkspaceConfig('non-existent', 'clone', '克隆')).rejects.toThrow(
        '配置不存在',
      );
    });
  });

  describe('exportWorkspaceConfig', () => {
    it('should export config as JSON', async () => {
      await saveWorkspaceConfig(mockConfig);
      const json = await exportWorkspaceConfig('test');
      expect(json).toBeDefined();
      const parsed = JSON.parse(json);
      expect(parsed.objectKey).toBe('test');
    });

    it('should throw error for non-existent config', async () => {
      await expect(exportWorkspaceConfig('non-existent')).rejects.toThrow('配置不存在');
    });
  });

  describe('importWorkspaceConfig', () => {
    it('should import valid config', async () => {
      const json = JSON.stringify(mockConfig);
      const imported = await importWorkspaceConfig(json);
      expect(imported.objectKey).toBe('test');
    });

    it('should reject invalid JSON', async () => {
      await expect(importWorkspaceConfig('invalid json')).rejects.toThrow('配置 JSON 格式错误');
    });

    it('should reject invalid config', async () => {
      const invalidConfig = { objectKey: '', title: '' };
      const json = JSON.stringify(invalidConfig);
      await expect(importWorkspaceConfig(json)).rejects.toThrow('配置验证失败');
    });
  });
});
