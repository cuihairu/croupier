/**
 * PageRenderer - 页面渲染器统一入口
 *
 * 根据 PageSpec 的类型自动选择合适的渲染器：
 * - ResourcePageRenderer: 资源 CRUD 页面
 * - OperationPageRenderer: 独立操作页面
 * - TaskPageRenderer: 异步任务页面
 * - ReportPageRenderer: 报表页面
 *
 * @module components/PageRenderer
 */

import React from 'react';
import { Result, Typography } from 'antd';
import { WarningOutlined } from '@ant-design/icons';
import ResourcePageRenderer from './ResourcePageRenderer';
import OperationPageRenderer from './OperationPageRenderer';
import TaskPageRenderer from './TaskPageRenderer';
import ReportPageRenderer from './ReportPageRenderer';
import type {
  PageSpec,
  PageExecuteFn,
  TaskStatusResult,
} from '@/types/dashboard';

const { Text } = Typography;

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface PageRendererProps {
  /** 页面规格 */
  pageSpec: PageSpec;
  /** 执行绑定函数 */
  onExecute: PageExecuteFn;
  /** 查询任务状态（仅 TaskPage 需要） */
  onQueryStatus?: (taskId: string) => Promise<TaskStatusResult>;
  /** 取消任务（仅 TaskPage 需要） */
  onCancelTask?: (taskId: string) => Promise<void>;
  /** 重试任务（仅 TaskPage 需要） */
  onRetryTask?: (taskId: string) => Promise<void>;
  /** 导出数据（仅 ReportPage 需要） */
  onExport?: (format: 'csv' | 'excel') => Promise<void>;
}

// ---------------------------------------------------------------------------
// PageRenderer 组件
// ---------------------------------------------------------------------------

const PageRenderer: React.FC<PageRendererProps> = ({
  pageSpec,
  onExecute,
  onQueryStatus,
  onCancelTask,
  onRetryTask,
  onExport,
}) => {
  const { type, bindings } = pageSpec;

  // 根据页面类型选择渲染器
  switch (type) {
    case 'resource':
      if (!pageSpec.resource) {
        return (
          <Result
            status="warning"
            title="配置错误"
            subTitle="资源页面缺少 resource 配置"
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <ResourcePageRenderer
          spec={pageSpec.resource}
          bindings={bindings}
          onExecute={onExecute}
          title={pageSpec.title?.['zh-CN'] || pageSpec.title?.['en']}
        />
      );

    case 'operation':
      if (!pageSpec.operation) {
        return (
          <Result
            status="warning"
            title="配置错误"
            subTitle="操作页面缺少 operation 配置"
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <OperationPageRenderer
          spec={pageSpec.operation}
          bindings={bindings}
          onExecute={onExecute}
          title={pageSpec.title?.['zh-CN'] || pageSpec.title?.['en']}
        />
      );

    case 'task':
      if (!pageSpec.task) {
        return (
          <Result
            status="warning"
            title="配置错误"
            subTitle="任务页面缺少 task 配置"
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <TaskPageRenderer
          spec={pageSpec.task}
          bindings={bindings}
          onExecute={onExecute}
          onQueryStatus={onQueryStatus}
          onCancelTask={onCancelTask}
          onRetryTask={onRetryTask}
          title={pageSpec.title?.['zh-CN'] || pageSpec.title?.['en']}
        />
      );

    case 'report':
      if (!pageSpec.report) {
        return (
          <Result
            status="warning"
            title="配置错误"
            subTitle="报表页面缺少 report 配置"
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <ReportPageRenderer
          spec={pageSpec.report}
          bindings={bindings}
          onExecute={onExecute}
          onExport={onExport}
          title={pageSpec.title?.['zh-CN'] || pageSpec.title?.['en']}
        />
      );

    default:
      return (
        <Result
          status="error"
          title="未知页面类型"
          subTitle={`不支持的页面类型: ${type}`}
          icon={<WarningOutlined />}
        />
      );
  }
};

export default PageRenderer;
