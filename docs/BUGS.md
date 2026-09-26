# 界面 Bug 验证与回归台账

> 范围：本地栈（`croupier-server` + `pnpm dev`）上复现/验证前端交互路径，发现的问题、
> 根因、修复与回归测试。
>
> 验收口径：**每个 BUG 都要有一条能真正判别修复与回归的测试**——「修复前红、修复后绿」。
> 仅断言渲染成功、或用被 mock 弱化到无法失败的断言，都不算回归测试（本文档多次踩到，
> 已在各条的「回归测试」里写明）。
>
> 相关：`todo.md`（T1–T13 已完成）、`CLAUDE.md`、`web/package.json`（antd 6）。

## 验证手段

| 手段 | 位置 | 用途 |
| --- | --- | --- |
| 控制台审计脚本 | `web/scripts/console-audit.mjs` | 登录真实栈，遍历 **52** 条路由，收集 console warning/error/pageerror，输出 JSON 报告。dev 模式下 antd 的运行时废弃告警、React 重复 key、缺失 locale key 只有这样才抓得到。 |
| 废弃属性静态守卫 | `web/tests/antd6Deprecations.test.ts` | 从 `node_modules/antd` 的**运行时告警表**反推废弃清单，扫描 `src/**/*.tsx`，任何新写入的废弃属性直接失败并报文件行号。 |
| 单元测试 | `pnpm --dir web test`（jest，251 个 suite） | 组件行为回归。 |
| Go 测试 | `make test`（`go test -short ./...`，169 个包） | 后端回归。 |

审计脚本的运行方式（需本地栈已起）：

```bash
cd web && node --import tsx scripts/console-audit.mjs http://127.0.0.1:8000
# 报告写入 web/test-results/console-audit.json
```

**最终结论**：**52 条**路由的 console 输出中，antd 废弃告警 **0 条**、其他
warning/error/pageerror **0 条**（BUG-005 / 007 / 008 / 010 / 011 修复前分别为
19、4、2、4、14 条）。

唯一的残留是 `/analytics/warehouse` 的 3 条
`Failed to load resource: 503`——后端返回
`{"error":"service_unavailable","message":"分析仓库未启用"}`，即本地未启用
ClickHouse 分析仓库（`services/api/analytics.ts:484` 已注明
「503 when the warehouse is not enabled」）。这是**预期内的后端状态**，不是界面缺陷；
浏览器对任何 503 都会打这条 resource 错误，前端无法消除。

---

## BUG-001 `TestNewTCPListener_NilConfigDefaults` 抢占生产默认端口，本地起栈即失败

**严重度**：高（阻断「go test 全绿」）

**现象**

本地跑着 croupier 栈时执行 `make test`，`internal/server` 必挂：

```
--- FAIL: TestNewTCPListener_NilConfigDefaults (0.00s)
    listen tcp :19090: bind: address already in use
```

**根因**

用例直接 `NewTCPListener(nil, nil, nil, nil)`，而 `nil` 配置会回落到生产默认地址
`:19090`（`configs/*.yaml` 的 `control.addr` 默认同为 `:19090`）。于是测试真的去
bind 了**业务端口**。任何跑着本地栈的开发者、或并行占着该端口的用例都会中招——
一个与被测逻辑完全无关的环境耦合。同包的 `tcp_listener_test.go` 其余用例早已改用
临时端口 `:0`，只有这一条漏网。

**修复**

把「配置兜底」从「建监听器」里拆出来，使其可测而不占端口：

- `internal/server/tcp_listener.go`：新增 `defaultControlAddress` 常量与
  `defaultTCPListenerConfig(config)`（非 nil 配置原样返回，不覆盖调用方显式值），
  `NewTCPListener` 改为调用它。生产行为不变。
- `internal/server/control_handler_test.go`：拆成三条——
  1. `TestNewTCPListener_NilConfigDefaults`：纯函数断言默认地址与 `Insecure`；
  2. `TestNewTCPListener_ConfigDefaults_PreservesExplicitValues`：断言恒等返回、
     不翻转显式 TLS 开关；
  3. `TestNewTCPListener_NilDepsUseDefaults`：用 `127.0.0.1:0` 验证 nil
     sessionStore/registry/logger 兜底，并断言实际端口**不等于**生产默认端口。

**回归测试**：同文件三条用例。修复前第 1 条在本地栈运行时必红。

---

## BUG-002 引导管理员的 nickname/email/phone 被静默丢弃

**严重度**：中（数据缺失，非崩溃）

**现象**

`GET /api/v1/profile` 对 `admin` 返回 `nickname:""`、`email:""`、`phone:""`，
尽管 `configs/users.json` 明确写了 `nickname: "系统管理员"`、`email: "admin@croupier.local"`。

**根因（两处叠加）**

1. `AdminManager.loadDefaultAdmins` 按 `admins.json` → `users.json` 顺序遍历，
   但只要某文件 `loadedCount > 0` 就 **`return`**。`configs/admins.json` 里
   `admin` 只有身份没有档案字段，于是它先「命中」，`users.json` **根本没被读**，
   同名去重把档案字段静默丢弃。
2. `seedBootstrapAdmins` 对已存在的 DB 行只同步 `status`，不回填空档案字段——
   即使配置正确，存量行也永远是空。

**修复**

- `loadDefaultAdmins`：不再提前 return；改为「首个贡献身份的文件生效后，后续文件
  仅做档案补齐（nickname/email/phone 只补空、不覆盖），不再新增账号」，
  保持「admins.json 优先、users.json 兜底」的语义不变。
- `seedBootstrapAdmins`：对存量行仅回填空档案字段，**不覆盖用户已设置的值**，
  二次 seed 幂等。

**回归测试**：`internal/svc/admin_manager_test.go` 四条
（`..._EnrichesProfileFromLaterConfig` / `..._EnrichDoesNotOverwriteExistingProfile` /
`..._LaterFileDoesNotAddIdentity` / `..._SecondFileLoadsWhenFirstEmpty`）+
`internal/svc/bootstrap_admin_seed_test.go` 的
`TestSeedBootstrapAdminsBackfillsEmptyProfileFields`（含二次 seed 幂等断言）。

---

## BUG-003 LB 监控「归属率」仪表盘永远不显示数值

**严重度**：中（功能静默失效）

**现象**

`/ops/lb` 的「归属 vs LB 对账（僵尸探测）」卡片里，归属率仪表盘是空白的，
读数不反映 `agent 节点数 / LB 后端数` 的真实比例。

**根因（已在本机复现，三段证据）**

1. `@ant-design/plots@2.6.8` 的 gauge adaptor 把入参改写成对象——
   实跑其 adaptor，`data: 0.5` 恒变为 **`{"value": 0.5}`**
   （`es/core/plots/gauge/adaptor.js`：`params.options.data = { value: data }`）。
2. `@antv/g2@5.4.8` 的 Gauge mark 只认 `number` 或
   `{target, total, percent, name, thresholds}`——其 `GaugeData` 类型里**没有
   `value` 这个键**。
3. g2 的 `getGaugeData` 对非 number 原样返回，`dataTransform` 只解构
   `{name, target, total, percent, thresholds}`。因此 `target/total/percent`
   全部为 `undefined`，`_target = percent || target = undefined`、
   `_total = percent ? 1 : total = undefined`，通道 y 退化成 `undefined` / `NaN`。

即：**百分数被静默丢弃**，指针与读数都不动。

> 早期注释写作「渲染即抛 TypeError」。按上述数据通路推导，值是**静默丢成
> undefined/NaN** 而非必然抛错；本文档以实测到的事实为准。

**修复**

- `web/src/pages/Ops/LBMonitor/index.tsx`：改用 antd `Progress type="dashboard"`
  呈现同一比例（不依赖 canvas 图表链路）。
- 抽出纯函数 `ownershipRatioPercent(nodes, backends)`：无 agent → 0（不是 NaN）、
  无 backend → 分母取 1 避免除零、超 100% 截断。

**回归测试**：`web/src/pages/Ops/LBMonitor/__tests__/reconciliation.test.tsx`
7 条——5 条纯函数边界（含 0–5 × 0–5 全组合断言恒为 0–100 整数、非 NaN/Infinity），
2 条组件渲染（读出 `归属 2 / LB 后端 4` 与 `50%`；无 agent 时 `0%`）。

---

## BUG-004 菜单树排序标签漏传 intl values，真实 intl 抛 MISSING_VALUE

**严重度**：低（界面正确、控制台报错），但**掩盖了错误的实现方式**

**现象**

菜单管理页每个带排序值的节点都在 console 刷 intl 解析错误（`MISSING_VALUE`），
可见文本却是对的。

**根因**

组件写成：

```ts
fmt('pages.menuManagement.page.order', '排序 {order}').replace('{order}', String(n))
```

