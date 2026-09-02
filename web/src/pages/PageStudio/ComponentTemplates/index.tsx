import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Row,
  Segmented,
  Space,
  Tag,
  Typography,
} from 'antd';
import {
  AppstoreOutlined,
  DeleteOutlined,
  EyeOutlined,
  ReloadOutlined,
  SearchOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import { request } from '@umijs/max';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';
import {
  instantiateTemplate,
  type ComponentTemplateDTO,
} from '../CompositeEditor/ComponentLibrary';
import PreviewRuntime from '../CompositeEditor/PreviewRuntime';
import type { PageNode } from '../CompositeEditor/model';

const { Text, Title } = Typography;

/** 组件模板 DTO。 */
interface TemplateDTO {
  key: string;
  name: Record<string, string> | string;
  description?: Record<string, string> | string;
  category?: string;
  icon?: string;
  requiredFunctions?: string[];
  tree: PageNode[];
  builtin: boolean;
  createdBy?: string;
}

function nameOf(t: TemplateDTO): string {
  if (typeof t.name === 'string') return t.name;
  return t.name?.['zh-CN'] ?? t.name?.['en-US'] ?? t.key;
}

function descOf(t: TemplateDTO): string {
  if (!t.description) return '';
  if (typeof t.description === 'string') return t.description;
  return t.description?.['zh-CN'] ?? t.description?.['en-US'] ?? '';
}

/** 树节点摘要（预览用）。 */
function treeSummary(tree: unknown[]): string {
  if (!Array.isArray(tree)) return '';
  return tree
    .map((n) => {
      const node = n as { type?: string; props?: { functionId?: string; title?: string } };
      const type = node?.type ?? '?';
      const fn = node?.props?.functionId ?? node?.props?.title ?? '';
      return fn ? `${type}(${fn})` : type;
    })
    .join(' → ');
}

/** 组件模板管理页面。 */
export default function ComponentTemplatesPage() {
  const { message } = App.useApp();
  const [templates, setTemplates] = useState<TemplateDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState('');
  const [previewKey, setPreviewKey] = useState<string | null>(null);
  const [previewTab, setPreviewTab] = useState<'ui' | 'json'>('ui');
  const [regenerating, setRegenerating] = useState(false);
  // 函数契约（fnForm/fnTable 渲染需要 schema；拉取失败按空集降级——
  // 组件结构仍可预览，仅表单缺字段提示）。
  const [fnById, setFnById] = useState<Map<string, FunctionDescriptor>>(new Map());
  useEffect(() => {
    listDescriptors()
      .then((fns) => setFnById(new Map(fns.map((f) => [f.id, f]))))
      .catch(() => undefined);
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const resp = (await request('/api/v1/component-templates', {
        skipErrorHandler: true,
      })) as { items?: TemplateDTO[] } | TemplateDTO[];
      setTemplates(Array.isArray(resp) ? resp : (resp?.items ?? []));
    } catch {
      message.error('加载组件模板失败');
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleRegenerate = useCallback(async () => {
    setRegenerating(true);
    try {
      const resp = (await request('/api/v1/component-templates/regenerate', {
        method: 'POST',
        skipErrorHandler: true,
      })) as { regenerated?: number };
      message.success(`已从 ${resp?.regenerated ?? 0} 个契约重新生成内置组件`);
      await load();
    } catch {
      message.error('重新生成失败');
    } finally {
      setRegenerating(false);
    }
  }, [load, message]);

  const handleDelete = useCallback(
    async (key: string) => {
      try {
        await request(`/api/v1/component-templates/${encodeURIComponent(key)}`, {
          method: 'DELETE',
          skipErrorHandler: true,
        });
        message.success(`已删除 ${key}`);
        await load();
      } catch {
        message.error('删除失败（内置组件不可删除）');
      }
    },
    [load, message],
  );

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return templates;
    return templates.filter(
      (t) =>
        nameOf(t).toLowerCase().includes(q) ||
        t.key.toLowerCase().includes(q) ||
        (t.category ?? '').toLowerCase().includes(q),
    );
  }, [templates, search]);

  const grouped = useMemo(() => {
    const byCat = new Map<string, TemplateDTO[]>();
    for (const t of filtered) {
      const cat = t.category || (t.builtin ? '内置' : '自定义');
      if (!byCat.has(cat)) byCat.set(cat, []);
      byCat.get(cat)!.push(t);
    }
    return Array.from(byCat.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [filtered]);

  const previewTpl = templates.find((t) => t.key === previewKey);
  // 实例化一次（重分配 id/重映射引用），previewKey 变化才重建——
  // PreviewRuntime 的 autoRun 语义依赖挂载时机。
  const previewTree = useMemo(
    () => (previewTpl ? instantiateTemplate(previewTpl as unknown as ComponentTemplateDTO) : []),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [previewKey],
  );

  return (
    <PageContainer
      header={{
        title: '组件模板',
        extra: [
          <Button
            key="regen"
            icon={<ThunderboltOutlined />}
            loading={regenerating}
            onClick={() => void handleRegenerate()}
          >
            从契约重新生成
          </Button>,
          <Button key="reload" icon={<ReloadOutlined />} onClick={() => void load()} />,
        ],
      }}
    >
      <Input
        allowClear
        prefix={<SearchOutlined style={{ color: '#999' }} />}
        placeholder="搜索组件名 / key / 分类"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        style={{ width: 320, marginBottom: 16 }}
      />

      {loading ? (
        <Empty description="加载中…" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : filtered.length === 0 ? (
        <Empty
          description="暂无组件模板——点击「从契约重新生成」或到编辑器选中节点保存为组件"
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      ) : (
        grouped.map(([category, items]) => (
          <div key={category} style={{ marginBottom: 24 }}>
            <Title level={5} style={{ marginBottom: 12 }}>
              {category}（{items.length}）
            </Title>
            <Row gutter={[12, 12]}>
              {items.map((tpl) => (
                <Col key={tpl.key} xs={24} sm={12} md={8} lg={6}>
                  <Card
                    size="small"
                    hoverable
                    actions={[
                      <Button
                        key="preview"
                        size="small"
                        type="text"
                        icon={<EyeOutlined />}
                        onClick={() => setPreviewKey(tpl.key)}
                      >
                        预览
                      </Button>,
                      ...(!tpl.builtin
                        ? [
                            <Popconfirm
                              key="del"
                              title="确认删除？"
                              onConfirm={() => void handleDelete(tpl.key)}
                            >
                              <Button size="small" type="text" danger icon={<DeleteOutlined />}>
                                删除
                              </Button>
                            </Popconfirm>,
                          ]
                        : []),
                    ]}
                  >
                    <Card.Meta
                      avatar={<AppstoreOutlined style={{ fontSize: 24, color: '#1677ff' }} />}
                      title={
                        <Space size={6}>
                          <Text strong>{nameOf(tpl)}</Text>
                          {tpl.builtin && <Tag style={{ fontSize: 10 }}>内置</Tag>}
                        </Space>
                      }
                      description={
                        <div>
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {descOf(tpl) || tpl.key}
                          </Text>
                          {tpl.requiredFunctions?.length ? (
                            <div>
                              <Text type="secondary" style={{ fontSize: 11 }}>
                                依赖：{tpl.requiredFunctions.join(', ')}
                              </Text>
                            </div>
                          ) : null}
                        </div>
                      }
                    />
                  </Card>
                </Col>
              ))}
            </Row>
          </div>
        ))
      )}

      <Modal
        title={previewTpl ? nameOf(previewTpl) : ''}
        open={!!previewKey}
        onCancel={() => setPreviewKey(null)}
        footer={null}
        width={880}
      >
        {previewTpl && (
          <div>
            <Space style={{ marginBottom: 12, width: '100%', justifyContent: 'space-between' }}>
              <Segmented
                value={previewTab}
                onChange={(v) => setPreviewTab(v as 'ui' | 'json')}
                options={[
                  { label: '界面预览', value: 'ui' },
                  { label: 'JSON', value: 'json' },
                ]}
              />
              <Text type="secondary" style={{ fontSize: 12 }}>
                结构：{treeSummary(previewTpl.tree)}
              </Text>
            </Space>
            {previewTab === 'ui' ? (
              <div
                style={{
                  border: '1px solid #f0f0f0',
                  borderRadius: 6,
                  padding: 16,
                  maxHeight: 480,
                  overflow: 'auto',
                  background: '#fff',
                }}
              >
                <PreviewRuntime tree={previewTree} fnById={fnById} />
              </div>
            ) : (
              <pre
                style={{
                  background: '#fafafa',
                  padding: 12,
                  borderRadius: 6,
                  fontSize: 12,
                  maxHeight: 400,
                  overflow: 'auto',
                }}
              >
                {JSON.stringify(previewTpl.tree, null, 2)}
              </pre>
            )}
          </div>
        )}
      </Modal>
    </PageContainer>
  );
}
