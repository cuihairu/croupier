/**
 * 页面配置服务
 *
 * 负责加载和管理页面配置
 */

import type { PageConfig } from '@/components/PageGenerator/types';

// 配置缓存
let configCache: Map<string, PageConfig> | null = null;

/**
 * 加载所有页面配置
 */
export async function loadAllPageConfigs(): Promise<PageConfig[]> {
  try {
    // TODO: 从后端API加载
    // const response = await fetch('/api/page-configs');
    // const data = await response.json();
    // return data.configs || [];

    // 暂时使用 Mock 数据
    const { mockPageConfigs } = await import('./mockPageConfigs');
    return mockPageConfigs;
  } catch (error) {
    console.error('Failed to load page configs:', error);
    return [];
  }
}

/**
 * 初始化配置缓存
 */
async function initConfigCache() {
  if (configCache) return;

  const configs = await loadAllPageConfigs();
  configCache = new Map();

  configs.forEach((config) => {
    configCache!.set(config.path, config);
  });
}

/**
 * 根据路径获取页面配置
 */
export async function getPageConfig(path: string): Promise<PageConfig | null> {
  await initConfigCache();
  return configCache?.get(path) || null;
}

/**
 * 根据ID获取页面配置
 */
export async function getPageConfigById(id: string): Promise<PageConfig | null> {
  await initConfigCache();

  if (!configCache) return null;

  for (const config of configCache.values()) {
    if (config.id === id) {
      return config;
    }
  }

  return null;
}

/**
 * 保存页面配置
 */
export async function savePageConfig(config: PageConfig): Promise<void> {
  // TODO: 调用后端API保存
  // await fetch('/api/page-configs', {
  //   method: 'POST',
  //   headers: { 'Content-Type': 'application/json' },
  //   body: JSON.stringify(config),
  // });

  // 更新缓存
  if (configCache) {
    configCache.set(config.path, config);
  }

  console.log('Config saved:', config);
}

/**
 * 更新页面配置
 */
export async function updatePageConfig(id: string, config: PageConfig): Promise<void> {
  // TODO: 调用后端API更新
  // await fetch(`/api/page-configs/${id}`, {
  //   method: 'PUT',
  //   headers: { 'Content-Type': 'application/json' },
  //   body: JSON.stringify(config),
  // });

  // 更新缓存
  if (configCache) {
    // 删除旧路径
    for (const [path, oldConfig] of configCache.entries()) {
      if (oldConfig.id === id) {
        configCache.delete(path);
        break;
      }
    }
    // 添加新路径
    configCache.set(config.path, config);
  }

  console.log('Config updated:', config);
}

/**
 * 删除页面配置
 */
export async function deletePageConfig(id: string): Promise<void> {
  // TODO: 调用后端API删除
  // await fetch(`/api/page-configs/${id}`, {
  //   method: 'DELETE',
  // });

  // 更新缓存
  if (configCache) {
    for (const [path, config] of configCache.entries()) {
      if (config.id === id) {
        configCache.delete(path);
        break;
      }
    }
  }

  console.log('Config deleted:', id);
}

/**
 * 清除配置缓存（用于刷新）
 */
export function clearConfigCache(): void {
  configCache = null;
}
