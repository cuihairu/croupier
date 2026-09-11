// pages.opsRateLimits.* — Ops/RateLimits 限速管理页
export default {
  'pages.opsRateLimits.error.operationFailed': '操作失败',
  'pages.opsRateLimits.error.previewFailed': '预览失败',
  'pages.opsRateLimits.form.percentTooltip':
    '按比例生效（函数灰度为按 trace 采样；服务灰度折算 QPS）',
  'pages.opsRateLimits.message.labelsIgnored': '标签JSON解析失败，已忽略',
  'pages.opsRateLimits.message.saved': '已保存',
  'pages.opsRateLimits.message.servicePreviewOnly': '仅支持服务级预览',
  'pages.opsRateLimits.preview.currentQpsLabel': '当前QPS: ',
  'pages.opsRateLimits.preview.limitLabel': '限速: ',
  'pages.opsRateLimits.preview.matched': '命中实例：{count}',
  'pages.opsRateLimits.preview.onlyOverCheckbox': '仅显示超限（当前QPS>限速）',
};
