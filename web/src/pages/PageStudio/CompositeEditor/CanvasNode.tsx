import React, { useCallback, useRef } from 'react';
import { Button, Card, Col, Dropdown, Row, Space, Tag, Typography } from 'antd';
import { CopyOutlined, DeleteOutlined, DragOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import type { FunctionDescriptor } from '@/services/api/functions';
import { getComponent } from './registry';
import { parseAction } from './actions';
import { localizedText } from '@/utils/localizedText';
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
  /** 点击选中（透传鼠标事件：Shift+点击=多选切换，普通点击=单选）。 */
  onSelect: (e?: React.MouseEvent) => void;
  onDelete: () => void;
  onDuplicate: () => void;
  onSpanChange: (span: number) => void;
  /** 选择父容器（嵌套向上导航）。 */
  onSelectParent?: () => void;
  /** 上移/下移（右键菜单）。 */
  onMoveUp?: () => void;
  onMoveDown?: () => void;
  /** 保存为组件（右键菜单）：多选集合含本节点时保存集合，否则保存本节点子树。 */
  onSaveAsComponent?: () => void;
  /** 容器子节点交互：选中/删除/同级移动。 */
  selectedChildId?: string | null;
  onChildSelect?: (id: string) => void;
  onChildDelete?: (id: string) => void;
  onChildMove?: (id: string, dir: -1 | 1) => void;
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
  onSelectParent,
  onMoveUp,
  onMoveDown,
  onSaveAsComponent,
  selectedChildId,
  onChildSelect,
  onChildDelete,
  onChildMove,
  dragHandleProps,
  canvasWidthRef,
}) => {
  const intl = useIntl();
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
    <Dropdown
      trigger={['contextMenu']}
      menu={{
        items: [
          {
            key: 'up',
            label: intl.formatMessage({
              id: 'pages.pageStudio.editor.node.moveUp',
              defaultMessage: '上移',
            }),
            onClick: ({ domEvent }) => {
              domEvent.stopPropagation();
              onMoveUp?.();
            },
            disabled: !onMoveUp,
          },
          {
            key: 'down',
            label: intl.formatMessage({
              id: 'pages.pageStudio.editor.node.moveDown',
              defaultMessage: '下移',
            }),
            onClick: ({ domEvent }) => {
              domEvent.stopPropagation();
              onMoveDown?.();
            },
            disabled: !onMoveDown,
          },
          {
            key: 'parent',
            label: intl.formatMessage({
              id: 'pages.pageStudio.editor.node.selectParent',
              defaultMessage: '选择父容器',
            }),
            onClick: ({ domEvent }) => {
              domEvent.stopPropagation();
              onSelectParent?.();
            },
            disabled: !onSelectParent,
          },
          { type: 'divider' as const },
          {
            key: 'save-component',
            label: intl.formatMessage({
              id: 'pages.pageStudio.editor.node.saveAsComponent',
              defaultMessage: '保存为组件',
            }),
            onClick: ({ domEvent }) => {
              domEvent.stopPropagation();
              onSaveAsComponent?.();
            },
            disabled: !onSaveAsComponent,
          },
          {
            key: 'dup',
            label: intl.formatMessage({
              id: 'pages.pageStudio.editor.node.duplicate',
              defaultMessage: '复制',
            }),
            onClick: ({ domEvent }) => {
              domEvent.stopPropagation();
              onDuplicate();
            },
          },
          {
            key: 'del',
            label: intl.formatMessage({
              id: 'pages.pageStudio.editor.node.delete',
              defaultMessage: '删除',
            }),
            danger: true,
            onClick: ({ domEvent }) => {
              domEvent.stopPropagation();
              onDelete();
            },
          },
        ],
      }}
    >
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
              {typeof node.props.sectionKey === 'string' && node.props.sectionKey && (
                <Tag
                  style={{ marginRight: 0, fontSize: 11, fontFamily: 'monospace' }}
                  color="geekblue"
                >
                  ⌗{node.props.sectionKey}
                </Tag>
              )}
            </Space>
          }
          extra={
            <Space size={2} onClick={(e) => e.stopPropagation()}>
              {node.type === 'button' && (
                <Tag
                  color={parseAction(node.props.onClick) ? 'blue' : 'default'}
                  style={{ marginRight: 0, cursor: 'pointer', fontSize: 11 }}
                  onClick={(e) => {
                    e.stopPropagation();
                    onSelect();
                  }}
                >
                  {parseAction(node.props.onClick)
                    ? intl.formatMessage({
                        id: 'pages.pageStudio.editor.node.actionBound',
                        defaultMessage: '已绑定动作',
                      })
                    : intl.formatMessage({
                        id: 'pages.pageStudio.editor.node.bindActionHint',
                        defaultMessage: '点击绑定动作 →',
                      })}
                </Tag>
              )}
              {node.props.autoRun === true && (
                <Tag color="green" style={{ marginRight: 0 }}>
                  {intl.formatMessage({
                    id: 'pages.pageStudio.editor.node.autoRunTag',
                    defaultMessage: '自动',
                  })}
                </Tag>
              )}
              {node.type === 'fnForm' && node.props.display === 'dialog' && (
                <Tag color="purple" style={{ marginRight: 0 }}>
                  {intl.formatMessage({
                    id: 'pages.pageStudio.editor.node.dialogTag',
                    defaultMessage: '弹窗',
                  })}
                </Tag>
              )}
              <Button size="small" type="text" icon={<CopyOutlined />} onClick={onDuplicate} />
              <Button
                size="small"
                type="text"
                danger
                icon={<DeleteOutlined />}
                onClick={onDelete}
              />
            </Space>
          }
        >
          <Comp node={node} fn={fn} />
          {/* 子节点交互渲染仅 container（V1 单层容器）；tabs 的页 container 由
              Tabs Preview 内部渲染（页签交互），modal 不经 CanvasNode 渲染。 */}
          {node.type === 'container' && node.children && node.children.length > 0 && (
            <Row gutter={[8, 8]} style={{ marginTop: 8 }}>
              {node.children.map((c, ci) => {
                const cdef = getComponent(c.type);
                if (!cdef) return null;
                return (
                  <Col key={c.id} span={spanOf(c)}>
                    <Dropdown
                      trigger={['contextMenu']}
                      menu={{
                        items: [
                          {
                            key: 'up',
                            label: intl.formatMessage({
                              id: 'pages.pageStudio.editor.node.moveUp',
                              defaultMessage: '上移',
                            }),
                            disabled: ci === 0,
                            onClick: ({ domEvent }) => {
                              domEvent.stopPropagation();
                              onChildMove?.(c.id, -1);
                            },
                          },
                          {
                            key: 'down',
                            label: intl.formatMessage({
                              id: 'pages.pageStudio.editor.node.moveDown',
                              defaultMessage: '下移',
                            }),
                            disabled: ci === node.children!.length - 1,
                            onClick: ({ domEvent }) => {
                              domEvent.stopPropagation();
                              onChildMove?.(c.id, 1);
                            },
                          },
                          { type: 'divider' as const },
                          {
                            key: 'del',
                            label: intl.formatMessage({
                              id: 'pages.pageStudio.editor.node.delete',
                              defaultMessage: '删除',
                            }),
                            danger: true,
                            onClick: ({ domEvent }) => {
                              domEvent.stopPropagation();
                              onChildDelete?.(c.id);
                            },
                          },
                        ],
                      }}
                    >
                      <div
                        onClick={(e) => {
                          e.stopPropagation();
                          onChildSelect?.(c.id);
                        }}
                        style={{
                          border:
                            selectedChildId === c.id ? '1px solid #1677ff' : '1px dashed #d9d9d9',
                          borderRadius: 6,
                          padding: 6,
                          cursor: 'pointer',
                        }}
                      >
                        <cdef.Preview node={c} fn={undefined} />
                        <div style={{ textAlign: 'right' }}>
                          <Button
                            size="small"
                            type="text"
                            danger
                            style={{ height: 20, fontSize: 11 }}
                            onClick={(e) => {
                              e.stopPropagation();
                              onChildDelete?.(c.id);
                            }}
                          >
                            {intl.formatMessage({
                              id: 'pages.pageStudio.editor.node.delete',
                              defaultMessage: '删除',
                            })}
                          </Button>
                        </div>
                      </div>
                    </Dropdown>
                  </Col>
                );
              })}
            </Row>
          )}
          {depth === 0 && node.type !== 'modal' && node.type !== 'text' && node.type !== 'tabs' && (
            <Text type="secondary" style={{ fontSize: 10 }}>
              {intl.formatMessage({
                id: 'pages.pageStudio.editor.node.editHint',
                defaultMessage: '拖手柄排序 · 拖右缘调宽 · 点击配置',
              })}
            </Text>
          )}
        </Card>
        {/* 右缘宽度手柄（tabs 编译为整行页签组，无栅格语义，不提供调宽） */}
        {node.type !== 'tabs' && (
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
        )}
      </div>
    </Dropdown>
  );
};
