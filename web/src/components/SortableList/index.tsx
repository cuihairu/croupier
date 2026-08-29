import React from 'react';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { HolderOutlined } from '@ant-design/icons';

export interface SortableListProps<T> {
  items: T[];
  getKey: (item: T) => string;
  onReorder: (items: T[]) => void;
  /** 渲染每个条目；dragHandleProps 需挂到拖拽手柄元素上（通常配合 HolderOutlined） */
  children: (
    item: T,
    index: number,
    dragHandleProps: React.HTMLAttributes<HTMLElement>,
  ) => React.ReactNode;
  /**
   * true 时不渲染内部 DndContext，sortable 项注册到最近的祖先 DndContext——
   * 用于父级需要单一拖拽域同时处理「外部拖入」与「列表内重排」的场景
   * （如 CompositeBuilder：左栏函数拖到指定区块卡之后插入）。此时父级
   * 的 onDragEnd 需自行处理条目重排（active.id 是条目 key 的场景）。
   */
  externalDnd?: boolean;
}

export interface SortableItemProps {
  id: string;
  children: (dragHandleProps: React.HTMLAttributes<HTMLElement>) => React.ReactNode;
}

export function SortableItem({ id, children }: SortableItemProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id,
  });

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  const dragHandleProps: React.HTMLAttributes<HTMLElement> = {
    ...attributes,
    ...listeners,
    style: { cursor: 'grab', touchAction: 'none' },
  };

  return (
    <div ref={setNodeRef} style={style}>
      {children(dragHandleProps)}
    </div>
  );
}

export function SortableList<T>({
  items,
  getKey,
  onReorder,
  children,
  externalDnd = false,
}: SortableListProps<T>) {
  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIndex = items.findIndex((item) => getKey(item) === active.id);
    const newIndex = items.findIndex((item) => getKey(item) === over.id);
    if (oldIndex === -1 || newIndex === -1) return;
    const next = [...items];
    next.splice(newIndex, 0, next.splice(oldIndex, 1)[0]);
    onReorder(next);
  };

  const content = (
    <SortableContext items={items.map(getKey)} strategy={verticalListSortingStrategy}>
      {items.map((item, index) => (
        <SortableItem key={getKey(item)} id={getKey(item)}>
          {(dragHandleProps) => children(item, index, dragHandleProps)}
        </SortableItem>
      ))}
    </SortableContext>
  );

  if (externalDnd) return content;

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
      {content}
    </DndContext>
  );
}

export { HolderOutlined as DragHandle };
