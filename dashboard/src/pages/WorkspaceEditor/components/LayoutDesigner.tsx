/**
 * 布局设计器组件
 *
 * 可视化设计 Workspace 布局。
 *
 * @module pages/WorkspaceEditor/components/LayoutDesigner
 */

import React from 'react';
import { Card, Tabs, Button, Modal, Form, Input, Alert, Space, Tag, Typography } from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  UpOutlined,
  DownOutlined,
  HolderOutlined,
} from '@ant-design/icons';
import type { WorkspaceConfig, TabConfig } from '@/types/workspace';
import type { FunctionDescriptor } from '@/services/api/functions';
import { descriptorToLayout } from '../utils/schemaToLayout';
import TabEditor from './TabEditor';
import IconPicker from './IconPicker';
import EditorEmptyState from './EditorEmptyState';

export interface LayoutDesignerProps {
  config: WorkspaceConfig | null;
  onChange: (config: WorkspaceConfig) => void;
  descriptors?: FunctionDescriptor[];
  activeTabKey?: string;
  onActiveTabChange?: (tabKey?: string) => void;
}

/**
 * 布局设计器组件
 */
export default function LayoutDesigner({
  config,
  onChange,
  descriptors = [],
  activeTabKey: controlledActiveTabKey,
  onActiveTabChange,
}: LayoutDesignerProps) {
  const [activeKey, setActiveKey] = React.useState<string>();
  const [showAddModal, setShowAddModal] = React.useState(false);
  const [form] = Form.useForm();
  const mergedActiveKey = controlledActiveTabKey ?? activeKey;

  if (!config) {
    return (
      <Card style={{ height: '100%', minHeight: 560 }}>
        <EditorEmptyState
          title="请先选择对象"
          description="选定对象后，这里会进入 Tab 布局设计流程。"
        />
      </Card>
    );
  }

  const safeLayout = config.layout || { type: 'tabs', tabs: [] };
  const safeConfig = {
    ...config,
    layout: safeLayout,
  };
  const isTabsLayout = safeLayout.type === 'tabs';
  const tabs = isTabsLayout ? safeLayout.tabs || [] : [];

  React.useEffect(() => {
    if (!tabs.length) {
      if (controlledActiveTabKey === undefined) setActiveKey(undefined);
      onActiveTabChange?.(undefined);
      return;
    }
    if (mergedActiveKey && tabs.some((tab) => tab.key === mergedActiveKey)) {
      return;
    }
    const defaultTab = tabs.find((tab) => tab.defaultActive) || tabs[0];
    if (controlledActiveTabKey === undefined) {
      setActiveKey(defaultTab?.key);
    }
    onActiveTabChange?.(defaultTab?.key);
  }, [controlledActiveTabKey, mergedActiveKey, onActiveTabChange, tabs]);

  // 添加 Tab
  const handleAddTab = () => {
    setShowAddModal(true);
  };

  // 确认添加 Tab
  const handleConfirmAdd = async () => {
    try {
      const values = await form.validateFields();
      const newTab: TabConfig = {
        key: `tab_${Date.now()}`,
        title: values.title,
        icon: values.icon,
        functions: [],
        defaultActive: tabs.length === 0,
        layout: {
          type: 'list',
          listFunction: '',
          columns: [],
        },
      };

      onChange({
        ...safeConfig,
        layout: {
          ...safeLayout,
          tabs: [...tabs, newTab],
        },
      });

      if (controlledActiveTabKey === undefined) {
        setActiveKey(newTab.key);
      }
      onActiveTabChange?.(newTab.key);
      setShowAddModal(false);
      form.resetFields();
    } catch (error) {
      // 验证失败
    }
  };

  const handleQuickStartTab = () => {
    const firstDescriptor = descriptors[0];
    if (!firstDescriptor) {
      handleAddTab();
      return;
    }

    const newTab: TabConfig = {
      key: `tab_${Date.now()}`,
      title: firstDescriptor.displayName?.zh || firstDescriptor.displayName?.en || '基础页',
      icon: 'appstore',
      functions: [firstDescriptor.id],
      defaultActive: true,
      layout: descriptorToLayout(firstDescriptor),
    };

    onChange({
      ...safeConfig,
      layout: {
        ...safeLayout,
        tabs: [newTab],
      },
    });
    if (controlledActiveTabKey === undefined) {
      setActiveKey(newTab.key);
    }
    onActiveTabChange?.(newTab.key);
  };

  // 删除 Tab
  const handleDeleteTab = (tabKey: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除这个标签页吗？',
      onOk: () => {
        onChange({
          ...safeConfig,
          layout: {
            ...safeLayout,
            tabs: (() => {
              const remainingTabs = tabs.filter((t) => t.key !== tabKey);
              if (remainingTabs.length === 0) return [];
              if (remainingTabs.some((tab) => tab.defaultActive)) return remainingTabs;
              return remainingTabs.map((tab, index) => ({ ...tab, defaultActive: index === 0 }));
            })(),
          },
        });

        // 如果删除的是当前激活的 Tab，切换到第一个
        if (mergedActiveKey === tabKey && tabs.length > 1) {
          const remainingTabs = tabs.filter((t) => t.key !== tabKey);
          if (controlledActiveTabKey === undefined) {
            setActiveKey(remainingTabs[0]?.key);
          }
          onActiveTabChange?.(remainingTabs[0]?.key);
        }
      },
    });
  };

  // 更新 Tab
  const handleUpdateTab = (tabKey: string, updatedTab: TabConfig) => {
    const nextTabs = tabs.map((t) => (t.key === tabKey ? updatedTab : t));
    const normalizedTabs = updatedTab.defaultActive
      ? nextTabs.map((tab) => ({ ...tab, defaultActive: tab.key === tabKey }))
      : nextTabs;

    onChange({
      ...safeConfig,
      layout: {
        ...safeLayout,
        tabs: normalizedTabs,
      },
    });
  };

  const moveTab = (tabKey: string, direction: 'up' | 'down') => {
    const index = tabs.findIndex((tab) => tab.key === tabKey);
    if (index < 0) return;
    const target = direction === 'up' ? index - 1 : index + 1;
    if (target < 0 || target >= tabs.length) return;

    const reordered = [...tabs];
    const [current] = reordered.splice(index, 1);
    reordered.splice(target, 0, current);

    onChange({
      ...safeConfig,
      layout: {
        ...safeLayout,
        tabs: reordered,
      },
    });
  };

  const activeTab =
    tabs.find((tab) => tab.key === mergedActiveKey) ||
    tabs.find((tab) => tab.defaultActive) ||
    tabs[0];

  const summarizeTab = React.useCallback(
    (tab: TabConfig) => {
      const layout = (tab.layout || {}) as any;
      const primaryFunctionId = tab.functions?.[0] || '';
      const primaryDescriptor = descriptors.find((d) => d.id === primaryFunctionId);
      const primaryFunctionLabel =
        primaryDescriptor?.displayName?.zh ||
        primaryDescriptor?.displayName?.en ||
        primaryFunctionId ||
        '未绑定';
      const hasConfiguredStructure =
        Boolean(
          layout?.listFunction ||
            layout?.submitFunction ||
            layout?.detailFunction ||
            layout?.queryFunction ||
            layout?.dataFunction,
        ) ||
        (Array.isArray(layout?.columns) && layout.columns.length > 0) ||
        (Array.isArray(layout?.fields) && layout.fields.length > 0) ||
        (Array.isArray(layout?.queryFields) && layout.queryFields.length > 0) ||
        (Array.isArray(layout?.sections) && layout.sections.length > 0);
      const status =
        (tab.functions?.length || 0) === 0
          ? {
              color: 'warning' as const,
              text: '待绑函数',
              hint: '先绑定主函数',
            }
          : !hasConfiguredStructure
          ? {
              color: 'processing' as const,
              text: '待生成骨架',
              hint: '先起基础页面',
            }
          : {
              color: 'success' as const,
              text: '可继续细化',
              hint: '可补字段或预览',
            };

      return {
        key: tab.key,
        title: tab.title || tab.key,
        layoutType: tab.layout?.type || 'list',
        functionCount: tab.functions?.length || 0,
        isDefault: Boolean(tab.defaultActive),
        hasConfiguredStructure,
        primaryFunctionLabel,
        status,
        isReady: (tab.functions?.length || 0) > 0 && hasConfiguredStructure,
        nextAction:
          (tab.functions?.length || 0) === 0
            ? '先绑定主函数'
            : !hasConfiguredStructure
            ? '先生成基础页面骨架'
            : '可以继续补字段和预览',
      };
    },
    [descriptors],
  );

  const activeTabSummary = activeTab ? summarizeTab(activeTab) : null;
  const tabSummaries = tabs.map((tab) => summarizeTab(tab));
  const readyCount = tabSummaries.filter((item) => item.isReady).length;
  const firstPendingTab = tabSummaries.find((item) => !item.isReady);

  // 生成 Tabs 配置
  const tabItems = tabs.map((tab, index) => ({
    key: tab.key,
    label: (() => {
      const summary = summarizeTab(tab);
      return (
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            gap: 12,
            minWidth: 220,
          }}
        >
          <Space direction="vertical" size={3} style={{ minWidth: 0, lineHeight: 1.2 }}>
            <Space size={6} wrap>
              <Typography.Text strong ellipsis style={{ maxWidth: 140 }}>
                {summary.title}
              </Typography.Text>
              <Tag color={summary.status.color} bordered={false} style={{ margin: 0 }}>
                {summary.status.text}
              </Tag>
              {summary.isDefault ? (
                <Tag color="success" bordered={false} style={{ margin: 0 }}>
                  默认
                </Tag>
              ) : null}
            </Space>
            <Typography.Text type="secondary" style={{ fontSize: 11 }}>
              {`${summary.layoutType} · ${summary.primaryFunctionLabel}`}
            </Typography.Text>
          </Space>
          <Space size={2} align="center">
            <HolderOutlined style={{ color: '#bfbfbf', fontSize: 12 }} />
            <Button
              type="text"
              size="small"
              icon={<UpOutlined />}
              disabled={index === 0}
              onClick={(e) => {
                e.stopPropagation();
                moveTab(tab.key, 'up');
              }}
            />
            <Button
              type="text"
              size="small"
              icon={<DownOutlined />}
              disabled={index >= tabs.length - 1}
              onClick={(e) => {
                e.stopPropagation();
                moveTab(tab.key, 'down');
              }}
            />
            <Button
              type="text"
              size="small"
              danger
              icon={<DeleteOutlined />}
              onClick={(e) => {
                e.stopPropagation();
                handleDeleteTab(tab.key);
              }}
            />
          </Space>
        </div>
      );
    })(),
    children: (
      <TabEditor
        tab={tab}
        onChange={(updatedTab) => handleUpdateTab(tab.key, updatedTab)}
        descriptors={descriptors}
      />
    ),
  }));

  return (
    <>
      <Card
        title="页面编排"
        style={{ height: '100%', display: 'flex', flexDirection: 'column' }}
        styles={{
          body: {
            padding: 16,
            flex: 1,
            overflow: 'auto',
            background:
              'linear-gradient(180deg, rgba(250,173,20,0.04) 0%, rgba(255,255,255,0.98) 100%)',
          },
        }}
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleAddTab}
            disabled={!isTabsLayout}
          >
            添加 Tab
          </Button>
        }
      >
        {!isTabsLayout && (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message="当前配置不是 tabs 布局"
            description="当前正式运行范围只支持 tabs 顶层布局，请先转换后再编辑。"
            action={
              <Button
                size="small"
                type="primary"
                onClick={() =>
                  onChange({
                    ...safeConfig,
                    layout: {
                      type: 'tabs',
                      tabs: [],
                    },
                  })
                }
              >
                转换为 tabs
              </Button>
            }
          />
        )}
        {tabs.length > 0 ? (
          <Card size="small" style={{ marginBottom: 12 }}>
            <Space direction="vertical" size={10} style={{ width: '100%' }}>
              <Space wrap size={[8, 8]}>
                <Tag color="blue">{`页面 ${tabs.length}`}</Tag>
                <Tag color="success">{`已就绪 ${readyCount}`}</Tag>
                <Tag color={readyCount === tabs.length ? 'success' : 'orange'}>
                  {`待完善 ${tabs.length - readyCount}`}
                </Tag>
                {activeTabSummary ? (
                  <Tag color="processing">{`当前 ${activeTabSummary.title}`}</Tag>
                ) : null}
              </Space>
              <Typography.Text type="secondary">
                {readyCount === tabs.length
                  ? `所有 ${tabs.length} 张页面都已具备基础条件，可以继续细化或准备发布。`
                  : `当前已完成 ${readyCount}/${tabs.length} 张页面，优先继续处理未完成页面。`}
              </Typography.Text>
              <Space wrap size={[8, 8]}>
                {activeTabSummary ? (
                  <>
                    <Tag>{`布局 ${activeTabSummary.layoutType}`}</Tag>
                    <Tag>{`函数 ${activeTabSummary.functionCount}`}</Tag>
                    <Tag>{`主函数 ${activeTabSummary.primaryFunctionLabel}`}</Tag>
                    {activeTabSummary.isDefault ? <Tag color="success">默认页</Tag> : null}
                    {!activeTabSummary.isReady ? (
                      <Tag color="orange">{`当前页待办：${activeTabSummary.nextAction}`}</Tag>
                    ) : (
                      <Tag color="success">当前页可继续细化</Tag>
                    )}
                  </>
                ) : null}
                {firstPendingTab ? (
                  <Button
                    size="small"
                    type="primary"
                    onClick={() => {
                      if (controlledActiveTabKey === undefined) {
                        setActiveKey(firstPendingTab.key);
                      }
                      onActiveTabChange?.(firstPendingTab.key);
                    }}
                  >
                    去第一个待完善页面
                  </Button>
                ) : null}
                <Button size="small" onClick={handleAddTab}>
                  新建页面
                </Button>
              </Space>
            </Space>
          </Card>
        ) : null}
        {tabs.length > 0 ? (
          <Tabs
            activeKey={mergedActiveKey}
            onChange={(nextKey) => {
              if (controlledActiveTabKey === undefined) {
                setActiveKey(nextKey);
              }
              onActiveTabChange?.(nextKey);
            }}
            items={tabItems}
            type="card"
          />
        ) : (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Alert
              type="info"
              showIcon
              message="当前还没有页面 Tab"
              description={
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <Typography.Text type="secondary">
                    一个 Tab
                    就是一张实际页面。先起第一个页面，再进入里面绑定函数、生成布局和补字段。
                  </Typography.Text>
                  <Space wrap size={[8, 8]}>
                    <Tag color="processing">1. 创建首个页面</Tag>
                    <Tag>2. 绑定函数</Tag>
                    <Tag>3. 生成布局</Tag>
                    <Tag>4. 调整字段并预览</Tag>
                  </Space>
                </Space>
              }
            />
            <EditorEmptyState
              title="先创建第一个页面"
              description={
                descriptors.length > 0
                  ? '可以直接用当前对象的首个函数生成一个基础页，后面再慢慢细化。'
                  : '当前还没拿到可直接使用的函数，建议先手动建一个空页面骨架。'
              }
              action={
                <Space wrap>
                  {descriptors.length > 0 ? (
                    <Button type="primary" onClick={handleQuickStartTab}>
                      用首个函数生成基础页
                    </Button>
                  ) : null}
                  <Button
                    type={descriptors.length > 0 ? 'default' : 'primary'}
                    icon={<PlusOutlined />}
                    onClick={handleAddTab}
                  >
                    手动新建页面
                  </Button>
                </Space>
              }
            />
          </Space>
        )}
      </Card>

      {/* 添加 Tab 模态框 */}
      <Modal
        title="新建页面"
        open={showAddModal}
        onOk={handleConfirmAdd}
        onCancel={() => {
          setShowAddModal(false);
          form.resetFields();
        }}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="title"
            label="页面名称"
            rules={[{ required: true, message: '请输入页面名称' }]}
          >
            <Input placeholder="例如：订单列表、客户详情、创建表单" />
          </Form.Item>

          <Form.Item name="icon" label="页面图标">
            <IconPicker />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
