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
};
