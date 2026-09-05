import React, { useMemo, useRef, useState } from 'react';
import { Alert, Button, Input, Space, Typography, Upload } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import * as XLSX from 'xlsx';

const { Text } = Typography;
const { TextArea } = Input;

/**
 * 常量表单 schema 编辑器：直接编辑 JSON Schema，或从 JSON / Excel 导入
 * 选项生成（常量下拉等场景：选项常来自游戏配置表）。
 *
 * 导入格式：
 * - JSON：完整 schema（{type:'object',properties:{…}}）或选项数组
 *   （["a","b"] 或 [{value,label}]，后者生成带标签的下拉）。
 * - Excel：首个工作表，第 1 列=值，第 2 列=标签（可选）。
 */

function optionsArrayToSchema(
  arr: unknown[],
  fieldKey: string,
  fieldTitle: string,
): Record<string, unknown> {
  const first = arr[0];
  if (first !== null && typeof first === 'object') {
    const pairs = arr as Array<Record<string, unknown>>;
    const values = pairs.map((p) => String(p.value));
    const labels = pairs.map((p) => String(p.label ?? p.value));
    return {
      type: 'object',
      properties: {
        [fieldKey]: {
          type: 'string',
          title: fieldTitle,
          enum: values,
          enumNames: labels,
        },
      },
    };
  }
  return {
    type: 'object',
    properties: {
      [fieldKey]: {
        type: 'string',
        title: fieldTitle,
        enum: arr.map((v) => String(v)),
      },
    },
  };
}

export default function StaticSchemaEditor({
  value,
  onChange,
}: {
  value?: string;
  onChange: (v: string) => void;
}) {
  const [error, setError] = useState('');
  const [importKey, setImportKey] = useState('field1');
  const fileRef = useRef<{ accept: '.json' | '.xlsx,.xls,.csv'; file?: File }>({ accept: '.json' });

  const parsed = useMemo((): { ok: boolean; fields: number; error: string } => {
    if (!value || !value.trim()) return { ok: false, fields: 0, error: '尚未定义字段' };
    try {
      const obj = JSON.parse(value) as { type?: string; properties?: Record<string, unknown> };
      if (obj.type !== 'object' || !obj.properties) {
        return { ok: false, fields: 0, error: '需要 {"type":"object","properties":{…}} 形态' };
      }
      return { ok: true, fields: Object.keys(obj.properties).length, error: '' };
    } catch (e) {
      return { ok: false, fields: 0, error: e instanceof Error ? e.message : 'JSON 解析失败' };
    }
  }, [value]);

  const applyOptions = (arr: unknown[]) => {
    if (arr.length === 0) {
      setError('导入内容为空');
      return;
    }
    const schema = optionsArrayToSchema(
      arr,
      importKey.trim() || 'field1',
      importKey.trim() || 'field1',
    );
    onChange(JSON.stringify(schema, null, 2));
    setError('');
  };

  const beforeUpload = (file: File) => {
    const isExcel = /\.(xlsx|xls|csv)$/i.test(file.name);
    const reader = new FileReader();
    if (isExcel) {
      reader.onload = () => {
        try {
          const wb = XLSX.read(reader.result, { type: 'array' });
          const sheet = wb.Sheets[wb.SheetNames[0]];
          const rows = XLSX.utils.sheet_to_json<unknown[]>(sheet, { header: 1, defval: '' });
          const options = rows
            .map((r) =>
              r.length > 1 ? { value: String(r[0]), label: String(r[1]) } : String(r[0] ?? ''),
            )
            .filter((v) => (typeof v === 'string' ? v.trim() !== '' : true));
          applyOptions(options);
        } catch (e) {
          setError(e instanceof Error ? e.message : 'Excel 解析失败');
        }
      };
      reader.readAsArrayBuffer(file);
    } else {
      reader.onload = () => {
        try {
          const parsed: unknown = JSON.parse(String(reader.result));
          if (Array.isArray(parsed)) {
            applyOptions(parsed);
          } else if (
            parsed &&
            typeof parsed === 'object' &&
            (parsed as { type?: string }).type === 'object'
          ) {
            onChange(JSON.stringify(parsed, null, 2));
            setError('');
          } else {
            setError('JSON 需为 object schema 或选项数组');
          }
        } catch (e) {
          setError(e instanceof Error ? e.message : 'JSON 解析失败');
        }
      };
      reader.readAsText(file);
    }
    // 阻止 antd 默认上传行为
    return Upload.LIST_IGNORE;
  };

  void fileRef;

  return (
    <div>
      <TextArea
        rows={12}
        style={{ fontFamily: 'monospace', fontSize: 12 }}
        value={value ?? ''}
        onChange={(e) => {
          onChange(e.target.value);
        }}
      />
      {!parsed.ok && parsed.error && (
        <Alert type="warning" showIcon message={parsed.error} style={{ marginTop: 6 }} />
      )}
      {parsed.ok && (
        <Text type="secondary" style={{ fontSize: 12 }}>
          已定义 {parsed.fields} 个字段；enum 字段渲染为下拉框
        </Text>
      )}
      {error && <Alert type="error" showIcon message={error} style={{ marginTop: 6 }} />}
      <Space size={8} style={{ marginTop: 8 }} wrap>
        <Input
          size="small"
          placeholder="导入后的字段 key"
          value={importKey}
          onChange={(e) => setImportKey(e.target.value)}
          style={{ width: 140 }}
        />
        <Upload
          accept=".json"
          showUploadList={false}
          beforeUpload={(file) => {
            fileRef.current = { accept: '.json', file };
            return beforeUpload(file);
          }}
        >
          <Button size="small" icon={<UploadOutlined />}>
            导入 JSON
          </Button>
        </Upload>
        <Upload
          accept=".xlsx,.xls,.csv"
          showUploadList={false}
          beforeUpload={(file) => {
            fileRef.current = { accept: '.xlsx,.xls,.csv', file };
            return beforeUpload(file);
          }}
        >
          <Button size="small" icon={<UploadOutlined />}>
            导入 Excel
          </Button>
        </Upload>
      </Space>
    </div>
  );
}
