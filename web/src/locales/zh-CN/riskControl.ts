export default {
  'pages.riskControl.title': '风控策略',
  'pages.riskControl.reload': '重载',
  'pages.riskControl.rollback': '回滚',
  'pages.riskControl.rollback.confirm': '以该历史版本内容再发一版？',
  'pages.riskControl.rollback.success':
    '已按版本 {version} 内容发布新版本，agent 将在轮询周期内热生效',
  'pages.riskControl.error.loadFailed': '加载策略版本失败',
  'pages.riskControl.error.rollbackFailed': '回滚失败',
  'pages.riskControl.column.version': '版本',
  'pages.riskControl.column.message': '说明',
  'pages.riskControl.column.author': '发布人',
  'pages.riskControl.column.time': '发布时间',
  'pages.riskControl.column.action': '操作',
  'pages.riskControl.current': '当前',
  'pages.riskControl.unpublished': '未发布（agent 用文件规则）',
  'pages.riskControl.empty': '该 scope 尚未发布过策略版本',
  'pages.riskControl.boundary':
    '回滚不改写历史：以所选版本内容复制为新版本发布，agent 照常版本轮询热生效；发布成功 ≠ 已生效，以上「生效对账」为 agent 自报（30s 轮询 + 重注册滞后），逐次生效真值在 agent 日志 capture gates version applied。',
};
