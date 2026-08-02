package spec

// NavigationSpec defines page navigation structure.
type NavigationSpec struct {
	Title      LocalizedText    `json:"title"`
	Breadcrumb []BreadcrumbItem `json:"breadcrumb,omitempty"`
	ShowBack   bool             `json:"showBack,omitempty"`
	BackPath   string           `json:"backPath,omitempty"`
}

// BreadcrumbItem is a single breadcrumb entry.
type BreadcrumbItem struct {
	Title LocalizedText `json:"title"`
	Path  string        `json:"path,omitempty"`
}

// ResourcePageSpec defines a resource CRUD page.
type ResourcePageSpec struct {
	// ListView configuration
	ListView *ListViewSpec `json:"listView,omitempty"`

	// DetailView configuration
	DetailView *DetailViewSpec `json:"detailView,omitempty"`

	// Actions available on the resource
	Actions []ActionSpec `json:"actions,omitempty"`

	// CreateForm configuration
	CreateForm *FormPresentationSpec `json:"createForm,omitempty"`

	// UpdateForm configuration
	UpdateForm *FormPresentationSpec `json:"updateForm,omitempty"`

	// DeleteAction configuration
	DeleteAction *ConfirmActionSpec `json:"deleteAction,omitempty"`
}

// ListViewSpec defines the list view configuration.
type ListViewSpec struct {
	// Columns to display
	Columns []ColumnSpec `json:"columns"`

	// DefaultSort defines default sort order
	DefaultSort *SortSpec `json:"defaultSort,omitempty"`

	// Filterable fields
	Filters []FilterSpec `json:"filters,omitempty"`

	// Pagination configuration
	Pagination *PaginationSpec `json:"pagination,omitempty"`

	// RowActions available on each row
	RowActions []ActionSpec `json:"rowActions,omitempty"`

	// BatchActions available on selected rows
	BatchActions []ActionSpec `json:"batchActions,omitempty"`

	// ToolbarActions available in the toolbar
	ToolbarActions []ActionSpec `json:"toolbarActions,omitempty"`
}

// ColumnSpec defines a table column.
type ColumnSpec struct {
	Key        string        `json:"key"`
	Title      LocalizedText `json:"title"`
	DataType   string        `json:"dataType"` // string|number|boolean|date|datetime|enum
	Width      int           `json:"width,omitempty"`
	Fixed      string        `json:"fixed,omitempty"` // left|right
	Sortable   bool          `json:"sortable,omitempty"`
	Filterable bool          `json:"filterable,omitempty"`
	Visible    bool          `json:"visible,omitempty"` // default true
	Enum       []EnumOption  `json:"enum,omitempty"`
	Format     string        `json:"format,omitempty"` // date/time format
	Render     string        `json:"render,omitempty"` // render hint: tag|link|copy|status
}

// EnumOption represents an enum value for display.
type EnumOption struct {
	Value string        `json:"value"`
	Label LocalizedText `json:"label"`
	Color string        `json:"color,omitempty"` // for tag rendering
}

// SortSpec defines sort configuration.
type SortSpec struct {
	Field string `json:"field"`
	Order string `json:"order"` // asc|desc
}

// FilterSpec defines a filterable field.
type FilterSpec struct {
	Key     string        `json:"key"`
	Title   LocalizedText `json:"title"`
	Type    string        `json:"type"` // text|select|date|daterange|number
	Options []EnumOption  `json:"options,omitempty"`
}

// PaginationSpec defines pagination configuration.
type PaginationSpec struct {
	Enabled     bool  `json:"enabled"`
	DefaultSize int   `json:"defaultSize,omitempty"` // default 20
	PageSizes   []int `json:"pageSizes,omitempty"`   // [10, 20, 50, 100]
}

// DetailViewSpec defines the detail view configuration.
type DetailViewSpec struct {
	// Fields to display
	Fields []DetailFieldSpec `json:"fields"`

	// Actions available on the detail view
	Actions []ActionSpec `json:"actions,omitempty"`

	// Layout hint
	Layout string `json:"layout,omitempty"` // vertical|horizontal|grid
}

// DetailFieldSpec defines a field in detail view.
type DetailFieldSpec struct {
	Key      string        `json:"key"`
	Title    LocalizedText `json:"title"`
	DataType string        `json:"dataType"`
	Span     int           `json:"span,omitempty"`    // grid span
	Render   string        `json:"render,omitempty"`  // render hint
	Visible  bool          `json:"visible,omitempty"` // default true
}

