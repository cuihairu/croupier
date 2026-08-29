import React from 'react';
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
import { VIEW_META, sectionParams, sectionOutputFields, type SectionDraft } from './types';

const { Text } = Typography;

const VIEW_ICON: Record<string, React.ReactNode> = {
  table: <TableOutlined />,
  fields: <BarsOutlined />,
  form: <FormOutlined />,
  actions: <AppstoreOutlined />,
};

/** 画布区块卡：结构化预览（列/字段来自函数 schema，非占位假框）。 */
export default function SectionCard({
  section,
  fn,
  selected,
  onSelect,
  onDelete,
  dragHandleProps,
  depCount,
  downstreamCount,
}: {
  section: SectionDraft;
  fn: FunctionDescriptor | undefined;
  selected: boolean;
  onSelect: () => void;
  onDelete: () => void;
  dragHandleProps: React.HTMLAttributes<HTMLElement>;
  depCount: number;
  downstreamCount: number;
}) {
  const params = sectionParams(fn);
  const outFields = sectionOutputFields(fn).slice(0, 6);

  return (
    <div onClick={onSelect} style={{ cursor: 'pointer' }}>
      <Card
        size="small"
        style={{
          borderColor: selected ? '#1677ff' : undefined,
          boxShadow: selected ? '0 0 0 2px rgba(22,119,255,0.15)' : undefined,
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
            {VIEW_ICON[section.view]}
            <Text strong style={{ fontSize: 13 }}>
              {section.title || section.functionId}
            </Text>
          </Space>
        }
        extra={
          <Space size={2} onClick={(e) => e.stopPropagation()}>
            {section.autoRun && (
              <Tag color="green" style={{ marginRight: 0 }}>
                <PlayCircleOutlined /> 自动
              </Tag>
            )}
            {depCount > 0 && <Badge color="purple" text={`依赖 ${depCount}`} />}
            {downstreamCount > 0 && <Badge color="blue" text={`被引 ${downstreamCount}`} />}
            <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={onDelete} />
          </Space>
        }
      >
        <Text type="secondary" style={{ fontSize: 11 }}>
          {section.functionId} · {VIEW_META[section.view].label}
          {section.span && section.span < 24 ? ` · 宽 ${section.span}/24` : ''}
        </Text>

        {/* 结构化预览：真实 schema 驱动 */}
        {section.view === 'table' && (
          <div
            style={{
              marginTop: 8,
              border: '1px solid #f0f0f0',
              borderRadius: 4,
              overflow: 'hidden',
            }}
          >
            <div
              style={{ display: 'flex', background: '#fafafa', borderBottom: '1px solid #f0f0f0' }}
            >
              {(outFields.length ? outFields : ['（无输出 schema）']).map((c) => (
                <Text key={c} strong style={{ fontSize: 11, padding: '4px 8px', flex: 1 }}>
                  {c}
                </Text>
              ))}
            </div>
            {[0, 1].map((r) => (
              <div
                key={r}
                style={{ display: 'flex', borderBottom: r === 0 ? '1px solid #f5f5f5' : undefined }}
              >
                {outFields.map((c) => (
                  <Text
                    key={c}
                    type="secondary"
                    style={{ fontSize: 11, padding: '4px 8px', flex: 1, color: '#d9d9d9' }}
                  >
                    —
                  </Text>
                ))}
              </div>
            ))}
          </div>
        )}

        {section.view === 'form' && params.length > 0 && (
          <div style={{ marginTop: 8 }}>
            <Space wrap size={6}>
              {params.slice(0, 8).map((p) => (
                <Tag key={p.name} style={{ fontSize: 11 }}>
                  {p.name}
                  {p.required ? ' *' : ''}
                </Tag>
              ))}
              {params.length > 8 && (
                <Text type="secondary" style={{ fontSize: 11 }}>
                  +{params.length - 8}
                </Text>
              )}
            </Space>
          </div>
        )}

        {section.view === 'fields' && outFields.length > 0 && (
          <div style={{ marginTop: 8 }}>
            <Space wrap size={6}>
              {outFields.map((f) => (
                <Tag key={f} style={{ fontSize: 11 }}>
                  {f}
                </Tag>
              ))}
            </Space>
          </div>
        )}

        {section.view === 'actions' && (
          <div style={{ marginTop: 8 }}>
            <Space size={6}>
              <Tag style={{ fontSize: 11 }}>执行按钮</Tag>
              <Text type="secondary" style={{ fontSize: 11 }}>
                点击即调用，无输入参数
              </Text>
            </Space>
          </div>
        )}
      </Card>
    </div>
  );
}
