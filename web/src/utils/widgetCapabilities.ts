/**
 * SUI-020：FormWidget 的共享能力表。
 *
 * 事实来源是 SchemaFormRenderer 的 widgetToRjsf 分支、自定义控件注册表
 * （widgets.tsx 的 treeSelect/cascader/rate/select、widgets-upload.tsx 的
 * upload/keyValue）以及 FormPresentationEditor 的 12 个选项。这里描述的是
 * **当前实现**，不是理想设计：DateRange/RichText 仅在枚举中声明，渲染端
 * 没有任何控件，会回退到 schema 类型默认控件；Code/JSON 落到 textarea，
 * 只产出普通字符串，不做代码高亮或 JSON 解析。
 */
import type { FormWidget } from '@/types/dashboard';

export interface WidgetCapability {
  /** 可接受的 JSON Schema type 字符串（含 format 约束的说明写在 valueShape/fallback）。 */
  acceptedTypes: string[];
  /** 人类可读的提交值形态说明。 */
  valueShape: string;
  /** 类型不匹配、未实现或复合字段接管时的实际行为。 */
  fallback: string;
  /** 渲染端是否真的有对应控件/字段实现（含 rjsf 内置控件）。 */
  implemented: boolean;
  /** 最终落到 rjsf 的 widget/field 名；复合字段与未实现项没有对应名。 */
  rjsfName?: string;
}

const capability = (
  acceptedTypes: string[],
  valueShape: string,
  fallback: string,
  implemented: boolean,
  rjsfName?: string,
): WidgetCapability => ({ acceptedTypes, valueShape, fallback, implemented, rjsfName });