`{order}` 是 ICU 占位符。真实 `react-intl` 在 defaultMessage 含占位符却没有
`values` 时会走 `onError`（MISSING_VALUE），返回未插值的串——`.replace` 只是把
**可见文本**兜住了，错误本身仍在。正确写法是经 `values` 传参。

**为什么长期没被发现**：`MenuTree.test.tsx` 自己 mock 了 `@umijs/max`：

```ts
formatMessage: ({ defaultMessage }) => defaultMessage   // 丢弃 values
```

这个 mock 把「漏传 values」的错误一并吞掉，等于替组件打了补丁，于是组件得以停留在
错误写法上。

**修复**

- `MenuTree.tsx`：新增 `fmtOrder(order)`，`intl.formatMessage(desc, { order })`。
- `MenuTree.test.tsx`：把 mock 换成**忠实实现**（按 `values` 插值），并记录
  每次调用的 `id/defaultMessage/values`。

**回归测试**（同文件）：`排序标签的 {order} 经 intl values 传入，不靠组件侧 replace 兜底`
——断言每个含 `{order}` 的调用都带上了数值型 `order`，且插值后的文本为 `排序 3`。

**变异验证**：把 `MenuTree.tsx` 改回 `.replace` 写法后，
- 旧用例 `已发布/草稿页面…排序 3` **仍然通过**（证明它没有判别力）；
- 新用例失败（`values` 为 `undefined`）。

这条对比是本文档「验收口径」里「必须能判别」的由来。

---

## BUG-005 antd 6 废弃属性 142 处，TypeScript 不报错、页面也不报错

**严重度**：中（污染控制台、掩盖真实问题、升级到 v7 会集体断裂）

**现象**

dev 模式每次进页面刷 `Warning: [antd: X] \`p\` is deprecated. Please use \`q\` instead.`。
其中 `Drawer height` 来自**全局布局**的 `GameSelector`，等于每翻一页都刷一次
（审计脚本量到单页 18 次）。

**根因**

仓库已升到 `antd@^6.4.3`（实测安装 6.6.0），但代码里仍有大量 antd 5 写法。
antd 6 对这批属性**只打告警、不改行为**，所以 `tsc` 全绿、页面也正常，废弃用法可以
无限期潜伏。真正的风险是 v7 移除后集中断裂，以及告警淹没控制台后真问题被埋掉。

清单不是照文档手抄的，而是从 `node_modules/antd/es/**/*.js` 里的
`devUseWarning('<Component>')` **运行时告警表**反推——antd 实际有三种写法，
抽取器三种都认（否则会漏）：

1. 对照表 + `forEach`：`[['message', 'title']].forEach(...)`（多数组件）
2. 直接调用：`warning.deprecated(!tip, 'tip', 'description')`（`Spin`）
3. 对象字面量映射：`const deprecatedProps = { dropdownRender: 'popupRender' }`（`Select`）

> 只收**属性名**级别的废弃。形如 `size="default"` 的值级建议不是 JSX 属性，不纳入。

**修复（142 处 → 0）**

| 属性 | 处数 | 处理方式 |
| --- | --- | --- |
| `Alert message` → `title` | 100 | 直接改名 |
| `Drawer width` / `height` → `size` | 20 | 直接改名（`drawerSize` 归一后取值一致） |
| `Space direction` → `orientation` | 12 | 直接改名 |
| `Input` / `InputNumber addonBefore` → `Space.Compact` | 6 | **结构改写**：前缀移出输入框，成为 `Space.Compact` 的相邻兄弟节点（静态表单：PathControls ×3、BindingModal ×2、SourceModal ×1） |
| `Input addonBefore` → `Input.prefix` | 3 | **结构改写**，但改用 `prefix` 而非 `Space.Compact`——见 BUG-009（热路径编辑器：ActionEditor ×1、ConstantFieldsEditor ×2） |
| `Statistic valueStyle` → `styles.content` | 1 | 嵌套路径，改为 `styles` 的 `content` 槽（`content: {...}`） |
| `Spin tip` → `description` | 5 | 直接改名 |

改写通过「按 JSX 开始标签定位」的 codemod 完成，跳过 `{}` 表达式与字符串字面量里的
`>`（否则 `<Alert title={a > b} />` 会在表达式中间被误判为标签结束），只改属性名，
不碰子元素/对象字面量/同名的其他组件。

**回归测试**：`web/tests/antd6Deprecations.test.ts` 3 条：

1. 抽取器本身有效（`> 50` 条对照项 + 抽样命中 `Alert.message` / `Drawer.width` /
   `Drawer.height` / `Space.direction` / `Statistic.valueStyle` / `Card.bordered` /
   `Divider.type` / `Spin.tip` / `Select.dropdownRender`）——防止清单失效后守卫变成
   「永远通过」的假阴性；
2. `src` 下不存在任何废弃 JSX 属性，失败时逐条报 `文件:行号 <X p=…> → 改用 q`；
3. 字面量兜底：`addonBefore=` / `addonAfter=` / `valueStyle=` 一律不得出现。

**变异验证**：把 `Welcome.tsx` 的 `variant="borderless"` 改回 `bordered={false}`，
第 2 条立即失败并指出 `src/pages/Welcome.tsx:126 <Card bordered=…> → 改用 variant`。

---

## BUG-006 组合页编辑器菜单缺 locale key

**严重度**：低

**现象**：`menu.FunctionsAndPages.CompositeEditor` 在 `zh-CN` / `en-US` 的
`menu.ts` 里都没有登记，菜单项回退显示原始 key。

**修复**：`web/src/locales/{zh-CN,en-US}/menu.ts` 补
`组合页编辑器` / `Composite Page Editor`。

---

## BUG-007 后端返回空头像串时渲染 `<img src="">`，浏览器把当前页重新请求一遍

**严重度**：中（真实的多余网络请求 + React 告警）

**现象**

`/admin/account/center`（亦即 `/account/center` 的重定向目标）控制台报：

```
An empty string ("") was passed to the src attribute. This may cause the
browser to download the whole page again over the network.
```

**根因**

`GET /api/v1/profile` 在未设置头像时返回 `"avatar":""`（本机实测确认）。
`Profile/index.tsx` 与 `AvatarModal.tsx` 把它直接透传给 `<Avatar src={...}>`。
空 `src` 被浏览器解释为「重新请求当前页面的 URL」。注意同处代码的
`icon={!profile?.avatar ? <UserOutlined /> : undefined}` **已经**判空——只有
`src` 漏了。

**修复**

抽出 `normalizeAvatarSrc()`（`web/src/pages/Profile/shared.ts`）：非字符串、空串、
纯空白 → `undefined`；合法 URL 去首尾空白后返回。两处调用点统一使用。

**回归测试**：`web/src/pages/Profile/__tests__/InfoTab.regression.test.tsx`
5 条——`normalizeAvatarSrc` 的空串/空白/null/undefined/非字符串（防后端类型漂移）/
合法 URL 六个分支，外加一条源码级锁定：两处 `src={...}` 必须走归一函数，且不得
出现未归一的直接透传（渲染整个 Profile 需拉起 7 个 Tab 与一批接口，代价与收益
不成比例，故此处用源码断言，并在用例注释里写明取舍）。

---

## BUG-008 个人中心表单实例未连接，antd 每次渲染都告警

**严重度**：低

**现象**：`/admin/account/center` 控制台报

```
Warning: Instance created by `useForm` is not connected to any Form element.
Forget to pass `form` prop?
```

**根因**

表单实例由主页 `Profile` 的 `useForm` 创建并复用（`useProfileData` 拉到资料后
`form.setFieldsValue` 回填、取消编辑再回填、保存时 `form.submit()`），但
`InfoTab` 里 `<Form form={form}>` **只在 `editing` 为真时渲染**。非编辑态下实例
处于「未连接」状态。

告警并非挂载时报的，而是 `FormHook.warningUnhooked`——由**表单实例方法**
（`setFieldsValue` / `getFieldsValue` / `submit` …）在 `setTimeout` 里检查
`formHooked` 触发。所以复现路径是「未连接 + 调方法」，只挂载不调方法不会告警。

**修复**：`InfoTab` 中 `<Form>` 改为**常驻挂载**，非编辑态用 `hidden` 容器隐藏
（`display:none` 不参与布局，视觉与之前一致）。

**回归测试**：同文件 4 条。要点：

- 一条**对照**用例：构造「不挂 `<Form>` 的孤立实例」并调 `setFieldsValue`，
  断言确实命中告警——证明告警通道有效，否则正向断言是假阴性；
- 回归锁用**结构不变量**（非编辑态 `<form>` 元素必须在 DOM 中、且带
  `div[hidden]` 祖先），而不是「断言无告警」。

