# Croupier Web 界面 impeccable 审核(2026-10-31)

日期：2026-10-31。范围：`web/`(Umi/max + ProComponents,`pnpm dev` @ :8000,后端 @ :18780,凭据 admin/admin123)。目标：对 Dashboard 全部主要页面做 UI audit/critique(层级/排版/对比度 a11y/动效/响应式/边界态/UX 文案),坏损当场最小修复。方法：真实渲染逐页截图(桌面 1440×900 + 窄屏 390×844,登录后遍历主要路由),程序化 WCAG 对比度采样(全文本节点)+ 键盘 Tab 探针 + 控制台错误采集,对照 `docs/design.md`。

审核时源码 HEAD：`fc6bf843`。本轮不 bump 版本、不动品牌色/logo、不改视觉方向。审核期间对 `web/` 内 2 个源文件做了最小修复(见「已修复」),各一 commit;其余问题列「待办」。

## 整体 critique(不留情面)

**这届 Dashboard 的骨架是体面的,但「国际化」这一层在英语界面上是裸奔的。** 布局、空态、边界态、错误页普遍做得讲究——函数目录的能力分层面板、资源目录的无数据空态、Page Studio 的三段步骤指引、运行控制台的菜单卡片、404 的警示卡,层级清楚、留白克制、空态有引导,这些是真功夫,不该被低估。

但把它当「英语产品」看就露馅了:

1. **两块完整页面整页说中文。** `Games`(游戏管理)整页、`Site Settings`(系统设置)大部分表单都是中文,而它们的上级菜单、面包屑、全局壳全是英文。同屏中英混杂,用户会以为产品坏了。根因是 `pages.gamesManage.*` 全部 10 个键、`pages.systemSiteSettings.*` 一子集、`pages.gamesEnvs.action.manageGames`、`pages.functionsDirectory.empty.*`、`pages.pageStudio.*` 若干键在 **en-US 语言包缺失**,运行时回退到代码里的中文 `defaultMessage`(控制台逐条 `[React Intl] Missing message`)。这不是审美偏好,是交付缺陷:英语用户看到的不是英文。
2. **登录页把模板残留当产品文案。** 密码框占位符是 `Password: ant.design`——antd 官方模板的默认占位,不是本产品密码(实际演示密码是 `admin123`)。副标题 `Croupier 是面向游戏运营的函数调度与审计平台` 写死在 `config/branding.ts`,任何 locale 都显示中文。
3. **对比度踩线。** 全站文本对比度总体优秀(标题 17.8:1、正文达标),唯一失败点是面包屑链接 `#1677ff` 对白底 **4.12:1**,低于 WCAG AA 常规文本 4.5:1。这是刻意选的品牌蓝,所以不建议全局改色,但面包屑这类交互文本值得单独用一个更深的可访问蓝。
4. **窄屏有两处真破碎。** 资源目录的信息提示条在 ≤480px 把描述挤成「一词一行」的竖排;页标题在窄屏截成 `Fun...`/`Res...`(标题与副标题挤同一行)。

结论:**骨架可给 B+,作为英语交付面只能给 C。** 本轮按任务书只把「布局破碎 + 明显文案错误」当场修掉(2 处),i18n 缺口与对比度是更大的工程,列待办并给定位与改法,不在此轮擅自动品牌与语言包结构。

## 方法说明

- 截图证据在 `evidence/ui-audit-2026-10-31/`;桌面与窄屏均逐页截图,报告引用关键图。
- 对比度采样:对所有可见文本节点,向上回溯至首个不透明背景,按 WCAG 相对亮度算比值,阈值 AA 4.5(常规)/3.0(大文本)。
- 键盘探针:登录页与资源目录逐 Tab 遍历,检查焦点可达与可见焦点指示(antd 用边框/阴影作焦点,非 outline,探针已考虑)。
- 控制台:仅捕获 `console.error`/`pageerror`/HTTP ≥400,dev 首次编译的 chunk 延迟不视为缺陷(Approvals、Menus 页在首次导航时内容区短暂空白,属 dev 慢编译,非 bug,已排除)。

## 已修复(坏损,当场最小修复)

### F1 · P1 · 布局破碎:资源目录信息条在窄屏被挤成「一词一行」

- **现象**:`Resource Catalog` 页顶部 info Alert,描述文案在 ≤480px 被右侧「Open Page Studio」按钮挤进约 40px 宽的一列,整段文字逐词竖排,不可读。桌面正常。
- **定位**:`web/src/pages/ResourceCatalog/index.tsx:581-608`。原实现把按钮放进 antd `Alert` 的 `action` 插槽——`action` 固定右侧且不换行,描述被压缩。
- **修复**:把描述与按钮并入 `description`,用可换行 `flex`(`flex:1 1 220px; minWidth:0` + `flexWrap:wrap`),窄屏按钮自动落到文字下方,桌面保持右侧布局不变。**已提交**(见 commit 1)。
- **验证**:`resource-catalog-narrow-after.png`(窄屏文字正常换行)、桌面版式不变(见 diff 后截图)。

