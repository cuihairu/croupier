# 用户反馈跟踪（OPEN ISSUES）

用户在 dashboard 验收中提出的问题统一记录在此，**逐项闭环（修复+线上验证）后才移动到「已闭环」**。
规则：用户不该重复提醒同一问题；每项必须有状态与闭环证据（提交/线上验证）。

最近更新：2026-09-26

## 未闭环

| # | 位置 | 问题 | 状态 |
| --- | --- | --- | --- |
| 1 | `sdk-distribution` | demo 实例应注册元数据（至少 serverId），页面才有数据可看 | **已闭环**：Go SDK 重连路径丢元数据已修复（154d236）；镜像重建+demo 容器重建后线上生效 |
| 2 | `sdk-distribution` | 元数据过滤应提供下拉框（选项来自实例实际元数据的 key/value） | 进行中：依赖 #11 存储方案落地后的聚合 API |
| 3 | `sdk-distribution` | 元数据列应展示在第二列（现为倒数第二），且列宽太窄 | 待做（UI 小改） |
| 4 | `functions/catalog` | 页面无数据，但 demo 注册了函数（API 实测已返回 29 个函数，疑似前端渲染/过滤问题） | 排查中 |
| 5 | `resource-catalog` | 「选择分类」下拉框失效（用户多次反馈） | 排查中（多次反馈，优先处理） |
| 6 | `component-templates` | 生成的常量组件应支持编辑——常量随功能开发会新增/修改，不能只读 | 待做（功能） |
| 7 | `functions/menus` | 菜单项之间间隙太小，应加大 | 待做（样式小改） |
| 8 | 菜单结构 | 账号中心不必拆「个人中心」+「消息通知」两个菜单项——一层「个人中心」（页内已有 tab）即可 | 待做 |
| 9 | 菜单结构 | 顶层「系统管理」下的「基础配置」层级多余，菜单应扁平化（单子菜单不上卷一层） | 待做 |
| 10 | `openapi-provider-demo` | openapi-demo-agent 也应支持注册元数据：agent 启动参数指定，支持多个 key=value | 待做 |
| 11 | 元数据存储 | 元数据目前纯内存态（agent agentlocal + server registry 快照，重启即失）。定案：EAV 单表 `(game_id, env, service_id, meta_key, meta_value)`；server 端缓存去重关键字（含 value 集合），每次注册刷新，查询接口返回给前端做下拉选项 | 待做（含编号迁移，按迁移契约三处同步） |
| 12 | `functions/pages` | 每次重启或 agent 重连后就出现一堆需人工处理的警告——demo 重启并不会改字段，属误报 bug（此前称修过，但仍在复现） | 排查中（回归 bug，优先） |
| 13 | `functions/pages` | 应支持按资源（resource）过滤：提供下拉框 | 待做（UI 小改） |

## 已闭环

| # | 位置 | 问题 | 闭环证据 |
| --- | --- | --- | --- |
| — | `sdk-distribution` | 元数据端到端丢失（SDK 交接链两处 + agent TCP 路径 + SDK 重连路径，共 4 个丢失点） | 34d9cb8 + fc578b3 + 154d236 + 重连修复（见 BUGS.md BUG-029）；线上 verify2 实例 metadata 可查、metaKey/metaValue 过滤命中 |
| — | `ops/nodes` | 无数据 | 结论：game/env scope 隔离，属设计行为（已向用户说明） |
| — | lb-stats | 500 报错 | telemetry 栈已拉起，PromQL 查询 200 |

> 维护约定：处理任意一项时先在本文件更新状态再动代码；闭环时把证据（提交号/线上验证结果）写进「已闭环」。
