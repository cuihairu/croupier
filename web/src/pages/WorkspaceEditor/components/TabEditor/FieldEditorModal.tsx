import React from 'react';
import {
  Modal,
  Form,
  Input,
  Select,
  Switch,
  Button,
  Space,
  Divider,
  InputNumber,
  Tooltip,
} from 'antd';
import {
  MinusCircleOutlined,
  PlusOutlined,
  QuestionCircleOutlined,
  HolderOutlined,
} from '@ant-design/icons';
import {
  DndContext,
  closestCenter,
  PointerSensor,
  KeyboardSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { FieldConfig, FieldRule } from '@/types/workspace';
import { HelpTooltip } from '../HelpTooltip';

export interface FieldEditorModalProps {
  open: boolean;
  editingField: FieldConfig | null;
  onOk: (values: FieldConfig) => void;
  onCancel: () => void;
}

const FIELD_TYPES = [
  { value: 'input', label: '文本输入' },
  { value: 'number', label: '数字输入' },
  { value: 'select', label: '下拉选择' },
  { value: 'radio', label: '单选框' },
  { value: 'checkbox', label: '多选框' },
  { value: 'date', label: '日期' },
  { value: 'datetime', label: '日期时间' },
  { value: 'textarea', label: '多行文本' },
  { value: 'switch', label: '开关' },
];

const RULE_TYPES: Array<{ value: NonNullable<FieldRule['type']>; label: string }> = [
  { value: 'string', label: '字符串' },
  { value: 'number', label: '数值' },
  { value: 'email', label: '邮箱' },
  { value: 'url', label: 'URL' },
  { value: 'pattern', label: '正则表达式' },
];

const TYPES_WITH_OPTIONS = ['select', 'radio', 'checkbox'];
const TYPES_WITH_PLACEHOLDER = ['input', 'number', 'select', 'date', 'datetime', 'textarea'];

/** 内置表达式函数 */
const EXPRESSION_FUNCTIONS = [
  { key: '$now()', label: '$now()', desc: '当前日期时间' },
  { key: '$today()', label: '$today()', desc: '当前日期' },
  { key: '$user.id', label: '$user.id', desc: '当前用户 ID' },
  { key: '$user.name', label: '$user.name', desc: '当前用户名' },
  { key: '$query.key', label: '$query.key', desc: 'URL 查询参数' },
  { key: '$localStorage.key', label: '$localStorage.key', desc: '本地存储' },
  { key: '$uuid()', label: '$uuid()', desc: '生成 UUID' },
  { key: '$timestamp()', label: '$timestamp()', desc: '当前时间戳' },
];

/** 表达式函数列表内容 */
const ExpressionHelpContent = () => (
  <div style={{ fontSize: 12 }}>
    <div style={{ marginBottom: 4, fontWeight: 500 }}>可用表达式：</div>
    <ul style={{ margin: 0, paddingLeft: 16 }}>
      {EXPRESSION_FUNCTIONS.map((fn) => (
        <li key={fn.key}>
          <code>{fn.key}</code> - {fn.desc}
        </li>
      ))}
    </ul>
  </div>
);

/** 根据字段类型和标签生成推荐占位符 */
function suggestPlaceholder(type?: string, label?: string): string {
  const name = label || '内容';
  switch (type) {
    case 'input':
    case 'textarea':
      return `请输入${name}`;
    case 'number':
      return `请输入${name}`;
    case 'select':
      return `请选择${name}`;
    case 'date':
      return '请选择日期';
    case 'datetime':
      return '请选择日期时间';
    default:
      return '';
  }
}

function SortableOptionRow({
  id,
  children,
}: {
  id: string;
  children: (dragHandleProps: React.HTMLAttributes<HTMLElement>) => React.ReactNode;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id,
  });
  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };
  const dragHandleProps: React.HTMLAttributes<HTMLElement> = {
    ...attributes,
    ...listeners,
    style: { cursor: 'grab', touchAction: 'none' },
  };
  return (
    <div ref={setNodeRef} style={style}>
      {children(dragHandleProps)}
    </div>
  );
}

