---
title: 虚拟对象与 Resource/Page 模型
icon: cubes
order: 2
category:
  - 核心概念
tag:
  - 虚拟对象
  - ResourceSpec
  - PageSpec
---

# 虚拟对象与 Resource/Page 模型

> **状态**：Current — “虚拟对象”是历史术语，新设计以 `ResourceSpec + OperationSpec + PageSpec` 为准。

早期文档用“虚拟对象”描述一组函数、实体和 UI 配置的组合。该概念容易把函数注册、实体建模、页面布局和菜单混在一起，因此不再作为新的目标模型。

当前设计统一使用：

```text
FunctionSpec
  -> ResourceSpec + OperationSpec
  -> PageSpec
  -> PublishedPageSpec
  -> ConsoleMenuSpec
```

## 概念映射

| 历史术语 | 当前模型 | 说明 |
| --- | --- | --- |
| Function | `FunctionSpec` | 单个可执行能力，包含输入/输出契约、风险和治理字段 |
| Entity / Virtual Object | `ResourceSpec` | 稳定业务对象或能力域，例如 `player`、`mail`、`inventory` |
| Operation Mapping | `OperationSpec` | 函数在资源上的业务动作、页面语义和放置位置 |
| Resource UI | `PageSpec` | 页面级 Formily Schema，组合查询、表格、详情、动作和分页 |
| Component / Module | 扩展包或 SDK manifest | 可选分发单位，不决定运行控制台菜单 |

## ResourceSpec

`ResourceSpec` 表达一个可被 Dashboard 组织的稳定业务对象或能力域。

```json
{
  "key": "player",
  "labels": {
    "zh-CN": "玩家",
    "en-US": "Players"
  },
  "category": {
    "key": "gameplay",
    "labels": {
      "zh-CN": "玩法运营",
      "en-US": "Gameplay Ops"
    }
  }
}
```

资源 key 可以来自 descriptor 中显式 `entity`，也可以在归一化阶段从函数 ID 的对象段生成候选值。进入发布态前必须明确，运行控制台不做临时推断。

## OperationSpec

`OperationSpec` 表达某个函数在资源或页面中的语义。

```json
{
  "functionId": "player.ban",
  "resourceKey": "player",
  "operation": "ban",
  "kind": "action",
  "placement": "rowAction",
  "labels": {
    "zh-CN": "封禁",
    "en-US": "Ban"
  },
  "risk": "danger"
}
```

关键边界：

- `operation` 是业务操作 key，例如 `ban`、`grant`、`send`。
- `kind` 是页面生成语义，例如 `list`、`get`、`action`、`task`、`report`。
- `placement` 是页面放置位置，例如 `tableData`、`rowAction`、`standalone`。
- 缺少 `kind` 或 `placement` 时，只能进入函数目录或待编排建议，不能自动发布页面。

## PageSpec

`PageSpec` 是页面编排产物，必须使用 Formily JSON Schema。

```json
{
  "pageKey": "player.manage",
  "type": "entity",
  "resourceKey": "player",
  "title": {
    "zh-CN": "玩家管理",
    "en-US": "Player Management"
  },
  "category": {
    "key": "gameplay",
    "labels": {
      "zh-CN": "玩法运营",
      "en-US": "Gameplay Ops"
    }
  },
  "schema": {
    "type": "void",
    "x-component": "ConsolePage",
    "properties": {
      "query": {
        "type": "void",
        "x-component": "QueryForm",
        "x-component-props": {
          "functionId": "player.list"
        }
      },
      "table": {
        "type": "void",
        "x-component": "DataTable",
        "x-component-props": {
          "dataSource": "$.query.response.items",
          "pagination": {
            "pageField": "page",
            "pageSizeField": "pageSize",
            "totalField": "$.query.response.total"
          }
        }
      }
    }
  }
}
```

Page 负责分页、表格、详情、弹窗、批量操作、任务状态和报表图表。Function UI 只负责单个函数的输入表单，不能代替 PageSpec。

## 适用场景

适合生成 Entity Page 的资源：

- `player`：玩家查询、详情、封禁、解封、备注。
- `inventory`：背包查询、道具发放、道具回收。
- `mail`：邮件模板、邮件发送记录、邮件撤回。
- `order`：订单查询、补单、退款标记。

不应强行归入 Entity Page 的函数：

- `cache.refresh`：全局操作，适合 Operation Page。
- `reward.batchGrant`：批量异步任务，适合 Task Page。
- `analytics.retention`：报表查询，适合 Report Page。
- `maintenance.rollback`：运维动作，适合 Tool Page 或 Operation Page。

## SDK 注册要求

SDK descriptor 需要参与默认页面生成时，应提供：

- `entity`
- `entity_display`
- `operation`
- `operation_display`
- `operation_kind`
- `placement`
- `category`
- `category_display`
- `input_schema`
- `output_schema`

完整契约见 [OpenAPI / SDK Descriptor v2](../../architecture/openapi-sdk-descriptor-v2.md)。

## 禁止项

- 禁止把虚拟对象描述符作为新的运行时 UI 协议。
- 禁止写入 ProTable 专属配置替代 Formily PageSpec。
- 禁止把所有函数套入 CRUD 映射。
- 禁止用 `operation` 表示页面类型；页面语义必须写入 `operation_kind`。
- 禁止运行控制台从虚拟对象或函数目录直接生成菜单。

## 相关文档

- [Dashboard Resource/Page 模型](../../architecture/dashboard-page-model.md)
- [Page Studio](./object-workspace.md)
- [函数注册与默认界面](./function-registration-ui.md)
