import { Form, Select, Space, Switch, Tag, Typography } from 'antd';
import type { FormFieldSpec, FormPresentationSpec, FormWidget } from '@/types/dashboard';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';

const { Text } = Typography;

const widgetOptions: Array<{ value: FormWidget; label: string }> = [
  { value: 'Input', label: '输入框' },
  { value: 'TextArea', label: '多行文本' },
  { value: 'InputNumber', label: '数字' },
  { value: 'Password', label: '密码' },
  { value: 'Select', label: '选择' },
  { value: 'MultiSelect', label: '多选' },
  { value: 'Switch', label: '开关' },
  { value: 'DatePicker', label: '日期' },
  { value: 'DateRange', label: '日期范围' },
  { value: 'Upload', label: '上传' },
  { value: 'Array', label: '数组' },
  { value: 'Object', label: '对象' },
];

export interface FormPresentationEditorProps {
  value: FormPresentationSpec;
  onChange: (value: FormPresentationSpec) => void;
  readonly?: boolean;
}

function updateField(
  fields: FormFieldSpec[] | undefined,
  index: number,
  updates: Partial<FormFieldSpec>,
): FormFieldSpec[] {
  return (fields || []).map((field, currentIndex) =>
    currentIndex === index ? { ...field, ...updates } : field,
  );
}

/** Edits only presentation metadata; JSON Schema and binding selectors remain immutable here. */
export default function FormPresentationEditor({
  value,
  onChange,
  readonly = false,
}: FormPresentationEditorProps) {
  const fields = value.fields || [];
  const applyField = (index: number, updates: Partial<FormFieldSpec>) =>
    onChange({ ...value, fields: updateField(value.fields, index, updates) });

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Text type="secondary">
        字段来自函数 JSON Schema；这里只调整展示，不改变输入结构、binding 或 selector。
      </Text>
      <Form layout="vertical" disabled={readonly}>
        <Form.Item label="布局">
          <Select
            value={value.layout || 'vertical'}
            onChange={(layout) => onChange({ ...value, layout })}
            options={[
              { value: 'vertical', label: '纵向' },
              { value: 'horizontal', label: '横向' },
              { value: 'inline', label: '行内' },
              { value: 'grid', label: '网格' },
            ]}
          />
        </Form.Item>
      </Form>
      {fields.length === 0 ? (
        <Tag>Schema 未生成可配置字段</Tag>
      ) : (
        fields.map((field, index) => (
          <Form
            key={field.key}
            layout="inline"
            disabled={readonly}
            style={{ alignItems: 'center' }}
          >
            <Form.Item label={<Text code>{field.key}</Text>}>
              <LocalizedTextEditor
                value={field.label}
                placeholder="字段标签"
                onChange={(label) => applyField(index, { label })}
              />
            </Form.Item>
            <Form.Item label="组件">
              <Select
                allowClear
                value={field.widget}
                style={{ width: 120 }}
                options={widgetOptions}
                onChange={(widget) => applyField(index, { widget })}
              />
            </Form.Item>
            <Form.Item label="占位">
              <LocalizedTextEditor
                value={field.placeholder}
                placeholder="占位提示"
                onChange={(placeholder) => applyField(index, { placeholder })}
              />
            </Form.Item>
            <Form.Item label="可见">
              <Switch
                checked={field.visible !== false}
                onChange={(visible) => applyField(index, { visible })}
              />
            </Form.Item>
            <Form.Item label="禁用">
              <Switch
                checked={Boolean(field.disabled)}
                onChange={(disabled) => applyField(index, { disabled })}
              />
            </Form.Item>
          </Form>
        ))
      )}
    </Space>
  );
}
