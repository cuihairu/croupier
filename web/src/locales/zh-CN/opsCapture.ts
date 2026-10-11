export default {
  'menu.Ops.Capture': '捕获规则',

  'pages.opsCapture.gates.title': '闸规则下发',
  'pages.opsCapture.reload': '重载',
  'pages.opsCapture.template': '插入模板',
  'pages.opsCapture.publish': '发布',
  'pages.opsCapture.publish.confirm': '发布为整份新版本并热下发到该 scope 的 capture-agent？',
  'pages.opsCapture.publish.success': '已发布版本 {version}，capture-agent 将在轮询周期内热生效',
  'pages.opsCapture.validate': '校验',
  'pages.opsCapture.validate.pass':
    '校验通过（与 capture-agent 生效同一实现）：对账 {reconcile} · 时段 {cepOffHours} · 突变 {cepBurst} · 试探 {cepProbing} · 血缘 {lineage} · 事件过滤 {eventOps}',
  'pages.opsCapture.validate.failed': '校验失败',
  'pages.opsCapture.error.loadFailed': '加载闸规则失败',
  'pages.opsCapture.error.publishFailed': '发布失败',
  'pages.opsCapture.gates.keyLabel': '配置键',
  'pages.opsCapture.gates.unpublished': '未发布（agent 用文件规则）',
  'pages.opsCapture.gates.version': '版本 {version}',
  'pages.opsCapture.gates.boundary':
    '「校验」按钮做与 capture-agent 生效同一实现的语义预检（lint + 表路由唯一性/阈值/事件过滤，config.ValidateGates）；发布时服务端仍只做结构检查，未跑校验直接发布的语义问题由 agent 保留旧版兜底。发布成功 ≠ 已生效，上方「生效对账」为 agent 自报（轮询 30s + 重注册滞后），日志 capture gates version applied 仍是逐次生效的原始真值。',
  'pages.opsCapture.gates.placeholder': 'capture.gates JSON（reconcile / cep / lineage 三段）',
  'pages.opsCapture.gates.help':
    '发布按顶栏 game/env scope 存 game 库（config_versions），capture-agent 经上行通道 30s 版本轮询拉取并整体热替换；断连或校验失败时按旧规则继续运行。',
  'pages.opsCapture.apply.summary': '生效对账（agent 自报）：',
  'pages.opsCapture.apply.noAgents': 'scope 内无在线 capture-agent',
  'pages.opsCapture.apply.allApplied': '{count}/{total} 台已生效',
  'pages.opsCapture.apply.partial': '{count}/{total} 台已生效（其余未上报或滞后）',
  'pages.opsCapture.apply.unknown': '未上报',
};
