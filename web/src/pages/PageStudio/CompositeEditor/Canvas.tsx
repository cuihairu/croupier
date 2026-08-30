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
  onEnterModal,
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
  /** 点击弹窗占位卡进入内部编辑（面包屑模式）。 */
  onEnterModal: (id: string) => void;
  canvasWidthRef: React.RefObject<HTMLDivElement | null>;
  /** SortableList 渲染的根级节点（含拖拽手柄 props 注入）。 */
  children: React.ReactNode;
}) {
  void onEnterModal;
  void fnById;
  void onDuplicate;
  void onSpanChange;

  return <>{tree.length === 0 ? <RootDropZone /> : children}</>;
}

/** 弹窗占位卡（栅格内）：droppable 拖入表单 + 双击/按钮进入内部编辑。 */
export function ModalPlaceholder({
  modal,
  selected,
  fnById,
  onSelect,
  onEnterModal,
}: {
  modal: PageNode;
  selected: boolean;
  fnById: Map<string, FunctionDescriptor>;
  onSelect: () => void;
  onEnterModal: () => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: `modal-drop:${modal.id}` });
  const kids = modal.children ?? [];
  return (
    <div
      ref={setNodeRef}
      onClick={(e) => {
        e.stopPropagation();
        onSelect();
      }}
      onDoubleClick={(e) => {
        e.stopPropagation();
        onEnterModal();
      }}
      style={{
        border: selected ? '1px solid #1677ff' : '1px dashed #b37feb',
        borderRadius: 8,
        padding: 12,
        cursor: 'pointer',
        background: isOver ? '#f6ffed' : '#faf5ff',
        minHeight: 100,
      }}
    >
      <Space direction="vertical" size={4} style={{ width: '100%' }}>
        <Badge
          color="purple"
          text={
            <Text strong style={{ fontSize: 13 }}>
              {String(modal.props.title ?? '弹窗')}
            </Text>
          }
        />
        {kids.length === 0 ? (
          <Text type="secondary" style={{ fontSize: 11 }}>
            空弹窗——拖入函数表单，或双击进入内部编辑
          </Text>
        ) : (
          kids.map((c) => {
            const fn = c.props.functionId ? fnById.get(String(c.props.functionId)) : undefined;
            return (
              <div
                key={c.id}
                style={{
                  border: '1px solid #e6d5f5',
                  borderRadius: 4,
                  padding: '3px 8px',
                  background: '#fff',
                }}
              >
                <Space size={6}>
                  <Tag color="green" style={{ marginRight: 0, fontSize: 11 }}>
                    {c.type === 'fnForm' ? '表单' : c.type}
                  </Tag>
                  <Text code style={{ fontSize: 11 }}>
                    {String(c.props.functionId ?? '')}
                  </Text>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    {fn?.summary?.['zh-CN'] ?? ''}
                  </Text>
                </Space>
              </div>
            );
          })
        )}
        <Button
          size="small"
          type="link"
          style={{ padding: 0 }}
          onClick={(e) => {
            e.stopPropagation();
            onEnterModal();
          }}
        >
          进入弹窗编辑 →
        </Button>
      </Space>
    </div>
  );
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