/** 覆盖 FormWidget 联合的全部 26 个取值——新增枚举值必须同步本表。 */
export const WIDGET_CAPABILITIES: Record<FormWidget, WidgetCapability> = {
  Input: capability(
    ['string'],
    '单个字符串（schema format/maxLength 决定校验，不改变控件）',
    '类型不匹配时交给 rjsf 按 schema 类型选择默认控件',
    true,
    'text（rjsf 默认）',
  ),
  TextArea: capability(
    ['string'],
    '普通字符串（多行文本，不做结构化解析）',
    '非 string 类型时 rjsf 按 schema 类型回退默认控件',
    true,
    'textarea',
  ),
  InputNumber: capability(
    ['number', 'integer'],
    'number / integer 数值',
    '非数值类型时 rjsf 按 schema 类型回退默认控件',
    true,
    'number（rjsf 默认）',
  ),
  Password: capability(
    ['string'],
    '字符串，输入内容掩码显示，提交时仍是明文字符串',
    '非 string 类型时 rjsf 按 schema 类型回退默认控件',
    true,
    'password',
  ),
  Select: capability(
    ['string'],
    '选中项的值（字符串）；multiple 或 schema.type=array 时为字符串数组',
    '无 enumOptions 且无 remoteOptions 时仍显示空下拉；remoteOptions 调用失败静默退化为本地选项',
    true,
    'select（自定义 RemoteSelectWidget）',
  ),
  MultiSelect: capability(
    ['array'],
    '字符串数组；选项值由 schema enum/enumNames 提供',
    '非 array 类型时 ui:widget 改写为 select + multiple:true，由 rjsf 按 schema 自行处理',
    true,
    'select + options.multiple',
  ),
  Radio: capability(
    ['string'],
    '选中项的值（字符串）',
    '缺少 enum 选项时 rjsf 仍渲染单选组；类型不匹配时按 schema 类型回退默认控件',
    true,
    'radio',
  ),
  Checkbox: capability(
    ['boolean', 'array'],
    'boolean 单选项；array 类型为选中值的数组',
    '非 boolean/array 类型时 rjsf 按 schema 类型回退默认控件',
    true,
    'checkbox',
  ),
  Switch: capability(
    ['boolean'],
    '布尔值 true / false（映射到 rjsf checkbox，提交值仍是 boolean）',
    '非 boolean 类型时 rjsf 按 schema 类型回退默认控件，不把字符串强转为布尔',
    true,
    'checkbox',
  ),
  DatePicker: capability(
    ['string'],
    '日期字符串（schema 通常带 format: date）',
    'format 缺失或非 string 类型时 rjsf 按 schema 类型回退默认控件',
    true,
    'date',
  ),
  TimePicker: capability(
    ['string'],
    '时间字符串（schema 通常带 format: time）',
    'format 缺失或非 string 类型时 rjsf 按 schema 类型回退默认控件',
    true,
    'time',
  ),
  DateRange: capability(
    ['array', 'string'],
    '期望为 [开始, 结束] 两个日期字符串，但当前没有产出该形态的实现',
    '未实现：widgetToRjsf 返回 undefined，无自定义控件注册；忽略 ui:widget，回退到 schema 类型默认控件',
    false,
  ),
  Upload: capability(
    ['string', 'array'],
    '已完成上传的 URL 字符串；multiple 或 schema.type=array 时为 URL 字符串数组',
    '上传中/失败的文件不进入值；非 string/array 类型时回退 rjsf 默认控件',
    true,
    'upload（自定义 UploadWidget）',
  ),
  ImageUpload: capability(
    ['string', 'array'],
    '图片 URL 字符串或 URL 字符串数组（上传控件按 URL 收集值）',
    '非 string/array 类型时回退 rjsf 默认控件；上传中/失败的文件被丢弃',
    true,
    'upload（自定义 UploadWidget）',
  ),
  FileUpload: capability(
    ['string', 'array'],
    '文件 URL 字符串或 URL 字符串数组（文本列表样式）',
    '非 string/array 类型时回退 rjsf 默认控件；上传中/失败的文件被丢弃',
    true,
    'upload（自定义 UploadWidget）',
  ),
  RichText: capability(
    ['string'],
    '期望为 HTML 富文本字符串，但当前没有富文本编辑器实现',
    '未实现：widgetToRjsf 返回 undefined，无自定义控件注册；忽略 ui:widget，回退到 schema 类型默认控件（普通输入/textarea）',
    false,
  ),
  Code: capability(
    ['string'],
    '普通字符串：代码内容原样提交，不解析为对象、不提供语法高亮',
    '映射到 textarea；非 string 类型时 rjsf 按 schema 类型回退默认控件',
    true,
    'textarea',
  ),
  Cascader: capability(
    ['string'],
    '级联选择末级的值（字符串）；多级路径由选项树表达',
    '无 cascaderOptions/remoteOptions 时为空选择；非 string 类型时按 schema 类型回退',
    true,
    'cascader（自定义 CascaderWidget）',
  ),
  TreeSelect: capability(
    ['string', 'array'],
    '单选为节点值字符串；multiple 或 schema.type=array 时为节点值字符串数组',
    '无 treeData/remoteOptions 时为空选择；非 string/array 类型时按 schema 类型回退',
    true,
    'treeSelect（自定义 TreeSelectWidget）',
  ),
  Color: capability(
    ['string'],
    '颜色字符串（如 #1677ff）',
    '非 string 类型时 rjsf 按 schema 类型回退默认控件',
    true,
    'color',
  ),
  Slider: capability(
    ['number', 'integer'],
    '数值；步长与上下界来自 schema 的 multipleOf/minimum/maximum',
    '非数值类型时 rjsf 按 schema 类型回退默认控件',
    true,
    'range',
  ),
  Rate: capability(
    ['integer', 'number'],
    '评分数值（星数）',
    '非数值类型时 rjsf 按 schema 类型回退默认控件',
    true,
    'rate（自定义 RateWidget）',
  ),
  JSON: capability(
    ['string'],
    '普通字符串：JSON 文本原样提交，不解析成 object，也不做 JSON 校验/格式化',
    '映射到 textarea；object 类型由 rjsf ObjectField 接管并忽略 ui:widget，非 string 类型按 schema 回退',
    true,
    'textarea',
  ),
  KeyValue: capability(
    ['object'],
    '键值对对象（key → string value），由自定义 field 管理行增删',
    'object 类型由 rjsf ObjectField 接管：widgetToRjsf 返回 undefined，实际接入靠 ui:field=keyValue；非 object 类型按 schema 回退',
    true,
    'keyValue（自定义 Field）',
  ),
  Array: capability(
    ['array'],
    '数组：rjsf ArrayField 管理增删与子项渲染，整体提交',
    'renderer-managed：widgetToRjsf 返回 undefined，不接受 ui:widget 改写；非 array 类型按 schema 回退',
    true,
    'ArrayField（renderer-managed）',
  ),
  Object: capability(
    ['object'],
    '对象：rjsf ObjectField 管理子字段，整体提交',
    'renderer-managed：widgetToRjsf 返回 undefined，不接受 ui:widget 改写；非 object 类型按 schema 回退',
    true,
    'ObjectField（renderer-managed）',
  ),
};

/** 按 widget 名查能力表；未知/缺省值返回 undefined，调用方自行决定兜底。 */
export function widgetCapability(widget: FormWidget | string | undefined): WidgetCapability | undefined {
  if (typeof widget !== 'string') return undefined;
  return Object.prototype.hasOwnProperty.call(WIDGET_CAPABILITIES, widget)
    ? WIDGET_CAPABILITIES[widget as FormWidget]
    : undefined;
}

/** 编辑器选项提示的一行摘要：接受类型 + 输出形态 + 回退行为。 */
export function widgetCapabilityHint(widget: FormWidget | string | undefined): string | undefined {
  const item = widgetCapability(widget);
  if (!item) return undefined;
  return `可接受 schema 类型：${item.acceptedTypes.join(' / ')}｜输出：${item.valueShape}｜回退：${item.fallback}`;
}
