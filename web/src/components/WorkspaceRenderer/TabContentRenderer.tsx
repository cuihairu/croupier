/**
 * Tab 内容渲染器
 *
 * 根据 Tab 的布局类型渲染对应的内容。
 *
 * @module components/WorkspaceRenderer/TabContentRenderer
 */

import React from 'react';
import { Alert } from 'antd';
import type { TabConfig } from '@/types/workspace';
import FormDetailRenderer from './renderers/FormDetailRenderer';
import ListRenderer from './renderers/ListRenderer';
import FormRenderer from './renderers/FormRenderer';
import DetailRenderer from './renderers/DetailRenderer';
import KanbanRenderer from './renderers/KanbanRenderer';
import TimelineRenderer from './renderers/TimelineRenderer';
import SplitRenderer from './renderers/SplitRenderer';
import WizardRenderer from './renderers/WizardRenderer';
import DashboardRenderer from './renderers/DashboardRenderer';
import GridRenderer from './renderers/GridRenderer';
import CustomRenderer from './renderers/CustomRenderer';

export interface TabContentRendererProps {
  /** Tab 配置 */
  tab: TabConfig;

  /** 对象标识 */
  objectKey: string;

  /** 额外的上下文数据 */
  context?: Record<string, any>;
}

/**
 * Tab 内容渲染器组件
 *
 * 根据 Tab 的布局类型分发到不同的渲染器。
 */
export default function TabContentRenderer({ tab, objectKey, context }: TabContentRendererProps) {
  const layout = normalizeLegacyLayout(tab);

  // 根据布局类型渲染
  switch (layout.type) {
    case 'form-detail':
      return <FormDetailRenderer layout={layout} objectKey={objectKey} context={context} />;

    case 'list':
      return <ListRenderer layout={layout} objectKey={objectKey} context={context} />;

    case 'form':
      return <FormRenderer layout={layout} objectKey={objectKey} context={context} />;

    case 'detail':
      return <DetailRenderer layout={layout} objectKey={objectKey} context={context} />;

    case 'kanban':
      return <KanbanRenderer config={{ layout }} context={context} />;

    case 'timeline':
      return <TimelineRenderer config={{ layout }} context={context} />;

    case 'split':
      return <SplitRenderer layout={layout} objectKey={objectKey} context={context} />;

    case 'wizard':
      return <WizardRenderer layout={layout} objectKey={objectKey} context={context} />;

    case 'dashboard':
      return <DashboardRenderer layout={layout} objectKey={objectKey} context={context} />;

    case 'grid':
      return <GridRenderer layout={layout} objectKey={objectKey} context={context} />;

    case 'custom':
      return <CustomRenderer layout={layout} objectKey={objectKey} context={context} />;

    default:
      return (
        <Alert
          message="当前 Tab 布局尚未接入运行时渲染"
          description={`请在编辑器中调整为已支持布局，当前类型: ${(layout as any).type}`}
          type="error"
          showIcon
          style={{ margin: '20px' }}
        />
      );
  }
}

function normalizeLegacyLayout(tab: TabConfig): any {
  const layout: any = tab?.layout || {};
  if (layout?.type !== 'single') {
    return layout;
  }

  const functionId = layout?.component?.functionId || tab?.functions?.[0] || '';
  // Legacy "single" layout is adapted to "form" so old configs remain renderable.
  return {
    type: 'form',
    submitFunction: functionId,
    fields: Array.isArray(layout?.fields) ? layout.fields : [],
  };
}
