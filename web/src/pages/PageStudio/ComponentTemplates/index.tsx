import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
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
  ControlOutlined,
  ThunderboltOutlined,
  ExperimentOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import ConstantImportModal from '../CompositeEditor/ConstantImportModal';
import { request } from '@umijs/max';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';
import {
  instantiateTemplate,
  type ComponentTemplateDTO,
} from '../CompositeEditor/ComponentLibrary';
import PreviewRuntime from '../CompositeEditor/PreviewRuntime';
import type { PageNode } from '../CompositeEditor/model';
import { demoConstantTemplatePayloads, findLegacyMergedTemplates } from './constantTemplateAudit';

const { Text, Title } = Typography;

/** 组件模板 DTO。 */
interface TemplateDTO {
  key: string;
  name: Record<string, string> | string;
  description?: Record<string, string> | string;
  category?: string;
  icon?: string;
  stale?: boolean;
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
  const [importOpen, setImportOpen] = useState(false);
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

  // 旧版合并常量模板（一个 staticForm 塞多个常量）检测——规范是
  // 「一种常量一个独立模板」，旧数据需清理后重新导入。
  const legacyMerged = useMemo(() => findLegacyMergedTemplates(templates), [templates]);
  const [cleaningLegacy, setCleaningLegacy] = useState(false);
  const handleCleanLegacy = useCallback(async () => {
    setCleaningLegacy(true);
    let removed = 0;
    try {
      for (const tpl of legacyMerged) {
        try {
          await request(`/api/v1/component-templates/${encodeURIComponent(tpl.key)}`, {
            method: 'DELETE',
            skipErrorHandler: true,
          });
          removed += 1;
        } catch {
          // 单条失败不阻断其余清理（如内置模板不可删）
        }
      }
      message.success(`已清理 ${removed} 个旧版合并模板——请重新「导入常量」生成独立组件`);
      await load();
    } finally {
      setCleaningLegacy(false);
    }
  }, [legacyMerged, load, message]);

  // 示例常量模板（演示假数据）：固定 consts--demo-* key，已存在自动跳过（幂等）。
  const [seedingDemo, setSeedingDemo] = useState(false);
  const handleSeedDemo = useCallback(async () => {
    setSeedingDemo(true);
    try {
      const existing = new Set(templates.map((t) => t.key));
      const payloads = demoConstantTemplatePayloads().filter((p) => !existing.has(p.key));
      if (payloads.length === 0) {
        message.info('示例常量模板已存在');
        return;
      }
      for (const payload of payloads) {
        await request('/api/v1/component-templates', {
          method: 'POST',
          data: payload,
          skipErrorHandler: true,
        });
      }
      message.success(`已生成 ${payloads.length} 个示例常量组件——组合页编辑器中可拖入使用`);
      await load();
    } catch {
      message.error('生成示例常量模板失败');
    } finally {
      setSeedingDemo(false);
    }
  }, [templates, load, message]);

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
            key="import-consts"
            icon={<ControlOutlined />}
            onClick={() => setImportOpen(true)}
          >
            导入常量
          </Button>,
          <Button
            key="seed-demo-consts"
            icon={<ExperimentOutlined />}
            loading={seedingDemo}
            onClick={() => void handleSeedDemo()}
          >
            生成示例常量
          </Button>,
          <Button
            key="regen"
            icon={<ThunderboltOutlined />}
            loading={regenerating}
            onClick={() => void handleRegenerate()}
          >
            从契约重新生成
          </Button>,
          <Button key="reload" icon={<ReloadOutlined />} onClick={() => void load()} />,
          <a key="editor" href="/functions/pages/composite-editor" target="_blank" rel="noreferrer">
            <Button icon={<AppstoreOutlined />}>在编辑器中使用</Button>
          </a>,
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

      {legacyMerged.length > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message={`检测到 ${legacyMerged.length} 个旧版合并常量模板（一个模板包含多个常量）`}
          description={
            <Space direction="vertical" size={4}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                现行规范是「一种常量一个独立组件」。旧模板：
                {legacyMerged.map((t) => t.key).join('、')}
                ——清理后请重新「导入常量」。
              </Text>
              <Button
                size="small"
                danger
                loading={cleaningLegacy}
                onClick={() => void handleCleanLegacy()}
              >
                一键清理旧模板
              </Button>
            </Space>
          }
        />
      )}

      {loading ? (
        <Empty description="加载中…" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : filtered.length === 0 ? (
        <Empty
          description="暂无组件模板——点击「生成示例常量」体验、导入常量或从契约重新生成"
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
                          {tpl.stale && (
                            <Tag color="orange" style={{ fontSize: 10 }}>
                              已过期
                            </Tag>
                          )}
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
      <ConstantImportModal
        open={importOpen}
        onCancel={() => setImportOpen(false)}
        onSaved={() => {
          setImportOpen(false);
          message.success('常量模板已保存——组合页编辑器组件库中可拖入使用');
          void load();
        }}
      />
    </PageContainer>
  );
}
