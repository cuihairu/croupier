/**
 * Tabs 布局组件
 *
 * 使用 Ant Design Tabs 组件渲染多标签页布局。
 *
 * @module components/WorkspaceRenderer/TabsLayout
 */

import React, { useState, useMemo } from 'react';
import { Tabs } from 'antd';
import type { TabsProps } from 'antd';
import type { WorkspaceConfig, TabConfig } from '@/types/workspace';
import TabContentRenderer from './TabContentRenderer';
import * as Icons from '@ant-design/icons';

export interface TabsLayoutProps {
  /** Workspace 配置 */
  config: WorkspaceConfig;

  /** 额外的上下文数据 */
  context?: Record<string, any>;
}

/**
 * Tabs 布局组件
 *
 * 根据配置渲染多标签页界面。
 */
export default function TabsLayout({ config, context }: TabsLayoutProps) {
  const { layout } = config;
  const tabs = layout.tabs || [];

  // 默认激活的 Tab
  const defaultActiveKey = useMemo(() => {
    const defaultTab = tabs.find((tab) => tab.defaultActive);
    return defaultTab?.key || tabs[0]?.key;
  }, [tabs]);

  const [activeKey, setActiveKey] = useState<string>(defaultActiveKey);
  React.useEffect(() => {
    setActiveKey(defaultActiveKey);
  }, [defaultActiveKey]);

  // 生成 Tabs 配置
  const tabItems: TabsProps['items'] = useMemo(() => {
    return tabs.map((tab) => ({
      key: tab.key,
      label: renderTabLabel(tab),
      children: <TabContentRenderer tab={tab} objectKey={config.objectKey} context={context} />,
    }));
  }, [tabs, config.objectKey, context]);

  // 处理 Tab 切换
  const handleTabChange = (key: string) => {
    setActiveKey(key);
  };

  if (tabs.length === 0) {
    return <div style={{ padding: '20px', textAlign: 'center', color: '#999' }}>暂无标签页</div>;
  }

  return (
    <div className="workspace-tabs-layout">
      <Tabs
        activeKey={activeKey}
        onChange={handleTabChange}
        items={tabItems}
        type="card"
        size="large"
      />
    </div>
  );
}

/**
 * 渲染 Tab 标签
 */
function renderTabLabel(tab: TabConfig): React.ReactNode {
  const IconComponent = tab.icon ? getIcon(tab.icon) : null;

  return (
    <span>
      {IconComponent && <IconComponent style={{ marginRight: 8 }} />}
      {tab.title}
    </span>
  );
}

/**
 * 根据图标名称获取图标组件
 */
function getIcon(iconName: string): React.ComponentType<any> | null {
  const IconComponent = (Icons as any)[iconName];
  return IconComponent || null;
}
