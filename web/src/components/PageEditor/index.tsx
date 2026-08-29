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
import { localizedText } from '@/utils/localizedText';
import { Card, Empty, Form, Input, InputNumber, Space, Tag, Typography } from 'antd';
import type { PageSpec } from '@/types/dashboard';
import ResourcePageEditor from './ResourcePageEditor';
import OperationPageEditor from './OperationPageEditor';
import TaskPageEditor from './TaskPageEditor';
import ReportPageEditor from './ReportPageEditor';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';

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

      case 'composite':
        return (
          <Card size="small" title="组合页（自由函数区块）">
            <Text type="secondary">
              组合页由生成器按资源契约自动维护（每资源一个 tab 视图）；如需调整资源集合，
              请在提案收件箱删除后重新创建，或等待契约变更触发的提案更新。
            </Text>
            {(value.composite?.sections || []).map((sec) => (
              <div key={sec.key} style={{ marginTop: 8 }}>
                <Tag color="cyan">{sec.bindingId}</Tag>
                <Text type="secondary">
                  {localizedText(sec.title, 'zh-CN', sec.key)} · 视图 {sec.view}
                  {sec.refreshOn?.length ? ` · 联动 ${sec.refreshOn.join(',')}` : ''}
                </Text>
              </div>
            ))}
          </Card>
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
          <Form.Item label="页面标题（多语言）" required>
            <LocalizedTextEditor
              value={value.title}
              onChange={(title) => onChange({ ...value, title })}
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
          <Form.Item label="分类标题（多语言）" required>
            <LocalizedTextEditor
              value={category.labels}
              onChange={(labels) => onChange({ ...value, category: { ...category, labels } })}
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
