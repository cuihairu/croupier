---
title: 插件架构调研——Grafana / VS Code / VitePress / WordPress（Provider 插件设计前置）
---

# 插件架构调研：Grafana / VS Code / VitePress / WordPress

## 状态

- 状态: 调研归档（2026-10-08，Provider 插件设计前置调研之一）
- 日期: 2026-10-08
- 范围: 四个成熟系统的插件机制，统一按六问口径核对官方文档——插件类型清单、发现/分发、契约、加载方式、沙箱/隔离、权限面、版本兼容
- 结论: 契约强度与运行时隔离基本成正比；强契约插件平台的标配三角是「分发可信（签名/审核）、能力声明（显式权限/兼容清单）、版本协商（semver 约束字段）」。Croupier 既有扩展体系（`internal/core/extension` + `official.<domain>` 统一模式）已具备能力声明与配置契约的雏形，缺版本协商与分发可信层；运行时加载与沙箱按现有形态（编译期内置 + HTTP 外联）暂无必要引入
- 关联: [商业游戏后台功能调研](./commercial-backend-survey.md)（同期批一产出）、[官方扩展统一模式](../architecture/official-extension-unified-pattern.md)、[扩展安装模型](../architecture/extension-installation-model.md)；Provider 插件设计文档在批二落 `docs/design/`

## 一、选例与口径

| 系统      | 选入理由                                                   |
| --------- | ---------------------------------------------------------- |
| Grafana   | 与 Croupier 同为管理后台形态，且有后端插件（进程外）先例   |
| VS Code   | 规模最大的插件生态，懒加载/扩展点/信任模型均有成熟设计     |
| WordPress | 普及度最高的 hook 式插件系统，弱契约强扩展点的另一极       |
| VitePress | 对照组——「无插件系统」：扩展即构建期代码组合，用于划定下界 |

每系统回答同一组问题：①有哪些插件类型；②插件怎么被发现/分发；③契约（manifest/扩展点）长什么样；④怎么加载；⑤沙箱/隔离与权限面；⑥版本兼容怎么声明。

## 二、Grafana

**类型**：`plugin.json` 的 `type` 允许 `app` / `datasource` / `panel` / `renderer` 四值；可选 `backend` 声明后端插件，`executable` 指定可执行文件名首段（按 `<executable>_<GOOS>_<GOARCH>` 命名，如 `plugin_linux_amd64`）。

**发现与安装**：Grafana 启动扫描 `[paths].plugins` 配置目录；用户可经 Plugin Catalog UI（Administration > Plugins and data > Plugins）、CLI、解压 ZIP 入插件目录、`plugins.preinstall` 预装（11.5+）或 Helm `GF_PLUGINS_PREINSTALL_SYNC` 安装。

**契约**：`plugin.json` 必填 `id`（有正则约束）、`type`、`name`、`info`（内含 `version`/`keywords`/`links`/`logos`/`updated`）、`dependencies`；可选 `preload`、`state`、`roles[]` 等。

**加载与沙箱**：后端插件由 Grafana 以子进程启动、经 gRPC 通信（基于 HashiCorp Go Plugin System over RPC），官方明言插件崩溃不会拖垮主进程（"Plugins can't crash your Grafana process"）。前端 Plugin Frontend Sandbox（11.5 public preview）把插件运行在独立 JavaScript context，经 `security.enable_frontend_sandbox_for_plugins` 启用；AngularJS 插件与 Grafana Labs 自签插件被排除在外，AngularJS 支持在 v11 默认关闭、下个大版本移除。

**签名**：启动时验证插件目录内所有插件签名，未签名插件不加载不启动（"If a plugin is unsigned, then Grafana neither loads nor starts it"）。`sign-plugin` 在 `dist/` 生成 `MANIFEST.txt`（元数据 + 文件 SHA256 校验和 + 私钥签名），Grafana 用内置公钥验证；分发级别分 Private / Community / Commercial，豁免走 `allow_loading_unsigned_plugins` 或开发模式。

