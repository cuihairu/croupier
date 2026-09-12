import React from 'react';
import { Input, Space, Typography } from 'antd';
import { getIntl, useIntl } from '@umijs/max';
import { registerComponent } from '../registry';
import type { ComponentDef } from '../registry';
import type { PageNode } from '../model';
import { spanSchema, visibleWhenSchema } from './shared';

const { Text } = Typography;

// 常量表单（staticForm）：字段由编辑器设计期 JSON 定义（常量下拉等），
// 不绑定函数；值并入页面状态供 refreshOn/动作链消费。
// DEFAULT_STATIC_SCHEMA 与 scaffold/default 值为落库 payload（PageSpec 契约），保持不动。
const DEFAULT_STATIC_SCHEMA = JSON.stringify(
  {
    type: 'object',
    properties: {
      env: { type: 'string', title: '环境', enum: ['prod', 'stage', 'dev'] },
    },
  },
  null,
  2,
);

// 展示 name/propSchema 标题经 getIntl 解析（模块级求值，先例 services/api/bugs.ts）。
const intl = getIntl();

export const staticFormDef: ComponentDef = {
  type: 'staticForm',
  name: intl.formatMessage({
    id: 'pages.pageStudio.editor.component.staticForm.name',
    defaultMessage: '常量表单',
  }),
  icon: '📋',
  category: 'basic',
  propSchema: () => ({
    type: 'object',
    properties: {
      title: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.staticForm.prop.title',
          defaultMessage: '标题',
        }),
        default: '常量表单',
      },
      span: { ...spanSchema(), default: 12 },
      staticSchema: {
        type: 'string',
        format: 'staticSchema',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.staticForm.prop.staticSchema',
          defaultMessage: '字段定义（JSON Schema）',
        }),
        default: DEFAULT_STATIC_SCHEMA,
      },
      visibleWhen: visibleWhenSchema(),
    },
  }),
  scaffold: () => ({ title: '常量表单', span: 12, staticSchema: DEFAULT_STATIC_SCHEMA }),
  Preview: ({ node }) => <StaticFormPreview node={node} />,
};

/** 常量表单画布预览：按 schema 渲染枚举→下拉、布尔→开关、其余→输入框。 */
const StaticFormPreview: React.FC<{ node: PageNode }> = ({ node }) => {
  const intl = useIntl();
  const raw = node.props.staticSchema;
  let schema: Record<string, unknown> = {};
  try {
    schema =
      typeof raw === 'string'
        ? (JSON.parse(raw) as Record<string, unknown>)
        : (raw as Record<string, unknown>);
  } catch {
    return (
      <Text type="warning">
        {intl.formatMessage({
          id: 'pages.pageStudio.editor.component.staticForm.preview.invalid',
          defaultMessage: '字段定义 JSON 无效',
        })}
      </Text>
    );
  }
  const properties = (schema.properties ?? {}) as Record<string, Record<string, unknown>>;
  const keys = Object.keys(properties);
  if (keys.length === 0)
    return (
      <Text type="secondary">
        {intl.formatMessage({
          id: 'pages.pageStudio.editor.component.staticForm.preview.empty',
          defaultMessage: '暂无字段——在属性面板定义 JSON',
        })}
      </Text>
    );
  return (
    <Space orientation="vertical" size={8} style={{ width: '100%' }}>
      {keys.map((key) => {
        const p = properties[key];
        const label = String(p.title ?? key);
        const options = Array.isArray(p.enum) ? (p.enum as unknown[]).map(String) : [];
        if (options.length > 0) {
          return (
            <label key={key} style={{ display: 'block', fontSize: 12 }}>
              {label}
              <select
                style={{
                  width: '100%',
                  height: 28,
                  marginTop: 2,
                  borderRadius: 6,
                  border: '1px solid #d9d9d9',
                }}
              >
                {options.map((o) => (
                  <option key={o}>{o}</option>
                ))}
              </select>
            </label>
          );
        }
        if (p.type === 'boolean') {
          return (
            <label key={key} style={{ display: 'block', fontSize: 12 }}>
              {label} <input type="checkbox" />
            </label>
          );
        }
        return (
          <label key={key} style={{ display: 'block', fontSize: 12 }}>
            {label}
            <input
              style={{
                width: '100%',
                height: 28,
                marginTop: 2,
                borderRadius: 6,
                border: '1px solid #d9d9d9',
              }}
              placeholder={String(p.type ?? 'string')}
            />
          </label>
        );
      })}
    </Space>
  );
};
