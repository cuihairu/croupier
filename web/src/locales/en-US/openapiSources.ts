// pages.openapiSources.* — OpenAPISources 页
export default {
  'pages.openapiSources.alert.notUi.description':
    'OpenAPI Sources are parsed into FunctionSpec / ResourceSpec / OperationSpec and PageCandidate diagnostics; a Source is not executable until a Provider is bound, and UI, menu, route, and renderer private fields in the uploaded document are rejected by the backend.',
  'pages.openapiSources.alert.notUi.message': 'A Source is not UI, nor auto-registration',
  'pages.openapiSources.alert.readOnly.description':
    'You can view sources, operations, diagnostics, and existing provider bindings; uploading, binding, and unbinding require OpenAPI Source write permission.',
  'pages.openapiSources.alert.readOnly.message': 'You are in read-only mode',
  'pages.openapiSources.binding.deleted': 'Binding deleted',
  'pages.openapiSources.binding.saveFailed': 'Failed to save binding',
  'pages.openapiSources.binding.savedModal.content':
    'A default page proposal has been generated: {proposalKey}. Open the proposal inbox to preview and publish it; it appears in the operations console menu only after publishing.',
  'pages.openapiSources.binding.savedModal.okText': 'Open Proposal',
  'pages.openapiSources.binding.savedModal.title': 'Provider binding saved',
  'pages.openapiSources.binding.savedWithoutProposal':
    'Provider binding saved, but no publishable proposal was returned. Check the proposal inbox for diagnostics.',
  'pages.openapiSources.bindingModal.alert.description':
    'httpConnector requires an allowlist, SecretRef, timeout/retry, and audit policies before it can be enabled.',
  'pages.openapiSources.bindingModal.alert.message': 'Only Provider binding is currently enabled',
  'pages.openapiSources.bindingModal.button.save': 'Save binding',
  'pages.openapiSources.bindingModal.function.placeholder': 'Select a registered function',
  'pages.openapiSources.bindingModal.providerId.placeholder':
    'Optional; leave empty to let the runtime route by function',
  'pages.openapiSources.bindingModal.title.bindProvider': 'Bind Provider',
  'pages.openapiSources.bindingModal.title.withOperation': 'Bind {operationId}',
  'pages.openapiSources.button.open': 'Open',
  'pages.openapiSources.button.refresh': 'Refresh',
  'pages.openapiSources.button.update': 'Update',
  'pages.openapiSources.button.upload': 'Upload Source',
  'pages.openapiSources.card.latestDiagnostics': 'Latest Diagnostics',
  'pages.openapiSources.column.actions': 'Actions',
  'pages.openapiSources.column.diagnosticCount': 'Diagnostics',
  'pages.openapiSources.column.operationCount': 'Operations',
  'pages.openapiSources.column.updatedAt': 'Updated At',
  'pages.openapiSources.column.version': 'Version',
  'pages.openapiSources.drawer.button.bind': 'Bind',
  'pages.openapiSources.drawer.button.delete': 'Delete',
  'pages.openapiSources.drawer.button.updateSource': 'Update Source',
  'pages.openapiSources.drawer.card.rawJson': 'Raw OpenAPI JSON',
  'pages.openapiSources.drawer.column.capabilityContract': 'Capability Contract',
  'pages.openapiSources.drawer.empty.diagnostics': 'No diagnostics',
  'pages.openapiSources.drawer.popconfirm.deleteBinding': 'Delete this binding?',
  'pages.openapiSources.drawer.tag.noApproval': 'No approval',
  'pages.openapiSources.drawer.tag.noCapability': 'No capability',
  'pages.openapiSources.drawer.tag.noExecution': 'No execution',
  'pages.openapiSources.drawer.tag.noOperation': 'No operation',
  'pages.openapiSources.drawer.tag.noPermission': 'No permission',
  'pages.openapiSources.drawer.tag.noResource': 'No resource',
  'pages.openapiSources.drawer.tag.noRisk': 'No risk',
  'pages.openapiSources.drawer.text.readonly': 'Read-only',
  'pages.openapiSources.error.createSourceFailed': 'Failed to create OpenAPI Source',
  'pages.openapiSources.error.loadSourceFailed': 'Failed to load OpenAPI Source',
  'pages.openapiSources.error.noWritePermission': 'No OpenAPI Source write permission',
  'pages.openapiSources.message.missingUpdateTarget': 'Missing OpenAPI Source to update',
  'pages.openapiSources.message.pasteJson': 'Please paste the new OpenAPI JSON',
  'pages.openapiSources.message.selectFunction': 'Please select a function to bind',
  'pages.openapiSources.message.sourceCreated': 'OpenAPI Source created',
  'pages.openapiSources.message.sourceUpdated': 'OpenAPI Source updated',
  'pages.openapiSources.message.uploadOrPaste': 'Please upload a file or paste OpenAPI JSON',
  'pages.openapiSources.message.validationFailed':
    'OpenAPI Source validation failed; check the diagnostics',
  'pages.openapiSources.page.subTitle':
    'Uploading OpenAPI only produces capability contracts and diagnostics; executability requires an explicitly bound Provider, and page UI is still decided in Page Studio.',
  'pages.openapiSources.parse.missingInfoObject': 'OpenAPI JSON is missing the info object',
  'pages.openapiSources.parse.missingOpenapiField': 'OpenAPI JSON is missing the openapi field',
  'pages.openapiSources.parse.mustBeObject': 'OpenAPI JSON must be an object',
  'pages.openapiSources.sourceModal.alert.description.update':
    'Updating refreshes the Source operations and diagnostics and keeps existing provider bindings; OpenAPI must not describe UI—only x-resource/x-operation/x-capability/x-execution/x-risk/x-enabled/x-permission are allowed.',
  'pages.openapiSources.sourceModal.alert.description.upload':
    'OpenAPI must not describe UI; only x-resource/x-operation/x-capability/x-execution/x-risk/x-enabled/x-permission are allowed.',
  'pages.openapiSources.sourceModal.alert.message.update':
    'Updating only produces a new Source revision',
  'pages.openapiSources.sourceModal.alert.message.upload': 'Do not describe UI in OpenAPI',
  'pages.openapiSources.sourceModal.button.selectFile': 'Select JSON/YAML file',
  'pages.openapiSources.sourceModal.name.placeholder': 'Optional; defaults to info.title',
  'pages.openapiSources.sourceModal.okText.create': 'Create',
  'pages.openapiSources.sourceModal.okText.update': 'Update revision',
  'pages.openapiSources.sourceModal.spec.placeholder.create':
    'Or paste OpenAPI JSON. Use file upload for YAML.',
  'pages.openapiSources.sourceModal.spec.placeholder.update':
    'Paste the new OpenAPI JSON. For YAML updates, use the raw PUT API.',
  'pages.openapiSources.sourceModal.title.update': 'Update OpenAPI Source',
  'pages.openapiSources.sourceModal.title.upload': 'Upload OpenAPI Source',
};