**为什么不用「断言无告警」**：rc-util 的 `warning` 按 message 去重，同一条文案在
一个模块生命周期内只报一次。对照用例一旦先报过，后续任何「无告警」断言都恒真。
这一点是**实测踩出来的**：把 `InfoTab` 改回条件渲染后，「无告警」断言仍然通过，
而结构断言失败。现已改为结构断言，变异验证下有 2 条用例转红。

---

## BUG-009 按 antd 官方建议用 `Space.Compact` 迁移 `addonBefore`，把编辑器拖慢 10 倍

**严重度**：中（不改变功能，但把测试/交互性能打到超时）

**发现经过**

修 BUG-005 时按 antd 官方迁移建议，把 `ActionEditor` / `ConstantFieldsEditor` 里
标签前缀包进 `Space.Compact`。改完 `tsc` 全绿、相关单测也过，直到跑**全量** jest：

```
Test Suites: 1 failed, 253 passed, 254 total
Tests:       1 failed, 3251 passed, 3252 total
```

且失败用例每次都换一批、单例耗时从 <1s 涨到 5–21s——典型的**超时**特征而非逻辑错误。

**根因**

antd 的 `Space.Compact` 会给**每个子项**包一层 `CompactItem` context provider：

```js
const CompactItem = props => {
  const { children, ...others } = props;
  return <SpaceCompactItemContext.Provider value={useMemo(() => others, [others])}>
    {children}
  </SpaceCompactItemContext.Provider>;
};
```

`others` 是每次渲染新建的对象，`useMemo` 依赖因此每渲染必变 → **context 值每渲染
都是新引用** → 所有消费该 context 的 `Input`（`useCompactItemContext`）每次都被
拖进重渲染。行操作编辑器在表格/预览里会被大批量实例化，这个成本被成倍放大。

> 注：这两个文件里包裹 `Select` 的 `Space.Compact` 是**既有写法**，且实测未触发
> 问题（Select 的重渲染成本低于 Input）。本条只针对「标签前缀」这一处新增的包裹。

**修复**

标签前缀改用 `Input` 自带的 `prefix` 属性（未被 antd 6 废弃，同样能承载标签），
不引入额外组件，因而不产生上述 context 重建。

- `web/src/pages/PageStudio/CompositeEditor/ActionEditor.tsx`
- `web/src/pages/PageStudio/CompositeEditor/ConstantFieldsEditor.tsx`

静态表单（`PathControls` / `BindingModal` / `SourceModal`）仍按官方建议用
`Space.Compact`——它们不是热路径，无此问题。

**回归测试**：`web/tests/antd6Deprecations.test.ts` 的
`热路径编辑器的标签前缀用 Input.prefix，未被 Space.Compact 包裹`——

- 截取「承载标签的那个元素」所在源码片段，断言其内有 `<Input` + `prefix={<span`，
  且**不含** `<Space.Compact`（先剥注释，否则解释性注释本身会命中子串匹配）；
- `ConstantFieldsEditor` 另按 `titleAddon` / `varNameAddon` 两个锚点回溯到各自的
  `<Input`，分别断言。

**变异验证**：把 `ActionEditor` 改回 `Space.Compact` 包裹后该用例立即失败。

**效果**：全量 jest 由 207s 回到 ~145s，连续两次全量通过（3252/3252）。

> 一处实测反直觉，值得记下：单文件 40 个 `ActionEditor` 的定向基准里，
> `prefix`（838ms 挂载 / 1115ms 重渲染 5 次）与 `Space.Compact`
> （863ms / 1070ms）**基本无差异**。也就是说，定向基准测不出这个问题，
> 只有全量并发下的真实负载才暴露。**性能回归不要指望单个基准用例来发现。**

---

## BUG-010 `app.cancel` 未登记 locale key，组合页编辑器每次打开都报错

**严重度**：低（文案回退到 `defaultMessage`，但控制台每次都刷红）

**现象**：`/functions/pages/composite-editor` 控制台连报 4 条

```
[React Intl] Missing message: "app.cancel" for locale: "zh-CN", using default message as fallback.
```

**根因**

`DanglingRefsModal.tsx` 与 `BindingDrawer.tsx` 都用
`intl.formatMessage({ id: 'app.cancel', defaultMessage: '取消' })`，但 `app.cancel`
在 `locales/{zh-CN,en-US}/app.ts` 里从未登记——该文件此前只承载
`app.layout.*` 与 `app.request.*` 两组键，没有通用动作文案这一组。

`formatMessage` 带 `defaultMessage` 时会回退渲染，功能不受影响，但缺失键会被
`onError` 上报，控制台每开一次弹窗就多两条。

**修复**：在 `zh-CN/app.ts` 与 `en-US/app.ts` 补齐通用动作键组
（`app.cancel` / `app.confirm` / `app.save` / `app.close`），两语言同步。

> 该问题不在 BUG-005 的静态扫描范围内——废弃告警与缺失 locale 是两类不同问题。
> 它是**控制台审计**（运行时证据）发现的，这正是本轮同时保留静态守卫与运行时
> 审计两条路径的原因。

---

## BUG-011 审计日志两页用 `hash` 当 rowKey，整页 key 全同

**严重度**：中（React 重复 key，行可能重复/漏渲染）

**现象**：`/admin/login-logs` 每页刷十余条

```
Encountered two children with the same key, `%s`. Keys should be unique so that
components maintain their identity across updates.
```

**根因**

`LoginLogs.tsx` 与 `OperationLogs.tsx` 都写 `rowKey={(r) => r.hash}`，而
`/api/v1/audit` 在当前部署**不回填 `hash`**——实测该接口返回 19 条记录，
`hash` 字段**全部缺失**：

```
items 19 / unique hash 1 / empty hash count 19
```

`normalizeAuditEvent` 又把缺失的 `hash` 兜底成 `''`，于是 `r.hash` 恒为 `''`，
整页所有行的 key 完全相同。服务端其实有唯一 `id`
（形如 `audit_1790375253236648061_bb169c724c610ee5`，19 条互不相同），
但归一化时**没有带过来**。

**修复**

- `services/api/audit.ts`：`AuditEvent` 增加 `id` 字段并在
  `normalizeAuditEvent` 中透传 `item.id`。
- 新增 `auditRowKey(event, index?)`：`id` → `hash` → `time|actor|kind|target`
  → 行序号，保证同页内唯一。
- 两个页面在**切片/取数阶段**就把 `__rowKey` 落到行上，Table 用字符串
  `rowKey="__rowKey"`。

**为什么不用 index 兜底在 rowKey 回调里**：antd 6 已废弃该参数
（`index` parameter of `rowKey` function is deprecated）。第一版实现写成
`rowKey={(r, i) => auditRowKey(r, i)}`，运行时审计立刻又抓到 2 条新告警——
这正是「先改完再审计」比「静态扫描」多一层保障的例证。

**回归测试**

- `web/src/services/api/auditRowKey.test.ts`（7 条）：归一化保留 `id`；`auditRowKey`
  四级回退各自命中；**还原真实响应**（19 条、`hash` 全空）后整页 key 互不相同。
  变异验证（把实现退回 `return event.hash` 并删掉 `id`）后 6 条转红。
- `web/src/pages/Admin/LoginLogs.test.tsx` 的 `hash/id 全缺时行 key 仍互不相同`：
  页面级，直接断言 DOM 上 `data-row-key` 互异且非空。变异验证（改回
  `rowKey={(r) => r.hash}`）后转红。

> 这里**没有**用「断言控制台无 duplicate-key 告警」的方式：React 的该告警受全局
> 去重影响，先跑到的用例会消费掉后续的告警，断言会恒真（BUG-008 已踩过一次同类坑）。
> 改为直接断言 DOM 上的 key 唯一性这一被测性质。

---

## BUG-012 账户中心头像四处独立缺陷（数据互清 / 死链 / 404 / 顶栏恒占位）

**严重度**：高（其一为数据丢失：任何一次资料保存都会清空其它字段）

**现象**：用户实测反馈「账户中心头像错误」。定位为 4 个互相独立的缺陷：

1. **数据互清**：`ProfileUpdateRequest` 四字段裸 string，Go 绑定缺失字段得空串，
   service 无条件四列齐写——「只改昵称」会清空头像/邮箱/手机。既有测试只断言
   `resp.Ok`，从不断言字段幸存，长期无人发现。
2. **死链**：上传接口返回的 `url` 是带签名、带过期时间的地址（S3/OSS/COS 默认
   15min TTL），原样存库等于存定时失效死链。
3. **file 驱动 404**：`fileStore.SignedURL` 返回 `/uploads/<key>` 相对路径，但
   全仓无 handler 提供该路径。
4. **顶栏恒占位**：`toCurrentUser` 丢掉 profile 的 avatar，`getInitialState`
   也不回填——顶栏头像从未生效过。

**修复**（提交 `e69db6f`）：

- 四字段改 `*string`：nil=未携带=保留原值，显式空串=清空，两种语义分开；
  空请求短路不写库。
