import { localizedText } from '@/utils/localizedText';
/**
 * PageRenderer - 页面渲染器统一入口
 *
 * 根据 PageSpec 的类型自动选择合适的渲染器：
 * - CompositeRenderer: 组合页（见 ./CompositeRenderer.tsx）
 * - ResourcePageRenderer: 资源 CRUD 页面
 * - OperationPageRenderer: 独立操作页面
 * - TaskPageRenderer: 异步任务页面
 * - ReportPageRenderer: 报表页面
 *
 * @module components/PageRenderer
 */

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Result } from 'antd';
import { useIntl } from '@umijs/max';
import { WarningOutlined } from '@ant-design/icons';
import { CompositeRenderer } from './CompositeRenderer';
import ResourcePageRenderer from './ResourcePageRenderer';
import OperationPageRenderer from './OperationPageRenderer';
import TaskPageRenderer from './TaskPageRenderer';
import ReportPageRenderer from './ReportPageRenderer';
import {
  contextWithPageState,
  mergePageState,
  outputPatchFromResult,
  projectBindingContext,
} from './runtime';
import type { PageState, PageStatePatch } from './runtime';
import type { JSONValue } from '@/types/dashboard';
import type {
  PageSpec,
  PageExecuteFn,
  TaskStatusResult,
  BindingExecutionContext,
  PageExecutionResult,
  ApprovalStatusResult,
} from '@/types/dashboard';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface PageRendererProps {
  /** 页面规格 */
  pageSpec: PageSpec;
  /** 执行绑定函数 */
  onExecute: PageExecuteFn;
  /** 预览模式只展示页面结构，禁止触发真实函数执行 */
  preview?: boolean;
  /** 查询任务状态（仅 TaskPage 需要） */
  onQueryStatus?: (taskId: string) => Promise<TaskStatusResult>;
  /** 取消任务（仅 TaskPage 需要） */
  onCancelTask?: (taskId: string) => Promise<void>;
  /** 查询审批状态（Operation/Task 等待审批需要） */
  onQueryApprovalStatus?: (approvalId: string) => Promise<ApprovalStatusResult>;
  /** 导出数据（仅 ReportPage 需要） */
  onExport?: (format: 'csv' | 'excel') => Promise<void>;
}

// ---------------------------------------------------------------------------
// PageRenderer 组件（类型路由器）
// ---------------------------------------------------------------------------

