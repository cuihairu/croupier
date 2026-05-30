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
  children: (item: T, index: number) => React.ReactNode;
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

export function SortableList<T>({ items, getKey, onReorder, children }: SortableListProps<T>) {
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

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
      <SortableContext items={items.map(getKey)} strategy={verticalListSortingStrategy}>
        {items.map((item, index) => (
          <SortableItem key={getKey(item)} id={getKey(item)}>
            {(dragHandleProps) => children(item, index)}
          </SortableItem>
        ))}
      </SortableContext>
    </DndContext>
  );
}

export { HolderOutlined as DragHandle };
