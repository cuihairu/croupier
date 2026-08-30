import React, { useCallback, useRef } from 'react';
import { Badge, Button, Card, Col, Empty, Row, Space, Tag, Typography } from 'antd';
import { CopyOutlined, DeleteOutlined, DragOutlined } from '@ant-design/icons';
import type { FunctionDescriptor } from '@/services/api/functions';
import { useDroppable } from '@dnd-kit/core';
import { getComponent } from './registry';
import { acceptsChild } from './registry';
import type { PageNode } from './model';

const { Text } = Typography;

function spanOf(node: PageNode): number {
  const raw = Number(node.props.span ?? 24);
  return Number.isFinite(raw) && raw >= 4 && raw <= 24 ? raw : 24;
}

export interface CanvasNodeProps {
  node: PageNode;
  fn: FunctionDescriptor | undefined;
  selected: boolean;
  depth: number;
  onSelect: () => void;
  onDelete: () => void;
  onDuplicate: () => void;
  onSpanChange: (span: number) => void;
  dragHandleProps: React.HTMLAttributes<HTMLElement>;
  canvasWidthRef: React.RefObject<HTMLDivElement | null>;
}

/** 画布节点：真实 Preview + 编辑装饰（选中框/拖拽手柄/右缘调宽/操作条）。 */
export const CanvasNode: React.FC<CanvasNodeProps> = ({
  node,
  fn,
  selected,
  depth,
  onSelect,
  onDelete,
  onDuplicate,
  onSpanChange,
  dragHandleProps,
  canvasWidthRef,
}) => {
  const def = getComponent(node.type);
  const Comp = def?.Preview;

  // 右缘拖拽调宽：px → 24 栅格
  const resizeRef = useRef<{ startX: number; startSpan: number } | null>(null);
  const onResizeDown = useCallback(
    (e: React.PointerEvent) => {
      e.preventDefault();
      e.stopPropagation();
      resizeRef.current = { startX: e.clientX, startSpan: spanOf(node) };
      const move = (ev: PointerEvent) => {
        if (!resizeRef.current || !canvasWidthRef.current) return;
        const total = canvasWidthRef.current.clientWidth || 1;
        const dx = ev.clientX - resizeRef.current.startX;
        const delta = Math.round((dx / total) * 24);
        onSpanChange(Math.min(24, Math.max(4, resizeRef.current.startSpan + delta)));
      };
      const up = () => {
        resizeRef.current = null;
        window.removeEventListener('pointermove', move);
        window.removeEventListener('pointerup', up);
      };
      window.addEventListener('pointermove', move);
      window.addEventListener('pointerup', up);
    },
    [node, onSpanChange, canvasWidthRef],
  );

  if (!Comp) return null;

  return (
    <div onClick={onSelect} style={{ cursor: 'pointer', position: 'relative' }}>
      <Card
        size="small"
        style={{
          borderColor: selected ? '#1677ff' : undefined,
          boxShadow: selected ? '0 0 0 2px rgba(22,119,255,0.15)' : undefined,
          height: '100%',
        }}
        title={
          <Space size={6}>
            <span
              {...dragHandleProps}
              onClick={(e) => e.stopPropagation()}
              style={{ cursor: 'grab', touchAction: 'none' }}
            >
              <DragOutlined style={{ color: selected ? '#1677ff' : '#999' }} />
            </span>
            {def?.icon}
            <Text strong style={{ fontSize: 13 }}>
              {String(node.props.title ?? node.props.content ?? def?.name ?? node.type)}
            </Text>
          </Space>
        }
        extra={
          <Space size={2} onClick={(e) => e.stopPropagation()}>
            {node.props.autoRun === true && (
              <Tag color="green" style={{ marginRight: 0 }}>
                自动
              </Tag>
            )}
            {node.type === 'fnForm' && node.props.display === 'dialog' && (
              <Tag color="purple" style={{ marginRight: 0 }}>
                弹窗
              </Tag>
            )}
            <Button size="small" type="text" icon={<CopyOutlined />} onClick={onDuplicate} />
            <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={onDelete} />
          </Space>
        }
      >
        <Comp node={node} fn={fn} />
        {node.children && node.children.length > 0 && (
          <Row gutter={[8, 8]} style={{ marginTop: 8 }}>
            {node.children.map((c) => {
              const cdef = getComponent(c.type);
              if (!cdef) return null;
              return (
                <Col key={c.id} span={spanOf(c)}>
                  <div
                    onClick={(e) => {
                      e.stopPropagation();
                    }}
                    style={{ border: '1px dashed #d9d9d9', borderRadius: 6, padding: 6 }}
                  >
                    <cdef.Preview node={c} fn={undefined} />
                  </div>
                </Col>
              );
            })}
          </Row>
        )}
        {depth === 0 && node.type !== 'modal' && node.type !== 'text' && (
          <Text type="secondary" style={{ fontSize: 10 }}>
            拖手柄排序 · 拖右缘调宽 · 点击配置
          </Text>
        )}
      </Card>
      {/* 右缘宽度手柄 */}
      <div
        onPointerDown={onResizeDown}
        onClick={(e) => e.stopPropagation()}
        style={{
          position: 'absolute',
          top: 0,
          right: -5,
          width: 8,
          height: '100%',
          cursor: 'col-resize',
          zIndex: 2,
        }}
      />
    </div>
  );
};