const PageRenderer: React.FC<PageRendererProps> = ({
  pageSpec,
  onExecute,
  preview = false,
  onQueryStatus,
  onCancelTask,
  onQueryApprovalStatus,
  onExport,
}) => {
  const { type, bindings } = pageSpec;
  const intl = useIntl();
  const [pageState, setPageState] = useState<PageState>({});
  const pageStateRef = useRef<PageState>({});

  useEffect(() => {
    pageStateRef.current = {};
    setPageState({});
  }, [pageSpec.pageKey]);

  useEffect(() => {
    pageStateRef.current = pageState;
  }, [pageState]);

  const executeWithPageState = useCallback<PageExecuteFn>(
    async (bindingId: string, context: BindingExecutionContext): Promise<PageExecutionResult> => {
      const binding = bindings.find((item) => item.id === bindingId);
      const result = await onExecute(
        bindingId,
        projectBindingContext(binding, contextWithPageState(context, pageStateRef.current)),
      );
      const patch = outputPatchFromResult(binding, result);
      setPageState((current) => {
        const next = mergePageState(current, patch);
        pageStateRef.current = next;
        return next;
      });
      return result;
    },
    [bindings, onExecute],
  );

  // 根据页面类型选择渲染器
  switch (type) {
    case 'composite': {
      if (!pageSpec.composite || pageSpec.composite.sections.length === 0) {
        return (
          <Result
            status="warning"
            title={intl.formatMessage({
              id: 'component.pageRenderer.shell.configErrorTitle',
              defaultMessage: '配置错误',
            })}
            subTitle={intl.formatMessage({
              id: 'component.pageRenderer.shell.missingComposite',
              defaultMessage: '组合页面缺少 composite 配置',
            })}
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <CompositeRenderer
          sections={pageSpec.composite.sections}
          bindings={bindings}
          onExecute={executeWithPageState}
          preview={preview}
          onPageStateMerge={(key, values, mode) => {
            setPageState((current) => {
              const prev = current[key];
              const nextValue =
                mode === 'merge' && prev && typeof prev === 'object' && !Array.isArray(prev)
                  ? ({ ...(prev as Record<string, JSONValue>), ...values } as never)
                  : (values as never);
              const patch: PageStatePatch = { [key]: nextValue };
              const next = mergePageState(current, patch);
              pageStateRef.current = next;
              return next;
            });
          }}
        />
      );
    }

    case 'resource':
      if (!pageSpec.resource) {
        return (
          <Result
            status="warning"
            title={intl.formatMessage({
              id: 'component.pageRenderer.shell.configErrorTitle',
              defaultMessage: '配置错误',
            })}
            subTitle={intl.formatMessage({
              id: 'component.pageRenderer.shell.missingResource',
              defaultMessage: '资源页面缺少 resource 配置',
            })}
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <ResourcePageRenderer
          spec={pageSpec.resource}
          bindings={bindings}
          onExecute={executeWithPageState}
          preview={preview}
          title={localizedText(pageSpec.title, 'zh-CN', '')}
        />
      );

    case 'operation':
      if (!pageSpec.operation) {
        return (
          <Result
            status="warning"
            title={intl.formatMessage({
              id: 'component.pageRenderer.shell.configErrorTitle',
              defaultMessage: '配置错误',
            })}
            subTitle={intl.formatMessage({
              id: 'component.pageRenderer.shell.missingOperation',
              defaultMessage: '操作页面缺少 operation 配置',
            })}
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <OperationPageRenderer
          spec={pageSpec.operation}
          bindings={bindings}
          onExecute={executeWithPageState}
          preview={preview}
          onQueryApprovalStatus={onQueryApprovalStatus}
          title={localizedText(pageSpec.title, 'zh-CN', '')}
        />
      );

    case 'task':
      if (!pageSpec.task) {
        return (
          <Result
            status="warning"
            title={intl.formatMessage({
              id: 'component.pageRenderer.shell.configErrorTitle',
              defaultMessage: '配置错误',
            })}
            subTitle={intl.formatMessage({
              id: 'component.pageRenderer.shell.missingTask',
              defaultMessage: '任务页面缺少 task 配置',
            })}
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <TaskPageRenderer
          spec={pageSpec.task}
          bindings={bindings}
          onExecute={executeWithPageState}
          preview={preview}
          onQueryStatus={onQueryStatus}
          onCancelTask={onCancelTask}
          onQueryApprovalStatus={onQueryApprovalStatus}
          title={localizedText(pageSpec.title, 'zh-CN', '')}
        />
      );

    case 'report':
      if (!pageSpec.report) {
        return (
          <Result
            status="warning"
            title={intl.formatMessage({
              id: 'component.pageRenderer.shell.configErrorTitle',
              defaultMessage: '配置错误',
            })}
            subTitle={intl.formatMessage({
              id: 'component.pageRenderer.shell.missingReport',
              defaultMessage: '报表页面缺少 report 配置',
            })}
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <ReportPageRenderer
          spec={pageSpec.report}
          bindings={bindings}
          onExecute={executeWithPageState}
          preview={preview}
          onExport={onExport}
          title={localizedText(pageSpec.title, 'zh-CN', '')}
        />
      );

    default:
      return (
        <Result
          status="error"
          title={intl.formatMessage({
            id: 'component.pageRenderer.shell.unknownType',
            defaultMessage: '未知页面类型',
          })}
          subTitle={intl.formatMessage(
            {
              id: 'component.pageRenderer.shell.unsupportedType',
              defaultMessage: `不支持的页面类型: ${String(type)}`,
            },
            { type: String(type) },
          )}
          icon={<WarningOutlined />}
        />
      );
  }
};

export default PageRenderer;

// ---------------------------------------------------------------------------
// 兼容再导出（调用方与测试导入路径零改动）
// ---------------------------------------------------------------------------

export { CompositeRenderer } from './CompositeRenderer';
export { resolveStepParams, RUNTIME_STATE_KEYS } from './runtime';
