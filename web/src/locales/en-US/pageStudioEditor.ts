// PageStudio/CompositeEditor 深层组件（面板/画布/预览/常量/行操作编辑器与 components/*）
// 注意：pages.pageStudio.editor.* 中与 pageStudio.ts 重复的 36 个键在 pageStudio.ts 定义，此处不重复（避免根聚合 spread 互相覆盖）。
export default {
  'pages.pageStudio.editor.action.event.onClick': 'On click',
  'pages.pageStudio.editor.action.event.onError': 'On error',
  'pages.pageStudio.editor.action.event.onRowClick': 'Row click',
  'pages.pageStudio.editor.action.event.onRowSelected': 'Row selected',
  'pages.pageStudio.editor.action.event.onSuccess': 'On success',
  'pages.pageStudio.editor.action.kind.closeModal': 'Close modal',
  'pages.pageStudio.editor.action.kind.navigate': 'Navigate to link',
  'pages.pageStudio.editor.action.kind.openModal': 'Open modal',
  'pages.pageStudio.editor.action.kind.refreshNode': 'Refresh',
  'pages.pageStudio.editor.action.kind.runBinding': 'Run',
  'pages.pageStudio.editor.action.kind.showMessage': 'Show message',
  'pages.pageStudio.editor.action.modalGuide.autoHint':
    'Done automatically: create a modal → load this function form → bind it to this button',
  'pages.pageStudio.editor.action.modalGuide.createButton': 'Create modal & bind',
  'pages.pageStudio.editor.action.modalGuide.fnPlaceholder':
    'Pick an action function (e.g. mail.send)',
  'pages.pageStudio.editor.action.modalGuide.title':
    'No modals on this page yet — pick an action function to create and bind one in a single step:',
  'pages.pageStudio.editor.action.param.message.label': 'Message',
  'pages.pageStudio.editor.action.param.message.placeholder': 'Message content',
  'pages.pageStudio.editor.action.param.url.label': 'URL',
  'pages.pageStudio.editor.action.param.url.placeholder': 'https://… or /page-path',
  'pages.pageStudio.editor.action.placeholder.kind': 'Select action',
  'pages.pageStudio.editor.action.placeholder.target': 'Select target',
  'pages.pageStudio.editor.action.target.empty': 'No available targets',
  'pages.pageStudio.editor.action.targetDeleted':
    'The target node has been deleted — please choose another',
  'pages.pageStudio.editor.canvas.containerFallback':
    'The container does not accept "{type}" children; placed after the container instead',
  'pages.pageStudio.editor.canvas.containerNotAllowed':
    'The container does not accept "{type}" children (only tables, field cards, buttons and text are allowed)',
  'pages.pageStudio.editor.canvas.emptyHint':
    'Click or drag a component from the left panel to start building the page',
  'pages.pageStudio.editor.canvas.missingDeps': 'Missing dependent functions: {fns}',
  'pages.pageStudio.editor.canvas.modalFormOnly':
    'Only function forms can be placed inside a modal (V1)',
  'pages.pageStudio.editor.canvas.showTemplates': 'View composite templates',
  'pages.pageStudio.editor.canvas.templateEmpty': 'The template is empty',
  'pages.pageStudio.editor.canvas.tabsFallback':
    'Tabs pages do not accept "{type}" children; placed after the tabs container',
  'pages.pageStudio.editor.canvas.tabsNoPage':
    'The tabs container has no page to place the template',
  'pages.pageStudio.editor.canvas.tabsNotAllowed':
    'Tabs pages only accept tables/field cards/buttons/text; "{type}" is not allowed',
  'pages.pageStudio.editor.chain.addParam': '+ Add parameter',
  'pages.pageStudio.editor.chain.addStep': '+ Add follow-up action',
  'pages.pageStudio.editor.chain.paramNamePlaceholder': 'Parameter name',
  'pages.pageStudio.editor.chain.placeholder.target': 'Target',
  'pages.pageStudio.editor.chain.step.closeModal': 'Close modal',
  'pages.pageStudio.editor.chain.step.navigate': 'Navigate',
  'pages.pageStudio.editor.chain.step.showMessage': 'Message',
  'pages.pageStudio.editor.chain.targetEmpty': 'None',
  'pages.pageStudio.editor.chain.title':
    'Follow-up actions (run in order after the main action completes)',
  'pages.pageStudio.editor.component.button.name': 'Button',
  'pages.pageStudio.editor.component.button.prop.btnStyle': 'Style',
  'pages.pageStudio.editor.component.button.prop.title': 'Button label',
  'pages.pageStudio.editor.component.button.style.danger': 'Danger',
  'pages.pageStudio.editor.component.button.style.default': 'Default',
  'pages.pageStudio.editor.component.button.style.primary': 'Primary',
  'pages.pageStudio.editor.component.fn.autoRun': 'Run automatically on page load',
  'pages.pageStudio.editor.component.fn.prop.functionId': 'Function (rebindable)',
  'pages.pageStudio.editor.component.fn.prop.span': 'Width (1-24 grid)',
  'pages.pageStudio.editor.component.fn.prop.title': 'Title',
  'pages.pageStudio.editor.component.fnFields.icon': 'Fields',
  'pages.pageStudio.editor.component.fnFields.name': 'Field card',
  'pages.pageStudio.editor.component.fnFields.preview.empty': 'No output schema',
  'pages.pageStudio.editor.component.fnForm.display.dialog': 'Dialog — triggered by a button',
  'pages.pageStudio.editor.component.fnForm.display.inline': 'Inline — embedded in the page',
  'pages.pageStudio.editor.component.fnForm.icon': 'Form',
  'pages.pageStudio.editor.component.fnForm.name': 'Function form',
  'pages.pageStudio.editor.component.fnForm.preview.dialog': 'Dialog mode',
  'pages.pageStudio.editor.component.fnForm.preview.inline': 'Inline form',
  'pages.pageStudio.editor.component.fnForm.preview.noParams':
    'This function has no input parameters',
  'pages.pageStudio.editor.component.fnForm.prop.display': 'Display mode',
  'pages.pageStudio.editor.component.fnTable.icon': 'Table',
  'pages.pageStudio.editor.component.fnTable.name': 'Function table',
  'pages.pageStudio.editor.component.fnTable.preview.emptyHint':
    'Columns come from the output schema; real data appears after a trial run/preview',
  'pages.pageStudio.editor.component.fnTable.prop.columns': 'Columns',
  'pages.pageStudio.editor.component.fnTable.prop.rowActions': 'Row actions',
  'pages.pageStudio.editor.component.modal.defaultTitle': 'Actions',
  'pages.pageStudio.editor.component.modal.name': 'Modal',
  'pages.pageStudio.editor.component.modal.previewHint':
    'Drag in a function form as the modal content (V1: one only)',
  'pages.pageStudio.editor.component.modal.prop.title': 'Modal title',
  'pages.pageStudio.editor.component.modal.prop.width': 'Width',
  'pages.pageStudio.editor.component.modal.width.medium': 'Medium 560',
  'pages.pageStudio.editor.component.modal.width.narrow': 'Narrow 420',
  'pages.pageStudio.editor.component.modal.width.wide': 'Wide 720',
  'pages.pageStudio.editor.component.staticForm.name': 'Static form',
  'pages.pageStudio.editor.component.staticForm.preview.empty':
    'No fields yet — define the JSON in the properties panel',
  'pages.pageStudio.editor.component.staticForm.preview.invalid': 'Invalid field definition JSON',
  'pages.pageStudio.editor.component.staticForm.prop.staticSchema':
    'Field definitions (JSON Schema)',
  'pages.pageStudio.editor.component.staticForm.prop.title': 'Title',
  'pages.pageStudio.editor.component.tabs.emptyTab': 'Empty tab — drop components here',
  'pages.pageStudio.editor.component.tabs.name': 'Tabs container',
  'pages.pageStudio.editor.component.tabs.previewHint':
    'Empty tabs container — drop tables/field cards/buttons/text (they go to the active tab)',
  'pages.pageStudio.editor.component.tabs.prop.sectionKey':
    'Tab group name (optional, auto by default)',
  'pages.pageStudio.editor.component.tabs.tabFallback': 'Tab {n}',
  'pages.pageStudio.editor.component.text.level.h2': 'Heading',
  'pages.pageStudio.editor.component.text.level.h3': 'Subheading',
  'pages.pageStudio.editor.component.text.level.p': 'Body',
  'pages.pageStudio.editor.component.text.name': 'Text',
  'pages.pageStudio.editor.component.text.prop.content': 'Content',
  'pages.pageStudio.editor.component.text.prop.level': 'Level',
  'pages.pageStudio.editor.constantFields.addField': 'Add constant',
  'pages.pageStudio.editor.constantFields.advancedJson': 'JSON',
  'pages.pageStudio.editor.constantFields.advancedJsonCollapse': 'Collapse JSON',
  'pages.pageStudio.editor.constantFields.emptyHint':
    'No constants yet. To import constants, use "Import constants" on the Component Library tab.',
  'pages.pageStudio.editor.constantFields.jsonSchemaLabel':
    'JSON Schema (advanced, two-way synced)',
  'pages.pageStudio.editor.constantFields.optionEditorPlaceholder':
    'One option per line: value or value|label',
  'pages.pageStudio.editor.constantFields.titleAddon': 'Display name',
  'pages.pageStudio.editor.constantFields.varInvalid':
    'Variable name cannot be empty or duplicate another constant',
  'pages.pageStudio.editor.constantFields.varNameAddon': 'Variable name',
  'pages.pageStudio.editor.constantFields.varRefHint':
    'Downstream references this variable name: {name}',
  'pages.pageStudio.editor.constantFields.varRefUnset': '(not set)',
  'pages.pageStudio.editor.constantImport.cancel': 'Cancel',
  'pages.pageStudio.editor.constantImport.emptyFields':
    'Import constants (Excel/JSON) or add fields first',
  'pages.pageStudio.editor.constantImport.excelParseFailed': 'Failed to parse Excel',
  'pages.pageStudio.editor.constantImport.formatHint':
    'Excel long format: name|value|label (rows with the same name aggregate); wide format: name|options…; JSON: {example}',
  'pages.pageStudio.editor.constantImport.jsonFormatHint': 'JSON must be {example}',
  'pages.pageStudio.editor.constantImport.jsonParseFailed': 'Failed to parse JSON',
  'pages.pageStudio.editor.constantImport.modeLong': 'Long format: name|value|label',
  'pages.pageStudio.editor.constantImport.modeWide': 'Wide format: name|options…',
  'pages.pageStudio.editor.constantImport.previewTitle':
    'Constant preview ({count}; each constant is saved as an independent dropdown component)',
  'pages.pageStudio.editor.constantImport.saveAll': 'Save all ({count} components)',
  'pages.pageStudio.editor.constantImport.saveFailed': 'Failed to save',
  'pages.pageStudio.editor.constantImport.savedCount': '{count} constant components',
  'pages.pageStudio.editor.constantImport.title': 'Import constants',
  'pages.pageStudio.editor.constantImport.uploadButton': 'Upload Excel / JSON',
  'pages.pageStudio.editor.dataPanel.runFailed': 'Run failed',
  'pages.pageStudio.editor.dataPanel.runHint':
    'Test-run {fnId} ({paramCount} params, empty input by default)',
  'pages.pageStudio.editor.dataPanel.title': 'Data',
  'pages.pageStudio.editor.expression.pathSegmentMissing':
    'Path segment "{name}" is not among the schema candidates for {variable}',
  'pages.pageStudio.editor.expression.placeholder':
    "Literal value, or '{{' to pick a variable '}}'",
  'pages.pageStudio.editor.expression.rowFieldMissing':
    'Row field "{field}" is not in the current table output schema',
  'pages.pageStudio.editor.expression.rowVariableHint': 'Current row (row action context)',
  'pages.pageStudio.editor.insertTpl.ok': 'Insert',
  'pages.pageStudio.editor.insertTpl.title': 'Configure component parameters: {name}',
  'pages.pageStudio.editor.library.category.builtin': 'Built-in',
  'pages.pageStudio.editor.library.category.custom': 'Custom',
  'pages.pageStudio.editor.library.createHint':
    'First multi-select nodes on the canvas (Shift+click), then save as a component',
  'pages.pageStudio.editor.library.createFromCanvas': 'Create from canvas selection',
  'pages.pageStudio.editor.library.empty': 'No component templates',
  'pages.pageStudio.editor.library.emptyHint':
    'Select multiple nodes on the canvas → use "Save as component" in the top bar to create one',
  'pages.pageStudio.editor.library.loading': 'Loading component library…',
  'pages.pageStudio.editor.library.manageLink': 'Manage templates →',
  'pages.pageStudio.editor.library.missingFunctions': 'Missing functions: {fns}',
  'pages.pageStudio.editor.library.searchPlaceholder': 'Search components',
  'pages.pageStudio.editor.modal.emptyHint':
    'Empty modal — drag in a function form, or double-click to edit inside',
  'pages.pageStudio.editor.modal.enterEdit': 'Edit modal →',
  'pages.pageStudio.editor.node.actionBound': 'Action bound',
  'pages.pageStudio.editor.node.autoRunTag': 'Auto',
  'pages.pageStudio.editor.node.bindActionHint': 'Click to bind an action →',
  'pages.pageStudio.editor.node.delete': 'Delete',
  'pages.pageStudio.editor.node.dialogTag': 'Modal',
  'pages.pageStudio.editor.node.duplicate': 'Duplicate',
  'pages.pageStudio.editor.node.editHint':
    'Drag handle to reorder · drag right edge to resize · click to configure',
  'pages.pageStudio.editor.node.moveDown': 'Move down',
  'pages.pageStudio.editor.node.moveUp': 'Move up',
  'pages.pageStudio.editor.node.saveAsComponent': 'Save as component',
  'pages.pageStudio.editor.node.selectParent': 'Select parent container',
  'pages.pageStudio.editor.outline.empty': 'The page is empty',
  'pages.pageStudio.editor.panel.basicsTitle': 'Basic components',
  'pages.pageStudio.editor.panel.categoryOther': 'Other',
  'pages.pageStudio.editor.panel.fnTitle': 'Function components',
  'pages.pageStudio.editor.panel.loading': 'Loading…',
  'pages.pageStudio.editor.panel.noMatch': 'No matching functions',
  'pages.pageStudio.editor.panel.probing': 'Probing function distribution across scopes…',
  'pages.pageStudio.editor.panel.scopeEmpty':
    'The current scope ({gameId}/{env}) has no function contracts',
  'pages.pageStudio.editor.panel.scopeEmptyOtherHint':
    'No functions in other scopes either — register functions via SDK/OpenAPI first',
  'pages.pageStudio.editor.panel.searchPlaceholder': 'Search functions / resources',
  'pages.pageStudio.editor.panel.switchScope': 'Switch to {gameId}/{env} ({count} functions)',
  'pages.pageStudio.editor.paramMapping.fieldPlaceholder': 'Field',
  'pages.pageStudio.editor.paramMapping.hint':
    'Parameter mapping: defaults to this section’s form values; declare cross-section inputs explicitly here (unlisted parameters stay automatic).',
  'pages.pageStudio.editor.paramMapping.kind.auto': 'Auto',
  'pages.pageStudio.editor.paramMapping.kind.literal': 'Literal',
  'pages.pageStudio.editor.paramMapping.kind.upstream': 'Upstream section',
  'pages.pageStudio.editor.paramMapping.literalPlaceholder':
    "Literal value, or '{{' to pick a variable '}}'",
  'pages.pageStudio.editor.paramMapping.sourcePlaceholder': 'Source section',
  'pages.pageStudio.editor.preview.dataSource': 'Data source',
  'pages.pageStudio.editor.preview.dialogEmpty':
    'The modal has no content — drag a function form into it in edit mode',
  'pages.pageStudio.editor.preview.dialogExecuted': '{title} completed successfully',
  'pages.pageStudio.editor.preview.invokeFailed': '{title} failed to run',
  'pages.pageStudio.editor.preview.mockEnabledHint':
    'Mock data: fake data generated from outputSchema, no real operations',
  'pages.pageStudio.editor.preview.mockHint':
    'Fake data is generated dynamically from each function’s outputSchema — safely verify bindings, linkage and modal prefill',
  'pages.pageStudio.editor.preview.mockNoSchema':
    '“{title}” has no usable outputSchema (or its top-level structure is unsupported), so mock data is empty',
  'pages.pageStudio.editor.preview.mockSuggest':
    'Turn on "Mock data" at the top to walk through the full flow safely (no real operations)',
  'pages.pageStudio.editor.preview.mockTag': 'Mocking',
  'pages.pageStudio.editor.preview.modalTitleFallback': 'Modal',
  'pages.pageStudio.editor.preview.modeTag': 'Preview',
  'pages.pageStudio.editor.preview.realEnabledHint':
    'Real invocation: functions actually run (note that action functions have real side effects)',
  'pages.pageStudio.editor.preview.realHint':
    'Functions actually run — action functions (e.g. sending email) have real side effects',
  'pages.pageStudio.editor.preview.realTag': 'Real invocation',
  'pages.pageStudio.editor.preview.rowActionConfirm': 'Confirm running "{label}"',
  'pages.pageStudio.editor.preview.rowActionFallback': 'Action',
  'pages.pageStudio.editor.preview.targetMissing':
    'Action target does not exist (it may have been deleted)',
  'pages.pageStudio.editor.previewNode.confirmButton': 'Confirm',
  'pages.pageStudio.editor.previewNode.emptyContainer': 'Empty container',
  'pages.pageStudio.editor.previewNode.executeButton': 'Run',
  'pages.pageStudio.editor.previewNode.rowActionColumn': 'Actions',
  'pages.pageStudio.editor.props.emptyHint': 'Click a canvas component to configure it',
  'pages.pageStudio.editor.props.noFields': 'No configuration fields',
  'pages.pageStudio.editor.props.paramMappingLabel': 'Parameter mapping',
  'pages.pageStudio.editor.props.rowActionsTitle':
    'Row actions (row-end buttons open a modal form)',
  'pages.pageStudio.editor.props.tab.actions': 'Actions',
  'pages.pageStudio.editor.props.tab.config': 'Config',
  'pages.pageStudio.editor.props.title': 'Properties',
  'pages.pageStudio.editor.props.varName.conflictError':
    'Conflicts with another component’s variable name',
  'pages.pageStudio.editor.props.varName.formatError':
    'Format: camelCase starting with a lowercase letter (ASCII letters/digits only)',
  'pages.pageStudio.editor.props.varName.label':
    'Variable name (unique per page; referenced by expressions/refreshOn/parameter mapping)',
  'pages.pageStudio.editor.props.varName.legacyHint':
    'Legacy key (not camelCase) — still referenceable in expressions',
  'pages.pageStudio.editor.props.varName.legacyTooltip':
    'Legacy page section key; keep it as-is to stay referenced, or rename to rewrite all references in sync',
  'pages.pageStudio.editor.props.varName.placeholder': 'Leave blank to auto-assign',
  'pages.pageStudio.editor.props.varName.renameHint':
    'Press Enter to rename (references rewritten in sync)',
  'pages.pageStudio.editor.quickStart.empty':
    'No composite templates yet — regenerate from contracts on the "Component Templates" page, or drag components in from the left panel',
  'pages.pageStudio.editor.quickStart.sectionCount': '{count} sections',
  'pages.pageStudio.editor.quickStart.startBlank': 'Start from blank',
  'pages.pageStudio.editor.quickStart.subtitle':
    'Pick a composite template as the page starting point, then keep dragging blocks to fine-tune',
  'pages.pageStudio.editor.quickStart.title': 'Start from a template',
  'pages.pageStudio.editor.rowActions.add': 'Add row action',
  'pages.pageStudio.editor.rowActions.addMapping': '+ Add mapping',
  'pages.pageStudio.editor.rowActions.addNoModalHint': '(create a modal with a form first)',
  'pages.pageStudio.editor.rowActions.buttonLabelPlaceholder': 'Button label (e.g. Send email)',
  'pages.pageStudio.editor.rowActions.dangerHint': 'Danger = red text + double confirmation',
  'pages.pageStudio.editor.rowActions.dangerSwitch': 'Danger',
  'pages.pageStudio.editor.rowActions.delete': 'Delete',
  'pages.pageStudio.editor.rowActions.fieldPlaceholder': "Row field, or '{{' row. '}}'",
  'pages.pageStudio.editor.rowActions.normalSwitch': 'Normal',
  'pages.pageStudio.editor.rowActions.paramMappingTitle':
    'Parameter feed-in (form params ← row fields)',
  'pages.pageStudio.editor.rowActions.paramNamePlaceholder': 'Parameter name',
  'pages.pageStudio.editor.rowActions.targetPlaceholder': 'Open modal',
};
