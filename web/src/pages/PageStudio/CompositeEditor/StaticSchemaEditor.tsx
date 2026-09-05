import React, { useMemo, useState } from 'react';
import { Alert, Button, Input, Space, Typography, Upload, Radio } from 'antd';
import { DeleteOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons';
import * as XLSX from 'xlsx';

const { Text } = Typography;
const { TextArea } = Input;

/**
 * 常量表单字段编辑器（结构化）：
 * - 每个常量 = 一个下拉/控件：显示名（title）+ 变量名（key）+ 选项列表
 * - 导入 Excel/JSON 批量生成常量（选项来自游戏配置表的常见场景）：
 *   - 长表：名称 | 值 | 标签(可选)，同名称多行聚合为一个常量
 *   - 宽表：名称 | 选项1 | 选项2 | …
 *   - JSON：{"常量名": [选项…]} 或 [{"name":"常量名","options":[…]}]
 * - 变量名可在父组件实例中修改（下游 refreshOn/参数引用用）
 */

type FieldOption = { value: string; label?: string };

type FieldDef = {
  key: string;
  title: string;
  options: FieldOption[];
};

type ImportMode = 'long' | 'wide';

function parseSchema(value: string | undefined): FieldDef[] {
  if (!value || !value.trim()) return [];
  try {
    const obj = JSON.parse(value) as { properties?: Record<string, Record<string, unknown>> };
    const props = obj.properties ?? {};
    return Object.entries(props).map(([key, p]) => {
      const enumArr = Array.isArray(p.enum) ? (p.enum as unknown[]).map(String) : [];
      const namesArr = Array.isArray(p.enumNames) ? (p.enumNames as unknown[]).map(String) : [];
      return {
        key,
        title: typeof p.title === 'string' && p.title ? p.title : key,
        options: enumArr.map((v, i) => ({ value: v, label: namesArr[i] ?? v })),
      };
    });
  } catch {
    return [];
  }
}

function serializeSchema(fields: FieldDef[]): string {
  const properties = Object.fromEntries(
    fields.map((f) => [
      f.key,
      {
        type: 'string',
        title: f.title,
        ...(f.options.length > 0
          ? {
              enum: f.options.map((o) => o.value),
              ...(f.options.some((o) => o.label && o.label !== o.value)
                ? { enumNames: f.options.map((o) => o.label ?? o.value) }
                : {}),
            }
          : {}),
      },
    ]),
  );
  return JSON.stringify({ type: 'object', properties }, null, 2);
}

/** 行 → 常量定义：长表按名称聚合，宽表每行一个常量。 */
function rowsToFields(rows: unknown[][], mode: ImportMode): FieldDef[] {
  const out: FieldDef[] = [];
  if (mode === 'long') {
    const byName = new Map<string, FieldOption[]>();
    const order: string[] = [];
    for (const row of rows) {
      const name = String(row[0] ?? '').trim();
      const value = String(row[1] ?? '').trim();
      if (!name || !value) continue;
      const label =
        row[2] !== undefined && String(row[2]).trim() !== '' ? String(row[2]).trim() : undefined;
      if (!byName.has(name)) {
        byName.set(name, []);
        order.push(name);
      }
      byName.get(name)!.push({ value, label });
    }
    for (const name of order) {
      out.push({ key: name, title: name, options: byName.get(name)! });
    }
    return out;
  }
  for (const row of rows) {
    const name = String(row[0] ?? '').trim();
    if (!name) continue;
    const options = row
      .slice(1)
      .map((v) => String(v ?? '').trim())
      .filter(Boolean)
      .map((v) => ({ value: v }));
    if (options.length === 0) continue;
    out.push({ key: name, title: name, options });
  }
  return out;
}

function jsonToFields(parsed: unknown): FieldDef[] {
  if (Array.isArray(parsed)) {
    const first = parsed[0];
    if (first !== null && typeof first === 'object') {
      return (parsed as Array<Record<string, unknown>>)
        .map((item) => {
          const name = String(item.name ?? '').trim();
          const options = Array.isArray(item.options)
            ? (item.options as unknown[]).map((v) =>
                typeof v === 'object' && v !== null
                  ? {
                      value: String((v as Record<string, unknown>).value ?? ''),
                      label: String((v as Record<string, unknown>).label ?? ''),
                    }
                  : { value: String(v) },
              )
            : [];
          return { key: name, title: name, options };
        })
        .filter((f) => f.key && f.options.length > 0);
    }
    return [
      {
        key: 'field1',
        title: 'field1',
        options: (parsed as unknown[]).map((v) => ({ value: String(v) })),
      },
    ];
  }
  if (parsed && typeof parsed === 'object') {
    return Object.entries(parsed as Record<string, unknown>)
      .filter(([, v]) => Array.isArray(v))
      .map(([name, opts]) => ({
        key: name,
        title: name,
        options: (opts as unknown[]).map((v) =>
          typeof v === 'object' && v !== null
            ? {
                value: String((v as Record<string, unknown>).value ?? ''),
                label: String((v as Record<string, unknown>).label ?? ''),
              }
            : { value: String(v) },
        ),
      }))
      .filter((f) => f.options.length > 0);
  }
  return [];
}

export default function StaticSchemaEditor({
  value,
  onChange,
}: {
  value?: string;
  onChange: (v: string) => void;
}) {
  const [error, setError] = useState('');
  const [importMode, setImportMode] = useState<ImportMode>('long');
  const [showAdvanced, setShowAdvanced] = useState(false);
  // fields 以 value（JSON 字符串）为唯一事实源；本地列表缓存用于输入流畅性
  const [fields, setFields] = useState<FieldDef[] | null>(null);

  const fieldsOut = useMemo(() => fields ?? parseSchema(value), [fields, value]);

  const commit = (next: FieldDef[]) => {
    setFields(next);
    onChange(serializeSchema(next));
  };

  const updateField = (index: number, patch: Partial<FieldDef>) => {
    const next = (fieldsOut ?? []).map((f, i) => (i === index ? { ...f, ...patch } : f));
    commit(next);
  };

  const removeField = (index: number) => {
    commit((fieldsOut ?? []).filter((_, i) => i !== index));
  };

  const addField = () => {
    const base = '新常量';
    let key = base;
    let i = 1;
    const keys = new Set((fieldsOut ?? []).map((f) => f.key));
    while (keys.has(key)) key = `${base}${++i}`;
    commit([...(fieldsOut ?? []), { key, title: key, options: [{ value: '选项1' }] }]);
  };

  const importRows = (rows: unknown[][]) => {
    const imported = rowsToFields(rows, importMode);
    if (imported.length === 0) {
      setError('导入内容为空或格式不匹配');
      return;
    }
    // 追加：重名跳过；覆盖：直接替换
    const merged = [...(fieldsOut ?? [])];
    for (const f of imported) {
      const idx = merged.findIndex((x) => x.key === f.key);
      if (idx >= 0) merged[idx] = f;
      else merged.push(f);
    }
    commit(merged);
    setError('');
  };

  const beforeUpload = (file: File) => {
    const reader = new FileReader();
    if (/\.(xlsx|xls|csv)$/i.test(file.name)) {
      reader.onload = () => {
        try {
          const wb = XLSX.read(reader.result, { type: 'array' });
          const sheet = wb.Sheets[wb.SheetNames[0]];
          const rows = XLSX.utils.sheet_to_json<unknown[]>(sheet, { header: 1, defval: '' });
          importRows(rows);
        } catch (e) {
          setError(e instanceof Error ? e.message : 'Excel 解析失败');
        }
      };
      reader.readAsArrayBuffer(file);
    } else {
      reader.onload = () => {
        try {
          const parsed: unknown = JSON.parse(String(reader.result));
          const imported = jsonToFields(parsed);
          if (imported.length === 0) {
            setError('JSON 需为 {"常量名":[选项…]} 或 [{"name":"…","options":[…]}]');
            return;
          }
          commit(imported);
          setError('');
        } catch (e) {
          setError(e instanceof Error ? e.message : 'JSON 解析失败');
        }
      };
      reader.readAsText(file);
    }
    return Upload.LIST_IGNORE;
  };

  return (
    <div>
      <Space size={8} style={{ marginBottom: 8 }} wrap>
        <Button size="small" icon={<PlusOutlined />} onClick={addField}>
          添加常量
        </Button>
        <Radio.Group
          size="small"
          value={importMode}
          onChange={(e) => setImportMode(e.target.value as ImportMode)}
        >
          <Radio.Button value="long">长表：名称|值|标签</Radio.Button>
          <Radio.Button value="wide">宽表：名称|选项…</Radio.Button>
        </Radio.Group>
        <Upload accept=".xlsx,.xls,.csv,.json" showUploadList={false} beforeUpload={beforeUpload}>
          <Button size="small" icon={<UploadOutlined />}>
            导入配置
          </Button>
        </Upload>
        <Button size="small" onClick={() => setShowAdvanced((v) => !v)}>
          {showAdvanced ? '收起 JSON' : '高级：JSON'}
        </Button>
      </Space>

      {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 8 }} />}

      {(fieldsOut ?? []).length === 0 && (
        <Text type="secondary" style={{ fontSize: 12 }}>
          暂无常量——添加字段或导入配置（Excel/JSON）。每个常量生成一个下拉框，显示名=常量名。
        </Text>
      )}

      {(fieldsOut ?? []).map((f, i) => (
        <div
          key={`${f.key}:${i}`}
          style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: 8, marginBottom: 8 }}
        >
          <Space size={6} style={{ width: '100%' }} wrap>
            <Input
              size="small"
              addonBefore="显示名"
              value={f.title}
              onChange={(e) => updateField(i, { title: e.target.value })}
              style={{ width: 160 }}
            />
            <Input
              size="small"
              addonBefore="变量名"
              value={f.key}
              onChange={(e) => updateField(i, { key: e.target.value.trim() })}
              style={{ width: 160 }}
            />
            <Button
              size="small"
              type="text"
              danger
              icon={<DeleteOutlined />}
              onClick={() => removeField(i)}
            />
          </Space>
          <TextArea
            rows={2}
            size="small"
            style={{ marginTop: 6, fontSize: 12 }}
            value={f.options
              .map((o) => (o.label && o.label !== o.value ? `${o.value}|${o.label}` : o.value))
              .join('\n')}
            onChange={(e) => {
              const options = e.target.value
                .split('\n')
                .map((line) => line.trim())
                .filter(Boolean)
                .map((line) => {
                  const idx = line.indexOf('|');
                  return idx > 0
                    ? { value: line.slice(0, idx), label: line.slice(idx + 1) }
                    : { value: line };
                });
              updateField(i, { options });
            }}
            placeholder={'每行一个选项：值 或 值|标签'}
          />
          <Text type="secondary" style={{ fontSize: 11 }}>
            下游引用变量名：{f.key || '（未设置）'}
          </Text>
        </div>
      ))}

      {showAdvanced && (
        <>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            JSON Schema（高级，与此列表双向同步）
          </Text>
          <TextArea
            rows={8}
            style={{ fontFamily: 'monospace', fontSize: 12 }}
            value={value ?? ''}
            onChange={(e) => {
              setFields(null);
              onChange(e.target.value);
            }}
          />
        </>
      )}
    </div>
  );
}
