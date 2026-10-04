# Croupier 身份验证与 OAuth 全量核对（需求清单 #51 核销归档）

## 状态

- 状态: 核对归档（2026-10-04，需求清单 #51 立项第一块产出）
- 日期: 2026-10-04
- 范围: 需求清单 #51「设置/身份验证」全拆批——51a 密码登录开关 + GitHub OAuth、51b 自助注册、51c 邮箱验证 + 域限制/别名限制/白名单、微信与自定义 OAuth（原「后续批次」）
- 结论: **全拆批零功能缺口，OPEN-ISSUES #51 核销**；本批无新代码，仅归档与台账核销

## 一、拆批全景与结论

| 拆批     | 需求原文                                         | 落地状态             | 关键证据                                                                                                                                                                          |
| -------- | ------------------------------------------------ | -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 51a      | 密码登录开关 `auth.local.enabled` + GitHub OAuth | ✅ 已落地            | 7 键 + GitHubProvider + login/callback 路由 + 登录页 SSO 按钮 + AuthTab 本地卡                                                                                                    |
| 51a 后续 | 微信 OAuth                                       | ✅ 已落地（964e40b） | `identity/wechat.go` 官方双域名回落 + 覆盖率 R27 88.2%→99.2%                                                                                                                      |
| 51a 后续 | 自定义 OAuth                                     | ✅ 已落地（964e40b） | `identity/genericoauth.go` 可配置 userinfo 字段                                                                                                                                   |
| 51b      | 自助注册（默认关）                               | ✅ 已落地            | `auth.register.enabled` / `auth.register.defaultRoles` + `POST /api/v1/auth/register`（403 registration_disabled）+ 登录页注册表单 + AuthTab 注册卡                               |
| 51c      | 邮箱验证                                         | ✅ 已落地            | `auth.email.verificationRequired` + `GET /api/v1/auth/verify-email`（email_verified 落库、无效/过期/已用统一 400 防探测）+ `/user/verify-email` 页 + 登录页 emailNotVerified 引导 |
| 51c      | 域白名单                                         | ✅ 已落地            | `auth.email.domainWhitelist`（空 = 不限）                                                                                                                                         |
| 51c      | 别名限制                                         | ✅ 已落地            | `auth.email.aliasRestriction`（拒绝 + 别名，local 去点归一查重）                                                                                                                  |

## 二、auth.* 键族全表（`internal/platform/settings/layered.go`）

```text
# 51a 密码登录 + GitHub OAuth
auth.local.enabled                    bool     账号密码登录开关（默认 true）
auth.github.enabled                   bool     GitHub OAuth 开关
auth.github.clientId                  string   GitHub OAuth App Client ID
auth.github.clientSecret              string   secret，回显脱敏
auth.github.redirectUrl               string   回调地址
auth.github.defaultRoles              string   逗号分隔，JIT 建号默认角色
auth.github.successUrl                string   登录后跳转（默认首页）

# 微信 / 自定义 OAuth（oidc/ldap 为更早批次，不在此列）
auth.wechat.*                         string/… 官方双域名回落 + AuthCodeURL 前缀
auth.genericoauth.{clientId,clientSecret,authorizeUrl,tokenUrl,userinfoUrl,…}
                                      string   自定义端点 + userinfo 字段映射 + 昵称/邮箱字段

# 51b 自助注册（默认关）
auth.register.enabled                 bool     自助注册开关（默认 false）
auth.register.defaultRoles            string   逗号分隔；留空则不赋角色

# 51c 邮箱策略
auth.email.domainWhitelist            string   注册邮箱域后缀白名单（空 = 不限）
auth.email.aliasRestriction           bool     拒绝 + 别名，local 去点归一查重
auth.email.verificationRequired       bool     注册后须邮箱验证才能登录
```

## 三、防锁死与安全链（跨拆批的横向设计）

- **防锁死守卫**：`RefreshIdentityProviders` 在 local/LDAP/OIDC/GitHub 全关时拒绝保存并回滚（「不能停用所有登录方式」）——测试覆盖见 `local_github_test.go`/`register_test.go`。
- **GitHub state 防 CSRF**：HMAC 签名，10 分钟过期。
- **JIT 建号**：GitHub/WeChat/generic OAuth 复用 `resolveAdminForIdentity` + provider 各自 defaultRoles；微信无公开邮箱场景有私密邮箱回退链。
- **邮箱验证反探测**：`verify-email` 对无效/过期/已用令牌统一 400 同文案，不区分原因。
- **弱密码/强度**：账号安全批次（#48，`security.*` 五键）与自助注册复用同一密码校验链，策略只收紧不放宽。

## 四、产品假设现状（原 51b「待注」项，均在实现中定形）

| 假设           | 现状                                                               |
| -------------- | ------------------------------------------------------------------ |
| 注册建何种账号 | 平台 admin 账号（`Register` 返回 username/nickname），非独立玩家面 |
| 默认角色       | `auth.register.defaultRoles`（逗号分隔）；留空 = 不赋角色          |
| 是否需审批     | 无审批流；开关即开放注册，`verificationRequired` 可加邮件验证门槛  |

## 五、边界与开放项

1. 域白名单/别名限制仅约束注册链，不回溯已有账号。
2. 微信/自定义 OAuth 暂无管理端「测试连接」按钮（与 LDAP OIDC 同口径，配置后登录页即时生效）。
3. `auth.email.verificationRequired` 开启后注册成功页落「查收验证邮件」提示，但无重发邮件入口（重发依赖 #55 SMTP 通知侧深化，可后续批次补）。
4. GitHub App 型企业实例（GHE）需自定义端点，当前 GitHubProvider 仅直连 github.com（企业 OAuth 可走 genericoauth 自定义端点）。

## 六、核销动作

- OPEN-ISSUES.md #51 行更新为「已核销（2026-10-04）全拆批落地」。
- todo.md 增 #51 核对段（本批）。
- 无代码改动，无新测试；后续批次顺延 #52 系统维护。
