import React, { useCallback, useMemo, useRef, useState } from 'react';
import { App, Button, Card, Col, Empty, Input, Row, Space, Tabs, Tag, Typography } from 'antd';
import { ArrowLeftOutlined, EyeOutlined, SaveOutlined } from '@ant-design/icons';
import { history } from '@umijs/max';
import { PageContainer } from '@ant-design/pro-components';
import ComponentPanel, { type AddFnEvent } from './ComponentPanel';
import PropsPanel from './PropsPanel';
import { registerBuiltinComponents } from './components/builtin';
import { getComponent } from './registry';
import type { FunctionDescriptor } from '@/services/api/functions';
import {
  countNodes,
  findNode,
  insertNode,
  nodeId,
  removeNode,
  updateProps,
  type PageNode,
} from './model';

const { Text } = Typography;

registerBuiltinComponents();

/** scaffold 按契约实例化节点 props。 */
function scaffoldProps(type: PageNode['type'], fn?: FunctionDescriptor): Record<string, unknown> {
  return getComponent(type)?.scaffold(fn) ?? {};
}

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
  const [, forceFn] = useState(0);
  const registerFn = useCallback((fn: FunctionDescriptor) => {
    fnById.current.set(fn.id, fn);
    forceFn((n) => n + 1);
  }, []);

  const selected = useMemo(() => findNode(tree, selectedId ?? ''), [tree, selectedId]);

  /** 函数 → 组件节点（scaffold 按契约实例化，amis 式拖入即骨架）。 */
  const addFunction = useCallback(
    (e: AddFnEvent) => {
      registerFn(e.fn);
      const node: PageNode = {
        id: nodeId(e.componentType),
        type: e.componentType,
        props: scaffoldProps(e.componentType, e.fn),
      };
      setTree((prev) => [...prev, node]);
      setSelectedId(node.id);
    },
    [registerFn],
  );

  /** 基础组件 → 节点。 */
  const addBasic = useCallback((type: 'button' | 'modal' | 'container' | 'text') => {
    const node: PageNode = { id: nodeId(type), type, props: scaffoldProps(type) };
    setTree((prev) => [...prev, node]);
    setSelectedId(node.id);
  }, []);

  const patchProps = useCallback(
    (patch: Record<string, unknown>) => {
      if (selectedId) setTree((prev) => updateProps(prev, selectedId, patch));
    },
    [selectedId],
  );

  /** 子节点放入容器（V1：modal 单 fnForm）。 */
  const addChild = useCallback((parentId: string, node: PageNode) => {
    setTree((prev) => insertNode(prev, node, parentId));
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
                    children: <ComponentPanel onAddBasic={addBasic} onAddFunction={addFunction} />,
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
                {tree.map((n) => {
                  const def = getComponent(n.type);
                  const Comp = def?.Preview;
                  return (
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
                      {Comp ? (
                        <Comp
                          node={n}
                          fn={
                            n.props.functionId
                              ? fnById.current.get(String(n.props.functionId))
                              : undefined
                          }
                        />
                      ) : (
                        <Text type="secondary" style={{ fontSize: 11 }}>
                          {String(n.props.functionId ?? '（基础组件）')}
                        </Text>
                      )}
                    </Card>
                  );
                })}
              </Space>
            )}
          </div>
        </Col>

        {/* 右：属性面板（rjsf schema 驱动） */}
        {!preview && (
          <Col flex="360px">
            <PropsPanel
              node={selected}
              nodes={tree}
              fnById={fnById.current}
              onPatch={patchProps}
              onDelete={() => selected && deleteNode(selected.id)}
            />
          </Col>
        )}
      </Row>
    </PageContainer>
  );
}
