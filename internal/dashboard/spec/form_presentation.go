package spec

import "encoding/json"

// FormPresentationSpec defines how to render a form from JSON Schema.
// It extends JSON Schema with presentation hints without modifying the schema itself.
type FormPresentationSpec struct {
	// JSONSchema is the raw JSON Schema for form validation
	JSONSchema JSONSchema `json:"jsonSchema"`

	// Layout defines form layout
	Layout FormLayout `json:"layout,omitempty"`

	// Groups organizes fields into sections
	Groups []FormGroupSpec `json:"groups,omitempty"`

	// Fields defines per-field presentation overrides
	Fields []FormFieldSpec `json:"fields,omitempty"`

	// SubmitButton configuration
	SubmitButton *FormButtonSpec `json:"submitButton,omitempty"`

	// CancelButton configuration
	CancelButton *FormButtonSpec `json:"cancelButton,omitempty"`

	// Variants 命名展示变体（SUI-031 协议，docs/architecture/
	// pagespec-protocol.md「命名展示变体」）：同一表单的多个命名呈现形态，
	// 数组顺序即管理列表顺序。additive 可选字段（page-spec:1 不变）；
	// 旧渲染器忽略并按基础呈现渲染。存储随表单节点进同一草稿/发布快照
	//（SUI-032），发布校验、编辑器与解析分别在 SUI-033/034/035 落地。
	Variants []FormVariantSpec `json:"variants,omitempty"`

	// DefaultVariant 生效默认变体 key（Variants 内唯一 key）；未声明 =
	// 基础呈现生效。声明后必须能解析到已声明变体（SUI-033 校验）。
	DefaultVariant string `json:"defaultVariant,omitempty"`
}

// FormVariantSpec 定义命名展示变体：只承载 SUI-045 纯展示覆盖（页面级
// layout/groups/buttons 与逐字段展示属性），调用相关项（required/
// defaultValue/visible/visibleWhen/disabled/validationRules）与 jsonSchema
// 不在类型面上——它们属于调用契约，任何变体不得改动（SUI-031）。
type FormVariantSpec struct {
	// Key 变体标识：本表单内唯一（SUI-033 校验重复）。
	Key string `json:"key"`

	// Title 操作者可见名称（风格列表/切换 UI）。
	Title LocalizedText `json:"title,omitempty"`

	// Layout 页面级布局覆盖；未声明继承基础。
	Layout FormLayout `json:"layout,omitempty"`

	// Groups 分组覆盖；未声明继承基础（整体替换，不按组合并）。
	Groups []FormGroupSpec `json:"groups,omitempty"`

	// Fields 逐字段纯展示覆盖（键同 FormFieldSpec.Key 契约，SUI-019；
	// 与基础同 key 条目按属性合并，V 独有 key 须能在 jsonSchema 解析）。
	Fields []FormVariantFieldSpec `json:"fields,omitempty"`

	// SubmitButton 覆盖；未声明继承基础。
	SubmitButton *FormButtonSpec `json:"submitButton,omitempty"`

	// CancelButton 覆盖；未声明继承基础。
	CancelButton *FormButtonSpec `json:"cancelButton,omitempty"`
}

// FormVariantFieldSpec 是 FormFieldSpec 的纯展示子集：变体逐字段只允许
// label/description/placeholder/role/unit/perTarget/widget/width/order/
// enumOptions/remoteOptions/widgetProps。发布前校验拒绝调用相关键
// （form_variant_illegal_override，SUI-033 落地）。
type FormVariantFieldSpec struct {
	// Key 同 FormFieldSpec.Key：裸顶层属性名或 JSON Pointer。
	Key string `json:"key"`

	// Widget overrides the default widget
	Widget FormWidget `json:"widget,omitempty"`

	// Label overrides the schema title
	Label LocalizedText `json:"label,omitempty"`

	// Placeholder text
	Placeholder LocalizedText `json:"placeholder,omitempty"`

	// Description/help text
	Description LocalizedText `json:"description,omitempty"`

	// Role is the explicit field semantics (x-field-role hint, D5)
	Role string `json:"role,omitempty"`

	// Unit is the display unit suffix (x-field-unit hint, D5)
	Unit LocalizedText `json:"unit,omitempty"`

	// PerTarget marks a quantity/amount that applies per selected target (D2)
	PerTarget bool `json:"perTarget,omitempty"`

	// Width in grid layout (1-12)
	Width int `json:"width,omitempty"`

	// Order defines field order (lower = first)
	Order int `json:"order,omitempty"`

	// EnumOptions for select/radio widgets
	EnumOptions []EnumOption `json:"enumOptions,omitempty"`

	// RemoteOptions points to a registered collection_query function as the
	// option source; mutually exclusive with EnumOptions.
	RemoteOptions *RemoteOptionsSpec `json:"remoteOptions,omitempty"`

	// WidgetProps passes extra props to the widget
	WidgetProps map[string]json.RawMessage `json:"widgetProps,omitempty"`
}

