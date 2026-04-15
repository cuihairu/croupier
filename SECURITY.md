# Security Policy

## Supported Versions

当前 Croupier Server 以下版本获得安全更新支持：

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |

## Reporting a Vulnerability

如果您发现安全漏洞，请通过以下方式负责任地披露：

1. **请勿** 在公开的 GitHub Issues 中报告安全漏洞
2. 发送邮件至项目维护者，或通过 [GitHub Security Advisories](https://github.com/cuihairu/croupier/security/advisories/new) 提交报告
3. 请在报告中包含：
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