- 库里只存**裸对象 key**，读取时现算有效 URL；`objstore.NormalizeAvatarKey`
  统一归一（裸 key / file 相对路径 / 绝对 URL），拒绝非 `avatars/` 目录与路径
  穿越；存量绝对 URL 读取时原样返回（拼成 `/uploads/https://…` 会把「可能
  过期」变成「必然 404」），用户下次保存即自愈。
- file 驱动挂载静态目录，只暴露 `avatars/` 子树（不泄露缺陷附件等通用上传物），
  免鉴权（`<img src>` 不带 Authorization），dev proxy 补 `/uploads/` 转发。
- `CurrentUser` 增加 avatar/nickname 透传回填；新增共享组件 `UserAvatar`，
  占位统一为姓名首字母（中文首字/英文首字母/username/? 四级回退），src 与
  占位用同一份归一结果判定，消除「纯空白头像渲染成空圆圈」的分叉。

**回归测试**：`objstore/avatar_test.go`（归一四类输入、拒绝五类非法输入、
穿越防护、每次读取现签、存量 URL 原样透传、静态目录校验）；
`api/profile/avatar_test.go`（只改单字段时其余字段幸存、空请求不写库、显式
空串即清空、头像落裸 key 四种输入、非约定目录拒绝）；`UserAvatar` 组件测试
11 条（首字母规则/渲染/独立占位）。本机独立实例（28780/29090）实测上传→
签名→保存→无 Authorization 头取回 200 且字节一致。

## BUG-013 MFA 绑定体验不可用 + 无恢复码兜底

**严重度**：高（丢失验证器 App 即被锁死在账号外）

**现象**：`/admin/account/center?tab=security` 开启两步验证只有「手动抄 base32
密钥」一条路径——微软/谷歌 Authenticator 并不提供「粘贴 otpauth 链接」的入口，
绑定流程实际不可用；且开启后**没有任何恢复手段**，丢手机=丢账号。

**根因**：

1. `otpauth://` URI 手写拼接，账号名未 percent-encode（含 `@`/`/`/非 ASCII 的
   用户名会让验证器解析失败），且缺 `algorithm/digits/period` 参数，部分 App
   按自己的默认值解析；
2. 前端无二维码渲染，无恢复码机制；
3. `VerifyTOTP` 依赖 `time.Now()`，RFC 6238 附录 B 的固定测试向量无法复现，
   实现正确性无规范级锁定。

**修复**：

- `otp.OtpauthURI`：label 规范 percent-encoding + issuer 参数显式 +
  algorithm/digits/period 齐全（Google Authenticator 校验 label 前缀与 issuer
  参数一致，不一致会静默丢弃）。
- `VerifyTOTPAt`/`CodeAt` 暴露可注入时间点；`DecodeSecret` 归一各 App 导出的
  密钥形态（大小写/空格/补位）。
- 备用恢复码：绑定时一次性签发 10 码（30 字符字母表去易混淆字符，约 49.5 bit
  熵），SHA-256 存储、明文仅 confirm 响应返回一次；逐条单次消费
  （`UPDATE ... WHERE used_at IS NULL` 天然防并发复用）；登录页动态码输入框
  兼容恢复码（`verifySecondFactor` 二选一）；关闭 MFA 随即清空恢复码。
- 前端：二维码扫码即绑（手动录入兜底保留）、恢复码一次性展示+下载、剩余数量
  提示（≤2 预警）；登录页 totpCode 输入 maxLength 6→12 并提示可用恢复码。
- 迁移 0032 `admin_otp_recovery_codes` 表（HasTable/CreateTable 幂等），
  `MinimumRequiredVersion` 31→32，migrate_test probe 同步。

**回归测试**：`security/otp/rfc6238_test.go`（RFC 6238 附录 B 官方向量 +
skew 行为）；`otpauth_test.go`（URI 归一/转义/参数）；`api/auth/mfa_recovery_test.go`
14 条（签发/单次消费/空格输入归一/关闭清空/重绑替换/剩余统计/恢复码登录三态）；
`MfaSettings.test.tsx` 8 条（二维码渲染/恢复码一次性展示/下载/关闭确认/外部
账号说明）。

线上链路实测（重启后的本地栈，curl 全流程）：setup → confirm（真实 TOTP 码）→
一次性返回 10 码 → 无码登录 401 `mfa_required` → 恢复码登录 200 → **同一码重放
401** → TOTP 登录 200 → disable（code+password）后库中恢复码清零、普通登录恢复。

---

## BUG-014 健康分数衰减用例依赖墙钟，机器繁忙时随机失败

**严重度**：中（不稳定的门禁：会让「全绿」随机器负载随机翻车）

**现象**

`internal/platform/dispatch` 的 `TestHealthTracker_ScoreDecay` 间歇性失败：

    expected score to decay, still at 100.000000

同一份代码在低负载时连续通过，机器繁忙时（本次即遇到同机 45 个 C++ 编译进程、
load average 144）失败。与被测逻辑无关。

**根因**

用例假定「100ms 的 decay ticker 一定会在 150ms 内至少跳一次」：

```go
tracker.Start()
// ...
time.Sleep(150 * time.Millisecond)
if state.HealthScore() >= 100.0 { t.Errorf("expected score to decay, ...") }
```

`scoreDecayLoop` 是后台 goroutine 的 `time.NewTicker`。调度器繁忙时 150ms 内
`ticker.C` 可能一次都没被消费，分数原封不动 → 用例失败。这与 BUG-001 同源：
**测试把环境时序当成了前提**。

**修复**

改为轮询到「已衰减」为止并给出宽松上界（3s deadline），断言换成与实际跳数
无关的不变量：

- 分数确实下降（否则报超时并带上实际等待时长，便于定位）；
- 分数落在 `100 × 0.5^n`（n≥1）的离散集合上——既锁住「按 `ScoreDecayRate`
  衰减」，又不依赖到底跳了几次。

**验证**：修复后连跑 5 次通过；再人为拉起 8 个 CPU 忙循环制造竞争，连跑 3 次
仍全绿（原实现在该条件下失败）。

---

## BUG-015 antd 6 整体废弃 `List` 组件——属性级守卫的盲区

**严重度**：中（控制台告警 + v7 升级即整体断裂）

**现象**：交互走查 `/admin/account/messages`（消息中心）时控制台报：

```
Warning: [antd: List] The List component is deprecated. And will be removed in next major version.
```

而 BUG-005 的静态守卫与此前 52 条路由的运行时审计都是 **0 告警**——因为消息中心、
工单详情、账户中心各 Tab 都不在当初的审计路由清单里（动态/详情页未纳入）。

**根因**：`List` 是 antd 6 里**组件级**废弃（`warning(false, 'deprecated', ...)`，
不挂在任何属性上）。BUG-005 守卫的抽取器从 `deprecated('p', 'q')` 表反推**属性**
对照，组件级废弃天然扫不到——两条守卫（属性级/组件级）是不同维度，缺一不可。

**修复**：

- 全仓 6 个真实调用方（`Profile/GamesTab` / `NotificationsTab` / `AuditList` /
  `PermissionsTab`、`Assignments/HistoryModal`、`Support/Tickets/Detail`；注意
  `PageRenderer` / `ResourcePageEditor` 的 `<ListTab` 是同前缀误匹配）全部只用
  `List / List.Item / List.Item.Meta` 只读面，统一迁到内部替身
  `src/components/SimpleList`：泛型 + `renderItem` + `Item(actions/style/onClick)` +
  `Item.Meta(title/description)` + `loading/pagination/locale.emptyText`，
  createStyles 对齐 antd List 默认视觉契约（分隔线/标题/次要描述/底部分页）。
- `web/tests/antd6Deprecations.test.ts` 新增**组件级**守卫：前置确认安装的 antd
  仍处于 List 已废弃状态（升级后自动提示重审），随后断言 `src` 下无 `<List` JSX
  （负向断言不误伤 `<ListTab`）也无 `import { List } from 'antd'`。

**回归测试**：`SimpleList.test.tsx` 8 条（条目/Meta 渲染、emptyText、actions、
整行 onClick、rowKey 逐条调用、Spin 包裹、pagination 翻页回调、泛型形态）+
守卫新用例；迁移文件的既有测试全绿（Profile/Tickets/Permissions 64 条、
Functions DetailSections 19 条）。运行时复证：重跑走查后 List 告警 0 条。

---

## BUG-016 安全中心「登录通知」是假开关（假状态 + 假交互）

**严重度**：中（用户被误导以为自己在收短信登录提醒）

**现象**：`/admin/account/center?tab=security` 的「登录通知」行，只要用户填了
手机号就渲染绿色「已开启」标签；辅助文案写「用于登录提醒和短信验证」——而仓内
**没有任何短信服务商**（`ChannelSMS` 只是个没人用的枚举常量，无 sender、无配置、
无 UI 入口）。用户据此以为自己在收短信。