// FormLayout defines form layout type.
type FormLayout string

const (
	FormLayoutVertical   FormLayout = "vertical"
	FormLayoutHorizontal FormLayout = "horizontal"
	FormLayoutInline     FormLayout = "inline"
	FormLayoutGrid       FormLayout = "grid"
)

// FormGroupSpec defines a group of fields.
type FormGroupSpec struct {
	Key         string        `json:"key"`
	Title       LocalizedText `json:"title,omitempty"`
	Fields      []string      `json:"fields"` // field keys
	Collapsible bool          `json:"collapsible,omitempty"`
	Collapsed   bool          `json:"collapsed,omitempty"`
}

// FormFieldSpec defines presentation hints for a single field.
type FormFieldSpec struct {
	// Key identifies the field: a bare top-level property name (e.g. "name")
	// or a JSON Pointer RFC 6901 into the schema (e.g. "/address/city",
	// "/tags/-/label"). A dotted name is a literal property name, not a path.
	Key string `json:"key"`

	// Widget overrides the default widget
	Widget FormWidget `json:"widget,omitempty"`

	// Label overrides the schema title
	Label LocalizedText `json:"label,omitempty"`

	// Placeholder text
	Placeholder LocalizedText `json:"placeholder,omitempty"`

	// Description/help text
	Description LocalizedText `json:"description,omitempty"`

	// Role is the explicit field semantics (x-field-role hint, D5):
	// identity|status|reason|expiry|amount|quantity. Drives confirm-dialog
	// semantic emphasis and default presentation.
	Role string `json:"role,omitempty"`

	// Unit is the display unit suffix (x-field-unit hint, D5), e.g. 金币/个.
	Unit LocalizedText `json:"unit,omitempty"`

	// PerTarget marks a quantity/amount that applies per selected target
	// (x-field-per-target hint, D2): a batch confirm with more than one
	// selected target additionally shows the total (value × targetCount).
	PerTarget bool `json:"perTarget,omitempty"`

	// Width in grid layout (1-12)
	Width int `json:"width,omitempty"`

	// Order defines field order (lower = first)
	Order int `json:"order,omitempty"`

	// Visible controls field visibility
	Visible *bool `json:"visible,omitempty"`

	// VisibleWhen controls field visibility from form/page state only.
	VisibleWhen *ConditionSpec `json:"visibleWhen,omitempty"`

	// Disabled controls if field is disabled
	Disabled *bool `json:"disabled,omitempty"`

	// Required overrides schema required
	Required *bool `json:"required,omitempty"`

	// DefaultValue overrides schema default
	DefaultValue json.RawMessage `json:"defaultValue,omitempty"`

	// EnumOptions for select/radio widgets
	EnumOptions []EnumOption `json:"enumOptions,omitempty"`

	// RemoteOptions points to a registered collection_query function as the
	// option source (x-options-source hint, F9). Mutually exclusive with
	// EnumOptions — remoteOptions wins on the renderer side.
	RemoteOptions *RemoteOptionsSpec `json:"remoteOptions,omitempty"`

	// WidgetProps passes extra props to the widget
	WidgetProps map[string]json.RawMessage `json:"widgetProps,omitempty"`

	// ValidationRules adds custom validation
	ValidationRules []ValidationRule `json:"validationRules,omitempty"`
}

// FormWidget defines available form widgets.
type FormWidget string

