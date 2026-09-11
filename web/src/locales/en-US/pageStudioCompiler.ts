// pages.pageStudio.compiler.* — CompositeEditor compiler (compile/decompile) diagnostics
// and types (view metadata / param-candidate prop labels)
export default {
  'pages.pageStudio.compiler.paramProp.autoRun': 'Auto run',
  'pages.pageStudio.compiler.paramProp.span': 'Grid span',
  'pages.pageStudio.compiler.paramProp.title': 'Title',
  'pages.pageStudio.compiler.view.actions.hint': 'Parameter-free actions that run on click',
  'pages.pageStudio.compiler.view.actions.label': 'Button group',
  'pages.pageStudio.compiler.view.fields.hint': 'Key-value details for a single object',
  'pages.pageStudio.compiler.view.fields.label': 'Field card',
  'pages.pageStudio.compiler.view.form.hint': 'Enter parameters and run an action',
  'pages.pageStudio.compiler.view.form.label': 'Action form',
  'pages.pageStudio.compiler.view.table.hint': 'List query showing multiple rows',
  'pages.pageStudio.compiler.view.table.label': 'Table',
  'pages.pageStudio.compiler.warning.buttonAfterTable':
    'Button "{title}" must be placed after a table (compiled as a table toolbar button); ignored',
  'pages.pageStudio.compiler.warning.buttonEmptyModalTarget':
    'Button "{title}" has an invalid modal target (empty modal); ignored',
  'pages.pageStudio.compiler.warning.buttonModalTargetInvalid':
    'Button "{title}" has an invalid modal target; ignored',
  'pages.pageStudio.compiler.warning.buttonNoAction':
    'Button "{title}" has no action configured; ignored',
  'pages.pageStudio.compiler.warning.buttonTargetInvalid':
    'Button "{title}" has an invalid action target; ignored',
  'pages.pageStudio.compiler.warning.buttonTargetLost':
    'Modal target {target} of button "{label}" could not be restored; dropped',
  'pages.pageStudio.compiler.warning.emptyModal': 'Modal "{title}" is empty; ignored',
  'pages.pageStudio.compiler.warning.expressionUnknownVariable':
    'Expression "{value}" of param "{param}" on section "{title}" references an unknown variable or row context; saved as literal',
  'pages.pageStudio.compiler.warning.invalidStaticSchema':
    'JSON definition of constant form "{title}" is invalid; skipped',
  'pages.pageStudio.compiler.warning.mappingSourceInvalid':
    'Source node of param "{param}" on section "{title}" is no longer valid; skipped',
  'pages.pageStudio.compiler.warning.mappingSourceMissing':
    'Mapping source "{sourceKey}" of section "{ownerKey}" does not exist; literal reference kept',
  'pages.pageStudio.compiler.warning.missingFunctionBinding':
    'Component "{title}" has no function binding; ignored',
  'pages.pageStudio.compiler.warning.rowActionMissingTarget':
    'Table "{title}" has a row action without a target; ignored',
  'pages.pageStudio.compiler.warning.rowActionParamNestedPath':
    'Nested field "{path}" of row action param "{param}" on table "{title}" cannot be evaluated after publishing; saved as literal',
  'pages.pageStudio.compiler.warning.rowActionParamRowOnly':
    'Expression "{value}" of row action param "{param}" on table "{title}" only supports \'{{\'row.field\'}}\'; saved as literal',
  'pages.pageStudio.compiler.warning.rowActionTargetLost':
    'Modal target {target} of row action "{label}" could not be restored; dropped',
  'pages.pageStudio.compiler.warning.sectionKeyInvalid':
    'Key "{key}" of section "{title}" is invalid or duplicated; auto-assigned',
  'pages.pageStudio.compiler.warning.sectionMissingFunction':
    'Section has no function binding; skipped',
  'pages.pageStudio.compiler.warning.successRefreshLost':
    '"Refresh on success" reference {target} could not be restored; dropped',
  'pages.pageStudio.compiler.warning.textSkipped':
    'Text "{content}" is not included in publishing (V1)',
  'pages.pageStudio.compiler.warning.unknownMappingKind':
    'Mapping kind "{kind}" of param "{param}" on section "{title}" is unknown; skipped',
  'pages.pageStudio.compiler.warning.unknownNodeType': 'Unknown component type {type}; ignored',
};
