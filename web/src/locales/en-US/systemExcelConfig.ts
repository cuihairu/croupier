// pages.systemExcelConfig.* — System/ExcelConfig
export default {
  'pages.systemExcelConfig.action.addRow': '+ Row',
  'pages.systemExcelConfig.action.export': 'Export .xlsx',
  'pages.systemExcelConfig.action.import': 'Import .xlsx as Draft',
  'pages.systemExcelConfig.action.resetDraft': 'Reset Draft',
  'pages.systemExcelConfig.action.save': 'Save and Publish',
  'pages.systemExcelConfig.action.uploadCompile': 'Upload for Server-side Compile',
  'pages.systemExcelConfig.cardTitle': 'Table Config (Excel Online Compiler)',
  'pages.systemExcelConfig.column.dataField': 'Fields / Data',
  'pages.systemExcelConfig.column.headerPlaceholder': 'Field name',
  'pages.systemExcelConfig.column.index': 'Column {index}',
  'pages.systemExcelConfig.column.typePlaceholder': 'Type',
  'pages.systemExcelConfig.commitPlaceholder': 'Version note (optional)',
  'pages.systemExcelConfig.confirm.description':
    'Saving creates a new gameplay config version; game servers will be notified of the change.',
  'pages.systemExcelConfig.confirm.title': 'Register a new version and hot-push it?',
  'pages.systemExcelConfig.convention':
    'Conventions: the first row holds field names; an optional second row whose first cell starts with # is the type row (int/string/float/bool, aligned per column); empty rows are ignored. Drafts are stored locally; after saving, view and roll back in "Config Versions".',
  'pages.systemExcelConfig.error.importParse': 'Failed to parse',
  'pages.systemExcelConfig.error.saveFailed': 'Save failed',
  'pages.systemExcelConfig.error.uploadFailed': 'Upload compile failed',
  'pages.systemExcelConfig.imported':
    'Imported {count} sheet(s) (draft; a new version is registered on save)',
  'pages.systemExcelConfig.keyPlaceholder': 'Config key (e.g. shop.items)',
  'pages.systemExcelConfig.latestVersion':
    'Latest version v{version} ({sheets} sheets / {rows} rows)',
  'pages.systemExcelConfig.noSheets': 'The file has no sheets',
  'pages.systemExcelConfig.registered':
    'Registered version v{version} ({sheets} sheets / {rows} rows)',
  'pages.systemExcelConfig.serverCompiled':
    'Server-side compile finished: v{version} ({rows} rows)',
};