### F2 · P1 · 文案错误:登录密码占位符为 `Password: ant.design`

- **现象**:登录页密码框占位符是 `Password: ant.design`,antd 官方模板残留,与本产品无关(演示密码是 `admin123`)。
- **定位**:`web/src/locales/en-US/pages.ts:43`(`'pages.login.password.placeholder'`)。zh-CN 为 `密码: admin`,en-US 漏改。
- **修复**:改为 `'Password'`。**已提交**(见 commit 2)。
- **验证**:`login-placeholder-before.png`(修复前,占位符含 `ant.design`)→ `login-placeholder-after.png`(修复后,`Password`)。用户名框占位符 `Username: admin or user` 保留(演示环境提示,属有意)。

## 修复波(2026-10-31 二轮,一问题一 commit)

按用户令对「待办」逐项修复;a11y/对比度优先。全部英文 commit、无 AI 痕迹、不动品牌色/不 bump 版本。验证:`pnpm --dir web run tsc` 通过;`max build` 通过;jest 全量并行跑出现 18 例失败(均为 20s 超时的异步用例,19 worker 满载所致,涉及套件与本波改动无关),隔离重跑本波涉及页面 12 套件(含此前超时的 `templateUsage`/`report-page-editor`)**168/168 全绿**,确认非本波回归。

| 项  | 问题                          | commit     |
| --- | ----------------------------- | ---------- |
| F3  | Games 管理 en-US 文案(33 键)  | `f9a0eaa8` |
| F4  | Site Settings en-US 缺失 4 键 | `5de921a2` |
| F6  | Game Environments 管理游戏键  | `d259c677` |
| F7  | Page Studio/函数目录 9 键     | `4dd84188` |
| F8  | 面包屑链接对比度 → AA         | `78592a76` |
| F9  | 页标题窄屏截断 → 竖排         | `0f81aa1c` |

本波已修 6 项;F5(登录副标题)待用户提供真实英文品牌文案;F10(SettingDrawer)复核为 dev-only,非交付缺陷,不修。

## 待办(按严重度;已修复项标注 commit,见「修复波」)

### F3 · P2 · i18n 缺口:Games 管理整页在英语界面显示中文 — ✅ 已修复

- **现象**:`/system/games` 页标题「游戏列表」、表头「图标/游戏标识/显示名称/环境数/状态/操作」、按钮「新增游戏/编辑/删除」全为中文,而菜单、面包屑、全局壳为英文。
- **定位**:`web/src/pages/GamesManage/index.tsx` 全部使用 `pages.gamesManage.*` 键(共 **33** 个,而非报告初估 10 个),**en-US 语言包无任何 `pages.gamesManage.*` 条目**(en-US 无 gamesManage 字典文件),运行时全部回退中文 `defaultMessage`。
- **修复**:新建 `web/src/locales/en-US/gamesManage.ts`(33 键英文文案)并接入 `en-US.ts`。**已提交** `f9a0eaa8`。

### F4 · P2 · i18n 缺口:Site Settings 表单在英语界面部分中文 — ✅ 已修复

- **现象**:`/system/site` 的「System Info」页签标题「系统设置」、表单「站点名称/站点 Logo 地址/保存」等为中文,页签名(Theme/Feature Flags/Authentication…)与说明为英文。
- **定位**:`web/src/pages/System/SiteSettings/` 用 `pages.systemSiteSettings.*` 键,`en-US/systemSiteSettings.ts` 比 zh-CN 少 4 条(`auth.saved.github`/`auth.saved.local`/`auth.successUrlLabel`/`auth.successUrlTooltip`),缺失键回退中文。
- **修复**:补齐上述 4 条 en-US 文案。**已提交** `5de921a2`。

### F5 · P2 · i18n 缺口:登录副标题任何 locale 都显示中文 — ⏸ 待用户

- **现象**:登录页副标题 `Croupier 是面向游戏运营的函数调度与审计平台`。
- **定位**:`web/src/config/branding.ts:8`(`BRAND.subTitle` 写死中文),`web/src/pages/User/Login/index.tsx:349-352` 优先取 `BRAND.subTitle`。
- **建议**:副标题改为按 locale 取(`pages.layouts.userLayout.title` 之类),`BRAND.subTitle` 仅作为 zh-CN 缺省;或拆成 `subTitleZh`/`subTitleEn`。
- **待用户原因**:en-US 的 `pages.layouts.userLayout.title` 目前是 antd 模板残留占位文案(`Ant Design is the most influential web design specification in Xihu district`),并非本产品真实 tagline;正确修复需要一条**真实的英文品牌副标题**(需用户提供/拍板)。未擅造品牌文案。

### F6 · P2 · i18n 缺口:Game Environments「管理游戏」按钮中文 — ✅ 已修复

- **现象**:`/system/environments` 页头「管理游戏」按钮中文,同排「Add Env」英文。
- **定位**:`web/src/pages/GamesEnvs/index.tsx:234`(`id="pages.gamesEnvs.action.manageGames"`),**en-US/gamesEnvs.ts 无该键**(仅有 `.add`/`.addGame`),回退中文。
- **修复**:补 `'pages.gamesEnvs.action.manageGames': 'Manage Games'`。**已提交** `d259c677`。