**根因**：前端用「用户填了手机号」（`hasPhone`）推断通道可用。手机号只是接收
目标，与「服务商是否接入」毫无关系。这正是本文档反复出现的**假状态**问题在
通知域的变体。

**修复**（新增 `GET /api/v1/profile/notification-channels`，可用性只由后端判定）：

- `approvals/sms.go`：固定短信接入点——`SMSProvider` 接口 + `ErrSMSNotConfigured`
  + 默认 `unconfiguredSMSProvider`（未接入时发送**明确失败**而非静默成功）；
  `SMSRegistry.Status()` 只在「已注册 provider 且凭据齐备」时报可用。
- `api/profile/notification_channels.go`：三通道事实来源——in_app（站内信零配置
  即通，平台设置可关）、email（SMTP 未配置时 EmailSender 是 no-op，不得声称可用）、
  sms（只看注册表）。`available` 与 `userEnabled` 是独立维度；不可用必带 `reason`。
- 前端 `SecurityTab`：状态完全由后端驱动。不可达 → warning 标签 + 原因 tooltip +
  开关禁用；**收口修正一**：开关改只读呈现 `userEnabled`——它当前来自平台级
  设置、没有用户侧写接口，「可点击但无任何效果」的假交互同样是本条要消灭的东西；
  **收口修正二**：状态标签三分支——通道可达但平台未开启时显示「已关闭」，否则
  「SMTP 已配置但平台关了邮件通知」会渲染成绿色「已开启」+ 未勾选的开关，自相
  矛盾（原实现就漏了这支）。
- 区头文案改用新键 `profile.channel.section`（旧键 `profile.login.notification`
  的现有值是「登录通知」，defaultMessage 不会生效，收口前区头实际显示的是旧文案）。

**回归测试**：`notification_channels_test.go` 10 条（未接入/凭据不齐/SMTP 有无/
管理员关闭/账号缺失/Registry 边界——含「填了手机号也不得变可用」的核心不变量）；
`SecurityTab.channels.test.tsx` 14 条（含「可达但未开启 → 已关闭」「in_app 被
关闭 → 已关闭+原因」「开关只读」三条收口新增）。

---

## BUG-017 飞书签名密钥漏登记 secretKeys，掩码回存会覆盖真值

**严重度**：中（配置损坏：通知静默失败，非泄露）

**现象**（代码审阅发现，未上网络）：`notification.feishuSecret` 在
`ValidKeys` 里却不在 `secretKeys` 里。

**根因与影响**：`GET /api/v1/site/notification` 快照对它有专门的
`feishuSecretMasked` 字段（掩码一直正常），但 `PutKey` 的「掩码回存保护」分支
只认 `IsSecretKey`——管理端把快照原样回存时，`****+尾4` 会被当成真值覆盖入库，
此后飞书通知用假密钥**静默失败**。同类的 SMTP 密码/DingTalk/Webhook 密钥都
登记了，唯独飞书漏网。

**修复**：`secretKeys` 补 `KeyNotifyFeishuSecret`；注释原文声称「明文回显泄露」，
经核对（快照掩码先于本修复存在、裸值 `FeishuSecret` 只进 sender 无 HTTP 暴露面）
属于误判，已改为上述真实缺陷。

**回归测试**：`layered_notify_test.go` 的 `TestKeyClassificationHelpers` 增加
`IsSecretKey(KeyNotifyFeishuSecret)` 断言（修复前为 false）。

---

## BUG-018 游戏访问权限恒为空：admin 看到的是一块空白

**严重度**：中（无法判断是「全部权限」还是「没有权限」）

**现象**

`/admin/account/center?tab=games`：admin 的每个游戏卡片上「权限」一栏永远是空的。

**根因**

`internal/api/profile/service.go` 的 `GetUserGames` 在构造 `ProfileGame` 时写死了

```go
Permissions: []string{},
```

不是「查不到」，是**字面量**——对所有用户、所有游戏恒为空。前端 `GamesTab` 直接
`{(game.permissions || []).map(...)}` 渲染，空数组即空区域。admin 与只读账号
看到的东西完全一样，无法区分「拥有全部权限」与「一个权限都没有」。

**修复**：如实上报，并把口径讲清楚。

- `GetUserGames` 改为按用户实际持有的权限计算
  （`internal/api/profile/permissions.go` 的 `perGamePermissions`）：
  持通配 → `["*"]`（前端渲染为绿色「全部权限」）；否则返回真实持有的权限 id。
- DTO 增 `accessLevel`（`full` / `scoped` / `none`）与 `permissionScope`（固定
  `"role"`）。空权限由此成为**可解释状态**而不是空白。
- 前端 `GamesTab` 按级别渲染三态（全部权限 / 逐条标签 / 无显式权限+原因说明），
  并说明「权限按角色授予，不按游戏单独切分」——避免用户以为每个游戏能单独授权。

**口径澄清**：RBAC 挂在角色上、**不按游戏维度切分**；按游戏/环境切分的是
「可见游戏与可见环境」（`admin_game_env_scopes`，`GetUserGames` 已用它过滤）。
因此同一用户在所有可见游戏上的权限集本来就是同一个，per-game 权限不存在
「只在这个游戏有某个操作」的语义。

**回归测试**：`internal/api/profile/permissions_test.go`（`GetUserGames_AdminSeesFullAccessPerGame`
断言 `["*"]`+`full`；`GetUserGames_NonAdminWithoutPermissionsIsNone` 断言 `none`）
+ `web/src/pages/Profile/__tests__/GamesTab.permissions.test.tsx` 5 条（含
「后端未升级、无 accessLevel 时也绝不留白」这条兼容路径）。

---

## BUG-019 权限概览是编造数据：resource 恒为 "role"、角色名混进权限 id

**严重度**：高（前端据此渲染的「权限概览」与实际授权无关）

**现象**

个人中心「权限」页/弹窗里的「已有权限」列表内容不对：资源名全是 `role`，
每条的操作是角色名。

**根因**

`GetPermissions` 的两处构造都是编造的：

```go
permissions = append(permissions, ProfilePermission{
    Resource: "role",              // 字面量
    Actions:  []string{role.Name}, // 把角色名当操作
})
```

更隐蔽的一处：

```go
for _, role := range roles {
    appendPermission(role)                    // ← 角色名进了 permissionIDs
    if role == "admin" || role == "super_admin" {
        appendPermission("admin")             // ← 同样是角色名，不是权限
        appendPermission("*")
    }
}
```

于是 admin 的 `permissionIDs` 是 `["*", "admin", "user:read", ...]`。前端拿
`permissionIDs` 与权限目录做差集来渲染「已授权 / 未授权」，`"admin"` 与
`"super_admin"` 在 `configs/permissions.json` 里查不到，于是变成两条永远
「未授权」的假权限。同时 `permissionIDs` **不含**具体资源的完整授权信息
（只到并集），树状结构无从构建。

**修复**

- 角色名彻底退出 `permissionIDs`，只留在 `roles`；`permissions[]` 改为
  真实的资源 → 操作分组（`resolvePermissions`）。
- 资源轴取自**权限 id 的前缀**（`pages:write` → resource=pages），而不是
  `permissions` 表的 `resource` 列——那一列存的是 module（如 `dashboard`），
  38 条目录会塌缩成 7 个值。这是「树只剩几个节点」的直接原因。
- 通配判定收紧为**两个维度都通配**才叫 `fullAccess`（`resource==* && action==*`，
  即 `*` / `admin:all`）。`user:*` 只是「user 这个资源的全部操作」，无权访问
  其它资源；把它算成 fullAccess 会让前端把所有条目渲染成「已授权」——又是一次
  假状态。单段 id（无冒号）同理按「整串即资源」处理，不按 `.` 猜切分。
- 通配 id 不生成假的 `*` 资源节点。
- 拆分语义统一到 `rbac.SplitLogicalPermission`（本次为导出），
  `splitLogicalPermission` 与 profile 侧不再各写一份。
- 增 `rolePermissions`（逐角色授权明细）：只有并集时无法回答「这个操作是哪个
  角色给的」，树就只能退化成单层列表。

**回归测试**：`permissions_test.go` 21 条，含
`KeepsRoleNamesOut`（角色名不进 permissionIDs）、
`ResourceFromIDPrefixNotModule`（锁住「不塌缩成 module」）、
`PartialWildcardIsNotFullAccess`（`user:*` 不等于全部权限）、
`SingleSegmentIDBecomesItsOwnResource`（不按 `.` 切分）、
`RoleGrantsIncludeEmptyRoles`。同时改写 4 条**锁住了旧错误行为**的既有用例
（`TestService_GetPermissions_WithAdminRole` 等断言 `"admin" ∈ permissionIDs`），
这些断言本身就是 BUG-019 的固化。

