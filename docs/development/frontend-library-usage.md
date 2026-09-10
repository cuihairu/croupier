---
title: 前端库分层与复用规范
---

# 前端库分层与复用规范

本规范回答两个问题：web 前端的几类库**如何协作**（分层模型），以及写新代码时**何时必须复用现成组件**而不是自己实现。核心原则：**不要什么都自己实现，尽量使用现有的库和组件**——自建只在库明确不满足时发生，且必须给出领域理由。

> 版本基线：antd `^6.4.3`、`@ant-design/pro-components` `^3.1.12-0`、React `^19.2.5`、Umi Max `^4.7.3`。

## 分层模型

```
┌─────────────────────────────────────────────────────────┐
│ L4 项目 utils（横切工具，src/utils/）                    │
│    formatDateTime / exportToCSV / extractErrorMessage    │
│    localizedText / getMessage / …                       │
├─────────────────────────────────────────────────────────┤
│ L3 项目领域组件（src/components/，有领域理由才存在）     │
│    PageRenderer* / SchemaFormRenderer / ProposalInbox …  │
├─────────────────────────────────────────────────────────┤
│ L2 @ant-design/pro-components（复合场景）                │
│    ProTable / ModalForm / QueryFilter / ProCard …        │
├─────────────────────────────────────────────────────────┤
│ L1 antd（原子组件）                                      │
│    Button / Input / Table / Modal / Form / Descriptions …│
└─────────────────────────────────────────────────────────┘
```

选型自下而上：**L1 能满足就不上升；L2 已有的复合形态禁止手写等价物；L3 必须有明确领域理由；L4 已有函数必须复用、禁止自造同类**。

### 各层职责

- **L1 antd**：一切 UI 的原子层。简单展示、简单表单、非分页非联动场景直接用。
- **L2 pro-components**：在 antd 之上封装"数据获取 + 状态联动 + 布局骨架"的复合形态。判定标准：当你要手写"分页三件套 + load + 竞态处理"、"Modal + Form + open/预填生命周期"、"筛选栏 + 查询/重置"这类**结构性样板**时，先查这一层。
- **L3 项目领域组件**：与 Croupier 后端契约绑定的运行时（PageSpec 渲染、descriptor 驱动表单、提案工作流）。库不可能提供，见下文「不替换清单」。
- **L4 项目 utils**：跨页面横切逻辑的唯一实现。改一处全局生效，因此**新增工具前先查此层是否已有**。

## L2 复用对照清单（2026-09 盘点）

