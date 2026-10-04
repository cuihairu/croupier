# Croupier 运维/系统维护全量核对（需求清单 #52 核销归档）

## 状态

- 状态: 核对归档（2026-10-04，需求清单 #52）
- 日期: 2026-10-04
- 范围: 用户需求「展示当前运行版本、运行开始时间、在线时长，检查更新、可自动更新」
- 结论: **已最小落地（2026-10-02）+ 已线上复证 + 记录与代码一致，零新缺口，OPEN-ISSUES #52 核销**；本批无代码改动，仅归档与台账核销

## 一、需求 → 落地对照

| 用户需求         | 落地            | 证据                                                                                                                                                                                                                                                                                                                            |
| ---------------- | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 展示当前运行版本 | ✅              | `GET /ops/system/runtime` → `version`（ldflags 注入，含 Tag/GitCommit/BuildTime）；前端运维 Tab 运行信息卡 Tag/提交/构建时间                                                                                                                                                                                                    |
| 运行开始时间     | ✅              | `startedAt` RFC3339（进程启动时间未知时为空）+ 前端人性化展示                                                                                                                                                                                                                                                                   |
| 在线时长         | ✅              | `uptimeSeconds` + 前端人性化（天/时/分）                                                                                                                                                                                                                                                                                        |
| 检查更新         | ✅              | `POST /ops/system/check-update`：L3 键 `system.updateCheckUrl`（http(s) 校验）拉 JSON 版本清单（version/tagName/tag_name/latestVersion 任意一键，兼容 GitHub releases/latest），点分数值比较（缺段补 0，非数字段诚实「无法比较」）；前端三态 Alert（未配置源 info / 已是最新 success / 发现新版本 warning，注记原样透出）+ 按钮 |
| 自动更新         | ⚠️ 最小可行收口 | **不自动执行升级**（二进制替换/重启编排另行立项），代码注释与 UI 均明示「只检查不升级」                                                                                                                                                                                                                                         |

## 二、既有记录 vs 代码核验（2026-10-04）

- 路由：`internal/handler/routes.go` 573-575 —— `/ops/system` 组下 `GET /system/runtime` + `POST /system/check-update`，组注释「只检查不升级，为 #53-57 运维家族留 /ops/system/* 组」✓
- handler：`internal/api/ops/systeminfo.go`（SystemRuntimeInfo 五字段 json 全 lowerCamelCase：version/gitCommit/buildTime/startedAt/uptimeSeconds；CheckUpdateResp currentVersion/latestVersion）+ `systeminfo_test.go` + `systeminfo_fetch_test.go`（合计 17 个 Test 函数，含版本比较表）✓
- 键：`internal/platform/settings/layered.go:171` `KeySystemUpdateCheckURL = "system.updateCheckUrl"`（L3 运行时键族）✓
- 前端：`web/src/pages/System/SiteSettings/MaintenanceTab.tsx`（运维 Tab：运行信息卡 + 检查更新卡 + docsUrl 文档链接）+ `MaintenanceTab.test.tsx` 9 用例 ✓
- docsUrl 登录后消费入口：#49 划归本板块，MaintenanceTab 读取 site.docsUrl 新窗口打开 ✓

## 三、边界（沿用原交付登记，核对后无变化）

1. 自动更新仅检查 + 版本注记，不执行升级（升级编排另行立项）。
2. 更新源未配置（updateCheckUrl 空/非法）时按钮仅回版本注记，不报错。
3. 运维 Tab 挂在 SiteSettings 下；#53-57 运维家族若成组可迁独立页，后端 `/ops/system/*` 路由组已预留。
4. 线上复证（2026-10-02 deploy run 36937558114，gitCommit 444d7f0，双实例 healthy，迁移 completed）。

## 四、核销动作

- OPEN-ISSUES.md #52 行更新为「已核销（2026-10-04）」注记。
- todo.md 增 #52 核对段（原交付记录未进 todo.md，本批补齐）。
- 无代码改动，无新测试；后续顺延 #53 性能参数核对。
