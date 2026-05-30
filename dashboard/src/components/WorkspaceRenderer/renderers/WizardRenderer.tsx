import React, { useMemo, useState } from 'react';
import { Alert, Button, Card, Space, Steps } from 'antd';
import type { RendererProps } from './types';
import ListRenderer from './ListRenderer';
import FormRenderer from './FormRenderer';
import DetailRenderer from './DetailRenderer';
import FormDetailRenderer from './FormDetailRenderer';
import { buildPreviewDetailConfig, buildPreviewFormConfig } from './mockData';
import { RendererEmpty, RendererModeNotice, isTemplatePreviewContext } from './state';

type WizardStepConfig = {
  key: string;
  title: string;
  description?: string;
  component?: {
    type: 'list' | 'form' | 'detail' | 'form-detail';
    config: Record<string, any>;
  };
};

type WizardLayout = {
  type: 'wizard';
  steps: WizardStepConfig[];
};

export default function WizardRenderer({
  layout,
  objectKey,
  context,
}: RendererProps<WizardLayout>) {
  const isTemplatePreview = isTemplatePreviewContext(context);
  const steps =
    Array.isArray(layout?.steps) && layout.steps.length > 0
      ? layout.steps
      : isTemplatePreview
      ? [
          {
            key: 'step1',
            title: '步骤一',
            component: { type: 'form', config: buildPreviewFormConfig('') },
          },
          {
            key: 'step2',
            title: '步骤二',
            component: { type: 'detail', config: buildPreviewDetailConfig('') },
          },
        ]
      : [];
  const [current, setCurrent] = useState(0);

  const safeCurrent = useMemo(() => {
    if (steps.length === 0) return 0;
    if (current < 0) return 0;
    if (current >= steps.length) return steps.length - 1;
    return current;
  }, [current, steps.length]);

  if (steps.length === 0) {
    return (
      <>
        <RendererModeNotice
          context={context}
          sampleDescription="当前 wizard 布局正在用示例步骤帮助你预览流程，正式运行仍需要真实 steps 配置。"
        />
        <RendererEmpty
          description={
            isTemplatePreview
              ? '当前 wizard 预览还没有正式步骤配置'
              : '当前 wizard 布局缺少 steps 配置，正式运行无法进入流程'
          }
        />
      </>
    );
  }

  const currentStep = steps[safeCurrent];

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      <RendererModeNotice
        context={context}
        sampleDescription="当前 wizard 布局正在用示例步骤帮助你预览流程，正式运行仍需要真实 steps 配置。"
      />
      <Steps
        current={safeCurrent}
        items={steps.map((s) => ({
          title: s.title,
          description: s.description,
        }))}
      />

      <Card>{renderStepContent(currentStep, objectKey, context)}</Card>

      <Space>
        <Button disabled={safeCurrent <= 0} onClick={() => setCurrent((x) => x - 1)}>
          上一步
        </Button>
        <Button
          type="primary"
          disabled={safeCurrent >= steps.length - 1}
          onClick={() => setCurrent((x) => x + 1)}
        >
          下一步
        </Button>
      </Space>
    </Space>
  );
}

function renderStepContent(
  step: WizardStepConfig,
  objectKey: string,
  context?: Record<string, any>,
) {
  const comp = step?.component;
  if (!comp) {
    return <Alert type="info" showIcon message={`步骤 ${step?.title || '-'} 未配置 component`} />;
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
        <Alert type="error" showIcon message={`不支持的 wizard 组件类型: ${(comp as any).type}`} />
      );
  }
}
