---
title: Excel 在线配置设计——前端编辑器选型与双源版本模型
---

# Excel 在线配置设计（Univer 编辑器 + 双源版本）

## 状态

Proposed → P1 已落地（Univer 编辑器 + 在线编译 + 双源统一版本）。

## 1. 问题

数值表配置的既有路径是「策划本地 Excel → Git → CI 导出（Luban 等）→ 下发」，对小团队太重：
没有 CI 也能在管理台里改表、即时编译下发；同时不能丢掉 Git 工作流（大团队仍依赖本地 Excel 协作）。

## 2. 前端编辑器选型（TS 生态）

| 库                            | 能力                                                            | 许可           | 评价                                                                   |
| ----------------------------- | --------------------------------------------------------------- | -------------- | ---------------------------------------------------------------------- |
| **Univer**（Luckysheet 继任） | 完整 Excel 语义：多 sheet、公式、样式、选区、协同（可选服务端） | **Apache-2.0** | **选定**：国产活跃维护、中文生态、有官方 React 封装与 SSR 注意事项文档 |
| Handsontable                  | 最成熟类 Excel 网格                                             | **商用收费**   | 许可受限，排除                                                         |
| x-spreadsheet                 | 轻量 canvas 表格                                                | MIT            | 公式/多 sheet 弱，只够极简场景                                         |
| Ag Grid                       | 数据网格（非 Excel 语义）                                       | 企业版收费     | 形态不符                                                               |
| SheetJS(xlsx)                 | 读写 .xlsx 文件（无编辑 UI）                                    | Apache-2.0     | **配套**：导入导出真实 xlsx 文件时用                                   |

## 3. 双源版本模型（Git vs DB）

```
策划本地 Excel(Git) ──上传 .xlsx──► ┐
                                    ├─► 统一编译(JSON 产物) ─► ConfigVersion(ns=gameplay)
Web Univer 编辑器在线改 ──保存────► ┘                            │  版本单调递增/diff/回滚
                                                    SSE watch 通知游戏服拉取
```

**铁律：DB（ConfigVersion）是唯一发布事实源。**

- Git 仓库只是"草稿区"之一：上传 .xlsx 与在线编辑殊途同归，都编译成 JSON 产物注册进 ConfigVersion；
- 发布动作统一走管理台（审批/按 namespace 灰度/审计/回滚都在平台），Git 侧不直接触发发布；
- 在线编辑每次保存产生新版本，版本号单调递增，可回滚不可覆盖；
- 大团队可继续 Git+CI：CI 产物通过上传端点注入同一条管线（与 Luban 导出结果共存）。

## 4. 编译协议（Excel → JSON）

P1 采用最简可解释协议（sheet 即配置表）：

- 每个 sheet = 一张表：**首行为字段名**，第二行可选 `#type` 注释行（int/string/float/bool），其余为数据行；
- 产物 JSON：`{"sheets": {"<sheet名>": {"fields":[...], "types":{...}, "rows":[...]}}}`，存入 ConfigVersion.Value；
- 空行/空 sheet 跳过；类型行按 JSON 类型校验，非法值报 400（details 指明 sheet/行/列）。

P2 再演进：跨表引用（`#ref` 列）、枚举字典、差量编译产物。

## 5. 端点

| 端点                                      | 说明                                                                                                              |
| ----------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `POST /api/v1/configs/excel/import`       | 上传 .xlsx（multipart）→ 服务端解析（SheetJS Go 等价：excelize 只读解析）→ 编译 → 注册 ConfigVersion(ns=gameplay) |
| `POST /api/v1/configs/excel/compile`      | 前端 Univer 内容（JSON 快照）直接提交编译 → 注册版本（在线编辑保存路径）                                          |
| `GET  /api/v1/configs?namespace=gameplay` | 既有端点：列表/版本/回滚复用                                                                                      |

导入解析服务端用 **excelize**（Go 生态标准 xlsx 读写库，Apache-2.0）——前端编辑器与后端文件解析是两件事，各自用各自生态的标准库。

## 6. 前端集成

- 依赖：`@univerjs/core` + `@univerjs/sheets` + `@univerjs/ui` + React 封装（umd 动态加载，避免 SSR/包体积问题）；
- 页面：系统管理 → 「表格配置」（/system/foundation/excel）：
  - Univer 编辑器（多 sheet 编辑、公式、复制粘贴）
  - 保存 = `POST /excel/compile`（携带快照 JSON）→ 展示新版本号
  - 导入 .xlsx = `POST /excel/import`；导出 .xlsx = 前端 SheetJS 生成下载（纯展示副本，发布仍走编译端点）
- 版本列表：复用 configs 版本端点（ns=gameplay 过滤），支持查看与回滚。

## 7. 分阶段

| 阶段             | 内容                                                                                                |
| ---------------- | --------------------------------------------------------------------------------------------------- |
| **P1（已落地）** | Univer 编辑器页 + 编译端点（快照/xlsx 双入口）+ ConfigVersion(ns=gameplay) 注册 + 版本列表/回滚复用 |
| P2               | 类型系统增强（#ref 跨表/枚举/常量表）、编译产物 diff 视图、保存前 schema 校验错误行内标注           |
| P3               | 协同编辑（Univer 协同服务端）；Git 仓库 webhook 自动拉取 .xlsx 草稿                                 |

## 8. Review Checklist

- 编译端点必须校验：sheet 名/字段名合法标识符、类型行匹配数据、单表 ≤2MB；
- ConfigVersion(ns=gameplay) 的写入必须走统一注册函数（版本号原子递增），禁止并发双写；
- Univer 快照与 xlsx 双入口的编译结果必须同构（同一编译器函数）；
- 导入的 .xlsx 仅服务端内存解析，不落盘明文（解析后即弃）。
