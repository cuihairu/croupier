import React, { useCallback, useMemo, useRef, useState } from 'react';
import { App, Button, Card, Col, Empty, Input, Row, Space, Tabs, Tag, Typography } from 'antd';
import { ArrowLeftOutlined, EyeOutlined, SaveOutlined } from '@ant-design/icons';
import { history } from '@umijs/max';
import { PageContainer } from '@ant-design/pro-components';
import type { FunctionDescriptor } from '@/services/api/functions';
import FunctionPanel from './FunctionPanel';
import { countNodes, findNode, nodeId, removeNode, type PageNode } from './model';
import { defaultView } from './types';

const { Text } = Typography;

/**
 * 组合页编辑器 V3（组件化）：左=组件面板/大纲 Tabs，中=画布（组件树），
 * 右=属性面板（rjsf schema 驱动），顶栏=pageKey/预览切换/保存。
 * 页面状态是 PageNode 组件树；保存时编译为 CompositePageSpec（P4）。
 */
export default function CompositeEditorPage() {
  const { message } = App.useApp();
  const [tree, setTree] = useState<PageNode[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [pageKey, setPageKey] = useState('');
  const [keyTouched, setKeyTouched] = useState(false);
  const [mode, setMode] = useState<'edit' | 'preview'>('edit');
  const [leftTab, setLeftTab] = useState('components');

  const fnById = useRef(new Map<string, FunctionDescriptor>());

  const selected = useMemo(() => findNode(tree, selectedId ?? ''), [tree, selectedId]);

  // 函数点击 → 最小 fnTable/fnForm 节点（P1 scaffold 接管默认值）
  const addFunction = useCallback((fn: FunctionDescriptor) => {
    fnById.current.set(fn.id, fn);
    const type =
      defaultView(fn) === 'table'
        ? 'fnTable'
        : defaultView(fn) === 'fields'
          ? 'fnFields'
          : 'fnForm';
    const node: PageNode = {
      id: nodeId(type),
      type,
      props: { functionId: fn.id, title: fn.summary?.['zh-CN'] || fn.id },
    };
    setTree((prev) => [...prev, node]);
    setSelectedId(node.id);
  }, []);

  const deleteNode = useCallback((id: string) => {
    setTree((prev) => removeNode(prev, id)[0]);
    setSelectedId((cur) => (cur === id ? null : cur));
  }, []);

  const preview = mode === 'preview';

  return (
    <PageContainer
      header={{
        title: '组合页编辑器',
        onBack: () => history.push('/functions/pages'),
        backIcon: <ArrowLeftOutlined />,
        extra: [
          <Button
            key="mode"
            type={preview ? 'primary' : 'default'}
            icon={<EyeOutlined />}
            onClick={() => setMode(preview ? 'edit' : 'preview')}
          >
            {preview ? '退出预览' : '预览'}
          </Button>,
          <Button
            key="save"
            type="primary"
            icon={<SaveOutlined />}
            disabled={preview}
            onClick={() => message.info('保存将在 P4 批次接线（树 → CompositeSection 编译）')}
          >
            保存为提案
          </Button>,
        ],
      }}
    >
      <Space wrap style={{ marginBottom: 12 }}>
        <Text strong>页面 Key</Text>
        <Input
          placeholder="按组件自动生成，可修改"
          value={pageKey}
          onChange={(e) => {
            setKeyTouched(true);
            setPageKey(e.target.value);
          }}
          style={{ width: 320 }}
        />
        <Text type="secondary">{countNodes(tree)} 个组件</Text>
      </Space>

      <Row gutter={12}>
        {/* 左：组件面板 / 大纲 */}
        {!preview && (
          <Col flex="300px">
            <Card size="small" styles={{ body: { padding: 8 } }}>
              <Tabs
                activeKey={leftTab}
                onChange={setLeftTab}
                items={[
                  {
                    key: 'components',
                    label: '组件',
                    children: <FunctionPanel addedIds={new Set()} onAdd={addFunction} />,
                  },
                  {
                    key: 'outline',
                    label: '大纲',
                    children: (
                      <Empty
                        image={Empty.PRESENTED_IMAGE_SIMPLE}
                        description="大纲树（P2 批次）"
                        style={{ marginTop: 40 }}
                      />
                    ),
                  },
                ]}
              />
            </Card>
          </Col>
        )}

        {/* 中：画布（P2 树渲染接管；当前为节点列表占位） */}
        <Col flex="auto" style={{ minWidth: 420 }}>
          <div
            style={{
              border: '1px solid #f0f0f0',
              borderRadius: 8,
              minHeight: 'calc(100vh - 300px)',
              padding: 12,
              background: '#fafafa',
            }}
          >
            {tree.length === 0 ? (
              <Empty
                style={{ marginTop: 120 }}
                description="从左侧点击函数或拖入组件，开始搭建页面"
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            ) : (
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                {tree.map((n) => (
                  <Card
                    key={n.id}
                    size="small"
                    style={{
                      borderColor: selectedId === n.id ? '#1677ff' : undefined,
                      cursor: 'pointer',
                    }}
                    onClick={() => setSelectedId(n.id)}
                    title={
                      <Space size={6}>
                        <Text strong style={{ fontSize: 13 }}>
                          {String(n.props.title ?? n.type)}
                        </Text>
                        <Tag style={{ marginRight: 0 }}>{n.type}</Tag>
                      </Space>
                    }
                    extra={
                      <Button
                        size="small"
                        type="text"
                        danger
                        onClick={(e) => {
                          e.stopPropagation();
                          deleteNode(n.id);
                        }}
                      >
                        删除
                      </Button>
                    }
                  >
                    <Text type="secondary" style={{ fontSize: 11 }}>
                      {String(n.props.functionId ?? '（基础组件）')}
                    </Text>
                    <div>
                      <Text type="secondary" style={{ fontSize: 10 }}>
                        画布树渲染在 P2 接管
                      </Text>
                    </div>
                  </Card>
                ))}
              </Space>
            )}
          </div>
        </Col>

        {/* 右：属性面板（P1 rjsf 渲染接管） */}
        {!preview && (
          <Col flex="360px">
            <Card size="small" title={<Text strong>属性</Text>}>
              {selected ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={`属性面板（P1 批次）：${selected.type}`}
                />
              ) : (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="点击画布组件进行配置" />
              )}
            </Card>
          </Col>
        )}
      </Row>
    </PageContainer>
  );
}
