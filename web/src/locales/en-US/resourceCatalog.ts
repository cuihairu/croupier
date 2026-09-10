// pages.resourceCatalog.detail.* — ResourceCatalog resource details modal
// pages.resourceCatalog.editSemantics.* — ResourceCatalog edit semantics modal
// pages.resourceCatalog.list.* — ResourceCatalog resource capability catalog list page
export default {
  // detail
  'pages.resourceCatalog.detail.alert.description':
    'identity, collection, and lifecycle function bindings are captured here; page titles, menus, columns, and button placement belong to Page Proposal/Page Studio.',
  'pages.resourceCatalog.detail.alert.message':
    'Resource Catalog maintains resource capability semantics only',
  'pages.resourceCatalog.detail.button.chooseSource': 'Choose Source',
  'pages.resourceCatalog.detail.button.viewProposals': 'View Related Proposals',
  'pages.resourceCatalog.detail.column.actions': 'Actions',
  'pages.resourceCatalog.detail.column.candidateValues': 'Candidate Values',
  'pages.resourceCatalog.detail.column.capability': 'Capability',
  'pages.resourceCatalog.detail.column.changeReason': 'Change Reason',
  'pages.resourceCatalog.detail.column.code': 'Code',
  'pages.resourceCatalog.detail.column.confidence': 'Confidence',
  'pages.resourceCatalog.detail.column.createdAt': 'Created At',
  'pages.resourceCatalog.detail.column.createdBy': 'Created By',
  'pages.resourceCatalog.detail.column.execution': 'Execution',
  'pages.resourceCatalog.detail.column.field': 'Field',
  'pages.resourceCatalog.detail.column.function': 'Function',
  'pages.resourceCatalog.detail.column.functionId': 'Function ID',
  'pages.resourceCatalog.detail.column.kind': 'Type',
  'pages.resourceCatalog.detail.column.message': 'Message',
  'pages.resourceCatalog.detail.column.pageKey': 'Page Key',
  'pages.resourceCatalog.detail.column.resolution': 'Resolution',
  'pages.resourceCatalog.detail.column.risk': 'Risk',
  'pages.resourceCatalog.detail.column.severity': 'Severity',
  'pages.resourceCatalog.detail.column.source': 'Source',
  'pages.resourceCatalog.detail.column.status': 'Status',
  'pages.resourceCatalog.detail.column.title': 'Title',
  'pages.resourceCatalog.detail.column.updatedAt': 'Updated At',
  'pages.resourceCatalog.detail.column.updatedBy': 'Updated By',
  'pages.resourceCatalog.detail.column.value': 'Value',
  'pages.resourceCatalog.detail.column.version': 'Version',
  'pages.resourceCatalog.detail.desc.category': 'Category',
  'pages.resourceCatalog.detail.desc.collectionPath': 'Collection Path',
  'pages.resourceCatalog.detail.desc.conflicts': 'Unresolved Conflicts',
  'pages.resourceCatalog.detail.desc.identityType': 'Identity Type',
  'pages.resourceCatalog.detail.desc.itemsField': 'Items Field',
  'pages.resourceCatalog.detail.desc.name': 'Name',
  'pages.resourceCatalog.detail.desc.resourceKey': 'Resource Key',
  'pages.resourceCatalog.detail.desc.source': 'Source',
  'pages.resourceCatalog.detail.desc.status': 'Status',
  'pages.resourceCatalog.detail.desc.totalField': 'Total Field',
  'pages.resourceCatalog.detail.desc.version': 'Version',
  'pages.resourceCatalog.detail.pagination.total': 'Total {total} items',
  'pages.resourceCatalog.detail.section.affectedPages.empty':
    'No drafts, published pages, or proposals for this resource yet',
  'pages.resourceCatalog.detail.section.affectedPages.title': 'Affected Pages',
  'pages.resourceCatalog.detail.section.conflicts.empty': 'No unresolved conflicts',
  'pages.resourceCatalog.detail.section.conflicts.title': 'Semantic Conflicts',
  'pages.resourceCatalog.detail.section.diagnostics.title': 'Diagnostics',
  'pages.resourceCatalog.detail.section.functions.title': 'Functions',
  'pages.resourceCatalog.detail.section.provenance.empty': 'No field-level provenance records',
  'pages.resourceCatalog.detail.section.provenance.title': 'Semantic Provenance',
  'pages.resourceCatalog.detail.section.semantics.title': 'Semantics',
  'pages.resourceCatalog.detail.section.versions.empty': 'No semantic version records',
  'pages.resourceCatalog.detail.section.versions.title': 'Semantic Versions',
  'pages.resourceCatalog.detail.section.versions.total': '({total} in total)',
  'pages.resourceCatalog.detail.tag.configured': 'Configured',
  'pages.resourceCatalog.detail.tag.disabled': 'Disabled',
  'pages.resourceCatalog.detail.tag.enabled': 'Enabled',
  'pages.resourceCatalog.detail.tag.notConfigured': 'Not Configured',
  'pages.resourceCatalog.detail.tag.unresolved': 'Unresolved',
  'pages.resourceCatalog.detail.title': 'Resource Details',
  // editSemantics
  'pages.resourceCatalog.editSemantics.actionList.add': 'Add Action',
  'pages.resourceCatalog.editSemantics.actionList.alert.description':
    'subject determines whether the action targets a single row, the selected set, or the entire resource; button placement is decided by the PageProposal generator and is not configured here.',
  'pages.resourceCatalog.editSemantics.actionList.alert.message':
    'Describe only the resource context required by the action',
  'pages.resourceCatalog.editSemantics.actionList.function.label': 'Function',
  'pages.resourceCatalog.editSemantics.actionList.function.placeholder':
    'Select an action function',
  'pages.resourceCatalog.editSemantics.actionList.function.required':
    'Please select an action function',
  'pages.resourceCatalog.editSemantics.actionList.identityInput.placeholder':
    '/playerId or /playerIds',
  'pages.resourceCatalog.editSemantics.actionList.identityInput.tooltip':
    'Required for resource_item/resource_selection, e.g. /playerId or /playerIds; can be left empty for none',
  'pages.resourceCatalog.editSemantics.actionList.remove': 'Delete',
  'pages.resourceCatalog.editSemantics.actionList.subject.none': 'Entire resource',
  'pages.resourceCatalog.editSemantics.actionList.subject.required': 'Please select a subject',
  'pages.resourceCatalog.editSemantics.actionList.subject.resourceItem': 'Single resource object',
  'pages.resourceCatalog.editSemantics.actionList.subject.resourceSelection':
    'Selected resource set',
  'pages.resourceCatalog.editSemantics.actionList.title': 'Resource Action Semantics',
  'pages.resourceCatalog.editSemantics.alert.description':
    'Select the function database ID under the current resource as the lifecycle binding; saving records the platform_review source, creates a semantics version, and triggers recalculation of related proposals.',
  'pages.resourceCatalog.editSemantics.alert.message':
    'Only capability semantics are captured here; page UI is not edited',
  'pages.resourceCatalog.editSemantics.button.cancel': 'Cancel',
  'pages.resourceCatalog.editSemantics.button.save': 'Save',
  'pages.resourceCatalog.editSemantics.form.changeReason.label': 'Change Reason',
  'pages.resourceCatalog.editSemantics.form.changeReason.placeholder':
    'Describe the reason for the change',
  'pages.resourceCatalog.editSemantics.form.collectionPath.label': 'Collection Path',
  'pages.resourceCatalog.editSemantics.form.collectionPath.placeholder': 'e.g. /players',
  'pages.resourceCatalog.editSemantics.form.collectionQuery.placeholder':
    'Select the collection query function',
  'pages.resourceCatalog.editSemantics.form.create.placeholder': 'Select the create function',
  'pages.resourceCatalog.editSemantics.form.delete.placeholder': 'Select the delete function',
  'pages.resourceCatalog.editSemantics.form.identityField.label': 'Identity Field',
  'pages.resourceCatalog.editSemantics.form.identityField.placeholder': 'e.g. id, player_id',
  'pages.resourceCatalog.editSemantics.form.identityPath.label': 'Identity Path',
  'pages.resourceCatalog.editSemantics.form.identityPath.placeholder': 'e.g. id or /data/id',
  'pages.resourceCatalog.editSemantics.form.identityType.label': 'Identity Type',
  'pages.resourceCatalog.editSemantics.form.itemPath.label': 'Item Path',
  'pages.resourceCatalog.editSemantics.form.itemPath.placeholder': "e.g. /players/'{'player_id'}'",
  'pages.resourceCatalog.editSemantics.form.itemQuery.placeholder':
    'Select the item query function',
  'pages.resourceCatalog.editSemantics.form.itemsFieldName.label': 'Items Field',
  'pages.resourceCatalog.editSemantics.form.itemsFieldName.placeholder': 'Defaults to items',
  'pages.resourceCatalog.editSemantics.form.pageFieldName.label': 'Page Field',
  'pages.resourceCatalog.editSemantics.form.pageFieldName.placeholder': 'Defaults to page',
  'pages.resourceCatalog.editSemantics.form.pageSizeFieldName.label': 'Page Size Field',
  'pages.resourceCatalog.editSemantics.form.pageSizeFieldName.placeholder': 'Defaults to page_size',
  'pages.resourceCatalog.editSemantics.form.totalFieldName.label': 'Total Field',
  'pages.resourceCatalog.editSemantics.form.totalFieldName.placeholder': 'Defaults to total',
  'pages.resourceCatalog.editSemantics.form.update.placeholder': 'Select the update function',
  'pages.resourceCatalog.editSemantics.reportList.add': 'Add Report',
  'pages.resourceCatalog.editSemantics.reportList.alert.description':
    'datasetPath points to the array in the query result; dimensions/metrics are JSON Pointers relative to the dataset item. Chart types and table presentation belong to Page Proposal/Page Studio.',
  'pages.resourceCatalog.editSemantics.reportList.alert.message':
    'Describe only the report dataset',
  'pages.resourceCatalog.editSemantics.reportList.datasetPath.placeholder':
    '/dataset or empty root path',
  'pages.resourceCatalog.editSemantics.reportList.datasetPath.tooltip':
    'Leave empty for a root array; for an object field array, e.g. /dataset or /data/items',
  'pages.resourceCatalog.editSemantics.reportList.dimensions.required':
    'Enter at least one dimension pointer',
  'pages.resourceCatalog.editSemantics.reportList.metrics.required':
    'Enter at least one metric pointer',
  'pages.resourceCatalog.editSemantics.reportList.query.label': 'Query Function',
  'pages.resourceCatalog.editSemantics.reportList.query.placeholder': 'Select the report function',
  'pages.resourceCatalog.editSemantics.reportList.query.required':
    'Please select the report function',
  'pages.resourceCatalog.editSemantics.reportList.remove': 'Delete',
  'pages.resourceCatalog.editSemantics.reportList.title': 'Report Semantics',
  'pages.resourceCatalog.editSemantics.taskList.add': 'Add Task',
  'pages.resourceCatalog.editSemantics.taskList.alert.description':
    'start must be a task-capability function; status/events/result/cancel only declare the real functions and the taskId input paths, and page button placement is not configured here. The platform currently has no retry runtime, so retry semantics are not captured.',
  'pages.resourceCatalog.editSemantics.taskList.alert.message':
    'Describe only task lifecycle capabilities',
  'pages.resourceCatalog.editSemantics.taskList.cancel.label': 'Cancel Function',
  'pages.resourceCatalog.editSemantics.taskList.cancelFunction.placeholder':
    'Select the cancel function',
  'pages.resourceCatalog.editSemantics.taskList.events.label': 'Events Function',
  'pages.resourceCatalog.editSemantics.taskList.eventsFunction.placeholder':
    'Select the events function',
  'pages.resourceCatalog.editSemantics.taskList.remove': 'Delete Task Semantics',
  'pages.resourceCatalog.editSemantics.taskList.result.label': 'Result Function',
  'pages.resourceCatalog.editSemantics.taskList.resultFunction.placeholder':
    'Select the result function',
  'pages.resourceCatalog.editSemantics.taskList.start.label': 'Start Function',
  'pages.resourceCatalog.editSemantics.taskList.start.required':
    'Please select the task start function',
  'pages.resourceCatalog.editSemantics.taskList.statePath.required':
    'Please enter the state output path',
  'pages.resourceCatalog.editSemantics.taskList.status.label': 'Status Function',
  'pages.resourceCatalog.editSemantics.taskList.status.required':
    'Please select the status function',
  'pages.resourceCatalog.editSemantics.taskList.statusFunction.placeholder':
    'Select the status function',
  'pages.resourceCatalog.editSemantics.taskList.statusTaskIdInput.required':
    'Please enter the status taskId input path',
  'pages.resourceCatalog.editSemantics.taskList.taskFunction.placeholder': 'Select a task function',
  'pages.resourceCatalog.editSemantics.taskList.taskIdResultPath.placeholder':
    '/taskId or empty root path',
  'pages.resourceCatalog.editSemantics.taskList.taskIdResultPath.required':
    'Please enter the taskId output path',
  'pages.resourceCatalog.editSemantics.taskList.taskIdType.label': 'TaskID Type',
  'pages.resourceCatalog.editSemantics.taskList.taskIdType.required':
    'Please select the taskId type',
  'pages.resourceCatalog.editSemantics.taskList.title': 'Task Semantics',
  'pages.resourceCatalog.editSemantics.title': 'Edit Semantics',
  // list
  'pages.resourceCatalog.list.button.refresh': 'Refresh',
  'pages.resourceCatalog.list.button.search': 'Search',
  'pages.resourceCatalog.list.card.title': 'Resource Capability Catalog',
  'pages.resourceCatalog.list.column.actions': 'Actions',
  'pages.resourceCatalog.list.column.category': 'Category',
  'pages.resourceCatalog.list.column.diagnostics': 'Diagnostics',
  'pages.resourceCatalog.list.column.functionCount': 'Functions',
  'pages.resourceCatalog.list.column.labels': 'Name',
  'pages.resourceCatalog.list.column.resourceKey': 'Resource Key',
  'pages.resourceCatalog.list.column.semanticsVersion': 'Semantics Version',
  'pages.resourceCatalog.list.column.status': 'Status',
  'pages.resourceCatalog.list.diagnostics.errorCount': '{count} errors',
  'pages.resourceCatalog.list.diagnostics.none': 'None',
  'pages.resourceCatalog.list.diagnostics.warningCount': '{count} warnings',
  'pages.resourceCatalog.list.error.fetchDetailFailed': 'Failed to load details: {message}',
  'pages.resourceCatalog.list.error.fetchVersionsFailed':
    'Failed to load semantic versions: {message}',
  'pages.resourceCatalog.list.error.operationFailed': 'Operation failed',
  'pages.resourceCatalog.list.error.resolveConflictFailed': 'Failed to resolve conflict: {message}',
  'pages.resourceCatalog.list.error.unknown': 'Unknown error',
  'pages.resourceCatalog.list.error.updateFailed': 'Update failed: {message}',
  'pages.resourceCatalog.list.message.conflictResolved':
    'Conflict resolved; related proposals have been recalculated',
  'pages.resourceCatalog.list.message.semanticsSaved': 'Semantics updated',
  'pages.resourceCatalog.list.pagination.total': 'Total {total} items',
  'pages.resourceCatalog.list.search.categoryPlaceholder': 'Select a category',
  'pages.resourceCatalog.list.search.placeholder': 'Search resources',
  'pages.resourceCatalog.list.tooltip.editSemantics': 'Edit semantics',
  'pages.resourceCatalog.list.tooltip.proposals': 'Proposals',
  'pages.resourceCatalog.list.tooltip.viewDetail': 'View details',
};
