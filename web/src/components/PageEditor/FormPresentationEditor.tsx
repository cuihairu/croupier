import { Card, Col, Form, Row, Select, Space, Switch, Tag, Typography } from 'antd';
import { HolderOutlined } from '@ant-design/icons';
import type { FormFieldSpec, FormPresentationSpec, FormWidget } from '@/types/dashboard';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';
import { SortableList } from '@/components/SortableList';

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

/** Edits only presentation metadata; JSON Schema and binding selectors remain immutable here.
 *  拖拽直接重排 fields 数组——渲染端按数组顺序生成 ui:order，保存后顺序即生效。 */
export default function FormPresentationEditor({
  value,
  onChange,
  readonly = false,
}: FormPresentationEditorProps) {
  const fields = value.fields || [];
  const applyField = (index: number, updates: Partial<FormFieldSpec>) =>
    onChange({ ...value, fields: updateField(value.fields, index, updates) });

  return (
    <Space orientation="vertical" size={12} style={{ width: '100%' }}>
      <Text type="secondary">
        字段来自函数 JSON Schema；这里只调整展示（拖动 ⠿ 调整顺序），不改变输入结构、binding 或
        selector。
      </Text>
      <Form layout="vertical" disabled={readonly}>
        <Form.Item label="布局" style={{ marginBottom: 0 }}>
          <Select
            value={value.layout || 'vertical'}
            onChange={(layout) => onChange({ ...value, layout })}
            style={{ maxWidth: 200 }}
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
        <SortableList
          items={fields}
          getKey={(field) => field.key}
          onReorder={(next) => onChange({ ...value, fields: next })}
        >
          {(field, index, dragHandleProps) => (
            <Card
              size="small"
              title={
                <Space size={12} wrap>
                  {!readonly && (
                    <span {...dragHandleProps}>
                      <HolderOutlined />
                    </span>
                  )}
                  <Text code>{field.key}</Text>
                  <Select
                    size="small"
                    allowClear
                    placeholder="组件"
                    value={field.widget}
                    style={{ width: 110 }}
                    options={widgetOptions}
                    onChange={(widget) => applyField(index, { widget })}
                  />
                  <Space size={4}>
                    <Text type="secondary">可见</Text>
                    <Switch
                      size="small"
                      checked={field.visible !== false}
                      onChange={(visible) => applyField(index, { visible })}
                    />
                  </Space>
                  <Space size={4}>
                    <Text type="secondary">禁用</Text>
                    <Switch
                      size="small"
                      checked={Boolean(field.disabled)}
                      onChange={(disabled) => applyField(index, { disabled })}
                    />
                  </Space>
                </Space>
              }
            >
              <Form layout="vertical" disabled={readonly} style={{ marginBottom: 0 }}>
                <Row gutter={12}>
                  <Col span={12}>
                    <Form.Item label="标签" style={{ marginBottom: 0 }}>
                      <LocalizedTextEditor
                        value={field.label}
                        placeholder="字段标签"
                        onChange={(label) => applyField(index, { label })}
                      />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="占位" style={{ marginBottom: 0 }}>
                      <LocalizedTextEditor
                        value={field.placeholder}
                        placeholder="占位提示"
                        onChange={(placeholder) => applyField(index, { placeholder })}
                      />
                    </Form.Item>
                  </Col>
                </Row>
              </Form>
            </Card>
          )}
        </SortableList>
      )}
    </Space>
  );
}