---

## BUG-020 权限概览是单层平铺，没有资源/操作分层与授权两态

**严重度**：中（看不出哪些有、哪些没有）

**现象**

「已有权限」是一层平铺的列表，没有资源→操作的层级，也没有任何「已授权 /
未授权」的区分。

**根因**

`PermissionsTab` 用 `SimpleList` 平铺后端返回的 `groups`，而那批数据
（1）本身是编造的（见 BUG-019）；（2）只包含**已授权**项。因此灰掉的
「未授权」操作根本没有数据来源——页面看上去就像「全部都有权限」。

**修复**：新增 `permissionTree.ts` + `PermissionTreeView.tsx`，渲染
角色 → 资源 → 操作三层。

- **资源/操作的候选集来自全量权限目录**（`GET /api/v1/permissions`），不是
  已授权 id；目录缺失时退化为「仅已授权」，仍不崩也不伪造。
- 每项按当前角色独立判定：绿（`#52c41a`）+ `CheckOutlined` = 已授权，
  灰（`#bfbfbf`）+ `CloseOutlined` = 未授权。颜色之外有第二重区分，不只靠色觉。
- 三层可展开收起；「展开/收起全部」在「全展开 ⇄ 仅角色层」间循环。
- 持账号级通配时整棵树按全绿呈现，并显式提示「灰色仅表示未由具体角色显式授予，
  实际可用」——否则用户会把灰色读成「不可用」。
- 有角色但零权限的角色仍出现在树上并标注「无显式权限」。
- 前端再兜一层：`getMyPermissions` 归一化时剔除被误塞进 `permissionIDs` 的角色名。

**回归测试**：`PermissionTree.test.tsx` 23 条（纯逻辑 17 + 渲染 6），
其中 `目录提供未授权项` 锁住核心缺口、`资源级通配 user:* 覆盖该资源全部操作，
但不影响别的资源` 锁住通配粒度。变异验证：把资源轴改回 module 后 7 条转红。

---

## BUG-021 「消息通知」Tab 挂着管理员广播入口，且公告功能无任何入口

**严重度**：高（越权入口）+ 中（功能缺失）

**现象**

`/admin/account/center?tab=notifications` 顶部有一个 primary 按钮「发送消息」，
点开是 `BroadcastModal`（群发站内消息）。个人中心是**所有用户都有**的页面，
普通用户与管理员看到的是同一个界面。

**根因**

- `NotificationsTab` 接收 `isAdminUser` / `onSendClick` 两个 prop，渲染「发送消息」
  按钮 + 挂 `BroadcastModal`。个人中心的语义是「我的资料/我的消息」，广播是管理
  动作，位置错位。
- 真正的公告接口 `/api/v1/admin/announcements`
  （`internal/api/announcement`，List/Create/Update/Delete + 用户侧 `/active`）
  一直存在且有测试，但**前端零引用**——即公告功能此前完全没有入口。
  `configs/permissions.json` 的 38 条权限里也没有公告相关项。

**修复**

- ① 收到的通知列表：保留未读标记/全部已读/详情，并补上此前缺失的
  **单条标为已读**（此前只能打开详情或一次性全标）。未读整行加底色 + 左侧竖条，
  不只靠「加粗」区分。
- ② 通知渠道偏好：站内/邮件/短信，状态**完全来自后端**
  `GET /api/v1/profile/notification-channels`（见 BUG-016），未接入的通道禁用开关。
  判定与渲染从 `SecurityTab` 抽到 `NotificationChannels.tsx`，两处共用同一份实现，
  避免漂移；开关为只读呈现（当前是平台级设置，没有用户侧写接口）。
- ③ 广播入口从个人中心**移除**：`NotificationsTab` 不再接受 `isAdminUser` /
  `onSendClick`，`BroadcastModal.tsx` 删除。新增管理后台
  `/admin/announcements`（`access: 'canAdmin'`，与后端 admin 路由组鉴权口径一致），
  接入 List/Create/Update/Delete，支持受众（全体/指定角色）、弹窗标记、生效时间区间。

**顺带修掉的两个缺陷**

- `SimpleList.Item` 不透传 DOM 属性，`data-testid` / `data-*` 被静默丢弃（antd 的
  `List.Item` 是透传的）。补上 `...rest` 透传，否则「按条目 id 断言未读状态」这类
  测试根本写不出来。
- 公告页 `load` 依赖 `intl` 派生的 `t`，测试环境的 intl mock 每次渲染返回新引用 →
  `useEffect([load])` 反复触发 → 一次提交连拉 9 次列表。改为经 `useRef` 转发让 `t`
  引用稳定。
- `submit` 里 `form.validateFields()` 的 reject 未接住 → 点「保存」而表单非法时产生
  unhandled rejection。

**回归测试**

- `web/src/pages/Profile/__tests__/NotificationsTab.test.tsx` 13 条：③ 无任何发送/
  广播控件、① 未读高亮/单条已读/全部已读/未读计数/详情、`标为已读` 不打开详情
  （stopPropagation）、② 通道状态渲染与「未加载时不得出现任何『已开启』」。
- `web/src/pages/Admin/Announcements/__tests__/index.test.tsx` 10 条：列表/空态/
  受众/标记如实展示/加载失败不白屏、创建必填校验、`audience=role` 必须填角色名、
  创建后重拉列表、编辑带 id 走 update、删除二次确认。
- `SecurityTab.channels.test.tsx`（14 条）改为从 `NotificationChannels` 导入
  `channelAvailability`，两处共用实现的契约由此被测试锁住。

---

## BUG-022 登录页无 antd App 上下文，登录成功/失败/MFA 的 message 提示全部丢失

**严重度**：高（登录失败时页面零反馈）

**现象**

Playwright 三条路径实测（修复前）：

1. 直接打开 `/user/login` 用正确密码登录 → 登录成功，但**没有任何「登录成功」提示**；
2. 从 `/` 重定向到登录页再登录 → 同样无提示，console 还报
   `[antd: Message] You are calling notice in render ...`；
3. **密码错误 → 页面完全零反馈**（既无 toast 也无内联错误，光停留在登录页）。

**根因**

`src/app.tsx` 把 `AppApiRegistrar`（`AntdApp.useApp()` → `setAppApi` 注册全局
message/notification/modal）挂在了**已登录布局**的 `childrenRender` 里，而登录页是
`layout: false`，根本不经过布局渲染。两条丢消息的路径：

- 直接打开登录页：`getMessage()` 一直是 `undefined`，所有 `getMessage()?.xxx()`
  被可选链静默吞掉；
- 先访问 `/` 再被重定向：`/` 短暂挂载过布局里的 `<AntdApp>`，`setAppApi` 注册了一个
  holder 尚未挂载（或已随布局卸载）的实例且 effect 无清理；登录页拿着这个死实例调
  `message.success`，antd 检测到 `holderRef.current` 为空，告警**并静默丢弃**该条消息。

另外登录页的内联错误兜底也是死代码：`const [userLoginState] = useState({})` 没有
setter，`status === 'error'` 的 Alert 分支永远不会成立——即 toast 是失败反馈的唯一
通道，丢了就是零反馈。

**修复**

- `src/app.tsx`：`AppApiRegistrar` 提升到模块级，新增 umi 运行时
  `rootContainer` 导出，用 `<AntdApp>` 包裹**整个应用**（含 `layout: false` 的
  登录页）；`childrenRender` 不再重复包裹（`ScopeMenuRefresher`/`SettingDrawer`
  留在原地，它们依赖 layout 运行时闭包）。
- effect 增加对称清理：`utils/antdApp.ts` 新增 `clearAppApi(api)`（仅当仍指向同一
  实例时清空，不误清后来者），卸载 `<AntdApp>` 时不再残留死实例。
- 顺带删除 `userLoginState` 死状态（`const [userLoginState] = useState({})` 与其
  永假的 `status === 'error'` 分支）。——若后续没动这段可忽略本条。

**回归测试**

- `web/tests/appAntdRoot.test.ts` 4 条：rootContainer 存在且 `<AntdApp>` +
  `AppApiRegistrar` 包裹 `{container}`、registrar effect 含 `setAppApi(inst)` 与
  `clearAppApi(inst)` 清理、childrenRender 不再渲染 `<AntdApp>`、antdApp 模块导出
  `clearAppApi`（防「搬回 childrenRender」/「删掉清理」两类回归）。
- `web/src/utils/antdApp.test.ts` 新增 3 条 `clearAppApi` 语义用例（清当前实例、
  不误清后来者、未注册 no-op）。
- 运行时验证（Playwright，修复后）：三条路径 toast 分别为「登录成功！」/
  「登录成功！」/「登录失败，请重试！」，`notice in render` 告警 0 条。

---

## BUG-023 公告页 40 个词条只登记了 26 个，双语目录缺 14 条 + 菜单键缺失

