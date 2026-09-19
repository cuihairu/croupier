/**
 * SchemaFormRenderer 渲染错误边界（2.2）。
 *
 * 既有降级链路只覆盖「schema 缺失」（formState.status:'unavailable'）；
 * 「schema 存在但畸形」（$ref 指向缺失 definitions、items 非 object 等）时
 * rjsf 内部抛错会击穿整页白屏。本边界把抛错降级为结构化告警，并提供重试；
 * spec 内容变化（调用方以 key 区分）时自动复位。
 */
import React from 'react';
import { Alert, Button } from 'antd';
import { getIntl } from '@umijs/max';

interface SchemaFormErrorBoundaryProps {
  children: React.ReactNode;
}

interface SchemaFormErrorBoundaryState {
  error: Error | null;
}

function text(id: string, defaultMessage: string, values?: Record<string, string>): string {
  try {
    return getIntl().formatMessage({ id, defaultMessage }, values);
  } catch {
    return defaultMessage;
  }
}

export default class SchemaFormErrorBoundary extends React.Component<
  SchemaFormErrorBoundaryProps,
  SchemaFormErrorBoundaryState
> {
  state: SchemaFormErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): SchemaFormErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error): void {
    // eslint-disable-next-line no-console
    console.error('[SchemaFormRenderer] form render crashed', error);
  }

  private handleRetry = (): void => {
    this.setState({ error: null });
  };

  render(): React.ReactNode {
    const { error } = this.state;
    if (!error) return this.props.children;
    return (
      <Alert
        type="error"
        showIcon
        message={text('component.schemaFormRenderer.renderCrash.title', '表单渲染失败')}
        description={
          <div>
            {text(
              'component.schemaFormRenderer.renderCrash.detail',
              '表单 schema 或展示配置存在异常（{message}）。请修正 schema，或改用 JSON 模式提交参数。',
              { message: error.message || String(error) },
            )}
            <div style={{ marginTop: 8 }}>
              <Button size="small" onClick={this.handleRetry}>
                {text('component.schemaFormRenderer.renderCrash.retry', '重试渲染')}
              </Button>
            </div>
          </div>
        }
      />
    );
  }
}
