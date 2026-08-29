import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Collapse,
  Empty,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Tree,
  Typography,
} from 'antd';
import {
  AppstoreOutlined,
  ArrowDownOutlined,
  ArrowUpOutlined,
  BarsOutlined,
  DeleteOutlined,
  DragOutlined,
  FormOutlined,
  TableOutlined,
} from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';

const { Text } = Typography;

export type CompositeSectionDraft = {
  key: string;
  functionId: string;
  view: string; // table | fields | form | actions
  title: string;
  span: number;
  autoRun: boolean;
  refreshOn: string[];
};

export type CompositeBuilderProps = {
  open: boolean;
  sections: CompositeSectionDraft[];
  onChange: (sections: CompositeSectionDraft[]) => void;
  pageKey: string;
  onPageKeyChange: (key: string) => void;
};

const VIEW_META: Record<string, { label: string; icon: React.ReactNode }> = {
  table: { label: '表格', icon: <TableOutlined /> },
  fields: { label: '字段', icon: <BarsOutlined /> },
  form: { label: '操作', icon: <FormOutlined /> },
  actions: { label: '按钮组', icon: <AppstoreOutlined /> },
};

/** 按资源分组的函数树数据 */
function buildTreeData(descriptors: FunctionDescriptor[]): DataNode[] {
  const byResource = new Map<string, FunctionDescriptor[]>();
  for (const d of descriptors) {
    const rk = (d.resource || '').trim() || '其他';
    if (!byResource.has(rk)) byResource.set(rk, []);
    byResource.get(rk)!.push(d);
  }
  const nodes: DataNode[] = [];
  for (const [rk, fns] of byResource) {
    nodes.push({
      title: rk,
      key: `res:${rk}`,
      selectable: false,
      children: fns.map((fn) => ({
        title: fn.id,
        key: `fn:${fn.id}`,
        isLeaf: true,
      })),
    });
  }
  return nodes;
}

/** 默认视图形态：collection_query→table、item_query→fields、其他→form */
function defaultView(fn: FunctionDescriptor | undefined): string {
  if (!fn) return 'form';
  if (fn.operation === 'list') return 'table';
  if (fn.operation === 'get') return 'fields';
  return 'form';
}

/**
 * 组合页三栏构建器：
 * 左=函数树（按资源分组，点选加入画布）
 * 中=区块画布（上移/下移/删块/调宽度/视图形态/联动键）
 * （右侧预览由 PageStudio 既有 livePreview 承担）
 */
