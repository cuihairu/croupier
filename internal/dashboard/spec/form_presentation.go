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
	// Key is the field path in JSON Schema (e.g., "name", "address.city")
	Key string `json:"key"`

	// Widget overrides the default widget
	Widget FormWidget `json:"widget,omitempty"`

	// Label overrides the schema title
	Label LocalizedText `json:"label,omitempty"`

	// Placeholder text
	Placeholder LocalizedText `json:"placeholder,omitempty"`

	// Description/help text
	Description LocalizedText `json:"description,omitempty"`

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

// ConditionSpec is a restricted expression for presentation visibility. It
// cannot read row/detail data or invoke functions.
type ConditionSpec struct {
	Kind       string          `json:"kind"` // equals|notEquals|exists|all|any
	Path       string          `json:"path,omitempty"`
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
			Text: LocalizedText{"zh-CN": "提交", "en": "Submit"},
			Type: "primary",
		},
		CancelButton: &FormButtonSpec{
			Text: LocalizedText{"zh-CN": "取消", "en": "Cancel"},
		},
	}
}
