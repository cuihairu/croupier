import React, { useMemo, useState } from 'react';
import {
  Button,
  Descriptions,
  Empty,
  Form,
  Input,
  InputNumber,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
} from 'antd';
import { PlayCircleOutlined } from '@ant-design/icons';
import type { FunctionDescriptor } from '@/services/api/functions';
import { schemaProperties, sectionParams, type SectionDraft } from './types';

const { Text } = Typography;

type JSONRecord = Record<string, unknown>;

/**
 * 区块真实渲染（编辑画布与预览模式共用）：
 * table=antd Table / fields=Descriptions / form=可交互表单 / actions=按钮。
 * data 存在时渲染真实数据（试跑或预览执行结果）。
 */
export default function SectionPreview({
  section,
  fn,
  data,
  running,
  onExecute,
  interactive,
}: {
  section: SectionDraft;
  fn: FunctionDescriptor | undefined;
  /** 试跑/预览执行结果（自动取 .data 或 .items） */
  data?: unknown;
  running?: boolean;
  /** 预览模式：表单可填写提交、按钮可点击 */
  interactive?: boolean;
  onExecute?: (params: JSONRecord) => void;
}) {
  const params = useMemo(() => sectionParams(fn), [fn]);
  const [values, setValues] = useState<JSONRecord>({});

  const payload = useMemo<JSONRecord>(() => {
    const raw = data as JSONRecord | undefined;
    if (!raw) return {};
    return (raw.data as JSONRecord) ?? raw;
  }, [data]);

  const items = useMemo<JSONRecord[]>(() => {
    const arr = payload?.items;
    return Array.isArray(arr) ? (arr as JSONRecord[]) : [];
  }, [payload]);

  const columns = useMemo(() => {
    const fromData = items.length ? Object.keys(items[0]) : schemaProperties(fn?.outputSchema);
    return fromData.slice(0, 8).map((k) => ({
      title: k,
      key: k,
      render: (_: unknown, row: JSONRecord) => (
        <Text style={{ fontSize: 12 }}>{formatCell(row[k])}</Text>
      ),
    }));
  }, [items, fn]);

  // ---------------------------------------------------------------- table
  if (section.view === 'table') {
    return (
      <Table
        size="small"
        rowKey={(_, i) => String(i)}
        loading={running}
        pagination={false}
        dataSource={items}
        columns={columns}
        locale={{
          emptyText: (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={
                <Text type="secondary" style={{ fontSize: 11 }}>
                  列来自函数输出 schema；试跑后此处显示真实数据
                </Text>
              }
            />
          ),
        }}
      />
    );
  }

  // --------------------------------------------------------------- fields
  if (section.view === 'fields') {
    const entries = Object.entries(payload ?? {}).filter(([k]) => k !== 'items' && k !== 'total');
    if (entries.length === 0) {
      return (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={
            <Text type="secondary" style={{ fontSize: 11 }}>
              输出字段：{schemaProperties(fn?.outputSchema).slice(0, 8).join(' / ') || '未知'}
              ；试跑后显示真实数据
            </Text>
          }
        />
      );
    }
    return (
      <Descriptions size="small" column={1} bordered>
        {entries.slice(0, 10).map(([k, v]) => (
          <Descriptions.Item key={k} label={<Text style={{ fontSize: 12 }}>{k}</Text>}>
            <Text style={{ fontSize: 12 }}>{formatCell(v)}</Text>
          </Descriptions.Item>
        ))}
      </Descriptions>
    );
  }

  // ----------------------------------------------------------------- form
  if (section.view === 'form') {
    return (
      <Form
        size="small"
        layout="vertical"
        component={false}
        disabled={!interactive}
        onFinish={() => onExecute?.(values)}
      >
        {params.map((p) => (
          <ParamInput
            key={p.name}
            name={p.name}
            required={p.required}
            schema={paramSchema(fn, p.name)}
            value={values[p.name]}
            onChange={(v) => setValues((prev) => ({ ...prev, [p.name]: v }))}
          />
        ))}
        {params.length === 0 && (
          <Text type="secondary" style={{ fontSize: 11 }}>
            该函数无输入参数
          </Text>
        )}
        {interactive && (
          <Button
            type="primary"
            size="small"
            htmlType="submit"
            icon={<PlayCircleOutlined />}
            loading={running}
            style={{ marginTop: 8 }}
          >
            执行
          </Button>
        )}
        {!interactive && params.length > 0 && (
          <div style={{ marginTop: 4 }}>
            <Text type="secondary" style={{ fontSize: 11 }}>
              字段来自函数输入 schema；预览模式可填写执行
            </Text>
          </div>
        )}
      </Form>
    );
  }

  // -------------------------------------------------------------- actions
  return (
    <Space>
      <Button
        size="small"
        type="primary"
        ghost={interactive ? undefined : true}
        disabled={!interactive}
        loading={running}
        icon={<PlayCircleOutlined />}
        onClick={() => onExecute?.({})}
      >
        {section.title || '执行'}
      </Button>
      {!interactive && (
        <Text type="secondary" style={{ fontSize: 11 }}>
          预览模式可点击执行
        </Text>
      )}
    </Space>
  );
}