**版本兼容**：`dependencies.grafanaDependency` 按 node-semver 校验，支持 `>=10.4.0 || 11.x` 式区间；旧字段 `grafanaVersion` 已废弃。

来源：<https://grafana.com/developers/plugin-tools/> · [plugin.json 参考](https://grafana.com/developers/plugin-tools/reference/plugin-json) · [后端插件](https://grafana.com/developers/plugin-tools/key-concepts/backend-plugins) · [插件安装](https://grafana.com/docs/grafana/latest/administration/plugin-management/plugin-install/) · [插件签名](https://grafana.com/docs/grafana/latest/administration/plugin-management/plugin-sign/) · [前端沙箱](https://grafana.com/docs/grafana/latest/administration/plugin-management/plugin-frontend-sandbox/) · [v11 breaking changes](https://grafana.com/docs/grafana/latest/breaking-changes/breaking-changes-v11-0/)

## 三、VS Code

**契约与扩展点**：`package.json` 必填 `name`/`version`/`publisher`/`engines`，入口 `main`（Node）/`browser`（web）；`contributes` 定义 38 个扩展点（`commands`、`configuration`、`views`、`viewsContainers`、`languages`、`grammars`、`debuggers`、`themes`、`keybindings`、`menus`、`snippets`、`walkthroughs`、`chatAgents`、`languageModelTools` 等）。`extensionPack` 是联合安装（松散打包），`extensionDependencies` 是硬依赖。

**懒加载**：`activationEvents`（`onLanguage`/`onCommand`/`onView`/`onStartupFinished`/`workspaceContains`/`*`）触发时才调用一次 `activate()`；`*` 因拖慢启动被官方劝阻；1.74 起 `contributes` 贡献自动生成激活事件。

**进程模型**：同一 Extension Host 内所有扩展**共享一个进程**——某扩展导出的 API 对同 host 内全部扩展可见。真正的隔离边界是 host 本身：UI 扩展跑本地 host，Workspace 扩展跑远端 VS Code Server 内的 Remote Extension Host，跨 host 经 JSON 序列化 IPC。webview 是受控 `iframe`：「scripts in a webview do not have access to the VS Code API」，仅 `postMessage` 通信，强制 CSP（`default-src 'none'`）并用 `asWebviewUri`/`localResourceRoots` 约束资源。

**权限面**：Workspace Trust 下扩展须声明 `capabilities.untrustedWorkspaces`（`supported: true | false | 'limited'` + `restrictedConfigurations`），未声明者在受限模式默认禁用；`extensionKind: ui|workspace` 决定运行位置。

**版本兼容**：`engines.vscode` 约束兼容版本且不允许 `*`；proposed API 需 `enabledApiProposals` 且仅 Insiders 可用、禁止发布 Marketplace——stable API 才有兼容承诺。

