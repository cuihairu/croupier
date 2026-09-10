import type React from 'react';
import { Badge, Button, Empty, Space, Tag, Typography } from 'antd';
import type { FunctionDescriptor } from '@/services/api/functions';
import { useDroppable } from '@dnd-kit/core';
import { localizedText } from '@/utils/localizedText';
import type { PageNode } from './model';

// CanvasNode 拆分至 ./CanvasNode（编辑装饰/右键菜单/调宽手柄）；此处再导出
// 保持 `import Canvas, { CanvasNode, ModalPlaceholder } from './Canvas'` 不变。
export { CanvasNode } from './CanvasNode';

const { Text } = Typography;

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
  /** 点击选中（透传鼠标事件：Shift+点击=多选切换，普通点击=单选）。 */
  onSelect: (e?: React.MouseEvent) => void;
  onEnterModal: () => void;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: `modal-drop:${modal.id}` });
  const kids = modal.children ?? [];
  return (
    <div
      ref={setNodeRef}
      onClick={(e) => {
        e.stopPropagation();
        onSelect(e);
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
      <Space orientation="vertical" size={4} style={{ width: '100%' }}>
        <Badge
          color="purple"
          text={
            <Space size={6}>
              <Text strong style={{ fontSize: 13 }}>
                {String(modal.props.title ?? '弹窗')}
              </Text>
              {typeof modal.props.sectionKey === 'string' && modal.props.sectionKey && (
                <Tag
                  style={{ marginRight: 0, fontSize: 11, fontFamily: 'monospace' }}
                  color="geekblue"
                >
                  ⌗{modal.props.sectionKey}
                </Tag>
              )}
            </Space>
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
                    {localizedText(fn?.summary, 'zh-CN')}
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