/** 参数输入控件：按 schema 类型选择（string/number/boolean 兜底文本）。 */
function ParamInput({
  name,
  required,
  schema,
  value,
  onChange,
}: {
  name: string;
  required: boolean;
  schema: JSONRecord | null;
  value: unknown;
  onChange: (v: unknown) => void;
}) {
  const label = (
    <Space size={4}>
      <Text style={{ fontSize: 12 }}>{name}</Text>
      {required && (
        <Tag color="red" style={{ fontSize: 10, lineHeight: '16px' }}>
          必填
        </Tag>
      )}
    </Space>
  );
  const type = typeof schema?.type === 'string' ? schema.type : 'string';
  const common = { style: { width: '100%' } as React.CSSProperties };

  if (type === 'number' || type === 'integer') {
    return (
      <Form.Item key={name} label={label} required={required}>
        <InputNumber
          size="small"
          {...common}
          value={typeof value === 'number' ? value : undefined}
          onChange={(v) => onChange(v ?? 0)}
        />
      </Form.Item>
    );
  }
  if (type === 'boolean') {
    return (
      <Form.Item key={name} label={label} required={required}>
        <Switch size="small" checked={value === true} onChange={onChange} />
      </Form.Item>
    );
  }
  const en = schema?.enum;
  if (Array.isArray(en) && en.length > 0) {
    return (
      <Form.Item key={name} label={label} required={required}>
        <select
          className="ant-select-selection-item"
          style={{
            width: '100%',
            height: 24,
            fontSize: 12,
            borderColor: '#d9d9d9',
            borderRadius: 4,
          }}
          value={typeof value === 'string' ? value : ''}
          onChange={(e) => onChange(e.target.value)}
        >
          <option value="">请选择</option>
          {en.map((opt) => (
            <option key={String(opt)} value={String(opt)}>
              {String(opt)}
            </option>
          ))}
        </select>
      </Form.Item>
    );
  }
  return (
    <Form.Item key={name} label={label} required={required}>
      <Input
        size="small"
        {...common}
        value={typeof value === 'string' ? value : ''}
        onChange={(e) => onChange(e.target.value)}
        placeholder={typeof schema?.description === 'string' ? schema.description : undefined}
      />
    </Form.Item>
  );
}

function paramSchema(fn: FunctionDescriptor | undefined, name: string): JSONRecord | null {
  const schema = fn?.inputSchema as JSONRecord | undefined;
  const props = schema?.properties as JSONRecord | undefined;
  const p = props?.[name];
  return p && typeof p === 'object' && !Array.isArray(p) ? (p as JSONRecord) : null;
}

function formatCell(v: unknown): string {
  if (v === null || v === undefined) return '';
  if (typeof v === 'object') return JSON.stringify(v);
  return String(v);
}