**严重度**：中（英文界面回落中文、console 刷 Missing message）

**现象**

`/admin/announcements` 页面在 en-US 语言下，表单标签/占位/校验提示/行操作按钮
（编辑/删除）/「公告已更新」提示全部显示**中文 defaultMessage**；console 每次渲染刷
`[React Intl] Missing message: "pages.announcements.field.*"`。侧边栏菜单该条目在
zh-CN/en-US 都没有 `menu.AccessControl.Announcements` 词条，每渲染一次刷一遍
（一次走查累计 61 条）。

**根因**

2c236aa 新增公告页时，页面引用 40 个 `pages.announcements.*` id，locale 目录只登记
了 26 个——漏掉的全部是**弹窗表单**（title/content/audience/role/popup/active/range
的标签与校验提示）、行操作（edit/delete）和 update.success，即「不点开新建/编辑就
看不到」的那批；走查脚本当时也只覆盖到列表层。菜单键则是 routes 的 `name` 加了、
两个 `menu.ts` 都没加。

**修复**

- `src/locales/zh-CN/pages.ts` / `en-US/pages.ts`：补齐 14 条
  （field.title/.placeholder/.required、field.content/.required、field.audience、
  field.role/.required、field.popup、field.active、field.range、action.edit、
  action.delete、update.success），值与页面 defaultMessage 语义一致。
- `src/locales/zh-CN/menu.ts` / `en-US/menu.ts`：补
  `menu.AccessControl.Announcements`（系统公告 / Announcements）。

**回归测试**

`web/tests/announcementsLocales.test.ts` 5 条：从**页面源码**提取全部
`pages.announcements.*` id 引用（≥40，防守卫自身空转），断言 zh-CN/en-US 目录都
存在；单独断言双语菜单键。新增词条时自动跟随，再出现「代码引用了未登记的 id」
即失败。变异验证：stash 掉本次 locale 改动后 4/5 转红。

修复后全站走查：console error/warn 0 条（修复前同走查 89 条）。

---

## 审核模块假数据验证（BUG-024 / 025 背景）

「跳转到审核人的信息都是错的」此前一直没有系统性假数据来复现。本轮用仓库已有的
dev 播种惯例（`internal/svc/approval_seed.go`，历史提交 2490e1e 引入）扩出一套
可重复的审核假数据，并沿「列表页显示的审核人 → 跳转路由/参数 → 目标页查询 →
后端过滤 → 展示字段」全链路逐层验证。

### 假数据种子（dev 模式自动播种，一条命令重建）

- **位置**：`internal/svc/approval_seed.go`（`seedDemoApprovals`，在
  `service_context.go` 启动序列中 `seedBootstrapGames` 之后调用）。
- **重建方式**：重启 dev server 即重建——审批库在 dev 配置下是
  `approvals.NewMemStore()`（`service_context.go`），内存态、不落盘，每次启动
  从种子全量重建，天然幂等，无需清理命令。
- **生产隔离**：`isDevelopmentConfig` 门禁（mode 为 dev/development/debug，或
  `CROUPIER_ENV=dev`；仅显式 production 才关闭），生产路径不执行任何播种。
- **覆盖面**（单 scope 8 条：4 approved + 2 rejected + 2 pending）：
  - 审批人 **3 个不同账号**：admin（3 条）/ reviewer01（2 条）/ reviewer02（1 条）；
  - 申请人 operator / gm01 与审批人**全部错开**（跨角色组合：运营申请—管理员复核、
    策划申请—安全复核、运营申请—安全复核）；
  - 三态齐全，rejected 均带拒绝理由；1 条 approved 带
    `metadata.delegatedFrom=reviewer02` 表达**转审**场景；
  - 演示账号：admin / operator / gm01 / reviewer01 / reviewer02
    （登录口令同 bootstrap，admin/admin123）。
- **锁种子的测试**：`internal/svc/approval_seed_test.go`
  `TestSeedDemoApprovalsInto_StatesAndTwoPerson` 断言三态计数、审批人覆盖
  `{admin, reviewer01, reviewer02}`、审批人≠申请人、恰好 1 条转审记录。

### 复现与判定（本地栈实测）

```bash
go build -o bin/croupier-server ./cmd/server
(bin/croupier-server --config configs/server-sqlite.yaml &)
# 登录后（X-Game-ID: default / X-Env: dev）：
# GET /api/v1/approvals/                → 8 条种子（三态 × 3 审批人）
# GET /api/v1/audit?kind=approval_approve&actor=<审批人>   → 跳转目标
```

- **修复前**：`kind=approval_approve` 任何 actor 都查不到（total=0）——
  BUG-024 现场；前端遇 approver 缺失还会拿申请人去查（BUG-025）。
- **修复后**：admin 批准一条 pending 后，`actor=admin&kind=approval_approve`
  恰 1 行（userId=审批人、gameId/env 落列、metadata 同时可读申请人
  actor=operator 与 operator=admin）；`actor=<申请人>&kind=approval_approve`
  恒 0 行。续跑失败（无 live agent）时记录仍 approved、审计仍落链，
  「审批事实」与「续跑结果」分离可判。

---

## BUG-024 审批动作从不写审计链 + 通知把申请人当审批人

**严重度**：高（用户点名「跳转到审核人的信息都是错的」的后端根因）

**现象**

审批列表/详情里点「查看审计（批准）」跳到
`/admin/operation-logs?actor=<审批人>&kind=approval_approve`，页面永远
「暂无数据」；任意 actor、任意时间都一样。审批通知文案写着
「已由 operator 通过」——operator 是**申请人**，不是审批人。

**根因**

两层叠加：

1. `internal/audit/audit.go` 定义了 `EventApprovalApproved/Rejected`
   （approval.approved / approval.rejected），但**全仓库没有任何写入点**——
   审批通过/拒绝只写扩展事件流（`recordApprovalEvent` → approvals 扩展表）和
   通知，`audit_records` 里从没有对应行。operation-logs 的 kind 过滤
   （kindAliases: approval_approve → approval.approved）于是永远空集。
2. 通知文案拼的是 `record.Actor`（申请人）而非 `record.Approver`
   （审批人），消息中心/站内信里审批归属张冠李戴。

**修复**

- `internal/api/approval/service.go`：`Approve`/`Reject` 在 store 落终态后
  **先于续跑**调用新增的 `recordApprovalAudit`，把 approval.approved/rejected
  写入哈希链审计：actor_id=审批人、game_id/env 随记录落列、details 同时带
  申请人（actor）与审批人（operator）及 functionId/reason。放在续跑之前是
  刻意的：续跑失败也是「已批准」这一事实，审计必须先落。写失败只吞掉
  （审计是旁路，不阻塞审批主流程）。注意 `WithResourceID` 会整体替换
  Resource，必须先于 `WithGameID` 调用，否则 scope 列被抹掉。
- `internal/api/approval/service.go` + `notify.go`：通知文案改用
  `record.Approver`，事件 Data 补 `approver` 字段。

**回归测试**

`internal/api/approval/audit_chain_test.go` 3 条：批准/拒绝各断言
「按审批人 + 事件类型 + scope 过滤恰 1 行、details.actor=申请人、
details.operator=审批人」，并反向断言**申请人名下不得出现
approval.approved 行**（正是修复前会产生的错误数据形态）；第三条断言
AuditService 缺席时审批主流程不受影响。线上栈复证：修复前基线 total=0，
批准后 `actor=admin&kind=approval_approve` total=1。

**已知边界（未修，属既有行为）**：续跑失败时 Approve 接口对前端返回
`service_unavailable` 错误，但记录已 approved、审计已落链——审批人会看到
报错提示，刷新列表才发现状态已通过。记录的 reason 字段保留
「approved but continuation failed: …」全文。改动该语义需要前端/SDK 同步
调整提示策略，本轮仅记录不修。

---

## BUG-025 前端审核人跳转回退到申请人（approver 缺失时）

**严重度**：高（用户点名「跳转到审核人的信息都是错的」的前端根因）

**现象**

审批详情里「查看审计（批准）/（拒绝）」按钮的 actor 参数用
`current.approver || current.actor` 兜底——只要记录缺 approver（历史数据、
导入数据、异常态），跳转就把**申请人**当成过滤条件，去 operation-logs 查
申请人名下的 approval_approve。正确情况下申请人名下根本没有这种行（审批是
别人做的），查出来要么空、要么是此人自己审批**别人**申请的无关记录——
无论哪种都是「跳过去人不对」。

**根因**

`web/src/pages/Approvals/index.tsx` 两处审计跳转按钮把「字段可能缺失」
错误地处理成「换个人查」。语义上批准/拒绝动作只有审批人一个主体，
缺 approver 时正确行为是**空串过滤**（明确查无），绝不能回退到申请人。
（「查看审计（申请人）」按钮本来就是 actor=current.actor，不受影响。）