export default function FieldEditorModal({
  open,
  editingField,
  onOk,
  onCancel,
}: FieldEditorModalProps) {
  const [form] = Form.useForm();
  const [useExpression, setUseExpression] = React.useState(false);
  const fieldType = Form.useWatch('type', form);
  const fieldLabel = Form.useWatch('label', form);

  const dndSensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleOptionDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const options: any[] = form.getFieldValue('options') || [];
    const oldIdx = options.findIndex((_: any, i: number) => `opt-${i}` === active.id);
    const newIdx = options.findIndex((_: any, i: number) => `opt-${i}` === over.id);
    if (oldIdx === -1 || newIdx === -1) return;
    const reordered = [...options];
    const [moved] = reordered.splice(oldIdx, 1);
    reordered.splice(newIdx, 0, moved);
    form.setFieldValue('options', reordered);
  };

  React.useEffect(() => {
    if (open && editingField) {
      form.setFieldsValue(editingField);
      setUseExpression(!!editingField.defaultValueExpression);
    } else if (open) {
      form.resetFields();
      setUseExpression(false);
    }
  }, [open, editingField, form]);

  const handleOk = async () => {
    const values = await form.validateFields();
    // 清理不适用的字段
    if (!TYPES_WITH_OPTIONS.includes(values.type)) {
      delete values.options;
    }
    if (!TYPES_WITH_PLACEHOLDER.includes(values.type)) {
      delete values.placeholder;
    }
    // 处理默认值表达式互斥
    if (useExpression) {
      delete values.defaultValue;
    } else {
      delete values.defaultValueExpression;
    }
    onOk(values);
    form.resetFields();
  };

  const handleCancel = () => {
    form.resetFields();
    onCancel();
  };

  const applyPlaceholderSuggestion = () => {
    const suggestion = suggestPlaceholder(fieldType, fieldLabel);
    if (suggestion) {
      form.setFieldValue('placeholder', suggestion);
    }
  };

  const showOptions = TYPES_WITH_OPTIONS.includes(fieldType);
  const showPlaceholder = TYPES_WITH_PLACEHOLDER.includes(fieldType);

  return (
    <Modal
      title={editingField ? '编辑字段' : '添加字段'}
      open={open}
      onOk={handleOk}
      onCancel={handleCancel}
      width={560}
    >
      <Form form={form} layout="vertical" size="small">
        {/* 基础信息 */}
        <Form.Item
          name="key"
          label={
            <Space size={4}>
              <span>字段名</span>
              <HelpTooltip helpKey="field.key" />
            </Space>
          }
          rules={[{ required: true, message: '请输入字段名' }]}
        >
          <Input placeholder="如: playerId" />
        </Form.Item>
        <Form.Item
          name="label"
          label={
            <Space size={4}>
              <span>字段标签</span>
              <HelpTooltip helpKey="field.label" />
            </Space>
          }
          rules={[{ required: true, message: '请输入字段标签' }]}
        >
          <Input placeholder="如: 玩家ID" />
        </Form.Item>
        <Form.Item
          name="type"
          label={
            <Space size={4}>
              <span>字段类型</span>
              <HelpTooltip helpKey="field.type" />
            </Space>
          }
          rules={[{ required: true, message: '请选择字段类型' }]}
        >
          <Select options={FIELD_TYPES} />
        </Form.Item>

        <Space style={{ width: '100%' }} size={16}>
          <Form.Item
            name="required"
            label={
              <Space size={4}>
                <span>必填</span>
                <HelpTooltip helpKey="field.required" />
              </Space>
            }
            valuePropName="checked"
            style={{ marginBottom: 8 }}
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name="disabled"
            label="禁用"
            valuePropName="checked"
            style={{ marginBottom: 8 }}
          >
            <Switch />
          </Form.Item>
        </Space>

        {/* 占位符 - 带智能推荐 */}
        {showPlaceholder && (
          <Form.Item
            label={
              <Space size={4}>
                <span>占位符</span>
                <HelpTooltip helpKey="field.placeholder" />
              </Space>
            }
            style={{ marginBottom: 8 }}
          >
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name="placeholder" noStyle>
                <Input
                  style={{ width: 'calc(100% - 70px)' }}
                  placeholder={suggestPlaceholder(fieldType, fieldLabel) || '请输入占位符'}
                />
              </Form.Item>
              <Tooltip title="自动生成推荐占位符">
                <Button onClick={applyPlaceholderSuggestion}>推荐</Button>
              </Tooltip>
            </Space.Compact>
          </Form.Item>
        )}

        {/* 默认值 */}
        <Form.Item
          label={
            <Space size={4}>
              <span>默认值</span>
              <HelpTooltip helpKey="field.defaultValue" />
            </Space>
          }
        >
          <Space.Compact style={{ width: '100%' }}>
            <Form.Item name="defaultValue" noStyle>
              {fieldType === 'number' ? (
                <InputNumber
                  style={{ width: '100%' }}
                  placeholder="默认数值"
                  disabled={useExpression}
                />
              ) : fieldType === 'switch' ? (
                <Switch disabled={useExpression} />
              ) : (
                <Input placeholder="默认值" disabled={useExpression} />
              )}
            </Form.Item>
            <Tooltip title={<ExpressionHelpContent />}>
              <Button
                type={useExpression ? 'primary' : 'default'}
                onClick={() => setUseExpression(!useExpression)}
              >
                表达式
              </Button>
            </Tooltip>
          </Space.Compact>
        </Form.Item>

        {/* 默认值表达式 */}
        {useExpression && (
          <Form.Item
            name="defaultValueExpression"
            label={
              <Space size={4}>
                <span>表达式</span>
                <Tooltip title={<ExpressionHelpContent />}>
                  <QuestionCircleOutlined style={{ color: '#999' }} />
                </Tooltip>
              </Space>
            }
            rules={[{ required: useExpression, message: '请输入表达式' }]}
          >
            <Select
              mode="tags"
              placeholder="输入表达式，如 $now()、$user.id"
              options={EXPRESSION_FUNCTIONS.map((fn) => ({
                label: `${fn.key} - ${fn.desc}`,
                value: fn.key,
              }))}
              tokenSeparators={[' ']}
              filterOption={(input, option) =>
                (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
        )}

        {/* 帮助提示 */}
        <Form.Item
          name="tooltip"
          label={
            <Space size={4}>
              <span>帮助提示</span>
              <HelpTooltip helpKey="field.tooltip" />
            </Space>
          }
        >
          <Input.TextArea
            rows={2}
            placeholder="输入帮助提示文本，如：请填写有效的玩家ID，支持数字和字母"
          />
        </Form.Item>

        {/* 选项列表 - select/radio/checkbox，支持拖拽排序 */}
        {showOptions && (
          <>
            <Divider plain style={{ margin: '8px 0' }}>
              <Space size={4}>
                <span>选项列表（可拖拽排序）</span>
                <HelpTooltip helpKey="field.options" />
              </Space>
            </Divider>
            <Form.List name="options">
              {(fields, { add, remove }) => (
                <DndContext
                  sensors={dndSensors}
                  collisionDetection={closestCenter}
                  onDragEnd={handleOptionDragEnd}
                >
                  <SortableContext
                    items={fields.map((f) => `opt-${f.name}`)}
                    strategy={verticalListSortingStrategy}
                  >
                    {fields.map(({ key, name, ...restField }) => (
                      <SortableOptionRow key={key} id={`opt-${name}`}>
                        {(dragHandleProps) => (
                          <Space style={{ display: 'flex', marginBottom: 4 }} align="baseline">
                            <span {...dragHandleProps}>
                              <HolderOutlined style={{ color: '#999' }} />
                            </span>
                            <Form.Item
                              {...restField}
                              name={[name, 'label']}
                              rules={[{ required: true, message: '标签' }]}
                              style={{ marginBottom: 0 }}
                            >
                              <Input placeholder="显示文本" style={{ width: 160 }} />
                            </Form.Item>
                            <Form.Item
                              {...restField}
                              name={[name, 'value']}
                              rules={[{ required: true, message: '值' }]}
                              style={{ marginBottom: 0 }}
                            >
                              <Input placeholder="值" style={{ width: 120 }} />
                            </Form.Item>
                            <MinusCircleOutlined onClick={() => remove(name)} />
                          </Space>
                        )}
                      </SortableOptionRow>
                    ))}
                  </SortableContext>
                  <Button
                    type="dashed"
                    onClick={() => add()}
                    block
                    icon={<PlusOutlined />}
                    style={{ marginBottom: 8 }}
                  >
                    添加选项
                  </Button>
                </DndContext>
              )}
            </Form.List>
          </>
        )}

        {/* 校验规则配置 */}
        <Divider plain style={{ margin: '8px 0' }}>
          <Space size={4}>
            <span>校验规则</span>
            <HelpTooltip helpKey="field.rules" />
          </Space>
        </Divider>
        <Form.List name="rules">
          {(fields, { add, remove }) => (
            <>
              {fields.map(({ key, name, ...restField }) => (
                <Space
                  key={key}
                  style={{ display: 'flex', marginBottom: 6, flexWrap: 'wrap' }}
                  align="start"
                >
                  <Form.Item
                    {...restField}
                    name={[name, 'type']}
                    style={{ marginBottom: 0, width: 120 }}
                  >
                    <Select placeholder="规则类型" allowClear options={RULE_TYPES} />
                  </Form.Item>
                  <Form.Item
                    noStyle
                    shouldUpdate={(prev, cur) =>
                      prev?.rules?.[name]?.type !== cur?.rules?.[name]?.type
                    }
                  >
                    {({ getFieldValue }) => {
                      const ruleType = getFieldValue(['rules', name, 'type']);
                      return (
                        <>
                          {ruleType === 'pattern' && (
                            <Form.Item
                              {...restField}
                              name={[name, 'pattern']}
                              style={{ marginBottom: 0 }}
                            >
                              <Input placeholder="正则表达式" style={{ width: 140 }} />
                            </Form.Item>
                          )}
                          {(ruleType === 'number' || ruleType === 'string') && (
                            <>
                              <Form.Item
                                {...restField}
                                name={[name, 'min']}
                                style={{ marginBottom: 0 }}
                              >
                                <InputNumber placeholder="最小值" style={{ width: 80 }} />
                              </Form.Item>
                              <Form.Item
                                {...restField}
                                name={[name, 'max']}
                                style={{ marginBottom: 0 }}
                              >
                                <InputNumber placeholder="最大值" style={{ width: 80 }} />
                              </Form.Item>
                            </>
                          )}
                        </>
                      );
                    }}
                  </Form.Item>
                  <Form.Item {...restField} name={[name, 'message']} style={{ marginBottom: 0 }}>
                    <Input placeholder="错误提示" style={{ width: 140 }} />
                  </Form.Item>
                  <MinusCircleOutlined onClick={() => remove(name)} style={{ marginTop: 8 }} />
                </Space>
              ))}
              <Button
                type="dashed"
                onClick={() => add({ type: 'string' })}
                block
                icon={<PlusOutlined />}
                style={{ marginBottom: 8 }}
              >
                添加校验规则
              </Button>
            </>
          )}
        </Form.List>

        {/* 字段联动规则 */}
        <Divider plain style={{ margin: '8px 0' }}>
          联动规则
        </Divider>
        <Form.Item
          name="visibleWhen"
          label={
            <Space size={4}>
              <span>显隐条件</span>
              <HelpTooltip helpKey="field.visibleWhen" />
            </Space>
          }
        >
          <Input placeholder={"如: status === 'active'（留空表示始终显示）"} />
        </Form.Item>
        <Form.Item
          name="disabledWhen"
          label={
            <Space size={4}>
              <span>禁用条件</span>
              <HelpTooltip helpKey="field.disabledWhen" />
            </Space>
          }
        >
          <Input placeholder={"如: type === 'readonly'（留空表示不联动禁用）"} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
