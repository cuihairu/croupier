---
title: Plugin Architecture Survey — Grafana / VS Code / VitePress / WordPress (Provider Plugin Design Input)
---

# Plugin Architecture Survey: Grafana / VS Code / VitePress / WordPress

## Status

- Status: survey archive (2026-10-08, one of the input surveys for the Provider plugin design)
- Date: 2026-10-08
- Scope: the plugin mechanisms of four mature systems, checked against official docs with the same six-question rubric — plugin type inventory, discovery/distribution, contract, loading, sandbox/isolation, permission surface, version compatibility
- Conclusion: contract strength is roughly proportional to runtime isolation. The standard triangle of strong-contract plugin platforms is "trusted distribution (signing/review), capability declaration (explicit permissions/compat lists), version negotiation (semver constraint fields)". Croupier's existing extension system (`internal/core/extension` + the `official.<domain>` unified pattern) already has the embryo of capability declaration and config contracts, and lacks version negotiation and a trusted-distribution layer; runtime loading and sandboxing are unnecessary to introduce given the current shape (compile-time built-in + HTTP outbound)

## 1. Selected Systems and Rubric

| System    | Why selected                                                                                               |
| --------- | ---------------------------------------------------------------------------------------------------------- |
| Grafana   | Same admin-console shape as Croupier, with out-of-process backend plugin precedent                         |
| VS Code   | The largest plugin ecosystem; mature designs for lazy loading, extension points, and the trust model       |
| WordPress | The most widespread hook-style plugin system; the other pole of weak contract + strong extension points    |
| VitePress | Control group — "no plugin system": extension is build-time code composition, used to mark the lower bound |

Each system answers the same questions: ① what plugin types exist; ② how plugins are discovered/distributed; ③ what the contract (manifest/extension points) looks like; ④ how loading works; ⑤ sandbox/isolation and permission surface; ⑥ how version compatibility is declared.

## 2. Grafana

**Types**: `plugin.json`'s `type` allows `app` / `datasource` / `panel` / `renderer`; the optional `backend` declares a backend plugin, and `executable` names the first segment of the executable (named `<executable>_<GOOS>_<GOARCH>`, e.g. `plugin_linux_amd64`).

**Discovery and install**: Grafana scans the `[paths].plugins` config directory at startup; users install via the Plugin Catalog UI (Administration > Plugins and data > Plugins), CLI, unzipping a ZIP into the plugin directory, `plugins.preinstall` preinstall (11.5+), or Helm `GF_PLUGINS_PREINSTALL_SYNC`.

**Contract**: `plugin.json` requires `id` (regex-constrained), `type`, `name`, `info` (containing `version`/`keywords`/`links`/`logos`/`updated`), and `dependencies`; optional fields include `preload`, `state`, `roles[]`.

**Loading and sandboxing**: backend plugins are launched by Grafana as subprocesses communicating over gRPC (based on the HashiCorp Go Plugin System over RPC); the docs state explicitly that a plugin crash cannot take down the main process ("Plugins can't crash your Grafana process"). The frontend Plugin Frontend Sandbox (11.5 public preview) runs plugins in a separate JavaScript context, enabled via `security.enable_frontend_sandbox_for_plugins`; AngularJS plugins and Grafana Labs self-signed plugins are excluded, AngularJS support is off by default in v11 and removed in the next major.

**Signing**: at startup Grafana verifies the signatures of all plugins in the plugin directory; unsigned plugins are neither loaded nor started ("If a plugin is unsigned, then Grafana neither loads nor starts it"). `sign-plugin` generates `MANIFEST.txt` in `dist/` (metadata + per-file SHA256 checksums + private-key signature), verified by Grafana's built-in public key; distribution tiers are Private / Community / Commercial, with exemptions via `allow_loading_unsigned_plugins` or development mode.

**Version compatibility**: `dependencies.grafanaDependency` is checked with node-semver and supports ranges like `>=10.4.0 || 11.x`; the old `grafanaVersion` field is deprecated.