**修复**

两处按钮改为 `current.approver || ''`，并注释禁止回退。

**回归测试**

`web/src/pages/Approvals/index.test.tsx` 新增「approver 缺失但 actor 存在」
用例：断言跳转为 `actor=&kind=approval_approve`，且**不得**出现
`actor=gm01&kind=approval_approve`（修复前行为即后者）。与既有的
「approver/actor 均为空按空串跳转」两条互补，覆盖全部分支。

**已知边界（记录不修）**：转审（委托审批）目前只有平台层实现
（`internal/platform/approvals/delegation.go` 的 DelegationService，
未接线到任何 API handler），审批 DTO 也不透出 metadata，前端无从展示
`delegatedFrom`。种子里用 `metadata.delegatedFrom` 表达的转审记录，
在 API/界面侧暂不可见——待转审 API 落地后再接展示。

---

## BUG-026 站内信「发送」无权限校验：任何登录用户可给任意账号投递消息

**严重度**：高（越权写：收件人无法分辨来源，可被用来伪造系统通知）

**现象与背景**

「通知」与「发通知」必须区分：收件（我收到的通知）人人可见；发送是后台
运营能力，必须独立入口 + 权限门禁（用户反馈原文：「发送信息应该单独的页面，
有些人没有权限就不应该看到，而信息通知每个人都可以看到」）。

UI 侧 BUG-021 已完成分离（个人中心消息 Tab 只剩收件箱 + 渠道偏好，jest 有
「页面里不存在任何发送消息/广播控件」守卫；发送唯一入口是 admin-only 的
`/admin/announcements`，路由 `access: 'canAdmin'`、侧边栏菜单由后端
`menu.AccessibleTree` 按权限过滤）。但 API 侧漏了一个洞：
`POST /api/v1/messages`（点对点单发）的 handler **没有任何权限校验**，
任何登录用户都能调用——群发 `POST /api/v1/messages/broadcast` 有
`isBroadcaster`（admin 角色）校验，单发反而没有。

**根因**

`internal/api/message/handler.go` 的 `Send` 直接绑定请求体落库，未校验
当前用户角色；且 `model.Message` 无发送者字段，消息来源完全不可追溯
（谁发的、是不是本人发的都无法判定）。

**修复**

`Send` 复用与 `Broadcast` 相同的 `isBroadcaster` 门禁（仅 admin 角色可发送，
含点对点单发）；收件侧（List/Detail/Read/UnreadCount/Stream）保持所有登录
用户可用。前端 `services/api/messages.ts` 的 `sendMessage/broadcastMessage`
无任何页面调用（发送 UI 唯一入口是公告管理页），保留为 admin API 客户端，
非管理员调用现在会被后端 403 兜底。既有消息测试中「无登录态
handler.Send 做种子/主流程」的用例按新契约修正：收件侧用例改模型层直种
（`seedMessage`），发送主流程/校验用例改管理员登录态
（`newMessageHandlerWithAdmin` + `withReqUsername`）。

**回归测试**

`internal/api/message/send_permission_test.go`：复用 broadcast 的
admin/ops 双账号夹具，断言未认证 403、ops 角色 403、admin 200 且消息落库、
ops 用户收件箱（GET /messages）仍 200 可读——收发两侧语义一次锁死。

**已知边界（记录不修）**：`messages` 表没有发送者（from）列，站内信不记录
来源账号；补该列属于模型变更，需按迁移契约走编号迁移并设计展示层，另行立项。
当前所有合法发送路径（公告/群发/系统通知）都是管理员或系统行为，风险可控。

---

## 汇总

| BUG | 位置 | 状态 | 回归测试 |
| --- | --- | --- | --- |
| 001 | `internal/server` 测试抢占 `:19090` | 已修 | 3 条 Go 用例 |
| 002 | 引导管理员档案字段被丢弃 | 已修 | 5 条 Go 用例 |
| 003 | LB 归属率仪表盘不显示 | 已修 | 7 条 jest |
| 004 | 排序标签漏传 intl values | 已修 | 1 条 jest（判别力已验证） |
| 005 | antd 6 废弃属性 142 处 | 已修 | 4 条 jest（判别力已验证） |
| 006 | 组合页编辑器缺 locale key | 已修 | 随 `consoleMenu` 相关用例覆盖 |
| 007 | 空头像串触发多余请求 | 已修 | 5 条 jest |
| 008 | 表单实例未连接 | 已修 | 4 条 jest（判别力已验证） |
| 009 | `Space.Compact` 迁移拖慢热路径编辑器 | 已修 | 1 条 jest（判别力已验证） |
| 010 | `app.cancel` 未登记 locale key | 已修 | 随控制台审计守护 |
| 011 | 审计日志两页 rowKey 全同 | 已修 | 8 条 jest（判别力已验证） |
| 012 | 头像数据互清/死链/404/顶栏恒占位 | 已修 | Go 2 套 + jest 11 条 + 实测全链路 |
| 013 | MFA 绑定不可用 + 无恢复码兜底 | 已修 | RFC 向量 + Go 14 条 + jest 8 条 |
| 014 | 健康分数衰减用例依赖墙钟 | 已修 | 改写为轮询 + 离散不变量 |
| 015 | antd 6 整体废弃 `List` 组件（组件级守卫盲区） | 已修 | jest 8 条 + 守卫新用例 + 走查复证 |
| 016 | 安全中心「登录通知」假开关（假状态 + 假交互） | 已修 | Go 10 条 + jest 14 条 |
| 017 | 飞书密钥漏登记 secretKeys，掩码回存覆盖真值 | 已修 | `IsSecretKey` 断言（修复前红） |
| 018 | 游戏访问权限恒为空（admin 看到空白） | 已修 | Go 2 条 + jest 5 条 |
| 019 | 权限概览是编造数据（resource="role"／角色名混入） | 已修 | Go 21 条（含改写 4 条固化旧错的用例） |
| 020 | 权限概览单层平铺、无授权两态 | 已修 | jest 23 条 + 变异验证 7 条转红 |
| 021 | 个人中心挂广播入口 + 公告无入口 | 已修 | jest 23 条 |
| 022 | 登录页无 antd App 上下文，登录提示全部丢失 | 已修 | 布线守卫 4 条 + antdApp 3 条 + Playwright 三路径复证 |
| 023 | 公告页双语词条缺 14 条 + 菜单键缺失 | 已修 | locale 覆盖守卫 5 条（stash 变异 4/5 转红） |
| 024 | 审批动作从不写审计链 + 通知把申请人当审批人 | 已修 | Go 3 条（audit_chain_test，含申请人名下恒空反断言）+ 线上栈复证 |
| 025 | 审核人跳转回退到申请人（approver 缺失时） | 已修 | jest 1 条（修复前 actor=申请人 转红）|
| 026 | 站内信「发送」无权限校验（人人可发任意账号） | 已修 | Go 1 条（未认证/ops 403、admin 200、收件侧不受影响）|

### 遗留 / 未修

- **`Input.addonBefore` 的 9 处结构改写**视觉上由 `addonBefore` 内置样式换成
  `Space.Compact`（6 处）或 `Input.prefix`（3 处）。`prefix` 把标签从输入框**外侧**
  移到**内侧**，这两处的标签位置与改写前不同；已通过组件级单测锁定交互与取值，
  像素级差异未做视觉回归。
- ~~**BUG-002 的存量数据**~~（已验证关闭）：重建二进制并重启 server 后，
  `seedBootstrapAdmins` 自动回填，`GET /api/v1/profile` 对 `admin` 返回
  `nickname: "系统管理员"`、`email: "admin@croupier.local"`。
- **`Space.Compact` 的宽度行为**：`Space.Compact` 宽度由内容撑开，
  `Analytics/Behavior` 的 `PathControls` 与 `OpenAPISources` 的两个弹窗宽度可能与
  改写前有细微差异。已通过组件级单测锁定交互与取值；像素级差异未做视觉回归。
- **`@ant-design/charts` 的 `Line`** 仍在 `LBMonitor` 中使用，本次只处理了
  `Gauge`。`Line` 未观察到丢值问题（`xField/yField/colorField` 走 plots 正常
  adaptor 路径），未改动。
- **审计覆盖的 52 条路由**按「含废弃属性的组件」+「此前完全没覆盖到的路由组」
  两条线索拼出（`config/routes.ts` 里 `/admin/*`、`/dev/*`、`/support/*`、
  `/system/*`、`/ops/*` 余下路径等），仍不是逐条穷举；动态路由（如 `/functions/:id`、
  `/support/tickets/:id`、`/console/:categoryKey`）需要具体 id，未纳入。
  守卫用例（BUG-005 第 2 条）是**全量静态扫描** `src/**/*.tsx`，不依赖路由清单；
  控制台审计是补充性的运行时证据。
