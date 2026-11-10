# Security Policy

## Supported Versions

1.0 之前采用单支持线策略：**最新发布的 0.1.x 次版本线 + main**。

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| main    | :white_check_mark: |
| < 0.1   | :x:                |

### Backport Policy

- 安全修复一律先落 main，经全量质量门禁后发布。
- 已发布的正式版本（版本号 tag 对应的 release）获得 backport：修复 cherry-pick
  到该版本线的补丁分支，随补丁版本发布（如 0.1.x -> 0.1.x+1）。
- nightly / 滚动构建不单独 backport——滚动升级即取修复。
- 预 1.0 阶段只维护**最新一条** 0.1.x 线；更早的版本线随新次版本发布即停收。

## Reporting a Vulnerability

如果您发现安全漏洞，请通过以下方式负责任地披露：

1. **请勿** 在公开的 GitHub Issues 中报告安全漏洞
2. 通过 [GitHub Security Advisories](https://github.com/cuihairu/croupier/security/advisories/new) 提交报告，或发送邮件至安全邮箱 **chuihairu@gmail.com**（标题请带 `[security]` 前缀）
3. 加密投递：项目 PGP 指纹发布后将在此处列出（当前未配置，明文邮件请勿附带机密细节，细节走 Security Advisories 的私有通道）
4. 请在报告中包含：
   - 漏洞描述
   - 复现步骤
   - 潜在影响
   - 如有可能，提供修复建议

## Response Timeline

- **确认收到**：48 小时内
- **初步评估**：7 个工作日内
- **修复发布**：视漏洞严重程度而定，高危漏洞优先处理

## Security Architecture

Croupier Server 采用多层安全架构：

- **控制面 TLS / mTLS**：`Agent <-> Server` 推荐默认启用 TLS，生产环境建议 mTLS
- **本地接入可选 TLS**：`SDK <-> Agent` 默认可在受信内网使用明文，也支持按需开启 TLS
- **RBAC/ABAC**：基于角色和属性的访问控制
- **审计日志**：完整的操作审计链，支持敏感字段脱敏
- **双人规则**：高风险操作强制要求双人审批

## Security Best Practices

部署 Croupier Server 时，建议遵循以下安全实践：

- 始终使用最新稳定版本
- 定期运行 `govulncheck` 检查 Go 依赖漏洞
- 生产环境必须配置有效的 TLS 证书（避免使用 dev-certs.sh 生成的自签名证书）
- 使用 Secret Manager（如 HashiCorp Vault）管理敏感配置
- 配置合适的日志级别，启用审计日志
- 限制网络访问，仅开放必要端口（REST: 18780, session/control: 19090, agent local gateway: 19091）
- 定期轮换 JWT 签名密钥和 API 凭据
