import { localizedText } from '@/utils/localizedText';
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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Button, Card, Col, Descriptions, Result, Row, Table } from 'antd';
import { WarningOutlined } from '@ant-design/icons';
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
import type { PageState } from './runtime';
import type {
  PageSpec,
  PageExecuteFn,
  TaskStatusResult,
  BindingExecutionContext,
  PageExecutionResult,
  ApprovalStatusResult,
  CompositeSection,
  PageFunctionBinding,
  ColumnSpec,
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
// PageRenderer 组件
// ---------------------------------------------------------------------------

/** 组合页渲染器：区块栅格布局；autoRun 区块加载即执行；refreshOn
 * 声明的 stateKey 变化时自动重跑本区块（跨函数联动）。 */
const CompositeRenderer: React.FC<{
  sections: CompositeSection[];
  bindings: PageFunctionBinding[];
  onExecute: PageExecuteFn;
  preview: boolean;
}> = ({ sections, bindings, onExecute, preview }) => {
  const [results, setResults] = useState<Record<string, PageExecutionResult | null>>({});
  const [running, setRunning] = useState<Record<string, boolean>>({});
  const [sectionInputs, setSectionInputs] = useState<Record<string, Record<string, unknown>>>({});

  const runSection = useCallback(
    async (section: CompositeSection) => {
      setRunning((prev) => ({ ...prev, [section.key]: true }));
      try {
        const result = await onExecute(section.bindingId, sectionInputs[section.key] || {});
        setResults((prev) => ({ ...prev, [section.key]: result || null }));
      } finally {
        setRunning((prev) => ({ ...prev, [section.key]: false }));
      }
    },
    [onExecute, sectionInputs],
  );

  // autoRun：加载即执行
  useEffect(() => {
    for (const sec of sections) {
      if (sec.autoRun) void runSection(sec);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const resultFor = (sec: CompositeSection): Record<string, unknown> | undefined => {
    const r = results[sec.key];
    return (r as { data?: Record<string, unknown> } | null | undefined)?.data;
  };

  return (
    <Row gutter={[12, 12]}>
      {sections.map((sec) => (
        <Col key={sec.key} span={sec.span && sec.span > 0 && sec.span <= 24 ? sec.span : 24}>
          <Card
            size="small"
            title={localizedText(sec.title, 'zh-CN', sec.key)}
            loading={running[sec.key] || false}
            extra={
              sec.view !== 'actions' && !sec.autoRun ? (
                <Button size="small" onClick={() => void runSection(sec)}>
                  执行
                </Button>
              ) : null
            }
          >
            {sec.view === 'table' ? (
              <Table
                size="small"
                rowKey={(_, i) => String(i)}
                columns={(sec.table?.columns || []).map((c) => ({
                  title: localizedText(c.title, 'zh-CN', c.key),
                  dataIndex: c.key,
                  ellipsis: true,
                }))}
                dataSource={
                  Array.isArray(resultFor(sec)?.items)
                    ? (resultFor(sec)?.items as Record<string, unknown>[])
                    : []
                }
                pagination={{ pageSize: 10, showSizeChanger: false }}
              />
            ) : sec.view === 'fields' ? (
              <Descriptions size="small" column={2}>
                {(resultFor(sec)
                  ? Object.entries(resultFor(sec) as Record<string, unknown>)
                  : []
                ).map(([k, v]) => (
                  <Descriptions.Item key={k} label={k}>
                    {String(v ?? '-')}
                  </Descriptions.Item>
                ))}
              </Descriptions>
            ) : (
              <Button type="primary" onClick={() => void runSection(sec)}>
                {localizedText(sec.title, 'zh-CN', sec.key)}
              </Button>
            )}
          </Card>
        </Col>
      ))}
    </Row>
  );
};

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
            title="配置错误"
            subTitle="组合页面缺少 composite 配置"
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
        />
      );
    }

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
          title="未知页面类型"
          subTitle={`不支持的页面类型: ${type}`}
          icon={<WarningOutlined />}
        />
      );
  }
};

export default PageRenderer;
