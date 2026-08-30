import React, { useState } from 'react';
import { Button, Input, Space, Typography, message } from 'antd';
import { CaretDownOutlined, CaretRightOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { sectionParams, type SectionDraft } from './types';
import type { FunctionDescriptor } from '@/services/api/functions';

const { Text } = Typography;

/**
 * 试跑：编辑态下按 inputSchema 生成参数表单执行选中区块。
 * 结果不在此展示——直接渲染进画布对应区块（真实数据所见即所得）。
 */
export default function TryRunPanel({
  section,
  fn,
  onExecute,
  running,
}: {
  section: SectionDraft | undefined;
  fn: FunctionDescriptor | undefined;
  onExecute: (params: Record<string, unknown>) => void;
  running: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const params = sectionParams(fn);

  if (!section) {
    return open ? (
      <div style={{ padding: '8px 16px', borderTop: '1px solid #f0f0f0', background: '#fff' }}>
        <Text type="secondary">点击画布区块后可试跑</Text>
      </div>
    ) : null;
  }

  return (
    <div style={{ borderTop: '1px solid #f0f0f0', background: '#fff' }}>
      <div
        style={{
          padding: '6px 16px',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
        }}
        onClick={() => setOpen(!open)}
      >
        {open ? <CaretDownOutlined /> : <CaretRightOutlined />}
        <Text strong style={{ fontSize: 12 }}>
          试跑：{section.title || section.functionId}
        </Text>
        <Text type="secondary" style={{ fontSize: 11 }}>
          真实调用函数，结果显示在画布区块中
        </Text>
      </div>
      {open && (
        <div style={{ padding: '0 16px 12px' }}>
          {params.length > 0 && (
            <Space wrap size={8} style={{ marginBottom: 8 }}>
              {params.map((p) => (
                <Input
                  key={p.name}
                  size="small"
                  style={{ width: 190 }}
                  addonBefore={
                    <span style={{ fontSize: 11 }}>
                      {p.name}
                      {p.required ? ' *' : ''}
                    </span>
                  }
                  placeholder="值（JSON 或文本）"
                  value={values[p.name] ?? ''}
                  onChange={(e) => setValues((v) => ({ ...v, [p.name]: e.target.value }))}
                />
              ))}
            </Space>
          )}
          <Button
            size="small"
            type="primary"
            ghost
            icon={<PlayCircleOutlined />}
            loading={running}
            onClick={() => {
              if (params.some((p) => p.required && !(values[p.name] ?? '').trim())) {
                message.warning('有必填参数未填');
                return;
              }
              const payload: Record<string, unknown> = {};
              for (const [k, raw] of Object.entries(values)) {
                if (raw === '') continue;
                try {
                  payload[k] = JSON.parse(raw) as unknown;
                } catch {
                  payload[k] = raw;
                }
              }
              onExecute(payload);
            }}
          >
            执行
          </Button>
        </div>
      )}
    </div>
  );
}