export default function CompositeBuilder({
  open,
  sections,
  onChange,
  pageKey,
  onPageKeyChange,
}: CompositeBuilderProps) {
  const { message } = App.useApp();
  const [descriptors, setDescriptors] = useState<FunctionDescriptor[]>([]);
  const [search, setSearch] = useState('');

  useEffect(() => {
    if (!open) return;
    listDescriptors()
      .then(setDescriptors)
      .catch(() => message.error('加载函数清单失败'));
  }, [open, message]);

  const treeData = useMemo(() => {
    const all = buildTreeData(descriptors);
    if (!search.trim()) return all;
    const q = search.trim().toLowerCase();
    return all
      .map((res) => ({
        ...res,
        children: (res.children || []).filter((c) => String(c.key).toLowerCase().includes(q)),
      }))
      .filter((res) => (res.children || []).length > 0);
  }, [descriptors, search]);

  const fnById = useMemo(() => {
    const m = new Map<string, FunctionDescriptor>();
    for (const d of descriptors) m.set(d.id, d);
    return m;
  }, [descriptors]);

  const addSection = useCallback(
    (functionId: string) => {
      if (sections.some((s) => s.functionId === functionId)) {
        message.warning(`函数 ${functionId} 已在画布中`);
        return;
      }
      const fn = fnById.get(functionId);
      onChange([
        ...sections,
        {
          key: functionId,
          functionId,
          view: defaultView(fn),
          title: fn?.summary?.['zh-CN'] || functionId,
          span: 24,
          autoRun: false,
          refreshOn: [],
        },
      ]);
    },
    [sections, onChange, fnById],
  );

  const updateSection = useCallback(
    (idx: number, patch: Partial<CompositeSectionDraft>) => {
      onChange(sections.map((s, i) => (i === idx ? { ...s, ...patch } : s)));
    },
    [onChange, sections],
  );

  const moveSection = useCallback(
    (idx: number, dir: -1 | 1) => {
      const next = [...sections];
      const target = idx + dir;
      if (target < 0 || target >= next.length) return;
      [next[idx], next[target]] = [next[target], next[idx]];
      onChange(next);
    },
    [onChange, sections],
  );

  const removeSection = useCallback(
    (idx: number) => {
      onChange(sections.filter((_, i) => i !== idx));
    },
    [onChange, sections],
  );

  return (
    <div>
      <Card size="small" title="页面 Key" style={{ marginBottom: 12 }}>
        <Input
          placeholder="如 composite--player-overview"
          value={pageKey}
          onChange={(e) => onPageKeyChange(e.target.value)}
        />
      </Card>
      <Row gutter={12}>
        {/* 左：函数树 */}
        <Col span={7}>
          <Card
            size="small"
            title="可用函数"
            styles={{ body: { maxHeight: 520, overflow: 'auto' } }}
          >
            <Input
              size="small"
              placeholder="搜索函数"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              style={{ marginBottom: 8 }}
            />
            {treeData.length === 0 ? (
              <Empty description="无函数契约" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            ) : (
              <Tree
                treeData={treeData}
                defaultExpandAll
                showLine={false}
                onSelect={(keys) => {
                  const key = String(keys[0] || '');
                  if (key.startsWith('fn:')) addSection(key.slice(3));
                }}
              />
            )}
          </Card>
        </Col>

        {/* 中：区块画布 */}
        <Col span={17}>
          <Card
            size="small"
            title={`区块画布（${sections.length} 个区块，上→下为页面渲染顺序）`}
            styles={{ body: { maxHeight: 520, overflow: 'auto' } }}
          >
            {sections.length === 0 ? (
              <Empty description="从左侧点击函数加入画布" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            ) : (
              <Space direction="vertical" style={{ width: '100%' }} size={8}>
                {sections.map((sec, idx) => {
                  const fn = fnById.get(sec.functionId);
                  return (
                    <Card
                      key={sec.key}
                      size="small"
                      title={
                        <Space size={6}>
                          <Text code>{sec.functionId}</Text>
                          {fn?.summary?.['zh-CN'] ? (
                            <Text type="secondary">{fn.summary['zh-CN']}</Text>
                          ) : null}
                        </Space>
                      }
                      extra={
                        <Space size={4}>
                          <Button
                            size="small"
                            type="text"
                            icon={<ArrowUpOutlined />}
                            disabled={idx === 0}
                            onClick={() => moveSection(idx, -1)}
                          />
                          <Button
                            size="small"
                            type="text"
                            icon={<ArrowDownOutlined />}
                            disabled={idx === sections.length - 1}
                            onClick={() => moveSection(idx, 1)}
                          />
                          <Button
                            size="small"
                            type="text"
                            danger
                            icon={<DeleteOutlined />}
                            onClick={() => removeSection(idx)}
                          />
                        </Space>
                      }
                    >
                      <Space wrap size={8}>
                        <Select
                          size="small"
                          value={sec.view}
                          onChange={(v) => updateSection(idx, { view: v })}
                          style={{ width: 120 }}
                          options={Object.entries(VIEW_META).map(([value, meta]) => ({
                            value,
                            label: (
                              <Space size={4}>
                                {meta.icon}
                                {meta.label}
                              </Space>
                            ),
                          }))}
                        />
                        <InputNumber
                          size="small"
                          min={0}
                          max={24}
                          value={sec.span}
                          onChange={(v) => updateSection(idx, { span: v || 0 })}
                          addonBefore="宽度"
                          style={{ width: 130 }}
                        />
                        <Input
                          size="small"
                          placeholder="联动键（逗号分隔）"
                          value={sec.refreshOn.join(',')}
                          onChange={(e) =>
                            updateSection(idx, {
                              refreshOn: e.target.value
                                .split(',')
                                .map((x) => x.trim())
                                .filter(Boolean),
                            })
                          }
                          style={{ width: 200 }}
                          prefix={<DragOutlined style={{ color: '#bbb' }} />}
                        />
                      </Space>
                      <div style={{ marginTop: 6 }}>
                        <Space size={12}>
                          <label style={{ fontSize: 12 }}>
                            <input
                              type="checkbox"
                              checked={sec.autoRun}
                              onChange={(e) => updateSection(idx, { autoRun: e.target.checked })}
                            />{' '}
                            加载即执行
                          </label>
                          {sec.refreshOn.length > 0 && (
                            <Text type="secondary" style={{ fontSize: 12 }}>
                              联动：{sec.refreshOn.join(' / ')} 变化时自动重跑
                            </Text>
                          )}
                        </Space>
                      </div>
                    </Card>
                  );
                })}
              </Space>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
}
