# Croupier 运维/SMTP 邮箱配置全量核对（需求清单 #55 核销归档）

## 状态

- 状态: 核对归档（2026-10-04，需求清单 #55）
- 日期: 2026-10-04
- 范围: 用户需求「SMTP 邮箱配置之类是不是也应该放在运维这里」——加密方式选择、强制 AUTH LOGIN、密码/访问令牌、用户名、跳过 SMTP TLS 证书验证、端口、发件地址（NotificationTab 基础字段扩充并归位运维）
- 结论: **已落地（2026-10-02）+ 已线上复证 + 记录与代码一致，零新缺口，OPEN-ISSUES #55 核销**；另发现原边界①「无发送测试邮件按钮」已被 #51c 批次补欠（记录更新）；SmtpCard 文件头过时注释顺手修正（唯一代码改动）；归档与台账核销

## 一、需求 → 落地对照

| 用户需求               | 落地 | 证据                                                                                                                                                                     |
| ---------------------- | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 归位运维               | ✅   | 通知 Tab SMTP 五字段迁出，运维 Tab「SMTP 邮件服务」卡（SmtpCard），通知 Tab 留迁移提示                                                                                   |
| 加密方式选择           | ✅   | `notification.smtpEncryption`（""\|none\|ssl\|starttls）+ `EmailSender.WithTransport` 加密矩阵（ssl=任意端口隐式 TLS、starttls=强制升级且服务器未宣告即报错、none=明文） |
| 强制 AUTH LOGIN        | ✅   | `notification.smtpAuthType`（plain\|login）+ `loginAuth` 实现（net/smtp 无 LOGIN 机制，自实现协议时序 Username/Password 提示）                                           |
| 密码/访问令牌          | ✅   | `notification.smtpPassword`（写入后读取接口脱敏，密码即访问令牌口径）                                                                                                    |
| 用户名                 | ✅   | `notification.smtpUser`                                                                                                                                                  |
| 跳过 SMTP TLS 证书验证 | ✅   | `notification.smtpInsecureSkipVerify`（bool）+ tls.Config 透传                                                                                                           |
| 端口                   | ✅   | `notification.smtpPort`                                                                                                                                                  |
| 发件地址               | ✅   | `notification.smtpFrom`                                                                                                                                                  |

## 二、既有记录 vs 代码核验（2026-10-04）

- 键族：`internal/platform/settings/layered.go` 63-72 —— Seven键家族（emailEnabled/smtpHost/Port/User/Password/From/Encryption/AuthType/InsecureSkipVerify）✓
- 传输接线：`internal/platform/approvals/notification.go` —— `EmailSender.WithTransport(encryption, authType, insecureSkipVerify)`（:381）+ `loginAuth` 时序（:379 前文件内）；调用方 `internal/api/sitesettings/testemail.go:22` ✓
- 枚举校验：sitesettings validateValue 非法值 400 ✓（Go 用例在册）
- 前端：`web/src/pages/System/SiteSettings/SmtpCard.tsx`（开关+服务器/端口/用户名/密码/发件人+加密 Select+认证 Select+跳过校验 Switch+中间人风险警示）+ `SmtpCard.test.tsx` 10 用例 + NotificationTab 改版用例 ✓
- 热生效：notify 服务发送前读 L3 快照 ✓
- 线上复证：2026-10-02 deploy run 36937558114，gitCommit 444d7f0，双实例 healthy ✓

## 三、边界（原登记四条 + 一条状态更新）

1. ~~无「发送测试邮件」按钮~~ → **已被 #51c 批次补欠**：`POST /api/v1/site/notification/test-email`（`internal/api/sitesettings/testemail.go` + handler.go:56 + `testemail_test.go` 2 Test + SmtpCard「测试邮件」describe 用例，SmtpCard.tsx:90-105）。SmtpCard 文件头注释此前残留旧边界，本批修正。
2. 跳过证书校验仅建议内网自签邮服使用（公网开启有中间人风险，UI 警示）。
3. 密码字段即访问令牌口径（SMTP 无独立 token 字段）。
4. AUTH LOGIN 凭据为明文传输语义（与所有客户端一致，依赖传输层加密保护）。

## 四、核销动作

- OPEN-ISSUES.md #55 行追加「已核销（2026-10-04）」注记（含边界①状态更新）。
- todo.md 增 #55 核对段。
- 代码改动仅一处：SmtpCard.tsx 文件头过时注释修正（纯注释，SmtpCard 10 用例 + tsc 复验绿）。
