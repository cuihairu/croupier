/**
 * Console/TemplatePlayerManage - 玩家管理 CRUD 模板预览页
 *
 * 路由：/console/templates/player-manage
 * 使用内置 demo 执行器渲染完整 CRUD 模板（列表/详情/封禁/充值/邮件），
 * 仅用于样式与交互预览，不触发真实函数调用。
 */

import PlayerManageTemplate from '@/components/PageRenderer/templates/playerManage';

export default function TemplatePlayerManagePage() {
  return <PlayerManageTemplate />;
}
