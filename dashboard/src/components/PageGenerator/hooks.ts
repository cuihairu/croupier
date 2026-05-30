/**
 * 动态数据 Hook
 *
 * 根据数据源配置加载数据
 */

import { useState, useEffect } from 'react';
import { message } from 'antd';
import type { DataSourceConfig } from './types';
import { invokeFunction } from '@/services/api';

export function useDynamicData(config: DataSourceConfig) {
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const load = async (params?: any) => {
    setLoading(true);
    setError(null);

    try {
      let result: any;

      if (config.type === 'function') {
        // 调用函数
        result = await invokeFunction(config.functionId!, {
          ...config.params,
          ...params,
        });
      } else if (config.type === 'api') {
        // 调用 API
        const url = new URL(config.apiEndpoint!, window.location.origin);
        if (config.method === 'GET' && params) {
          Object.keys(params).forEach((key) => {
            url.searchParams.append(key, params[key]);
          });
        }

        const response = await fetch(url.toString(), {
          method: config.method || 'GET',
          headers: { 'Content-Type': 'application/json' },
          body:
            config.method !== 'GET' ? JSON.stringify({ ...config.params, ...params }) : undefined,
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        result = await response.json();
      } else if (config.type === 'static') {
        // 静态数据
        result = config.staticData || [];
      }

      // 处理结果
      const dataArray = Array.isArray(result) ? result : result?.data || result?.list || [];
      setData(dataArray);
    } catch (err: any) {
      setError(err);
      message.error(err.message || '加载数据失败');
      console.error('Failed to load data:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, [config.functionId, config.apiEndpoint]);

  return {
    data,
    loading,
    error,
    refresh: load,
  };
}
