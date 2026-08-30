import React, { useCallback, useRef } from 'react';
import { Badge, Button, Card, Space, Tag, Typography } from 'antd';
import {
  DeleteOutlined,
  DragOutlined,
  TableOutlined,
  BarsOutlined,
  FormOutlined,
  AppstoreOutlined,
  PlayCircleOutlined,
} from '@ant-design/icons';
import type { FunctionDescriptor } from '@/services/api/functions';
import { VIEW_META, type SectionDraft } from './types';
import SectionPreview from './SectionPreview';

const { Text } = Typography;

const VIEW_ICON: Record<string, React.ReactNode> = {
  table: <TableOutlined />,
  fields: <BarsOutlined />,
  form: <FormOutlined />,
  actions: <AppstoreOutlined />,
};

/**
 * 画布区块卡：真实组件渲染（SectionPreview）+ 编辑装饰
 * （选中框/拖拽手柄/右缘宽度拖拽/删除）。
 * preview=true 时不渲染任何编辑装饰（发布后形态）。
 */
export default function SectionCard({
  section,
  fn,
  selected,
  preview,
  data,
  running,
  onSelect,
  onDelete,
  onExecute,
  onSpanChange,
  dragHandleProps,
  depCount,
  downstreamCount,
  canvasWidthRef,
}: {
  section: SectionDraft;
  fn: FunctionDescriptor | undefined;
  selected: boolean;
  preview?: boolean;
  data?: unknown;
  running?: boolean;
  onSelect: () => void;
  onDelete: () => void;
  onExecute?: (params: Record<string, unknown>) => void;
  onSpanChange?: (span: number) => void;
  dragHandleProps: React.HTMLAttributes<HTMLElement>;
  depCount: number;
  downstreamCount: number;
  /** 画布像素宽度 ref，用于 span 拖拽换算 */
  canvasWidthRef: React.RefObject<HTMLDivElement | null>;
}) {
  // 右缘拖拽调宽：px → 24 栅格
  const resizing = useRef<{ startX: number; startSpan: number } | null>(null);
  const handleResizeDown = useCallback(
    (e: React.PointerEvent) => {
      e.preventDefault();
      e.stopPropagation();
      resizing.current = { startX: e.clientX, startSpan: section.span || 24 };
      const move = (ev: PointerEvent) => {
        if (!resizing.current || !canvasWidthRef.current) return;
        const total = canvasWidthRef.current.clientWidth || 1;
        const dx = ev.clientX - resizing.current.startX;
        const delta = Math.round((dx / total) * 24);
        const next = Math.min(24, Math.max(4, resizing.current.startSpan + delta));
        onSpanChange?.(next);
      };
      const up = () => {
        resizing.current = null;
        window.removeEventListener('pointermove', move);
        window.removeEventListener('pointerup', up);
      };
      window.addEventListener('pointermove', move);
      window.addEventListener('pointerup', up);
    },
    [section.span, onSpanChange, canvasWidthRef],
  );

  return (
    <div
      onClick={!preview ? onSelect : undefined}
      style={{ cursor: preview ? 'default' : 'pointer' }}
    >
      <Card
        size="small"
        style={{
          borderColor: !preview && selected ? '#1677ff' : undefined,
          boxShadow: !preview && selected ? '0 0 0 2px rgba(22,119,255,0.15)' : undefined,
          position: 'relative',
          height: '100%',
        }}
        title={
          <Space size={6}>
            {!preview && (
              <span
                {...dragHandleProps}
                onClick={(e) => e.stopPropagation()}
                style={{ cursor: 'grab', touchAction: 'none' }}
              >
                <DragOutlined style={{ color: selected ? '#1677ff' : '#999' }} />
              </span>
            )}
            {preview && section.autoRun && <PlayCircleOutlined style={{ color: '#52c41a' }} />}
            {!preview && VIEW_ICON[section.view]}
            <Text strong style={{ fontSize: 13 }}>
              {section.title || section.functionId}
            </Text>
          </Space>
        }
        extra={
          preview ? undefined : (
            <Space size={2} onClick={(e) => e.stopPropagation()}>
              {section.autoRun && (
                <Tag color="green" style={{ marginRight: 0 }}>
                  <PlayCircleOutlined /> 自动
                </Tag>
              )}
              {depCount > 0 && <Badge color="purple" text={`依赖 ${depCount}`} />}
              {downstreamCount > 0 && <Badge color="blue" text={`被引 ${downstreamCount}`} />}
              <Button
                size="small"
                type="text"
                danger
                icon={<DeleteOutlined />}
                onClick={onDelete}
              />
            </Space>
          )
        }
      >
        <SectionPreview
          section={section}
          fn={fn}
          data={data}
          running={running}
          interactive={preview}
          onExecute={onExecute}
        />
        {!preview && (
          <Text type="secondary" style={{ fontSize: 10 }}>
            {VIEW_META[section.view].label} · 拖右缘调宽 · 点击配置
          </Text>
        )}

        {/* 右缘宽度手柄 */}
        {!preview && (
          <div
            onPointerDown={handleResizeDown}
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
      </Card>
    </div>
  );
}
