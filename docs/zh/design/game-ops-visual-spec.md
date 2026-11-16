---
title: 游戏运营控制台视觉与交互规范（GUX-016）
---

# 游戏运营控制台视觉与交互规范（GUX-016）

状态：todo GUX-016（P2，设计）交付，依据
`docs/design/game-operations-ux-review-2026-11-16.md`。本文沉淀运营控制台（web）
在标题、间距、表格、表单、抽屉、语义色与交互状态上已经遵循（或今后必须遵循）的
规则，覆盖明暗主题、中英文与键盘焦点。本文本身不改运行时行为；执行缺口在 §12
如实列出，不静默假设。

约定用词：**必须** = 新增/改动 UI 的硬性要求；**应该** = 除非有记录在案的理由；
**可以** = 允许。

## 1. 范围与依据

- 适用对象：web 运营控制台（`web/`，Umi + antd 6 + pro-components）。
- 本文沉淀的事实源：
  - 主题架构：`web/src/global.less`、`web/src/utils/antdThemeConfig.ts`、
    `web/src/utils/themeMode.ts`、`web/config/config.ts`（theme token 块）。
  - 页面节奏：`DASHBOARD_PAGE_TOKENS`（`web/src/components/StandardPage/index.tsx`）。
  - 已交付行为基线：GUX-005~007 共享语义、GUX-008~013 标杆流程、GUX-014 空态、
    GUX-015 文案收敛（证据在 `docs/review/evidence/2026-11-17/`）。
  - 表单契约基线：`docs/design/form-component-reuse.md`。

## 2. 主题架构

两套主题套 × 明/暗/跟随系统三档：

- 主题套：`blue`（出厂默认，antd 6 种子 `#1677ff`）与 `inkpink`（品牌墨粉，
  种子 `#93394d`）。偏好持久化在 localStorage（`croupier-theme-preset`）；
  明暗档（`light`/`dark`/`system`）在 `croupier-theme`。
- 生效状态由 `utils/themeMode.ts` 写入 `<html data-preset="blue|inkpink">` ×
  `<html data-theme="light|dark">`；antd algorithm/token 经 `<ThemeSync>` 与
  `app.tsx` 的 antd 运行时导出跟随。

规则：

1. 组件**必须**消费 antd token（`theme.useToken()`）或 `--brand-*` / `--app-*`
   CSS 变量。组件代码新增裸十六进制色值为 review failure。
2. `global.less`（CSS 变量）与 `utils/antdThemeConfig.ts`（antd token 种子）
   是**同源共维**的单一事实源——改色必须同变更内两侧同步。
3. antd 6 会按算法从十六进制种子再派生调色板。对比度**必须**按**渲染值**计算，
   不能按种子值：如种子 `#4096ff` 在暗色算法下渲染为 `#3983dc`。暗色对比度
   检查**必须**按主题套分别做（同一种子、两套暗底：一套 4.78:1 通过，另一套
   4.24:1 不通过）。

## 3. 标题

| 层级             | Token/样式                   | 用途               |
| ---------------- | ---------------------------- | ------------------ |
| 页面标题         | `PageContainer title`        | 每页一个           |
| 页面副标题       | `PageContainer subTitle`     | 一句话定位         |
| 卡片/区块头      | `Typography.Title level={4}` | 卡片内 `margin: 0` |
| 子块头           | `Typography.Text strong`     | 清单/帮助块、抽屉  |
| 辅助文字         | `Typography.Text secondary`  | 区块头下的说明     |
| 弱化标签（12px） | `mutedTextSize` token        | 卡片脚注、帮助行   |

内部标识（菜单/分类 key、内部表名）**不得**作为可见标题或徽章出现（GUX-015）；
仅当无本地化标题时允许 raw key 作为兜底身份展示。

## 4. 间距与表面

`DASHBOARD_PAGE_TOKENS` 是间距词汇表；新页面**必须**引用它而不是自造数值：

| Token                | 值                  | 用途           |
| -------------------- | ------------------- | -------------- |
| `cardPadding`        | 20                  | 标准卡片内边距 |
| `compactCardPadding` | 12                  | 嵌套/小卡片    |
| `heroCardPadding`    | 24                  | Hero/概览卡    |
| `sectionGap`         | 16                  | 区块/卡片之间  |
| `itemGap`            | 10                  | 区块内部       |
| `mutedTextSize`      | 12                  | 辅助文字       |
| `softSurface`        | `rgba(0,0,0,0.03)`  | 芯片/统计容器  |
| `filterSurface`      | `rgba(0,0,0,0.025)` | 筛选栏         |
| `infoGradient`       | 品牌色渐变          | 概览卡底纹     |

圆角只来自 antd 配置：`borderRadius: 8`（控件）与 `borderRadiusLG: 12`（表面）。
不引入更多圆角值。

黑透明叠层（`softSurface`/`filterSurface`）天然暗色安全；新增叠层**必须**用
alpha 通道，不得用固定浅色 hex。

## 5. 表格（ProTable）

已交付基线（GUX-008~009，生成型资源页；手写页同规则）：

- 查询工具栏：`{ labelWidth: 'auto', defaultCollapsed: true }`；搜索项由
  `valueType`/`valueEnum` 派生（`search: filterable ? undefined : false`）；
  高频列（如玩家 id/昵称）在前。