| pro 组件                                                                         | 解决的结构性样板                                        | 项目现状                                                                                                                                     | 结论                                                                                                                                                                                                                                         |
| -------------------------------------------------------------------------------- | ------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `PageContainer`                                                                  | 页头/面包屑/内容容器                                    | 全站已用                                                                                                                                     | ✅ 基准                                                                                                                                                                                                                                      |
| `ProTable`（request 模式）                                                       | 分页三件套 + loading + 竞态防护 + 刷新                  | 15 个分页页面手写 `useState(1)/(20)/(0)` + `useCallback load`；仅 Ops/Terms 用了手动 dataSource 模式                                         | **迁移中**：保留现有筛选栏 UI，经 `params` 传入；`params` 变化时组件内部 `abortFetch()` 中止在途请求后重查（源码 `useFetchData` 的 `useDeepCompareEffect`），乱序防护内建；筛选变化回第 1 页需显式 `actionRef.current?.setPageInfo(1, size)` |
| `ModalForm` / `DrawerForm`                                                       | Modal+Form 共生：open 生命周期、提交 loading、预填/重置 | 2026-09 已完成迁移：23 文件/29 个表单弹窗改为 `ModalForm` + `destroyOnHidden` + 动态 `initialValues`（结构性消灭"新增残留上次编辑值"缺陷类） | ✅ 新写弹窗表单一律用 `ModalForm`，禁止再新增手写 `<Modal><Form>` 组合。配方见下文「ModalForm 迁移配方」；8 个文件因 RJSF 内核/异步回填/受控 form 联动保留手写（清单见配方节）                                                               |
| `QueryFilter` / `LightFilter`                                                    | 筛选栏 + 查询/重置 + 与表格联动                         | 页面各自手写筛选栏（Card extra + Input/Select + 查询按钮）                                                                                   | 已有页面迁移时**保留现有筛选 UI**（交互不变）；**新页面**优先 `QueryFilter`                                                                                                                                                                  |
| `StatisticCard` / `ProCard`                                                      | 统计卡/卡片栅格布局                                     | 11 个文件手写 `Statistic` 组合                                                                                                               | Overview/Analytics 类页面改造时评估                                                                                                                                                                                                          |
| `ProDescriptions`                                                                | 描述列表 + columns 配置/request 取数                    | 22 个文件用 antd 原生 `Descriptions`                                                                                                         | antd 原生已够用，存量**不迁移**；新页面需要 columns 化配置或 request 取数时用                                                                                                                                                                |
| `ProList`                                                                        | 列表型分页/配置化渲染                                   | 8 个文件手写 `List`                                                                                                                          | 个别评估，无强制                                                                                                                                                                                                                             |
| `StepsForm` / `LoginForm`                                                        | 分步表单/登录表单                                       | 手写分步/登录表单                                                                                                                            | 重构对应页面时评估                                                                                                                                                                                                                           |
| `EditableProTable` / `CheckCard` / `FooterToolbar` / `WaterMark` / `ProSkeleton` | 行编辑表格/勾选卡/页底操作栏/水印/骨架屏                | 未使用                                                                                                                                       | 按需引入，出现对应需求时禁止手写                                                                                                                                                                                                             |
| `BetaSchemaForm`                                                                 | Schema 驱动表单                                         | —                                                                                                                                            | **不采用**，见下                                                                                                                                                                                                                             |

## L3 不替换清单（自建的领域理由）

以下组件**不是技术债**，禁止用通用库"替换"：

- **`SchemaFormRenderer`**：后端 JSON Schema descriptor 驱动的表单运行时，是 descriptor-driven 架构（单一事实源：函数描述符→UI 自动生成）的落点。`BetaSchemaForm` 不理解 Croupier 的 descriptor 约定（表达式参数、动态联动、后端校验契约）。
- **`PageRenderer` 系列**（Resource/Task/Report/Composite/Realtime）：PageSpec 发布链的浏览器运行时（见 `docs/architecture/dashboard-page-model.md`）。菜单与页面只来自已发布 PageSpec，这是产品边界而非 UI 选择。
- **`ProposalInbox` / `PageEditor` / `PageWorkflowGuide`**：提案工作流领域组件，无库对应。
- **`MonacoDynamic`**：Monaco 按需加载封装。

## L4 utils 复用规则（强制）

写任何格式化/导出/错误处理前，先查 `src/utils/` 是否已有：

| 需求               | 必须使用                                                                                       | 禁止的重复形态                                                            |
| ------------------ | ---------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| 日期时间展示       | `formatDateTime`（`utils/format`，空值返回 `-`）                                               | `v ? new Date(v).toLocaleString() : '-'`                                  |
| CSV 下载           | `exportToCSV` / `exportToXLSX`（`utils/export`，含引号转义）                                   | 手写 `text/csv` Blob + `URL.createObjectURL` + `a.click`                  |
| 错误信息提取       | `extractErrorMessage`（`utils/errors`，依次挖 data.message/response.data.message/err.message） | `e instanceof Error ? e.message : '兜底'`                                 |
| 本地化文本渲染     | `localizedText`（`utils/localizedText`）                                                       | 组件内 `value['zh-CN']                                                    |     | …` 取值链 |
| message/Modal 弹层 | `App.useApp()` 或 `getMessage()`（`utils/antdApp`）                                            | `import { message, Modal } from 'antd'` 静态调用（不消费 App 上下文主题） |
| 字节数/百分比/时长 | `formatBytes` / `formatPercent` / `formatDuration`（`utils/format`）                           | 手写除法与单位拼接                                                        |