// ActionSpec defines an action (row, batch, toolbar, or detail).
type ActionSpec struct {
	Key          string        `json:"key"`
	Title        LocalizedText `json:"title"`
	Icon         string        `json:"icon,omitempty"`
	Type         string        `json:"type"` // primary|default|danger|link
	Confirm      bool          `json:"confirm,omitempty"`
	ConfirmTitle LocalizedText `json:"confirmTitle,omitempty"`
	ConfirmDesc  LocalizedText `json:"confirmDescription,omitempty"`
	BindingID    string        `json:"bindingId,omitempty"` // reference to binding
	Permission   string        `json:"permission,omitempty"`
	Risk         string        `json:"risk,omitempty"`
}

// ConfirmActionSpec defines a confirmation action (like delete).
type ConfirmActionSpec struct {
	Title       LocalizedText `json:"title"`
	Description LocalizedText `json:"description,omitempty"`
	ConfirmText LocalizedText `json:"confirmText"`
	CancelText  LocalizedText `json:"cancelText,omitempty"`
	BindingID   string        `json:"bindingId"`
	Permission  string        `json:"permission,omitempty"`
	Risk        string        `json:"risk,omitempty"`
}

// OperationPageSpec defines a standalone operation page.
type OperationPageSpec struct {
	// Form to collect input
	Form *FormPresentationSpec `json:"form"`

	// Confirm before execution
	Confirm *ConfirmActionSpec `json:"confirm,omitempty"`

	// ResultView to display output
	ResultView *ResultViewSpec `json:"resultView,omitempty"`
}

// TaskPageSpec defines an async task page.
type TaskPageSpec struct {
	// Form to collect input
	Form *FormPresentationSpec `json:"form"`

	// TaskView to display progress
	TaskView *TaskViewSpec `json:"taskView"`

	// ResultView to display final result
	ResultView *ResultViewSpec `json:"resultView,omitempty"`
}

// TaskViewSpec defines task progress display.
type TaskViewSpec struct {
	ShowTimeline bool `json:"showTimeline"`
	ShowProgress bool `json:"showProgress"`
	ShowEvents   bool `json:"showEvents"`
	Cancelable   bool `json:"cancelable"`
	Retryable    bool `json:"retryable"`
}

// ReportPageSpec defines a report page.
type ReportPageSpec struct {
	// QueryForm to collect query parameters
	QueryForm *FormPresentationSpec `json:"queryForm"`

	// Dataset configuration
	Dataset *DatasetSpec `json:"dataset"`

	// Charts to display
	Charts []ChartSpec `json:"charts,omitempty"`

	// Table to display data
	Table *ListViewSpec `json:"table,omitempty"`

	// Exportable
	Exportable bool `json:"exportable,omitempty"`
}

// DatasetSpec defines the data source for a report.
type DatasetSpec struct {
	// Dimensions for grouping
	Dimensions []DimensionSpec `json:"dimensions"`

	// Metrics to measure
	Metrics []MetricSpec `json:"metrics"`
}

// DimensionSpec defines a data dimension.
type DimensionSpec struct {
	Key      string        `json:"key"`
	Title    LocalizedText `json:"title"`
	DataType string        `json:"dataType"` // string|number|date
}

// MetricSpec defines a data metric.
type MetricSpec struct {
	Key      string        `json:"key"`
	Title    LocalizedText `json:"title"`
	DataType string        `json:"dataType"`          // number
	AggType  string        `json:"aggType,omitempty"` // sum|avg|count|min|max
	Format   string        `json:"format,omitempty"`  // number|percent|currency
}

// ChartSpec defines a chart display.
type ChartSpec struct {
	Type        string        `json:"type"` // line|bar|pie|area|scatter
	Title       LocalizedText `json:"title"`
	XField      string        `json:"xField,omitempty"`
	YField      string        `json:"yField,omitempty"`
	SeriesField string        `json:"seriesField,omitempty"`
	GroupField  string        `json:"groupField,omitempty"`
}

// ResultViewSpec defines how to display execution results.
type ResultViewSpec struct {
	// Fields to display from result
	Fields []ResultFieldSpec `json:"fields,omitempty"`

	// SuccessMessage to show on success
	SuccessMessage LocalizedText `json:"successMessage,omitempty"`

	// ErrorMessage to show on error
	ErrorMessage LocalizedText `json:"errorMessage,omitempty"`
}

// ResultFieldSpec defines a field in result view.
type ResultFieldSpec struct {
	Key      string        `json:"key"`
	Title    LocalizedText `json:"title"`
	DataType string        `json:"dataType"`
	Render   string        `json:"render,omitempty"`
}
