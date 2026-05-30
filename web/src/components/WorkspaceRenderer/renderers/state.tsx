import React from 'react';
import { Alert, Empty, Spin } from 'antd';

export type RendererRuntimeMode = 'console' | 'workspace';

export function isTemplatePreviewContext(context?: Record<string, any>) {
  return Boolean(context?.templatePreview);
}

export function getRendererRuntimeMode(context?: Record<string, any>): RendererRuntimeMode | null {
  if (context?.runtimeMode === 'console') return 'console';
  if (context?.runtimeMode === 'workspace') return 'workspace';
  return null;
}

export function RendererLoading({ tip = '加载中...' }: { tip?: string }) {
  return (
    <div style={{ textAlign: 'center', padding: '100px 0' }}>
      <Spin size="large" tip={tip} />
    </div>
  );
}

export function RendererEmpty({ description = '暂无数据' }: { description?: string }) {
  return <Empty description={description} style={{ marginTop: 48, marginBottom: 24 }} />;
}

export function RendererNotice({
  type = 'info',
  title,
  description,
}: {
  type?: 'info' | 'warning' | 'success' | 'error';
  title: string;
  description: string;
}) {
  return (
    <Alert
      type={type}
      showIcon
      message={title}
      description={description}
      style={{ marginBottom: 16 }}
    />
  );
}

export function RendererModeNotice({
  context,
  sampleTitle = '编辑器预览',
  sampleDescription = '当前区域正在使用示例数据或示例骨架，只用于预览结构，不代表正式运行结果。',
}: {
  context?: Record<string, any>;
  sampleTitle?: string;
  sampleDescription?: string;
}) {
  if (isTemplatePreviewContext(context)) {
    return (
      <RendererNotice type="info" title={sampleTitle} description={sampleDescription} />
    );
  }

  if (getRendererRuntimeMode(context) === 'console') {
    return (
      <RendererNotice
        type="warning"
        title="正式运行态"
        description="控制台展示的是正式运行结果。没有真实函数、真实绑定或真实数据时，这里不会自动补示例内容。"
      />
    );
  }

  return null;
}

export function RendererError({
  message = '加载失败',
  description,
}: {
  message?: string;
  description: string;
}) {
  return (
    <Alert
      type="error"
      showIcon
      message={message}
      description={description}
      style={{ marginBottom: 16 }}
    />
  );
}