> 存量债务：静态 `message`/`Modal` 调用约 20 个文件、历史内联日期渲染与 CSV 下载正在按批次清理（见任务 #27/#28）；**新代码零容忍**。

## ModalForm 迁移配方（2026-09 批次定型）

存量弹窗表单迁移到 `ModalForm` 的标准形态（参照 `pages/Support/Feedback`、`pages/Ops/Terms`、`pages/Ops/Notifications`）：

```tsx
const [open, setOpen] = useState(false);
const [editing, setEditing] = useState<Row | null>(null);
// 打开即同步可确定的预填值走 initialValues；destroyOnHidden 使弹窗每次
// 关闭即卸载表单，重开按最新 initialValues 重挂载——新增/编辑切换零残留
<ModalForm<FormValues>
  open={open}
  onOpenChange={setOpen}
  modalProps={{ destroyOnHidden: true }}
  width={520}                              // 对齐原 Modal（原有 width 则保留）
  submitter={{ searchConfig: { submitText: '确定' } }} // 原自定义按钮则保留原文案
  initialValues={editing ?? { ...默认值 }}
  onFinish={async (v) => {
    try {
      await api(v);
      reload();                            // 原成功副作用全部保留
      return true;                         // 自动关闭
    } catch {
      return false;                        // 失败保持开启；原有本地 toast 则保留
    }
  }}
>
  <Form.Item ...>...</Form.Item>           {/* 字段树原样搬入 */}
</ModalForm>
```

要点：

- **预填**：禁止 `setFieldsValue`/`resetFields`/打开时 `useEffect` 预填；全部收敛为动态 `initialValues`。
- **失败语义**：`return false` 等价于原 antd Modal `onOk` 返回 rejected Promise 时弹窗保持打开；全局请求拦截器已 toast，无本地 catch 的页面静默 `return false` 即可（加注释说明）。
- **字段联动**（切换 A 清空/回填 B）：字段组件经 `Form.useFormInstance()` 局部组件承接，或 `formRef` 指向 ModalForm 托管实例；**禁止**外部 `Form.useForm` 传入受控 form（`destroyOnHidden` 重挂载时 rc-field-form 会 merge 出旧 store 值）。
- **已知边界**：`submitter.submitButtonProps` 覆盖内建提交 loading（源码 merge 顺序用户 props 在后）；antd Modal `onOk` 返回 Promise 的自动 loading 不再存在，需要上传期 loading 的弹窗自行传 `submitButtonProps.loading`。

**保留手写 Modal 不迁的 8 个文件**（有明确理由，新改动时不要"顺手迁移"）：

| 文件                                         | 理由                                                                                          |
| -------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `Assignments/CanaryModal` / `CloneModal`     | 内核是 RJSF `SchemaFormRenderer`（L3），外面包 ModalForm 会产生嵌套 `<form>` 且回车误触发提交 |
| `Ops/RateLimits/index`                       | body 内非提交动作按钮（预览/导出）+ 200ms debounce 自动预览等 4 处 form 实例联动              |
| `Operations/Configs/index`                   | 4 个弹窗均为展示型或无 name 绑定的受控 Input，非表单收集型                                    |
| `Extensions/Store/InstallModal`              | 弹窗打开后异步 fetch schema 再回填，`initialValues` 挂载时序不适用                            |
| `ResourceCatalog/EditSemanticsModal` 等 2 个 | 父页持受控 form 实例 + 异步 fetch 回填 + 提交逻辑在父页                                       |

## 新代码选型流程

1. 要手写结构性样板（分页取数/弹窗表单/筛选栏）？→ 查 L2 对照表，有则用。
2. 要写格式化/下载/错误提取/本地化取值？→ 查 L4 表，有则用。
3. 要新建公共组件？→ 先确认 L1/L2 无对应，再在 PR/提案中写明领域理由（对照 L3 清单的风格）。
4. Review 检查项：出现"分页三件套"、"手写 Modal+Form"、"内联 `toLocaleString`"、"内联 CSV blob"、"`instanceof Error` 取 message"、静态 `message.` 调用，一律视为 review failure，按本文替换。
