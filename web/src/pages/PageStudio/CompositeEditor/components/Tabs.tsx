import { useState } from 'react';
import { Space, Tabs as AntTabs, Tag, Typography } from 'antd';
import { FormattedMessage, getIntl } from '@umijs/max';
import { registerComponent } from '../registry';
import { getComponent } from '../registry';
import type { ComponentDef } from '../registry';
import type { PageNode } from '../model';
import { spanSchema } from './shared';

const { Text } = Typography;

// 展示 name/propSchema 标题经 getIntl 解析（模块级求值，先例 Modal.tsx）；
// scaffold 默认值为编辑树数据，保持中文不动。
const intl = getIntl();

/** 未命名页签的默认标签（页签 N）。 */
const pageLabel = (page: PageNode, idx: number): string =>
  typeof page.props.title === 'string' && page.props.title.trim()
    ? page.props.title
    : intl.formatMessage(
        {
          id: 'pages.pageStudio.editor.component.tabs.tabFallback',
          defaultMessage: '页签 {n}',
        },
        { n: idx + 1 },
      );

/**
 * 页签容器（V2 组合容器）：children=container（每 container 一页，
 * props.title=页签标签）。编译产物平铺为 display='tab' + group（同组渲染
 * 进同一 Tabs）+ tab（页签标签）的 sections；发布端按 group/tab 聚合还原。
 */
export const tabs: ComponentDef = {
  type: 'tabs',
  name: intl.formatMessage({
    id: 'pages.pageStudio.editor.component.tabs.name',
    defaultMessage: '页签容器',
  }),
  icon: (
    <Tag color="cyan">
      <FormattedMessage
        id="pages.pageStudio.editor.component.tabs.name"
        defaultMessage="页签容器"
      />
    </Tag>
  ),
  category: 'basic',
  allowedChildren: ['container'],
  propSchema: () => ({
    type: 'object',
    properties: {
      sectionKey: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.tabs.prop.sectionKey',
          defaultMessage: '页签组名（可选，缺省自动）',
        }),
      },
      span: { ...spanSchema(), default: 24 },
    },
  }),
  scaffold: () => ({ span: 24 }),
  Preview: ({ node }) => {
    // activeTab 是编辑期 UI 态：本地 state 只为触发重渲染，真值轻量写在
    // node.props.activeTab（不进 undo/编译产物；drop 落 tabs 时读它定位激活页）。
    const [, bump] = useState(0);
    const pages = (node.children ?? []).filter((c) => c.type === 'container');
    const activeProp = typeof node.props.activeTab === 'string' ? node.props.activeTab : '';
    const activeKey = pages.some((p) => p.id === activeProp) ? activeProp : pages[0]?.id;
    if (pages.length === 0) {
      return (
        <Text type="secondary" style={{ fontSize: 11 }}>
          {intl.formatMessage({
            id: 'pages.pageStudio.editor.component.tabs.previewHint',
            defaultMessage: '空页签容器——拖入表格/字段卡/按钮/文本（自动进当前页签）',
          })}
        </Text>
      );
    }
    return (
      <AntTabs
        size="small"
        activeKey={activeKey}
        onChange={(k) => {
          node.props.activeTab = k;
          bump((n) => n + 1);
        }}
        items={pages.map((page, i) => ({
          key: page.id,
          label: pageLabel(page, i),
          children:
            page.children && page.children.length > 0 ? (
              <Space orientation="vertical" size={6} style={{ width: '100%' }}>
                {page.children.map((c) => {
                  const def = getComponent(c.type);
                  return def ? (
                    <div key={c.id}>
                      <def.Preview node={c} />
                    </div>
                  ) : null;
                })}
              </Space>
            ) : (
              <Text type="secondary" style={{ fontSize: 11 }}>
                {intl.formatMessage({
                  id: 'pages.pageStudio.editor.component.tabs.emptyTab',
                  defaultMessage: '空页签——拖入组件',
                })}
              </Text>
            ),
        }))}
      />
    );
  },
};