const (
	// Basic inputs
	FormWidgetInput    FormWidget = "Input"
	FormWidgetTextArea FormWidget = "TextArea"
	FormWidgetNumber   FormWidget = "InputNumber"
	FormWidgetPassword FormWidget = "Password"

	// Selection
	FormWidgetSelect      FormWidget = "Select"
	FormWidgetMultiSelect FormWidget = "MultiSelect"
	FormWidgetRadio       FormWidget = "Radio"
	FormWidgetCheckbox    FormWidget = "Checkbox"
	FormWidgetSwitch      FormWidget = "Switch"

	// Date/Time
	FormWidgetDatePicker FormWidget = "DatePicker"
	FormWidgetTimePicker FormWidget = "TimePicker"
	FormWidgetDateRange  FormWidget = "DateRange"

	// Upload
	FormWidgetUpload      FormWidget = "Upload"
	FormWidgetImageUpload FormWidget = "ImageUpload"
	FormWidgetFileUpload  FormWidget = "FileUpload"

	// Rich text
	FormWidgetRichText FormWidget = "RichText"
	FormWidgetCode     FormWidget = "Code"

	// Structured
	FormWidgetCascader   FormWidget = "Cascader"
	FormWidgetTreeSelect FormWidget = "TreeSelect"
	FormWidgetColor      FormWidget = "Color"
	FormWidgetSlider     FormWidget = "Slider"
	FormWidgetRate       FormWidget = "Rate"

	// Special
	FormWidgetJSON     FormWidget = "JSON"     // raw JSON editor
	FormWidgetKeyValue FormWidget = "KeyValue" // key-value pairs
	FormWidgetArray    FormWidget = "Array"    // array items
	FormWidgetObject   FormWidget = "Object"   // nested object
)

// ValidationRule defines custom validation.
type ValidationRule struct {
	Type    string          `json:"type"` // required|min|max|pattern|custom
	Value   json.RawMessage `json:"value,omitempty"`
	Message LocalizedText   `json:"message"`
}

// RemoteOptionsSpec defines a remote option source (F9): points to a
// collection_query function registered in the same scope. Invocation goes
// through the existing RBAC pipeline.
type RemoteOptionsSpec struct {
	// FunctionID is the data source function id (e.g. "player.list").
	FunctionID string `json:"functionId"`

	// LabelPath selects option labels from the invocation result as a JSON
	// Pointer with `*` wildcard array segments (e.g. "/items/*/name").
	LabelPath string `json:"labelPath,omitempty"`

	// ValuePath selects option values (e.g. "/items/*/id"); empty = reuse
	// LabelPath (renderer-side default).
	ValuePath string `json:"valuePath,omitempty"`

	// SearchParam is the search keyword parameter name; when set, dropdown
	// search re-invokes the function with it.
	SearchParam string `json:"searchParam,omitempty"`
}

// ConditionSpec is a restricted expression for presentation visibility. It
// cannot read row/detail data or invoke functions.
type ConditionSpec struct {
	Kind string `json:"kind"` // equals|notEquals|exists|all|any
	// Path 求值路径：表单字段级条件（Form.Hints）为表单内相对路径；
	// 区块级条件（CompositeSection.VisibleWhen）须配合 Key 使用。
	Path string `json:"path,omitempty"`
	// Key 来源区块标识（区块级条件专用）：非空时从页面状态 results[key]
	// 按 Path 取值求值（key=区块 key，同 page_state 联动寻址）；空=表单
	// 内相对路径（字段级 visibleWhen 原语义，additive 不影响既有数据）。
	Key        string          `json:"key,omitempty"`
	Value      json.RawMessage `json:"value,omitempty"`
	Conditions []ConditionSpec `json:"conditions,omitempty"`
}

// FormButtonSpec defines a form button.
type FormButtonSpec struct {
	Text    LocalizedText `json:"text"`
	Type    string        `json:"type,omitempty"` // primary|default|danger|link
	Icon    string        `json:"icon,omitempty"`
	Loading bool          `json:"loading,omitempty"`
}

// DefaultFormPresentation creates a default FormPresentationSpec from JSON Schema.
func DefaultFormPresentation(schema JSONSchema) *FormPresentationSpec {
	return &FormPresentationSpec{
		JSONSchema: schema,
		Layout:     FormLayoutVertical,
		SubmitButton: &FormButtonSpec{
			Text: LocalizedText{"zh-CN": "提交", "en-US": "Submit"},
			Type: "primary",
		},
		CancelButton: &FormButtonSpec{
			Text: LocalizedText{"zh-CN": "取消", "en-US": "Cancel"},
		},
	}
}