来源：[扩展 manifest](https://code.visualstudio.com/api/references/extension-manifest) · [激活事件](https://code.visualstudio.com/api/references/activation-events) · [contributes 扩展点](https://code.visualstudio.com/api/references/contribution-points) · [Workspace Trust](https://code.visualstudio.com/api/extension-guides/workspace-trust) · [Remote 扩展](https://code.visualstudio.com/api/advanced-topics/remote-extensions) · [webview 指南](https://code.visualstudio.com/api/extension-guides/webview) · [proposed API](https://code.visualstudio.com/api/advanced-topics/using-proposed-api)

## 四、WordPress

**契约**：主文件头部注释即 manifest，15 个字段中仅 `Plugin Name` 必填，其余含 `Version`、`Requires at least`（最低 WP 版本）、`Requires PHP`、`Requires Plugins`（依赖声明，WP 6.5+）、`Update URI`、`Network`。官方目录分发另需 `readme.txt`：`Tested up to` 定义为「已成功测试过的最高版本」（软声明，非硬门槛）。

**扩展点**：hooks 是官方定义的插件机制基座（"the foundation for how plugins and themes interact with WordPress Core"）——action 执行副作用不返回值，filter 必须返回修改后的值，经 `add_action`/`do_action`/`add_filter`/`apply_filters` 注册。生命周期：`register_activation_hook`（建表/种子数据）、`register_deactivation_hook`（清缓存），卸载走根目录 `uninstall.php`（必须先检查 `WP_UNINSTALL_PLUGIN` 常量防直访）或 `register_uninstall_hook()`。

**权限与边界**：权限面是 roles/capabilities 体系（六个默认角色 Super Admin→Subscriber），`add_role`/`add_cap`/`current_user_can` 供插件自行门控。官方文档未定义任何沙箱、进程隔离或能力声明机制——插件与核心同处一个 PHP 运行时，信任边界就是「管理员能装什么」。

来源：[header 要求](https://developer.wordpress.org/plugins/plugin-basics/header-requirements/) · [hooks](https://developer.wordpress.org/plugins/plugin-basics/hooks/) · [激活/停用](https://developer.wordpress.org/plugins/plugin-basics/activation-deactivation-hooks/) · [卸载](https://developer.wordpress.org/plugins/plugin-basics/uninstall-methods/) · [roles 与 capabilities](https://developer.wordpress.org/plugins/users/roles-and-capabilities/) · [readme.txt 标准](https://wordpress.org/plugins/readme.txt)

## 五、VitePress（对照组：无插件系统）

VitePress 没有传统插件系统：自定义主题以 `.vitepress/theme/index.ts` 存在即生效，主题对象含 `Layout`（必填）、`enhanceApp({app, router, siteData})`（注册全局组件）、`setup()`、`extends`（继承基座主题，其钩子先于自身执行）；官方明言「默认导出即唯一契约」（"The default export is the only contract for a custom theme"），无 manifest、无签名、无沙箱。配置透传：`config.vite` 直接传原生 Vite 配置，`config.markdown` 配置底层 markdown-it 实例。扩展点即构建期代码组合，无运行时发现、权限或版本协商机制。

来源：[自定义主题](https://vitepress.dev/guide/custom-theme) · [站点配置](https://vitepress.dev/reference/site-config)

## 六、横向对比

| 维度      | Grafana                                         | VS Code                                                 | VitePress                  | WordPress                                                         |
| --------- | ----------------------------------------------- | ------------------------------------------------------- | -------------------------- | ----------------------------------------------------------------- |
| 发现/分发 | 目录扫描 + Catalog UI/CLI/preinstall            | Marketplace / VSIX 手动装                               | npm 包，无目录             | wordpress.org 目录 + SVN                                          |
| 契约      | `plugin.json`（id/type/info/dependencies 必填） | `package.json` + `contributes` 38 扩展点                | 无——默认导出即契约         | 文件头注释（仅 Plugin Name 必填）                                 |
| 加载      | 启动加载，签名不过不加载                        | 懒加载 `activationEvents`，贡献静态生成激活             | 构建期静态组合             | 全量加载，hook 回调按需触发                                       |
| 沙箱/隔离 | 后端子进程 + gRPC；前端独立 JS context          | host 内共享进程（隔离靠 host 边界）；webview=iframe+CSP | 无                         | 无，同 PHP 运行时                                                 |
| 权限      | 签名分级 + Admin 角色控制安装/配置              | Workspace Trust `untrustedWorkspaces` 声明              | 无                         | roles/capabilities + `current_user_can`                           |
| 版本兼容  | `grafanaDependency` semver 区间                 | `engines.vscode`（禁 `*`）；proposed/stable 双轨        | 无显式字段（npm 生态自管） | `Requires at least`/`Requires PHP` 硬检查 + `Tested up to` 软声明 |

## 七、结论与对 Croupier 的启示

### 7.1 四案例的规律

1. **契约强度与隔离强度成正比**。Grafana（强契约 + 双侧隔离）→ VS Code（强契约、host 内共享进程、iframe 补充隔离）→ WordPress（弱契约、同进程、hook 兜底）→ VitePress（无契约、构建期组合）。
2. **强契约平台的标配三角**：分发可信（Grafana 签名分级 / VS Code Marketplace 审核）、能力声明（`untrustedWorkspaces` 式显式声明 + `roles[]`）、版本协商（semver 约束字段，且都禁 `*` 或有双轨 API 策略）。
3. **hook 是最耐用的扩展点**。WordPress 的 action/filter 契约 20 年基本未变；VS Code 的 contributes 静态声明让扩展点可枚举、可审计。两者共同点：宿主对扩展的调用面收窄为一个稳定的注册函数。
4. **进程外隔离是重量级选项**。只有 Grafana 走到子进程 + gRPC（并明确宣传崩溃隔离）；VS Code 明确不做扩展级进程隔离。进程外化带来分发/调试/兼容成本，适合第三方不受信代码，内部可信代码用不上。

### 7.2 Croupier 现状对照（六问口径）

| 六问     | 现状                                                                                                                                                                                                                                                                                                                                                  |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 类型     | 三处 provider 抽象并存：`internal/platform/provider`（第三方平台集成，Name/Init/SupportedMethods/Call 契约）、`internal/cicd`（CI/CD，Factory 注册表）、`internal/security/identity`（PasswordProvider/OAuthProvider）；另有 `internal/core/extension` 六子包（manifest/catalog/installation/runtime/sync/externalfunc）承载 `official.<domain>` 扩展 |
| 发现     | 全部编译期内置：Go `init()` 自注册（cicd）或 import + Registry（platform/provider）；扩展经 installation 落库 + runtime binding 挂载                                                                                                                                                                                                                  |
| 契约     | 扩展已有 manifest + 统一模式（`official.<domain>`、`<domain>.read/operate/admin` 三层 capability、config/secrets 拆分 schema 声明）；三处 provider 接口无 manifest，契约=Go 接口方法集                                                                                                                                                                |
| 加载     | 进程内直接调用；扩展 externalfunc 走 dispatcher（`external.<platform>.<method>`）                                                                                                                                                                                                                                                                     |
| 沙箱     | 无插件级隔离；出站 HTTP 走 secguard 守卫客户端（SSRF 防护），是最实际的安全边界                                                                                                                                                                                                                                                                       |
| 版本兼容 | extension release 已有 `min_core_version` 最低内核版本约束、依赖图校验（missing_dependency / dependency_cycle / version_mismatch）与 checksum 校验；三处 provider 接口与 cicd/identity 内置模块无 per-provider 版本协商字段；SDK 侧另有 sdkVersion 高水位门禁（registry sdkfloor），仅约束游戏侧 agent 不约束平台插件                                 |

另一处现状事实：`internal/platform/provider` 的 Registry（`NewRegistry`/`Register`）当前未接生产路径——平台调用的生产链路已改走 extension externalfunc dispatcher，Registry 仅存于测试。provider 体系当前是「Go 接口 + 编译期注册」而非「manifest + 运行时发现」，这是与 Grafana/VS Code 的本质差异。

### 7.3 机制启示（设计输入，非结论）

- **版本协商是当前最明确的缺口**：对照 `grafanaDependency`/`engines.vscode`，provider/pack 若走向运行时分发，需要 `croupierVersion` 约束字段；即便维持编译期内置，manifest 里声明兼容区间也有助于升级排查。
- **能力声明已有基础**：`official.<domain>` 三层 capability 与统一配置 schema 声明，等价于「contributes + capability」思想；Provider 插件应复用这套声明而非另造权限面。
- **签名/分发可信按需引入**：Grafana 的签名分级面向公网目录分发；Croupier 是自托管单公司部署，扩展来源是内部 pack 导入——分发可信的优先级低于版本协商，可在设计文档中登记为远期项。
- **沙箱维持现状口径**：provider 出站调用已有 secguard HTTP 守卫（进程内 + 受控出站），与 WordPress「信任边界=管理员能装什么」同构；引入子进程/gRPC 隔离缺乏对应威胁模型，不建议在设计文档立项。
