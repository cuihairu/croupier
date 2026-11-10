# 前端样式系统边界

范围：`web/`（Umi Max + React + antd 6）。本页定义四种样式机制各自负责
什么，让新代码有一个唯一决策规则，而不是四套机制互相竞争。2026-11-10
对照实际代码树复核。

## 四层机制与归属

| 层                        | 入口                                                                                                        | 负责                                           | 现状                            |
| ------------------------- | ----------------------------------------------------------------------------------------------------------- | ---------------------------------------------- | ------------------------------- |
| antd 主题 token           | `web/src/utils/antdThemeConfig.ts`（运行时真相源）+ `web/config/config.ts` `antd.theme.token`（构建期兜底） | antd 组件观感：色板、圆角、间距语义            | 使用中；双主题套 × 明暗         |
| antd-style `createStyles` | `web/src/**` 组件文件                                                                                       | 非 antd 自建组件的组件级样式                   | 使用中；6 个生产文件（+1 测试） |
| `web/src/global.less`     | 全局样式表                                                                                                  | 应用壳层（body/顶栏/侧栏/卡片）、主题 CSS 变量 | 使用中；双主题套 × 明暗四套变量 |
| Tailwind CSS 4            | `package.json` devDependency + umi `@tailwindcss/postcss` generator                                         | 无                                             | **已安装未激活**                |

## antd 主题 token —— 改 antd 组件观感的唯一入口

- 运行时真相源是 `web/src/utils/antdThemeConfig.ts`：四套 `ThemeConfig`
  （blue/inkpink × 明暗）。消费方两处：`app.tsx`（umi antd 插件的运行时
  导出，保证首帧即正确）与 `<ThemeSync>`（`web/src/components/ThemeSync/`，
  切换时实时推送）。
- `web/config/config.ts` 只钉构建期兜底（`borderRadius: 8`、
  `borderRadiusLG: 12`、`colorPrimary: '#1677ff'`）。不得在这里长出第二份
  色板；完整 token 集都在 `antdThemeConfig.ts`。
- 对比度驱动的 token 调整要 inline 写明 WCAG 依据（例如
  `colorTextSecondary` 从 antd 默认 `#8c8c8c`（3.3:1）提到 `#595959`
  （≥7:1））。保持这个惯例：每一处对 antd 默认值的刻意偏离都带注释和
  实测比值。
- 禁止从 `global.less` 或临时 CSS 覆写 antd 组件内部。组件观感不对就改
  token：单个组件的问题用 `antdThemeConfig.ts` 里组件级 `components:` 块，
  系统性的问题用全局 `token:`。

## antd-style `createStyles` —— 组件级样式

- 属于单个非 antd 组件的样式（List 替身、下拉包装器、页面局部布局）用
  `antd-style` 的 `createStyles(({ token, css }) => ...)`。
- 颜色必须从 `token` 派生（`token.colorBgContainer`、`token.colorText`
  等），禁止硬编码 hex——主题套与暗色切换才能自动生效。只有颜色确实
  属于壳层色板而非 antd token 集时，才交叉引用 `global.less` 的主题变量。
- 先例：`web/src/components/SimpleList/index.tsx`（antd 6 List 替身）、
  `web/src/components/HeaderDropdown/`、
  `web/src/components/RightContent/AvatarDropdown.tsx`。

## `global.less` —— 壳层与主题变量

- 负责：壳层表面（body/顶栏/侧栏/卡片）与四套 CSS 变量（`--app-*`、
  `--brand-*`），按 `<html data-preset="blue|inkpink">` ×
  `<html data-theme="light|dark">` 切换。属性由 `web/src/utils/themeMode`
  写入；antd token 由 `<ThemeSync>` 保持同步。
- 消费方：约 32 个 `.tsx` 内联读取 `var(--app-*)` / `var(--brand-*)`，
  集中在 PageStudio / CompositeEditor 画布代码——这些值必须随主题套
  切换而变化且不触发 React 重渲染。
- 同源维护契约：`global.less` 变量块与 `utils/antdThemeConfig.ts` token
  块描述同样的两套主题，刻意并行维护（antd token 表达不了壳层变量，
  CSS 变量喂不进 antd algorithm）。任何色板变更必须同一 PR 内两侧同步，
  外加 `docs/.vitepress/theme/custom.css`（docs 站）。
- 禁止在这里加逐组件覆盖。这是壳层/变量表，不是组件样式表。

## Tailwind 4 —— 已安装未激活

`tailwindcss` 4.x 是 devDependency，umi 的 tailwind generator 已接线
（umi app data 可见 `@tailwindcss/postcss`），但没有任何样式表 import
`tailwindcss`，无 `@tailwind`/`@theme` 指令，`web/src` 无 utility class
用法。实际状态：**休眠**。

改变现状前的决策规则：

- 不要顺手开始用 utility class。激活 Tailwind 是一个刻意的架构决策——
  它的 preflight reset 与 utility 词汇和 antd token 体系、`global.less`
  变量体系都有重叠。
- 未来若有需求足以支撑激活，先扩充本页：激活点（哪个样式表 import）、
  preflight 处理（关闭或圈定作用域，保证 antd 基线不受扰动）、它替代或
  补充四层中的哪一层。然后文档更新与激活代码同一 PR 落地。

## 新增样式的决策规则

1. 是 antd 组件的观感？→ `antdThemeConfig.ts` token（单组件用
   `components:` 块）。
2. 是某个自建组件的样式？→ `createStyles`，颜色从 `token` 派生。
3. 是壳层表面或主题相关的共享色？→ `global.less` 变量（并检查同一颜色
   是否需要 antd token 孪生值）。
4. 其余（真正一次性、位置性的）：允许 inline style，但颜色读
   `var(--app-*)` / `var(--brand-*)` 或 `token`，禁止硬编码 hex。

## 树内守卫

- `web/tests/antd6Deprecations.test.ts` 在组件 import 层面禁用已废弃的
  antd 组件（如 antd `List`）；替身组件在 `web/src/components/` 下。
- 没有 stylelint。层边界靠按本页 review 把关，不靠 lint 规则。若违规
  累积，后续动作是补 stylelint 规则集，而不是默许。

## 相关

- UI audit 与对比度基线：[交付审计](../../review/2026-10-31-ui-audit-impeccable.md)（仅英文版）
- antd 6 废弃台账：`docs/BUGS.md`（BUG-015、BUG-022）
- 约束 antd/antd-style/tailwind 包版本的策略：
  [版本与依赖策略](./versioning.md)
