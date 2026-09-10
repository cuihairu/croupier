import React, { useMemo, useState } from 'react';
import { Alert, Button, Input, Space, Typography } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { fieldsToSchemaJson, schemaToFields, type ConstantField } from './constants';

const { Text } = Typography;
const { TextArea } = Input;

/**
 * 常量字段编辑器（实例属性面板用）：显示名/变量名/选项编辑。
 * 导入入口在组件库（ConstantImportModal）——实例不重复导入。
 */
export default function ConstantFieldsEditor({
  value,
  onChange,
}: {
  /** JSON Schema 字符串（staticSchema）。 */
  value?: string;
  onChange: (v: string) => void;
}) {
  const fields: ConstantField[] = useMemo(() => schemaToFields(value), [value]);
  const [advanced, setAdvanced] = useState(false);
  // 变量名编辑草稿（按已提交 key 键控）：输入不逐键提交，失焦时校验（非空/查重）
  // 后提交——逐键 trim+提交会让重复/空 key 在 schema 往返中塌缩成回跳。
  const [keyDrafts, setKeyDrafts] = useState<Record<string, string>>({});

  const commit = (next: ConstantField[]) => onChange(fieldsToSchemaJson(next));

  const updateField = (index: number, patch: Partial<ConstantField>) => {
    commit(fields.map((f, i) => (i === index ? { ...f, ...patch } : f)));
  };

  const removeField = (index: number) => {
    commit(fields.filter((_, i) => i !== index));
  };

  const addField = () => {
    const base = '新常量';
    let key = base;
    let i = 1;
    const keys = new Set(fields.map((f) => f.key));
    while (keys.has(key)) key = `${base}${++i}`;
    commit([...fields, { key, title: key, options: [{ value: '选项1' }] }]);
  };

  /** 变量名失焦提交：trim 后非空且不与其他行重复才落 schema，否则回退。 */
  const commitRename = (index: number) => {
    const f = fields[index];
    const draft = keyDrafts[f.key];
    if (draft === undefined) return;
    const next = draft.trim();
    const dup = fields.some((o, oi) => oi !== index && o.key === next);
    if (next && next !== f.key && !dup) updateField(index, { key: next });
    setKeyDrafts((prev) => {
      const nextDrafts = { ...prev };
      delete nextDrafts[f.key];
      return nextDrafts;
    });
  };

  return (
    <div>
      <div style={{ marginBottom: 8 }}>
        <Button size="small" icon={<PlusOutlined />} onClick={addField}>
          添加常量
        </Button>
        <Button size="small" style={{ marginLeft: 8 }} onClick={() => setAdvanced((v) => !v)}>
          {advanced ? '收起 JSON' : 'JSON'}
        </Button>
      </div>

      {fields.length === 0 && (
        <Text type="secondary" style={{ fontSize: 12 }}>
          暂无常量。导入常量请到组件库 Tab 的「导入常量」。
        </Text>
      )}

      {fields.map((f, i) => {
        const draft = keyDrafts[f.key];
        // 草稿即时校验（提示不阻断输入；失焦提交时拒绝非法值）
        const draftInvalid =
          draft !== undefined &&
          (draft.trim() === '' || fields.some((o, oi) => oi !== i && o.key === draft.trim()));
        return (
          <div
            key={i}
            style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: 8, marginBottom: 8 }}
          >
            <Space size={6} style={{ width: '100%' }} wrap>
              <Input
                size="small"
                addonBefore="显示名"
                value={f.title}
                onChange={(e) => updateField(i, { title: e.target.value })}
                style={{ width: 150 }}
              />
              <Input
                size="small"
                addonBefore="变量名"
                status={draftInvalid ? 'error' : undefined}
                value={draft ?? f.key}
                onChange={(e) => setKeyDrafts((prev) => ({ ...prev, [f.key]: e.target.value }))}
                onBlur={() => commitRename(i)}
                onPressEnter={() => commitRename(i)}
                style={{ width: 150 }}
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
            {draftInvalid ? (
              <Text type="danger" style={{ fontSize: 11, marginTop: 4 }}>
                变量名不能为空、且不能与其他常量重复
              </Text>
            ) : (
              <Text type="secondary" style={{ fontSize: 11 }}>
                下游引用变量名：{f.key || '（未设置）'}
              </Text>
            )}
          </div>
        );
      })}

      {advanced && (
        <>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            JSON Schema（高级，双向同步）
          </Text>
          <TextArea
            rows={8}
            style={{ fontFamily: 'monospace', fontSize: 12 }}
            value={value ?? ''}
            onChange={(e) => onChange(e.target.value)}
          />
        </>
      )}
    </div>
  );
}