Sources: <https://grafana.com/developers/plugin-tools/> · [plugin.json reference](https://grafana.com/developers/plugin-tools/reference/plugin-json) · [backend plugins](https://grafana.com/developers/plugin-tools/key-concepts/backend-plugins) · [plugin install](https://grafana.com/docs/grafana/latest/administration/plugin-management/plugin-install/) · [plugin signing](https://grafana.com/docs/grafana/latest/administration/plugin-management/plugin-sign/) · [frontend sandbox](https://grafana.com/docs/grafana/latest/administration/plugin-management/plugin-frontend-sandbox/) · [v11 breaking changes](https://grafana.com/docs/grafana/latest/breaking-changes/breaking-changes-v11-0/)

## 3. VS Code

**Contract and extension points**: `package.json` requires `name`/`version`/`publisher`/`engines`, with entry `main` (Node) / `browser` (web); `contributes` defines 38 extension points (`commands`, `configuration`, `views`, `viewsContainers`, `languages`, `grammars`, `debuggers`, `themes`, `keybindings`, `menus`, `snippets`, `walkthroughs`, `chatAgents`, `languageModelTools`, etc.). `extensionPack` is a joint install (loose bundling); `extensionDependencies` is a hard dependency.

**Lazy loading**: `activationEvents` (`onLanguage`/`onCommand`/`onView`/`onStartupFinished`/`workspaceContains`/`*`) trigger a single `activate()` call; `*` is officially discouraged because it slows startup; since 1.74, `contributes` entries auto-generate activation events.

**Process model**: all extensions in one Extension Host **share one process** — an API exported by one extension is visible to all extensions in the same host. The real isolation boundary is the host itself: UI extensions run on the local host; Workspace extensions run in the Remote Extension Host inside a remote VS Code Server; cross-host communication goes through JSON-serialized IPC. A webview is a controlled `iframe`: "scripts in a webview do not have access to the VS Code API" — only `postMessage` communication, an enforced CSP (`default-src 'none'`), and resources constrained by `asWebviewUri`/`localResourceRoots`.

**Permission surface**: under Workspace Trust, extensions must declare `capabilities.untrustedWorkspaces` (`supported: true | false | 'limited'` + `restrictedConfigurations`); undeclared extensions are disabled by default in restricted mode; `extensionKind: ui|workspace` decides where they run.

**Version compatibility**: `engines.vscode` constrains compatible versions and does not allow `*`; proposed APIs require `enabledApiProposals`, are available only in Insiders, and may not be published to the Marketplace — only stable APIs carry a compatibility promise.

