/**
 * PageEditor - 语义化页面编辑器
 *
 * 根据页面类型自动选择对应的编辑器面板：
 * - ResourcePage: 资源页面编辑器
 * - OperationPage: 操作页面编辑器
 * - TaskPage: 任务页面编辑器
 * - ReportPage: 报表页面编辑器
 */

import React from 'react';
import { Card, Empty, Form, Input, InputNumber, Space, Typography } from 'antd';
import type { PageSpec } from '@/types/dashboard';
import ResourcePageEditor from './ResourcePageEditor';
import OperationPageEditor from './OperationPageEditor';
import TaskPageEditor from './TaskPageEditor';
import ReportPageEditor from './ReportPageEditor';

const { Text } = Typography;

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface PageEditorProps {
  /** 当前 PageSpec */
  value: PageSpec;
  /** 值变化回调 */
  onChange: (value: PageSpec) => void;
  /** 是否只读 */
  readonly?: boolean;
}

// ---------------------------------------------------------------------------
// PageEditor Component
// ---------------------------------------------------------------------------

export default function PageEditor({ value, onChange, readonly = false }: PageEditorProps) {
  const category = value.category || { key: '', labels: {} };
  const renderBody = () => {
    switch (value.type) {
      case 'resource':
        return value.resource ? (
          <ResourcePageEditor
            value={value.resource}
            onChange={(resource) => onChange({ ...value, resource })}
            readonly={readonly}
          />
        ) : (
          <Empty description="无资源页面配置" />
        );

      case 'operation':
        return value.operation ? (
          <OperationPageEditor
            value={value.operation}
            onChange={(operation) => onChange({ ...value, operation })}
            readonly={readonly}
          />
        ) : (
          <Empty description="无操作页面配置" />
        );

      case 'task':
        return value.task ? (
          <TaskPageEditor
            value={value.task}
            onChange={(task) => onChange({ ...value, task })}
            readonly={readonly}
          />
        ) : (
          <Empty description="无任务页面配置" />
        );

      case 'report':
        return value.report ? (
          <ReportPageEditor
            value={value.report}
            onChange={(report) => onChange({ ...value, report })}
            readonly={readonly}
          />
        ) : (
          <Empty description="无报表页面配置" />
        );

      default:
        return <Empty description="未知页面类型" />;
    }
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      <Card title="页面与菜单信息">
        <Text type="secondary">
          这些字段会进入 PublishedPageSpec，并作为运行控制台动态菜单的唯一文本来源；函数注册和静态
          locale 不提供页面显示文案。
        </Text>
        <Form layout="vertical" disabled={readonly} style={{ marginTop: 16 }}>
          <Form.Item label="页面标题 zh-CN" required>
            <Input
              value={value.title?.['zh-CN'] || ''}
              onChange={(event) =>
                onChange({
                  ...value,
                  title: { ...value.title, 'zh-CN': event.target.value },
                })
              }
            />
          </Form.Item>
          <Form.Item label="页面标题 en-US">
            <Input
              value={value.title?.['en-US'] || ''}
              onChange={(event) =>
                onChange({
                  ...value,
                  title: { ...value.title, 'en-US': event.target.value },
                })
              }
            />
          </Form.Item>
          <Form.Item label="分类 key" required>
            <Input
              value={category.key}
              onChange={(event) =>
                onChange({
                  ...value,
                  category: { ...category, key: event.target.value },
                })
              }
            />
          </Form.Item>
          <Form.Item label="分类标题 zh-CN" required>
            <Input
              value={category.labels?.['zh-CN'] || ''}
              onChange={(event) =>
                onChange({
                  ...value,
                  category: {
                    ...category,
                    labels: { ...category.labels, 'zh-CN': event.target.value },
                  },
                })
              }
            />
          </Form.Item>
          <Form.Item label="分类标题 en-US">
            <Input
              value={category.labels?.['en-US'] || ''}
              onChange={(event) =>
                onChange({
                  ...value,
                  category: {
                    ...category,
                    labels: { ...category.labels, 'en-US': event.target.value },
                  },
                })
              }
            />
          </Form.Item>
          <Form.Item label="页面排序">
            <InputNumber
              value={value.order}
              onChange={(order) => onChange({ ...value, order: order ?? undefined })}
            />
          </Form.Item>
          <Form.Item label="图标">
            <Input
              value={value.icon || ''}
              onChange={(event) => onChange({ ...value, icon: event.target.value })}
              placeholder="可选，仅作为菜单图标 hint"
            />
          </Form.Item>
        </Form>
      </Card>
      <Card title="页面结构">{renderBody()}</Card>
    </Space>
  );
}

// 导出子编辑器
export { default as ResourcePageEditor } from './ResourcePageEditor';
export { default as OperationPageEditor } from './OperationPageEditor';
export { default as TaskPageEditor } from './TaskPageEditor';
export { default as ReportPageEditor } from './ReportPageEditor';
