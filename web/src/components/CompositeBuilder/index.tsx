import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Empty,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Typography,
} from 'antd';
import {
  AppstoreOutlined,
  ArrowDownOutlined,
  ArrowUpOutlined,
  BarsOutlined,
  CopyOutlined,
  DeleteOutlined,
  DragOutlined,
  FormOutlined,
  FunctionOutlined,
  PlusOutlined,
  ReloadOutlined,
  TableOutlined,
} from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { Tree } from 'antd';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  useSensor,
  useSensors,
  useDraggable,
  useDroppable,
  type DragStartEvent,
  type DragEndEvent,
} from '@dnd-kit/core';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';
import { SortableList } from '@/components/SortableList';
import { localizedText } from '@/utils/localizedText';

const { Text, Title } = Typography;

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

const PAGE_KEY_PREFIX = 'composite--';

/** 从当前区块自动推导 pageKey 预填值（主函数或资源组合） */
function derivePageKey(sections: CompositeSectionDraft[]): string {
  const resources = Array.from(
    new Set(sections.map((s) => s.functionId.split('.')[0]).filter((r) => r && r.length > 0)),
  );
  const base = resources.length > 0 ? resources.join('-') : 'page';
  return `${PAGE_KEY_PREFIX}${base}`;
}

/** pageKey 是否为自动推导值（用户改过则不再自动覆盖） */
function isAutoKey(pageKey: string, sections: CompositeSectionDraft[]): boolean {
  return pageKey === '' || pageKey === derivePageKey(sections);
}

function buildTreeData(descriptors: FunctionDescriptor[]): DataNode[] {
  const byResource = new Map<string, FunctionDescriptor[]>();
  for (const d of descriptors) {
    const rk = (d.resource || '').trim() || '其他';
    if (!byResource.has(rk)) byResource.set(rk, []);
    byResource.get(rk)!.push(d);
  }
  return Array.from(byResource.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([rk, fns]) => ({
      title: (
        <Space size={4}>
          <FunctionOutlined style={{ color: '#8c8c8c' }} />
          <Text strong>{rk}</Text>
          <Text type="secondary" style={{ fontSize: 11 }}>
            {fns.length}
          </Text>
        </Space>
      ),
      key: `res:${rk}`,
      selectable: false,
      children: fns
        .sort((a, b) => a.id.localeCompare(b.id))
        .map((fn) => ({
          title: (
            <DraggableFnNode
              functionId={fn.id}
              label={fn.operation || fn.id.split('.').pop() || fn.id}
            />
          ),
          key: `fn:${fn.id}`,
          isLeaf: true,
        })),
    }));
}

function defaultView(fn: FunctionDescriptor | undefined): string {
  if (!fn) return 'form';
  if (fn.operation === 'list') return 'table';
  if (fn.operation === 'get') return 'fields';
  return 'form';
}