Sources: [extension manifest](https://code.visualstudio.com/api/references/extension-manifest) · [activation events](https://code.visualstudio.com/api/references/activation-events) · [contributes extension points](https://code.visualstudio.com/api/references/contribution-points) · [Workspace Trust](https://code.visualstudio.com/api/extension-guides/workspace-trust) · [remote extensions](https://code.visualstudio.com/api/advanced-topics/remote-extensions) · [webview guide](https://code.visualstudio.com/api/extension-guides/webview) · [proposed API](https://code.visualstudio.com/api/advanced-topics/using-proposed-api)

## 4. WordPress

**Contract**: the main file's header comment is the manifest; of 15 fields only `Plugin Name` is required; the rest include `Version`, `Requires at least` (minimum WP version), `Requires PHP`, `Requires Plugins` (dependency declaration, WP 6.5+), `Update URI`, `Network`. Distribution via the official directory additionally requires `readme.txt`, where `Tested up to` is defined as "the highest version successfully tested" (a soft claim, not a hard gate).

**Extension points**: hooks are the officially defined foundation of the plugin mechanism ("the foundation for how plugins and themes interact with WordPress Core") — an action performs side effects and returns nothing, a filter must return the modified value, registered via `add_action`/`do_action`/`add_filter`/`apply_filters`. Lifecycle: `register_activation_hook` (table creation/seed data), `register_deactivation_hook` (cache clearing); uninstall goes through a root-level `uninstall.php` (which must first check the `WP_UNINSTALL_PLUGIN` constant against direct access) or `register_uninstall_hook()`.

**Permissions and boundaries**: the permission surface is the roles/capabilities system (six default roles, Super Admin→Subscriber), with `add_role`/`add_cap`/`current_user_can` for plugins to gate themselves. The official docs define no sandbox, process isolation, or capability-declaration mechanism — plugins share one PHP runtime with core, and the trust boundary is simply "what an administrator can install".

Sources: [header requirements](https://developer.wordpress.org/plugins/plugin-basics/header-requirements/) · [hooks](https://developer.wordpress.org/plugins/plugin-basics/hooks/) · [activation/deactivation](https://developer.wordpress.org/plugins/plugin-basics/activation-deactivation-hooks/) · [uninstall](https://developer.wordpress.org/plugins/plugin-basics/uninstall-methods/) · [roles and capabilities](https://developer.wordpress.org/plugins/users/roles-and-capabilities/) · [readme.txt standard](https://wordpress.org/plugins/readme.txt)

## 5. VitePress (Control Group: No Plugin System)

VitePress has no traditional plugin system: a custom theme takes effect simply by existing at `.vitepress/theme/index.ts`; the theme object contains `Layout` (required), `enhanceApp({app, router, siteData})` (register global components), `setup()`, and `extends` (inherit a base theme, whose hooks run before its own); the official docs state plainly that "The default export is the only contract for a custom theme" — no manifest, no signing, no sandbox. Config passthrough: `config.vite` passes native Vite config straight through, and `config.markdown` configures the underlying markdown-it instance. Extension is build-time code composition, with no runtime discovery, permissions, or version negotiation.

Sources: [custom theme](https://vitepress.dev/guide/custom-theme) · [site config](https://vitepress.dev/reference/site-config)

## 6. Cross-Comparison

| Dimension              | Grafana                                                 | VS Code                                                                            | VitePress                                      | WordPress                                                                  |
| ---------------------- | ------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------- | -------------------------------------------------------------------------- |
| Discovery/distribution | Directory scan + Catalog UI/CLI/preinstall              | Marketplace / manual VSIX install                                                  | npm package, no directory                      | wordpress.org directory + SVN                                              |
| Contract               | `plugin.json` (id/type/info/dependencies required)      | `package.json` + 38 `contributes` extension points                                 | None — the default export is the contract      | File header comment (only Plugin Name required)                            |
| Loading                | Loaded at startup; signature failure means not loaded   | Lazy loading via `activationEvents`; contributions statically generate activations | Build-time static composition                  | Fully loaded; hook callbacks fire on demand                                |
| Sandbox/isolation      | Backend subprocess + gRPC; frontend separate JS context | Shared process within a host (isolation via host boundary); webview = iframe + CSP | None                                           | None; same PHP runtime                                                     |
| Permissions            | Signature tiers + Admin role controls install/config    | Workspace Trust `untrustedWorkspaces` declaration                                  | None                                           | roles/capabilities + `current_user_can`                                    |
| Version compatibility  | `grafanaDependency` semver ranges                       | `engines.vscode` (`*` forbidden); proposed/stable dual track                       | No explicit field (npm ecosystem self-manages) | `Requires at least`/`Requires PHP` hard checks + `Tested up to` soft claim |

## 7. Conclusions and Implications for Croupier

### 7.1 Patterns Across the Four Cases

1. **Contract strength is proportional to isolation strength**. Grafana (strong contract + isolation on both sides) → VS Code (strong contract, shared process within a host, iframe as a supplement) → WordPress (weak contract, same process, hooks as the fallback) → VitePress (no contract, build-time composition).
2. **The standard triangle of strong-contract platforms**: trusted distribution (Grafana signature tiers / VS Code Marketplace review), capability declaration (`untrustedWorkspaces`-style explicit declaration + `roles[]`), version negotiation (semver constraint fields, all forbidding `*` or running a dual-track API policy).
3. **Hooks are the most durable extension point**. WordPress's action/filter contract has barely changed in 20 years; VS Code's statically declared `contributes` makes extension points enumerable and auditable. The common denominator: the host narrows its call surface into extensions to one stable registration function.
4. **Out-of-process isolation is the heavyweight option**. Only Grafana went to subprocess + gRPC (and advertises crash isolation explicitly); VS Code explicitly does not do per-extension process isolation. Going out-of-process carries distribution/debugging/compatibility costs and suits untrusted third-party code; internal trusted code has no use for it.

### 7.2 Croupier Status Mapped to the Six Questions

| Question              | Current status                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Types                 | Three coexisting provider abstractions: `internal/platform/provider` (third-party platform integration, Name/Init/SupportedMethods/Call contract), `internal/cicd` (CI/CD, Factory registry), `internal/security/identity` (PasswordProvider/OAuthProvider); plus `internal/core/extension`'s six subpackages (manifest/catalog/installation/runtime/sync/externalfunc) carrying `official.<domain>` extensions                                                      |
| Discovery             | All compile-time built-in: Go `init()` self-registration (cicd) or import + Registry (platform/provider); extensions land via installation rows in the DB and mount via runtime binding                                                                                                                                                                                                                                                                              |
| Contract              | Extensions already have manifest + unified pattern (`official.<domain>`, three capability tiers `<domain>.read/operate/admin`, config/secrets split schema declarations); the three provider interfaces have no manifest — the contract is the Go interface method set                                                                                                                                                                                               |
| Loading               | In-process direct calls; extension externalfunc goes through the dispatcher (`external.<platform>.<method>`)                                                                                                                                                                                                                                                                                                                                                         |
| Sandbox               | No plugin-level isolation; outbound HTTP goes through the secguard guard client (SSRF protection), which is the most real security boundary                                                                                                                                                                                                                                                                                                                          |
| Version compatibility | Extension releases already have a `min_core_version` minimum-kernel constraint, dependency-graph validation (missing_dependency / dependency_cycle / version_mismatch), and checksum validation; the three provider interfaces and the cicd/identity built-in modules have no per-provider version negotiation field; the SDK side separately has an sdkVersion high-water-mark gate (registry sdkfloor) that only constrains game-side agents, not platform plugins |

One more status fact: the Registry of `internal/platform/provider` (`NewRegistry`/`Register`) is currently not wired into production — the platform invocation production path has moved to the extension externalfunc dispatcher, and the Registry survives only in tests. The provider system today is "Go interfaces + compile-time registration", not "manifest + runtime discovery" — the essential difference from Grafana/VS Code.

### 7.3 Mechanism Implications (Design Inputs, Not Conclusions)

- **Version negotiation is the clearest current gap**: compared with `grafanaDependency`/`engines.vscode`, if provider/pack ever moves to runtime distribution it needs a `croupierVersion` constraint field; even staying compile-time built-in, declaring a compatibility range in the manifest helps upgrade troubleshooting.
- **Capability declaration already has a foundation**: the `official.<domain>` three-tier capabilities and unified config schema declarations are equivalent to the "contributes + capability" idea; Provider plugins should reuse this declaration set rather than build another permission surface.
- **Signing/trusted distribution introduced on demand**: Grafana's signature tiers target public directory distribution; Croupier is a self-hosted single-company deployment where extensions come from internal pack imports — trusted distribution ranks below version negotiation and can be logged in the design doc as a long-term item.
- **Sandbox stays as-is**: provider outbound calls already pass the secguard HTTP guard (in-process + controlled outbound), isomorphic to WordPress's "trust boundary = what an administrator can install"; introducing subprocess/gRPC isolation lacks a matching threat model and is not recommended for design-doc chartering.