### F7 · P2 · i18n 缺口:函数目录空态 / Page Studio 筛选与状态缺 en-US 键 — ✅ 已修复

- **现象**:控制台逐条报 `[React Intl] Missing message: "pages.functionsDirectory.empty.scopeTitle" / "...scopeDescription"`(函数目录空态)与 `"pages.pageStudio.filter.keyword" / ".filter.status" / ".status.draft|published|archived" / ".filter.resource" / ".studio.column.resources"`(Page Studio 筛选/状态/列)。en-US 下这些位置回退中文默认文案。
- **定位**:`web/src/pages/Functions/Directory/index.tsx:437`、`web/src/pages/PageStudio/index.tsx:1060+`;对应 en-US 语言包缺键。
- **修复**:补上述 9 键 en-US 文案(空态标题/描述含 `{game}`/`{env}` 占位、筛选占位、状态标签、资源列)。**已提交** `4dd84188`。

### F8 · P3 · 对比度:面包屑链接低于 WCAG AA — ✅ 已修复

- **现象**:面包屑中间层链接(如「Functions & Pages」)`#1677ff` 对白底 **4.12:1**,低于 AA 常规文本 4.5:1(阈值差 0.38)。
- **定位**:`web/src/global.less:157-162`(`.ant-breadcrumb a { color: var(--brand-2) }`,--brand-2=#1677ff)。
- **修复**:仅对面包屑链接改用更深蓝 `var(--vp-c-brand-2)`(默认亮主题 = `#0958d9`,5.57:1),hover 加下划线保留反馈;**未动全局品牌蓝**(`#1677ff` 是近期刻意恢复的默认主题,见 `8278a5ba`)。**已提交** `78592a76`。

### F9 · P3 · 响应式:页标题在窄屏截断 — ✅ 已修复

- **现象**:窄屏(390px)页面标题被截为 `Fun...`/`Res...`,标题与副标题挤同一行且各自省略。
- **定位**:ProLayout `PageContainer`/`ProPageHeader` 标题+副标题单行布局(`.ant-page-header-heading` 内 `-left` flex 行,`-title`/`-sub-title` 均带 `text-overflow: ellipsis`),`web/src/pages/Functions/Directory`、`ResourceCatalog` 等页头。
- **修复**:`web/src/global.less` 的 `@media (max-width: 768px)` 内,令页头 heading `flex-wrap`、`-left` 改纵向堆叠、`-title`/`-sub-title` 允许换行(标题独占一行、副标题换行)。**已提交** `0f81aa1c`。

### F10 · P3 · 仅 dev 现象:浮动设置齿轮在窄屏压内容 — ⏭ 不修(非缺陷)

- **现象**:`SettingDrawer` 的浮动齿轮按钮(右缘)在窄屏覆盖卡片标签/正文(如 Console「5 menus」标签、Dashboard 标题),桌面亦压住卡片右缘。
- **定位**:`web/src/app.tsx:322`(`<SettingDrawer>`),仅 `isDev` 渲染。
- **结论**:**不进生产**(`isDev` 门控),属开发期便利工具,不计入交付缺陷。本轮复核确认仍为 dev-only,不处理。

## 通过项(审核确认无问题,简述)

- **层级/排版**:函数目录、资源目录、Page Studio、运行控制台、Game Environments、Invoke Workbench、Approvals、菜单管理均层级清晰、间距一致、信息密度合理;404 错误页、空态、加载骨架、无数据引导均完善。
- **对比度(除 F8)**:正文/标题/按钮/表头均 ≥AA,标题 17.8:1,无第二处失败。
- **键盘可达**:登录与数据页 Tab 遍历正常,焦点用边框/阴影呈现,无焦点丢失或陷阱;交互元素均可达。
- **动效**:未发现过度/干扰性动画;图表与卡片动效克制。
- **空态/加载/错误态**:资源目录/OpenAPI/Console/Page Studio 空态文案明确并带引导;菜单页有骨架屏;404 有返回首页按钮。

## 汇总

| 仓       | 报告路径                                        | 问题数 | 已修数 | 待用户 | commit |
| -------- | ----------------------------------------------- | ------ | ------ | ------ | ------ |
| croupier | `docs/review/2026-10-31-ui-audit-impeccable.md` | 10     | 8      | 1      | 见下方 |

提交(均为英文、无 AI 痕迹):

1. `fix(ui): wrap resource catalog alert description on narrow screens` — F1
2. `fix(ui): correct login password placeholder text` — F2
3. `fix(ui): add en-US translations for games manage pages` — F3
4. `fix(ui): add missing en-US site settings auth keys` — F4
5. `fix(ui): add en-US manage games action key` — F6
6. `fix(ui): add en-US page studio and function directory keys` — F7
7. `fix(ui): raise breadcrumb link contrast to wcag aa` — F8
8. `fix(ui): stack page header title on narrow screens` — F9