/**
 * 组合页编辑工作台（全屏三栏）：
 * 左 = 资源/函数树（点击加入画布）
 * 中 = 区块画布（拖拽排序 + 属性编辑）
 * 右 = 实时布局预览
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
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState('');
  const [keyTouched, setKeyTouched] = useState(false);

  // 左栏→画布拖放：activeFnId 记录正在拖的函数
  const [activeFnId, setActiveFnId] = useState<string | null>(null);

  const handleDragStart = useCallback((event: DragStartEvent) => {
    const id = String(event.active.id || '');
    if (id.startsWith('fn:')) setActiveFnId(id.slice(3));
  }, []);

  const handleDragEnd = useCallback((event: DragEndEvent) => {
    setActiveFnId(null);
    const { active, over } = event;
    if (!over) return;
    const fnId = String(active.id || '').slice(3);
    if (!String(active.id).startsWith('fn:')) return; // 画布内排序由 SortableList 自己的 DndContext 处理
    const overId = String(over.id || '');
    if (overId === 'composite-canvas') {
      addSectionRef.current?.(fnId);
    } else if (overId.startsWith('sec:')) {
      // 拖到某区块卡上方：插入到该位置（append after index）
      const targetKey = overId.slice(4);
      const idx = sectionsRef.current.findIndex((x) => x.key === targetKey);
      if (idx >= 0 && addSectionRef.current) {
        addSectionRef.current(fnId, idx + 1);
      }
    }
  }, []);

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }));

  const addSectionRef = React.useRef<(id: string, at?: number) => void>(() => {});
  const sectionsRef = React.useRef<CompositeSectionDraft[]>([]);
  sectionsRef.current = sections;

  const loadDescriptors = useCallback(async () => {
    setLoading(true);
    try {
      setDescriptors(await listDescriptors());
    } catch {
      message.error('加载函数清单失败');
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    if (open) void loadDescriptors();
  }, [open, loadDescriptors]);

  // 左栏节点点击（短按）加入画布：与拖拽共存
  useEffect(() => {
    const handler = (ev: Event) => {
      const fnId = (ev as CustomEvent<string>).detail;
      if (fnId) addSectionRef.current(fnId);
    };
    window.addEventListener('composite-add-fn', handler);
    return () => window.removeEventListener('composite-add-fn', handler);
  }, []);

  // pageKey 自动预填：用户未手动改过时，跟随区块变化更新
  useEffect(() => {
    if (!keyTouched && sections.length > 0 && isAutoKey(pageKey, sections)) {
      onPageKeyChange(derivePageKey(sections));
    }
  }, [sections, keyTouched, pageKey, onPageKeyChange]);

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
    (functionId: string, atIndex?: number) => {
      if (sections.some((s) => s.functionId === functionId)) {
        message.warning(`函数 ${functionId} 已在画布中`);
        return;
      }
      const fn = fnById.get(functionId);
      const section: CompositeSectionDraft = {
        key: functionId,
        functionId,
        view: defaultView(fn),
        title: localizedText(fn?.summary, 'zh-CN', functionId),
        span: 24,
        autoRun: false,
        refreshOn: [],
      };
      if (atIndex === undefined || atIndex < 0 || atIndex > sections.length) {
        onChange([...sections, section]);
      } else {
        const next = [...sections];
        next.splice(atIndex, 0, section);
        onChange(next);
      }
    },
    [sections, onChange, fnById, message],
  );
  // ref 供外层 DndContext 的 dragEnd 读取最新闭包
  sectionsRef.current = sections;
  addSectionRef.current = addSection;

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
    <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
      <div style={{ position: 'relative' }}>
        {/* 顶部：页面 Key（自动预填 + 可改） */}
        <Card size="small" style={{ marginBottom: 12 }}>
          <Space wrap>
            <Text strong>页面 Key</Text>
            <Input
              placeholder="自动生成（如 composite--player-order），可修改"
              value={pageKey}
              onChange={(e) => {
                setKeyTouched(true);
                onPageKeyChange(e.target.value);
              }}
              style={{ width: 360 }}
              suffix={
                <CopyOutlined
                  style={{ color: '#999', cursor: 'pointer' }}
                  onClick={() => {
                    void navigator.clipboard?.writeText(pageKey);
                    message.success('已复制');
                  }}
                />
              }
            />
            {keyTouched && (
              <Button
                size="small"
                type="link"
                onClick={() => {
                  setKeyTouched(false);
                  onPageKeyChange(sections.length > 0 ? derivePageKey(sections) : '');
                }}
              >
                恢复自动
              </Button>
            )}
            <Text type="secondary">
              {sections.length} 个区块 · 联动{' '}
              {sections.filter((s) => s.refreshOn.length > 0).length} 处
            </Text>
          </Space>
        </Card>

        <Row gutter={12}>
          {/* 左：资源/函数树 */}
          <Col span={5} style={{ minWidth: 260 }}>
            <Card
              size="small"
              title={
                <Space size={4}>
                  <Text strong>函数</Text>
                  <Text type="secondary">{descriptors.length}</Text>
                </Space>
              }
              extra={
                <Button
                  size="small"
                  type="text"
                  icon={<ReloadOutlined />}
                  onClick={() => void loadDescriptors()}
                />
              }
              styles={{ body: { padding: 8, maxHeight: 640, overflow: 'auto' } }}
              loading={loading}
            >
              <Input
                size="small"
                placeholder="搜索函数 / 资源"
                allowClear
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
                  blockNode
                  onSelect={(keys) => {
                    const key = String(keys[0] || '');
                    if (key.startsWith('fn:')) addSection(key.slice(3));
                  }}
                />
              )}
            </Card>
          </Col>

          {/* 中：区块画布（拖拽排序 + 属性编辑 + 接收左栏拖入） */}
          <Col span={12}>
            <CanvasDropTarget sections={sections}>
              <Card
                size="small"
                title={
                  <Space size={4}>
                    <Text strong>区块画布</Text>
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      拖拽 <DragOutlined /> 排序 · 上→下为渲染顺序
                    </Text>
                  </Space>
                }
                styles={{ body: { maxHeight: 640, overflow: 'auto' } }}
              >
                {sections.length === 0 ? (
                  <Empty
                    description="从左侧点击函数加入画布"
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                    style={{ marginTop: 120 }}
                  />
                ) : (
                  <SortableList items={sections} getKey={(sec) => sec.key} onReorder={onChange}>
                    {(sec, idx, dragHandleProps) => {
                      const fn = fnById.get(sec.functionId);
                      return (
                        <Card
                          size="small"
                          style={{ marginBottom: 8 }}
                          title={
                            <Space size={6}>
                              <span {...dragHandleProps}>
                                <DragOutlined style={{ cursor: 'grab', color: '#1677ff' }} />
                              </span>
                              {VIEW_META[sec.view]?.icon}
                              <Text code>{sec.functionId}</Text>
                              {fn?.summary?.['zh-CN'] ? (
                                <Text type="secondary" style={{ fontSize: 12 }}>
                                  {fn.summary['zh-CN']}
                                </Text>
                              ) : null}
                            </Space>
                          }
                          extra={
                            <Space size={2}>
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
                            <Input
                              size="small"
                              placeholder="标题"
                              value={sec.title}
                              onChange={(e) => updateSection(idx, { title: e.target.value })}
                              style={{ width: 140 }}
                            />
                            <Select
                              size="small"
                              value={sec.view}
                              onChange={(v) => updateSection(idx, { view: v })}
                              style={{ width: 110 }}
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
                              addonBefore="宽"
                              style={{ width: 110 }}
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
                              style={{ width: 170 }}
                            />
                            <label style={{ fontSize: 12, whiteSpace: 'nowrap' }}>
                              <input
                                type="checkbox"
                                checked={sec.autoRun}
                                onChange={(e) => updateSection(idx, { autoRun: e.target.checked })}
                              />{' '}
                              自动执行
                            </label>
                          </Space>
                          {sec.refreshOn.length > 0 && (
                            <div style={{ marginTop: 6 }}>
                              <Text type="secondary" style={{ fontSize: 11 }}>
                                联动：{sec.refreshOn.join(' / ')} 变化时自动重跑
                              </Text>
                            </div>
                          )}
                        </Card>
                      );
                    }}
                  </SortableList>
                )}
              </Card>
            </CanvasDropTarget>
          </Col>

          {/* 右：实时布局预览 */}
          <Col span={7}>
            <Card
              size="small"
              title={<Text strong>布局预览</Text>}
              styles={{ body: { maxHeight: 640, overflow: 'auto' } }}
            >
              {sections.length === 0 ? (
                <Empty description="加入区块后预览布局" image={Empty.PRESENTED_IMAGE_SIMPLE} />
              ) : (
                <>
                  <div style={{ marginBottom: 8 }}>
                    <Title level={5} style={{ margin: 0 }}>
                      {pageKey || '（未命名）'}
                    </Title>
                  </div>
                  <Row gutter={[8, 8]}>
                    {sections.map((sec) => (
                      <Col
                        key={sec.key}
                        span={sec.span && sec.span > 0 && sec.span <= 24 ? sec.span : 24}
                      >
                        <div
                          style={{
                            border: '1px dashed #bfbfbf',
                            borderRadius: 6,
                            padding: '8px 10px',
                            minHeight: 68,
                            background: sec.autoRun ? '#f6ffed' : '#fafafa',
                          }}
                        >
                          <Space size={6}>
                            {VIEW_META[sec.view]?.icon ?? <AppstoreOutlined />}
                            <Text strong style={{ fontSize: 12 }}>
                              {sec.title || sec.key}
                            </Text>
                          </Space>
                          <div>
                            <Text type="secondary" style={{ fontSize: 11 }}>
                              {sec.functionId}
                            </Text>
                          </div>
                          <div>
                            <Text type="secondary" style={{ fontSize: 11 }}>
                              {sec.autoRun ? '⏵ 自动执行' : ''}
                              {sec.refreshOn?.length ? ` ⇄ ${sec.refreshOn.join(',')}` : ''}
                            </Text>
                          </div>
                          {sec.view === 'table' && (
                            <div style={{ marginTop: 4 }}>
                              {[0, 1, 2].map((r) => (
                                <div
                                  key={r}
                                  style={{
                                    height: 13,
                                    borderBottom: '1px solid #eee',
                                    fontSize: 10,
                                    color: '#bfbfbf',
                                  }}
                                >
                                  row {r + 1}
                                </div>
                              ))}
                            </div>
                          )}
                        </div>
                      </Col>
                    ))}
                  </Row>
                </>
              )}
            </Card>
          </Col>
        </Row>
      </div>
      <DragOverlay dropAnimation={null}>
        {activeFnId ? (
          <Card size="small" style={{ opacity: 0.9, width: 240, pointerEvents: 'none' }}>
            <Space size={6}>
              <FunctionOutlined />
              <Text code>{activeFnId}</Text>
            </Space>
          </Card>
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}

/** 左栏函数节点：可拖入画布（点击仍可用）。 */
function DraggableFnNode({ functionId, label }: { functionId: string; label: string }) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: `fn:${functionId}`,
  });
  return (
    <div
      ref={setNodeRef}
      {...attributes}
      {...listeners}
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        cursor: 'grab',
        opacity: isDragging ? 0.4 : 1,
        padding: '1px 4px',
        borderRadius: 4,
      }}
      onClick={(e) => {
        // 点击仍触发加入（dnd 与 click 共存：短按=点击）
        e.stopPropagation();
        const ev = new CustomEvent('composite-add-fn', { detail: functionId });
        window.dispatchEvent(ev);
      }}
    >
      <Space size={4} style={{ fontSize: 12 }}>
        <Text code>{label}</Text>
        <Text type="secondary" style={{ fontSize: 11 }}>
          {functionId}
        </Text>
      </Space>
      <PlusOutlined style={{ color: '#1677ff', fontSize: 11 }} />
    </div>
  );
}

/** 画布容器 drop 目标：接收左栏函数拖入（追加到末尾）。 */
function CanvasDropTarget({
  sections,
  children,
}: {
  sections: CompositeSectionDraft[];
  children: React.ReactNode;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: 'composite-canvas' });
  return (
    <div
      ref={setNodeRef}
      style={{
        outline: isOver && sections.length === 0 ? '2px dashed #1677ff' : undefined,
        outlineOffset: 4,
        borderRadius: 8,
      }}
    >
      {children}
    </div>
  );
}
