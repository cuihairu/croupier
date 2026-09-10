// pages.openapiSources.* — OpenAPISources 页
export default {
  'pages.openapiSources.alert.notUi.description':
    'OpenAPI Source 用于解析 FunctionSpec / ResourceSpec / OperationSpec 和 PageCandidate 诊断；Source 未绑定 Provider 前不可执行，上传文档中的 UI、菜单、路由和 renderer 私有字段会被后端拒绝。',
  'pages.openapiSources.alert.notUi.message': 'Source 不是 UI，也不是自动注册',
  'pages.openapiSources.alert.readOnly.description':
    '你可以查看 Source、operation、diagnostics 和现有 Provider binding；上传、绑定和解绑需要 OpenAPI Source 写权限。',
  'pages.openapiSources.alert.readOnly.message': '当前是只读模式',
  'pages.openapiSources.binding.deleted': 'binding 已删除',
  'pages.openapiSources.binding.saveFailed': '保存 binding 失败',
  'pages.openapiSources.binding.savedModal.content':
    '已生成默认页面 Proposal：{proposalKey}。请进入 Proposal 队列预览并发布，发布后才会出现在运行控制台菜单。',
  'pages.openapiSources.binding.savedModal.okText': '打开 Proposal',
  'pages.openapiSources.binding.savedModal.title': 'Provider binding 已保存',
  'pages.openapiSources.binding.savedWithoutProposal':
    'Provider binding 已保存，但未返回可发布 Proposal。请在 Proposal 队列查看诊断。',
  'pages.openapiSources.button.open': '打开',
  'pages.openapiSources.button.refresh': '刷新',
  'pages.openapiSources.button.update': '更新',
  'pages.openapiSources.button.upload': '上传 Source',
  'pages.openapiSources.card.latestDiagnostics': '最近一次诊断',
  'pages.openapiSources.column.actions': '操作',
  'pages.openapiSources.column.diagnosticCount': '诊断',
  'pages.openapiSources.column.operationCount': '操作数',
  'pages.openapiSources.column.updatedAt': '更新时间',
  'pages.openapiSources.column.version': '版本',
  'pages.openapiSources.error.createSourceFailed': '创建 OpenAPI Source 失败',
  'pages.openapiSources.error.loadSourceFailed': '加载 OpenAPI Source 失败',
  'pages.openapiSources.error.noWritePermission': '没有 OpenAPI Source 写权限',
  'pages.openapiSources.message.missingUpdateTarget': '缺少要更新的 OpenAPI Source',
  'pages.openapiSources.message.pasteJson': '请粘贴新的 OpenAPI JSON',
  'pages.openapiSources.message.selectFunction': '请选择要绑定的函数',
  'pages.openapiSources.message.sourceCreated': 'OpenAPI Source 已创建',
  'pages.openapiSources.message.sourceUpdated': 'OpenAPI Source 已更新',
  'pages.openapiSources.message.uploadOrPaste': '请上传文件或粘贴 OpenAPI JSON',
  'pages.openapiSources.message.validationFailed': 'OpenAPI Source 校验失败，请查看诊断',
  'pages.openapiSources.page.subTitle':
    '上传 OpenAPI 只产生能力契约和诊断；可执行性必须显式绑定 Provider，页面 UI 仍在 Page Studio 确定。',
};