- 行内动作为 link 态按钮。工具栏级一次性动作（如批量操作）可保留
  `type="primary"`。操作列超过 4 条折叠进「更多」下拉；删除类在下拉内保持
  danger 确认语义不降级。
- 数字/时间列由 `dataType` 机械派生右对齐。
- 单元格渲染走共享语义层（`renderFieldSemantic`，§8）并跟随界面 locale。
- 空/加载/错误遵循 GUX-014 状态模型：区分无数据/未挂载/无权限/加载失败；
  合并空态卡替代逐行重复；下一步按角色给出。

## 6. 表单

- 生成型表单经 `SchemaFormRenderer`（RJSF antd 封装）按 `FormPresentationSpec`
  渲染；uiSchema 内存派生，不落库。
- 容器复用目标为 `ModalForm` / `DrawerForm` / `StepsForm`，契约基线在
  `web/src/components/FormContract/__tests__/formContainers.contract.test.tsx`
  （渲染/数据/校验三契约）。ProComponents 升级必须保基线绿。
- 必填标记与校验文案用 antd 默认；阻断校验必须拦住 `onFinish`（契约测试锁定）。
- 提交按钮：每个表单一个 primary 提交；取消为默认态。破坏性确认用
  `modal.confirm` + `danger` 确认按钮。

## 7. 抽屉与弹窗

- 详情抽屉：右侧 `Drawer`，资源渲染器 `size={640}`（手写页有 600 的存量——
  新页**应该**用 640）。声明了 `identityKey` 的详情顶部渲染行身份锚
  （`detail-identity`）。
- 字段较多的行动作表单用 `DrawerForm`；短表单用 `ModalForm`。
- 关闭行为保持 antd 默认（Esc/遮罩）。未保存数据守卫是已知后续项，
  不静默假设已具备。

## 8. 语义色与状态

- 状态文案/标签词汇只来自**已声明**的枚举选项；颜色可来自契约（`opt.color`），
  缺省 `default`。
- 未声明的枚举值渲染为 未知（{value}）+ 中性标签——不猜语义、不只靠颜色。
  布尔（是/否）与无效时间（无效时间（{value}））同守非仅颜色规则。
- 状态色用 antd 语义 token（`success`/`processing`/`error`/`default` 标签名，
  `colorSuccess` 等 token）；品牌强调只用 `--brand-1/2/3`。
- 金额/数量/身份语义（单位跟随界面 locale、不杜撰单位、身份可复制）遵循
  GUX-005~007；列表与详情必须同值同义（共享语义层）。

## 9. 交互状态

每个交互表面**必须**定义或继承的完整状态集：

1. Hover / active——antd 原生组件自带；自定义组件必须定义。
2. 键盘焦点——antd 6 原生组件自带可见焦点环。自定义可交互元素**必须**要么用
   真实 `button`/`a`（继承焦点环），要么显式加 `:focus-visible` 轮廓；
   不可聚焦的 `div` 上挂点击处理器是 review failure。
3. Disabled——antd 禁用样式；提交在途时一并禁用（提交按钮 `loading`）。
4. Loading——表格 `loading`、页面级 `Spin`、详情抽屉 `Skeleton`。
5. 空/错误——GUX-014 模型；错误面显示 `extractErrorMessage` 抽取的消息，
   可重试处给重试。
6. 部分成功——回执只展示真实返回字段（GUX-013 白名单）；未知保持未知，
   不发明回滚或成功。

## 10. 明暗与中英文覆盖

- 视觉改动**必须**明暗两档都验证；涉及品牌色时按主题套分别验证（§2 规则 3）。
- 所有用户可见文案走 intl key（`useIntl`/`FormattedMessage`）或 LocalizedText
  契约值的 `localizedText`；组件 `defaultMessage` 为中文且必须与 locale 文件
  同步（zh-CN/en-US 键位对齐由
  `web/src/locales/__tests__/featureDictParity.test.ts` 测试锁定）。
- 文案收敛遵循 GUX-015：引导按需（折叠帮助）；模板/演示字符串与内部术语
  不出现在默认视图。

## 11. Review 检查单

- [ ] diff 内无裸 hex / 自造间距（只用 token 或 CSS 变量）。
- [ ] 状态含义不只靠颜色；未声明值渲染 未知（原值）。
- [ ] 自定义可交互元素可聚焦且有可见焦点环。
- [ ] 明暗两档已验证（动品牌色时按主题套）。
- [ ] zh-CN/en-US 键位对齐；新文案无硬编码 locale 串。
- [ ] 空/加载/错误/部分成功状态有用例覆盖。
- [ ] 表格/表单/抽屉符合 §5–7（link 动作、折叠搜索、640 抽屉、契约测试覆盖
      的表单容器）。

## 12. 已知边界（诚实）

1. 本文是设计交付：§11 尚无自动化守卫（dashboard_vnext_guard 不扫裸 hex 或
   焦点样式）。把守卫代码化是后续项，不在本文声称已完成。
2. 存量偏差存在：如部分 analytics/详情页仍带 token 化之前的 hero 渐变或
   手写间距；本文不追溯改造。新工作必须合规；触碰存量面即随改随合规。
3. 本文在 `docs/design/index.md` 的登记暂缓：该文件是另一并行会话的在途编辑，
   此处登记会把他会话未提交行卷进本提交。
4. 抽屉/弹窗的未保存数据守卫（§7）与存量页面的完整键盘操作审计未做；
   antd 原生面由框架覆盖，自定义面未穷尽审计。
