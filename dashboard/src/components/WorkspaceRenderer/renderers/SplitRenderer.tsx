import React from 'react';
import { Alert, Col, Row } from 'antd';
import type { RendererProps } from './types';
import ListRenderer from './ListRenderer';
import FormRenderer from './FormRenderer';
import DetailRenderer from './DetailRenderer';
import FormDetailRenderer from './FormDetailRenderer';
import { buildPreviewDetailConfig, buildPreviewListConfig } from './mockData';
import { RendererEmpty, RendererModeNotice, isTemplatePreviewContext } from './state';

type SplitPanelConfig = {
  key: string;
  title?: string;
  span?: number;
  component?: {
    type: 'list' | 'form' | 'detail' | 'form-detail';
    config: Record<string, any>;
  };
};

type SplitLayout = {
  type: 'split';
  panels: SplitPanelConfig[];
};

export default function SplitRenderer({ layout, objectKey, context }: RendererProps<SplitLayout>) {
  const isTemplatePreview = isTemplatePreviewContext(context);
  const panels =
    Array.isArray(layout?.panels) && layout.panels.length > 0
      ? layout.panels
      : isTemplatePreview
      ? [
          {
            key: 'left',
            title: '主列表',
            span: 12,
            component: { type: 'list', config: buildPreviewListConfig('') },
          },
          {
            key: 'right',
            title: '详情',
            span: 12,
            component: { type: 'detail', config: buildPreviewDetailConfig('') },
          },
        ]
      : [];
  if (panels.length === 0) {
    return (
      <>
        <RendererModeNotice
          context={context}
          sampleDescription="当前 split 布局正在用示例主从面板帮助你预览结构，正式运行仍需要真实 panels 配置。"
        />
        <RendererEmpty
          description={
            isTemplatePreview
              ? '当前 split 预览还没有正式 panels 配置'
              : '当前 split 布局缺少 panels 配置，正式运行无法展示内容'
          }
        />
      </>
    );
  }

  return (
    <>
      <RendererModeNotice
        context={context}
        sampleDescription="当前 split 布局正在用示例主从面板帮助你预览结构，正式运行仍需要真实 panels 配置。"
      />
      <Row gutter={16}>
        {panels.map((panel) => (
          <Col key={panel.key} span={panel.span || Math.floor(24 / panels.length) || 12}>
            {renderPanel(panel, objectKey, context)}
          </Col>
        ))}
      </Row>
    </>
  );
}

function renderPanel(panel: SplitPanelConfig, objectKey: string, context?: Record<string, any>) {
  const comp = panel.component;
  if (!comp) {
    return <Alert type="info" message={`面板 ${panel.key} 未配置 component`} showIcon />;
  }

  switch (comp.type) {
    case 'list':
      return (
        <ListRenderer
          layout={{ type: 'list', ...(comp.config || {}) } as any}
          objectKey={objectKey}
          context={context}
        />
      );
    case 'form':
      return (
        <FormRenderer
          layout={{ type: 'form', ...(comp.config || {}) } as any}
          objectKey={objectKey}
          context={context}
        />
      );
    case 'detail':
      return (
        <DetailRenderer
          layout={{ type: 'detail', ...(comp.config || {}) } as any}
          objectKey={objectKey}
          context={context}
        />
      );
    case 'form-detail':
      return (
        <FormDetailRenderer
          layout={{ type: 'form-detail', ...(comp.config || {}) } as any}
          objectKey={objectKey}
          context={context}
        />
      );
    default:
      return (
        <Alert type="error" message={`不支持的 split 组件类型: ${(comp as any).type}`} showIcon />
      );
  }
}
