/**
 * Workspace 渲染器
 *
 * 根据 WorkspaceConfig 动态渲染 Workspace 界面。
 *
 * @module components/WorkspaceRenderer
 */

import React from 'react';
import { Spin, Empty, Alert } from 'antd';
import type { WorkspaceConfig } from '@/types/workspace';
import { trackWorkspaceEvent } from '@/services/workspace/telemetry';
import {
  getWorkspaceErrorMessage,
  parseWorkspaceError,
  type WorkspaceErrorCode,
} from '@/services/workspace/errors';
import TabsLayout from './TabsLayout';

export interface WorkspaceRendererProps {
  /** Workspace 配置 */
  config: WorkspaceConfig | null;

  /** 是否加载中 */
  loading?: boolean;

  /** 错误信息 */
  error?: string;

  /** 额外的上下文数据 */
  context?: Record<string, any>;
}

/**
 * Workspace 渲染器组件
 *
 * 这是 Layout Engine 的入口组件，根据配置类型分发到不同的布局组件。
 */
export default function WorkspaceRenderer({
  config,
  loading = false,
  error,
  context,
}: WorkspaceRendererProps) {
  React.useEffect(() => {
    if (error) {
      trackWorkspaceEvent('workspace_render_error', {
        reason: 'load_error',
        error,
        objectKey: config?.objectKey,
      });
      return;
    }
    if (config?.layout?.type && config.layout.type !== 'tabs') {
      trackWorkspaceEvent('workspace_render_error', {
        reason: 'unsupported_layout',
        objectKey: config.objectKey,
        layoutType: config.layout.type,
      });
    }
  }, [error, config?.objectKey, config?.layout?.type]);

  // 渲染加载状态
  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <Spin size="large" />
      </div>
    );
  }

  // 渲染错误状态
  if (error) {
    return (
      <Alert
        message="加载失败"
        description={error}
        type="error"
        showIcon
        style={{ margin: '20px' }}
      />
    );
  }

  // 渲染空状态
  if (!config) {
    return <Empty description="暂无配置" style={{ marginTop: '100px' }} />;
  }

  // 根据布局类型渲染
  return <div className="workspace-renderer">{renderLayout(config, context)}</div>;
}

/**
 * 根据布局类型渲染对应的布局组件
 */
function renderLayout(config: WorkspaceConfig, context?: Record<string, any>): React.ReactNode {
  const { layout } = config;

  if (!layout) {
    return (
      <Alert
        message="配置无效"
        description="Workspace 配置缺少 layout 字段"
        type="error"
        showIcon
        style={{ margin: '20px' }}
      />
    );
  }

  switch (layout.type) {
    case 'tabs':
      return <TabsLayout config={config} context={context} />;

    default:
      return (
        <Alert
          message="当前顶层布局暂未接入正式运行范围"
          description={`运行时当前正式支持 tabs 顶层布局，当前类型: ${(layout as any).type}`}
          type="error"
          showIcon
        />
      );
  }
}

/**
 * 使用 Workspace 配置的 Hook
 *
 * @param objectKey - 对象标识
 * @returns 配置、加载状态、错误信息
 */
export function useWorkspaceConfig(objectKey: string) {
  const [config, setConfig] = React.useState<WorkspaceConfig | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string>();
  const [errorCode, setErrorCode] = React.useState<WorkspaceErrorCode | undefined>();

  React.useEffect(() => {
    loadConfig();
  }, [objectKey]);

  const loadConfig = async () => {
    setLoading(true);
    setError(undefined);
    setErrorCode(undefined);

    try {
      // 动态导入配置服务
      const { loadWorkspaceConfig } = await import('@/services/workspaceConfig');
      const workspaceConfig = await loadWorkspaceConfig(objectKey);
      setConfig(workspaceConfig);
    } catch (err: any) {
      const parsedError = parseWorkspaceError(err);
      setErrorCode(parsedError.code);
      setError(getWorkspaceErrorMessage(err, '加载配置失败'));
    } finally {
      setLoading(false);
    }
  };

  const reload = () => {
    loadConfig();
  };

  return {
    config,
    loading,
    error,
    errorCode,
    reload,
  };
}
