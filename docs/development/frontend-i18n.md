---
title: 前端 i18n 规范
---

# 前端 i18n 规范

本规范回答：web 前端的 UI 文案如何国际化。核心边界：

- **UI 自身文案**（页面标题、列头、按钮、提示、校验消息、空态文案）→ umi plugin-locale（`useIntl` / `FormattedMessage`）
- **内容数据本地化**（后端 `spec.LocalizedText` 字段的渲染）→ `localizedText()`（`src/utils/localizedText.ts`），见 CLAUDE.md 本地化文本契约——两者是不同机制，禁止混用

> 版本基线：umi locale 插件（`config/config.ts` `locale: { default: 'zh-CN', antd: true, baseNavigator: true }`）。

## 键名规范

命名空间用点分隔的 camelCase 段，镜像模块路径，语义命名（描述含义而非文案）：

| 场景     | 模式                                    | 示例                                        |
| -------- | --------------------------------------- | ------------------------------------------- |
| 页面     | `pages.<module>[.<subPage>].<语义路径>` | `pages.opsJobs.column.status`               |
| 页面子件 | `pages.<module>.<subPage>.<语义路径>`   | `pages.ticketsDetail.comment.submit`        |
| 公共组件 | `component.<component>.<语义路径>`      | `component.resourceRenderer.column.actions` |
| 模板继承 | antd-pro 既有键（`pages.login.*` 等）   | 直接复用，禁止重复定义                      |

规则：

- 键名描述语义不描述文案：`button.submit` ✅，`button.提交` ❌
- 同名不同义不共用键；同义不同页可以各自定义（本批不做跨页去重，键的去重是后续优化）
- 段内 camelCase：`pages.opsAlerts.rules.threshold` ✅，`pages.ops_alerts.rules.threshold` ❌

## 文件布局

```
src/locales/
├── zh-CN.ts                  # 根聚合文件（umi 只发现根级 {lang}.ts，非递归）
├── zh-CN/
│   ├── pages.ts              # 既有模板键 + 早期页面键
│   ├── opsJobs.ts            # 按模块一个命名空间文件（本规范起的做法）
│   └── ...
└── en-US/
    ├── pages.ts
    ├── opsJobs.ts            # 与 zh-CN 文件一一对应
    └── ...
```

- 每模块一个文件，`export default { 'ns.key': '文案' }` 扁平对象，键按字母序排列
- 新文件必须在根 `{lang}.ts` 显式 `import` + spread 接线（umi 的 glob 只扫 `locales/*.{ts,js,json}`，不进子目录）

## 调用点模式（defaultMessage 必填）

```tsx
import { FormattedMessage, useIntl } from '@umijs/max';

const intl = useIntl();

// 逻辑/属性值（列头、placeholder、message/modall 文案、CSV 表头）
title: intl.formatMessage({ id: 'pages.opsJobs.column.status', defaultMessage: '状态' }),
placeholder={intl.formatMessage({ id: 'pages.opsJobs.search.placeholder', defaultMessage: '搜索任务 ID' })}

// JSX 文本节点
<FormattedMessage id="pages.opsJobs.empty" defaultMessage="暂无任务" />

// 插值（ICU 语法）
intl.formatMessage(
  { id: 'pages.opsJobs.summary.total', defaultMessage: '任务 {total}' },
  { total },
)
```

**defaultMessage 必填**，两个原因：

1. 6 个非全量翻译语言（见下）缺失键时以 defaultMessage 回退，行为不劣于迁移前的硬编码
2. 测试契约：`tests/setupTests.jsx` 的 `@umijs/max` mock 将 `formatMessage` 实现为返回 `defaultMessage`——现有用例断言中文文案正是依赖这一点

注意：

- defaultMessage 里字面量 `{` `}` 需转义为 `'{'`
- `useCallback`/`useMemo` 内使用 `intl` 时依赖数组补 `intl`
- 一个组件多次取值先 `const intl = useIntl()` 一次，不在行内重复调用 hook

## 语言覆盖策略

现状（2026-09 盘点）：`zh-CN`（333 键）/ `en-US`（355 键）为全量第一梯队；`zh-TW` / `ja-JP` / `pt-BR` / `bn-BD` / `fa-IR` / `id-ID` 仅含 antd-pro 模板键（65-70 键），页面键靠 defaultMessage 回退显示中文。

- 新增键必须同时进 `zh-CN` 与 `en-US` 对应文件（中文为语义源，英文为第一翻译目标）
- 其余 6 语言按命名空间文件粒度后续补译，不阻塞功能迁移
- `src/locales/supported.ts` 是语言枚举单一事实源，新增语言文件时同步

## 不迁移项（硬边界）

以下内联中文**不是** i18n 债务，禁止 intl 化：

- **逻辑字符串**：枚举比较（`if (x === '运行中')`）、Map/Switch 的 key、API payload 值、测试选择器——它们是行为契约不是文案（这类比较本身应重构为常量，但那是另一个话题）
- `src/locales/` 目录自身
- 后端 `spec.LocalizedText` 数据的兜底链已由 `localizedText()` 处理；其 `fallback` 参数若是 UI 兜底文案，可以传 `intl.formatMessage(...)` 的结果

## Review 检查项

- 新增用户可见文案出现内联中文字符串（未经 `intl.formatMessage` / `FormattedMessage`）→ review failure
- `formatMessage` 缺 `defaultMessage` → review failure
- 新命名空间文件未在根 `{lang}.ts` 接线 → review failure
- 键名含下划线/大写/文案转写 → review failure
- 用 `intl` 替换了逻辑字符串 → review failure（行为回归）
