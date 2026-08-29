import React, { useCallback, useState } from 'react';
import { Button, Input, Space, Table, Typography, message } from 'antd';
import { CaretDownOutlined, CaretRightOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { invokeFunction } from '@/services/api/functions';
import { extractErrorMessage } from '@/utils/errors';
import { sectionParams, type SectionDraft } from './types';
import type { FunctionDescriptor } from '@/services/api/functions';

const { Text } = Typography;

type JSONRecord = Record<string, unknown>;

/** 试跑：按 inputSchema 生成参数表单，真实调用函数，展示结果。 */
export default function TryRunPanel({
  section,
  fn,
}: {
  section: SectionDraft | undefined;
  fn: FunctionDescriptor | undefined;
}) {
  const [open, setOpen] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<JSONRecord | null>(null);
  const [error, setError] = useState('');

  const params = sectionParams(fn);

  const run = useCallback(async () => {
    if (!section) return;
    setRunning(true);
    setError('');
    setResult(null);
    try {
      const payload: JSONRecord = {};
      for (const [k, raw] of Object.entries(values)) {
        if (raw === '') continue;
        try {
          payload[k] = JSON.parse(raw) as unknown;
        } catch {
          payload[k] = raw;
        }
      }
      const resp = await invokeFunction(section.functionId, payload as never);
      setResult((resp as unknown as JSONRecord) ?? {});
    } catch (err) {
      setError(extractErrorMessage(err, '调用失败'));
    } finally {
      setRunning(false);
    }
  }, [section, values]);

  if (!section) {
    return open ? (
      <div style={{ padding: '8px 16px', borderTop: '1px solid #f0f0f0' }}>
        <Text type="secondary">点击画布区块后可试跑</Text>
      </div>
    ) : null;
  }

  const data = (result?.data ?? result) as JSONRecord | undefined;
  const items = Array.isArray(data?.items) ? (data!.items as JSONRecord[]) : undefined;

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
          真实调用函数，验证参数与输出
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
                  style={{ width: 180 }}
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
              void run();
            }}
          >
            执行
          </Button>

          {error && <div style={{ marginTop: 8, color: '#ff4d4f', fontSize: 12 }}>{error}</div>}

          {items && items.length > 0 && (
            <Table<JSONRecord>
              size="small"
              style={{ marginTop: 8 }}
              rowKey={(_, i) => String(i)}
              pagination={{ pageSize: 5, size: 'small' }}
              dataSource={items}
              columns={Object.keys(items[0])
                .slice(0, 8)
                .map((k) => ({
                  title: k,
                  key: k,
                  render: (_: unknown, row: JSONRecord) => (
                    <Text style={{ fontSize: 12 }}>{String(row[k] ?? '')}</Text>
                  ),
                }))}
            />
          )}

          {data && !items && (
            <pre
              style={{
                marginTop: 8,
                background: '#fafafa',
                padding: 8,
                borderRadius: 4,
                fontSize: 12,
                maxHeight: 200,
                overflow: 'auto',
              }}
            >
              {JSON.stringify(data, null, 2)}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}
