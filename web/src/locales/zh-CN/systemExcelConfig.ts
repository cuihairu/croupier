// pages.systemExcelConfig.* — System/ExcelConfig
export default {
  'pages.systemExcelConfig.action.addRow': '+ 行',
  'pages.systemExcelConfig.action.export': '导出 .xlsx',
  'pages.systemExcelConfig.action.import': '导入 .xlsx 为草稿',
  'pages.systemExcelConfig.action.resetDraft': '重置草稿',
  'pages.systemExcelConfig.action.save': '保存并发布',
  'pages.systemExcelConfig.action.uploadCompile': '服务端编译上传',
  'pages.systemExcelConfig.cardTitle': '表格配置（Excel 在线编译）',
  'pages.systemExcelConfig.column.dataField': '字段 / 数据',
  'pages.systemExcelConfig.column.headerPlaceholder': '字段名',
  'pages.systemExcelConfig.column.index': '列 {index}',
  'pages.systemExcelConfig.column.typePlaceholder': '类型',
  'pages.systemExcelConfig.commitPlaceholder': '版本说明（可选）',
  'pages.systemExcelConfig.confirm.description':
    '保存会生成新的 gameplay 配置版本，游戏服将收到变更通知。',
  'pages.systemExcelConfig.confirm.title': '注册新版本并热更下发？',
  'pages.systemExcelConfig.convention':
    '约定：首行=字段名；可选第二行首格以 # 开头=类型行（int/string/float/bool，逐列对齐）；空行忽略。草稿自动存本地，保存后在「配置版本」中可查看与回滚。',
  'pages.systemExcelConfig.error.importParse': '解析失败',
  'pages.systemExcelConfig.error.saveFailed': '保存失败',
  'pages.systemExcelConfig.error.uploadFailed': '上传编译失败',
  'pages.systemExcelConfig.imported': '已导入 {count} 个 sheet（草稿，保存后注册新版本）',
  'pages.systemExcelConfig.keyPlaceholder': '配置 key（如 shop.items）',
  'pages.systemExcelConfig.latestVersion': '最新版本 v{version}（{sheets} 表 / {rows} 行）',
  'pages.systemExcelConfig.noSheets': '文件没有 sheet',
  'pages.systemExcelConfig.registered': '已注册版本 v{version}（{sheets} 表 / {rows} 行）',
  'pages.systemExcelConfig.serverCompiled': '服务端编译完成：v{version}（{rows} 行）',
};