/** 画布：根级栅格渲染 + 弹窗收纳区。拖拽上下文由父级提供（T2.2/T2.3）。 */
export default function Canvas({
  tree,
  selectedId,
  fnById,
  onSelect,
  onDelete,
  onDuplicate,
  onSpanChange,
  canvasWidthRef,
  children,
}: {
  tree: PageNode[];
  selectedId: string | null;
  fnById: Map<string, FunctionDescriptor>;
  onSelect: (id: string) => void;
  onDelete: (id: string) => void;
  onDuplicate: (id: string) => void;
  onSpanChange: (id: string, span: number) => void;
  canvasWidthRef: React.RefObject<HTMLDivElement | null>;
  /** SortableList 渲染的根级节点（含拖拽手柄 props 注入）。 */
  children: React.ReactNode;
}) {
  const modals = tree.filter((n) => n.type === 'modal');

  return (
    <>
      {tree.filter((n) => n.type !== 'modal').length === 0 && modals.length === 0 ? (
        <RootDropZone />
      ) : (
        children
      )}

      {modals.length > 0 && (
        <div style={{ marginTop: 12, borderTop: '1px dashed #d9d9d9', paddingTop: 8 }}>
          <Space size={8}>
            <Text strong style={{ fontSize: 12 }}>
              弹窗（{modals.length}）
            </Text>
            <Text type="secondary" style={{ fontSize: 11 }}>
              由按钮/行操作触发打开；V1 弹窗内放一个函数表单
            </Text>
          </Space>
          <Row gutter={[8, 8]} style={{ marginTop: 8 }}>
            {modals.map((m) => {
              const def = getComponent(m.type);
              return (
                <Col key={m.id} span={8}>
                  <ModalDropZone
                    modalId={m.id}
                    selected={selectedId === m.id}
                    title={String(m.props.title ?? '弹窗')}
                    childSummary={
                      m.children?.length
                        ? `内容：${m.children.map((c) => String(c.props.functionId ?? c.type)).join(', ')}`
                        : '空弹窗——从函数组件拖入表单'
                    }
                    onSelect={() => onSelect(m.id)}
                    child={(() => {
                      const c = m.children?.[0];
                      if (!c) return undefined;
                      const fn = fnById.get(String(c.props.functionId ?? ''));
                      return { node: c, fnSummary: String(fn?.summary?.['zh-CN'] ?? fn?.id ?? '') };
                    })()}
                  />
                </Col>
              );
            })}
          </Row>
        </div>
      )}
    </>
  );
}

/** modal 收纳区：droppable（拖 fnForm 装入）。 */
function ModalDropZone({
  modalId,
  selected,
  title,
  childSummary,
  onSelect,
  child,
}: {
  modalId: string;
  selected: boolean;
  title: string;
  childSummary: string;
  onSelect: () => void;
  child?: { node: PageNode; fnSummary: string };
}) {
  const { setNodeRef, isOver } = useDroppable({ id: `modal-drop:${modalId}` });
  return (
    <div
      ref={setNodeRef}
      onClick={(e) => {
        e.stopPropagation();
        onSelect();
      }}
      style={{
        border: selected ? '1px solid #1677ff' : '1px dashed #bbb',
        borderRadius: 6,
        padding: '6px 10px',
        cursor: 'pointer',
        background: isOver ? '#f6ffed' : '#fff',
      }}
    >
      <Badge
        color="purple"
        text={
          <Text strong style={{ fontSize: 12 }}>
            {title}
          </Text>
        }
      />
      {child ? (
        <div
          onClick={(e) => {
            e.stopPropagation();
            onSelectChild(child.node.id);
          }}
          style={{
            marginTop: 6,
            border: '1px solid #e6d5f5',
            borderRadius: 4,
            padding: '4px 8px',
            background: '#faf5ff',
          }}
        >
          <Space size={6}>
            <Tag color="green" style={{ marginRight: 0, fontSize: 11 }}>
              表单
            </Tag>
            <Text code style={{ fontSize: 11 }}>
              {String(child.node.props.functionId ?? '')}
            </Text>
          </Space>
          <div>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {child.fnSummary}（点击配置）
            </Text>
          </div>
        </div>
      ) : (
        <div>
          <Text type="secondary" style={{ fontSize: 11 }}>
            {childSummary}
          </Text>
        </div>
      )}
    </div>
  );
}

/** modal 收纳区选中子节点回调（经由 ModalDropZone child 卡片点击）。 */
function onSelectChild(id: string): void {
  // Canvas 外部通过 onSelect prop 已能选中任意 id（PropsPanel 全树查找）
  window.dispatchEvent(new CustomEvent('composite-select-node', { detail: id }));
}

/** 空画布根落区：droppable('canvas-root')。 */
function RootDropZone() {
  const { setNodeRef, isOver } = useDroppable({ id: 'canvas-root' });
  return (
    <div
      ref={setNodeRef}
      style={{
        marginTop: 100,
        textAlign: 'center',
        padding: '40px 0',
        border: isOver ? '2px dashed #1677ff' : '1px dashed #d9d9d9',
        borderRadius: 8,
        background: isOver ? '#f0f7ff' : 'transparent',
      }}
    >
      从左侧点击或拖入组件，开始搭建页面
    </div>
  );
}
